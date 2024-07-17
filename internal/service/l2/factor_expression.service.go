package l2_service

import (
	"context"
	"database/sql"
	"errors"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	l1_service "factorbacktest/internal/service/l1"
	"math"

	"fmt"
	"sync"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/maja42/goval"
)

type ScoresResultsOnDay struct {
	SymbolScores map[string]*float64
	Errors       []error
}

// calculate scores over a range of days

type FactorExpressionService interface {
	CalculateFactorScores(ctx context.Context, tradingDays []time.Time, tickers []model.Ticker, factorExpression string) (map[time.Time]*ScoresResultsOnDay, error)
	CalculateFactorScoresOnDay(ctx context.Context, date time.Time, tickers []model.Ticker, factorExpression string) (*ScoresResultsOnDay, error)
}

type factorExpressionServiceHandler struct {
	Db                    *sql.DB
	FactorMetricsHandler  factorMetricCalculations
	PriceService          l1_service.PriceService
	FactorScoreRepository repository.FactorScoreRepository
}

func NewFactorExpressionService(
	db *sql.DB,
	factorMetricsHandler factorMetricCalculations,
	priceService l1_service.PriceService,
	factorScoreRepository repository.FactorScoreRepository,
) FactorExpressionService {
	return factorExpressionServiceHandler{
		Db:                    db,
		FactorMetricsHandler:  factorMetricsHandler,
		PriceService:          priceService,
		FactorScoreRepository: factorScoreRepository,
	}
}

type workInput struct {
	Ticker           model.Ticker
	Date             time.Time
	FactorExpression string
}

type workResult struct {
	Date             time.Time
	Ticker           model.Ticker
	ExpressionResult *expressionResult
	Err              error
	elapsedMs        int64
	span             *domain.Span
}

// CalculateFactorScores asynchronously processes factor expression calculations for every relevant day in the backtest
// using the list of workInputs, it spawns workers to calculate what the score for a particular asset would be on that day
// despite using workers, this is still the slowest part of the flow
func (h factorExpressionServiceHandler) CalculateFactorScores(ctx context.Context, tradingDays []time.Time, tickers []model.Ticker, factorExpression string) (map[time.Time]*ScoresResultsOnDay, error) {
	profile, endProfile := domain.GetProfile(ctx)
	defer endProfile()

	// convert params to list of inputs
	inputs := []workInput{}
	for _, tradingDay := range tradingDays {
		for _, ticker := range tickers {
			inputs = append(inputs, workInput{
				Ticker:           ticker,
				Date:             tradingDay,
				FactorExpression: factorExpression,
			})
		}
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("cannot calculate factor scores with 0 inputs")
	}
	fmt.Printf("computing %d scores\n", len(inputs))

	out := map[time.Time]*ScoresResultsOnDay{}

	// if we have any of the inputs stored already, load them and remove
	// from the inputs list
	if false {
		_, endSpan := profile.StartNewSpan("get precomputed scores")
		precomputedScores, err := h.getPrecomputedScores(&inputs)
		if err != nil {
			return nil, err
		}
		numFound := 0
		numErrors := 0
		for date, valuesOnDate := range precomputedScores {
			scoresOnDate := map[string]*float64{}
			errList := []error{}
			for symbol, score := range valuesOnDate {
				if score.Error != nil {
					errList = append(errList, errors.New(*score.Error))
				} else {
					scoresOnDate[symbol] = score.Score
				}
			}
			out[date] = &ScoresResultsOnDay{
				SymbolScores: scoresOnDate,
				Errors:       []error{},
			}
			numFound += len(valuesOnDate)
			numErrors += len(errList)
		}
		endSpan()

		fmt.Printf("found %d scores and %d errors, computing data for %d scores\n", numFound, numErrors, len(inputs))
	}
	span, endSpan := profile.StartNewSpan("load price cache")
	cache, err := h.loadPriceCache(domain.NewCtxWithSubProfile(ctx, span), inputs)
	if err != nil {
		return nil, err
	}
	endSpan()

	inputCh := make(chan workInput, len(inputs))
	resultCh := make(chan workResult, len(inputs))
	numGoroutines := 10
	var wg sync.WaitGroup
	for _, f := range inputs {
		wg.Add(1)
		inputCh <- f
	}
	close(inputCh)

	_, endSpan = profile.StartNewSpan("evaluate factor expressions")
	// newProfile, endNewProfile := span.NewSubProfile()
	// i want a list of spans - one for each element in this
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case input, ok := <-inputCh:
					if !ok {
						return
					}
					start := time.Now()
					span, endSpan := domain.NewSpan(fmt.Sprintf("evaluating expression for %s on %s", input.Ticker.Symbol, input.Date.Format(time.DateOnly)))
					subProfile, endProfile := span.NewSubProfile()
					res, err := evaluateFactorExpression(
						context.WithValue(ctx, domain.ContextProfileKey, subProfile),
						h.Db,
						cache,
						input.FactorExpression,
						input.Ticker.Symbol,
						h.FactorMetricsHandler,
						input.Date,
					)
					if err != nil {
						err = fmt.Errorf("failed to compute factor score for %s on %s: %w", input.Ticker.Symbol, input.Date.Format(time.DateOnly), err)
					}
					endSpan()
					endProfile()
					resultCh <- workResult{
						ExpressionResult: res,
						Ticker:           input.Ticker,
						Date:             input.Date,
						Err:              err,
						elapsedMs:        time.Since(start).Milliseconds(),
						span:             span,
					}
					wg.Done()
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	totalMs := 0.0
	results := []workResult{}
	for res := range resultCh {
		results = append(results, res)
		totalMs += float64(res.elapsedMs)
		// newProfile.AddSpan(res.span)
	}
	fmt.Printf("avg score processing: %f\n", totalMs/float64(len(results)))
	// endNewProfile()
	endSpan()

	_, endSpan = profile.StartNewSpan("adding factor scores to db")
	addManyInput := []*model.FactorScore{}
	for _, res := range results {
		if _, ok := out[res.Date]; !ok {
			out[res.Date] = &ScoresResultsOnDay{
				SymbolScores: map[string]*float64{},
				Errors:       []error{},
			}
		}

		m := &model.FactorScore{
			TickerID:             res.Ticker.TickerID,
			FactorExpressionHash: internal.HashFactorExpression(factorExpression),
			Date:                 res.Date,
		}

		if res.Err != nil && !errors.As(res.Err, &factorMetricsMissingDataError{}) {
			out[res.Date].Errors = append(out[res.Date].Errors, res.Err)
			errString := res.Err.Error()
			m.Error = &errString
		} else if res.Err == nil {
			out[res.Date].SymbolScores[res.Ticker.Symbol] = &res.ExpressionResult.Value
			m.Score = &res.ExpressionResult.Value
		}

		addManyInput = append(addManyInput, m)
	}

	if false {

		fmt.Printf("adding %d scores to db\n", len(addManyInput))

		err = h.FactorScoreRepository.AddMany(addManyInput)
		if err != nil {
			return nil, err
		}
		endSpan()
	}

	return out, nil
}

// loadPriceCache "dry-runs" the factor expression to determine which dates are needed
// then loads them into a price cache. it has no concept of trading days, so it
// may produce cache misses on holidays
func (h factorExpressionServiceHandler) loadPriceCache(ctx context.Context, in []workInput) (*l1_service.PriceCache, error) {
	dataHandler := DryRunFactorMetricsHandler{
		Prices: []l1_service.LoadPriceCacheInput{},
		Stdevs: []l1_service.LoadStdevCacheInput{},
	}
	for _, n := range in {
		_, err := evaluateFactorExpression(
			ctx,
			nil,
			nil,
			n.FactorExpression,
			n.Ticker.Symbol,
			&dataHandler,
			n.Date,
		)
		if err != nil {
			return nil, err
		}
	}

	priceCache, err := h.PriceService.LoadPriceCache(ctx, dataHandler.Prices, dataHandler.Stdevs)
	if err != nil {
		return nil, fmt.Errorf("failed to populate price cache: %w", err)
	}

	return priceCache, nil
}

func (h factorExpressionServiceHandler) getPrecomputedScores(inputsPtr *[]workInput) (map[time.Time]map[string]model.FactorScore, error) {
	inputs := *inputsPtr
	// work backwards so we can pop from the inputs array
	getScoresInput := []repository.FactorScoreGetManyInput{}
	for _, in := range inputs {
		getScoresInput = append(getScoresInput, repository.FactorScoreGetManyInput{
			FactorExpressionHash: internal.HashFactorExpression(in.FactorExpression),
			Ticker:               in.Ticker,
			Date:                 in.Date,
		})
	}

	scoreResults, err := h.FactorScoreRepository.GetMany(getScoresInput)
	if err != nil {
		return nil, err
	}

	sortedIndicesToRemove := []int{}
	out := map[time.Time]map[string]model.FactorScore{}
	for i := 0; i < len(inputs); i++ {
		if valuesOnDate, ok := scoreResults[inputs[i].Date]; ok {
			if score, ok := valuesOnDate[inputs[i].Ticker.TickerID]; ok {
				if _, ok := out[inputs[i].Date]; !ok {
					out[inputs[i].Date] = map[string]model.FactorScore{}
				}
				out[inputs[i].Date][inputs[i].Ticker.Symbol] = score
				sortedIndicesToRemove = append(sortedIndicesToRemove, i)
			}
		}
	}

	removeIndicesInPlace(inputsPtr, sortedIndicesToRemove)

	return out, nil
}

func removeIndicesInPlace(slice *[]workInput, sortedIndexesToRemove []int) {
	// Sort indexes to remove

	// Initialize pointers
	j := 0 // Pointer for the new slice position
	k := 0 // Pointer for indexesToRemove

	for i := 0; i < len(*slice); i++ {
		// If current index matches the next index to remove, skip it
		if k < len(sortedIndexesToRemove) && i == sortedIndexesToRemove[k] {
			k++
			continue
		}
		// Otherwise, copy the element to the 'j' position and increment 'j'
		(*slice)[j] = (*slice)[i]
		j++
	}

	// Slice the original slice to its new size
	*slice = (*slice)[:j]
}

func (h factorExpressionServiceHandler) CalculateFactorScoresOnDay(ctx context.Context, date time.Time, tickers []model.Ticker, factorExpression string) (*ScoresResultsOnDay, error) {
	results, err := h.CalculateFactorScores(ctx, []time.Time{date}, tickers, factorExpression)
	if err != nil {
		return nil, err
	}
	r, ok := results[date]
	if !ok {
		return nil, fmt.Errorf("scores missing from result: %w", err)
	}
	return r, nil
}

// combined everything related to factor expressions into this one file
// good luck haha

func constructFunctionMap(
	ctx context.Context,
	db *sql.DB,
	pr *l1_service.PriceCache,
	symbol string,
	h factorMetricCalculations,
	debug formulaDebugger,
	currentDate time.Time,
) map[string]goval.ExpressionFunction {
	return map[string]goval.ExpressionFunction{
		// we could break this up

		// helper functions
		"addDate": func(args ...interface{}) (interface{}, error) {
			// addDate(date, years, months, days)
			if len(args) < 4 {
				return 0, fmt.Errorf("addDate needs needed 4 args, got %d", len(args))
			}
			date, err := time.Parse(time.DateOnly, args[0].(string))
			if err != nil {
				return 0, err
			}

			var years, months, days = args[1].(int), args[2].(int), args[3].(int)

			date = date.AddDate(years, months, days)

			return date.Format(time.DateOnly), nil
		},

		"nDaysAgo": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("nDaysAgo needs needed 1 arg, got %d", len(args))
			}
			n := args[0].(int)
			d := currentDate.AddDate(0, 0, -n)

			return d.Format(time.DateOnly), nil
		},
		"nMonthsAgo": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("nMonthsAgo needs needed 1 arg, got %d", len(args))
			}
			n := args[0].(int)
			d := currentDate.AddDate(0, -n, 0)

			return d.Format(time.DateOnly), nil
		},
		"nYearsAgo": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("nYearsAgo needs needed 1 arg, got %d", len(args))
			}
			n := args[0].(int)
			d := currentDate.AddDate(-n, 0, 0)

			return d.Format(time.DateOnly), nil
		},

		// metric functions

		// price(date strDate)
		"price": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("price needs needed 1 arg, got %d", len(args))
			}
			date, err := time.Parse(time.DateOnly, args[0].(string))
			if err != nil {
				return 0, err
			}
			p, err := h.Price(pr, symbol, date)
			if err != nil {
				return 0, err
			}

			debug.Add("price", p)

			return p, nil
		},

		// pricePercentChange(start, end strDate)
		"pricePercentChange": func(args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return 0, fmt.Errorf("pricePercentChange needs needed 2 args, got %d", len(args))
			}
			start, err := time.Parse(time.DateOnly, args[0].(string))
			if err != nil {
				return 0, err
			}
			end, err := time.Parse(time.DateOnly, args[1].(string))
			if err != nil {
				return 0, err
			}

			p, err := h.PricePercentChange(pr, symbol, start, end)
			if err != nil {
				return 0, err
			}

			debug.Add("pricePercentChange", p)

			return p, nil
		},

		// stdev(start, end strDate)
		"stdev": func(args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return 0, fmt.Errorf("stdev needs needed 2 args, got %d", len(args))
			}

			start, err := time.Parse(time.DateOnly, args[0].(string))
			if err != nil {
				return 0, err
			}
			end, err := time.Parse(time.DateOnly, args[1].(string))
			if err != nil {
				return 0, err
			}

			p, err := h.AnnualizedStdevOfDailyReturns(ctx, pr, symbol, start, end)
			if err != nil {
				return 0, err
			}
			debug.Add("stdev", p)

			return p, nil
		},
		"marketCap": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("marketCap needs needed 1 arg, got %d", len(args))
			}

			date, err := time.Parse(time.DateOnly, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.MarketCap(db, symbol, date)
		},
		"pbRatio": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("pbRatio needs needed 1 arg, got %d", len(args))
			}

			date, err := time.Parse(time.DateOnly, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.PbRatio(db, symbol, date)
		},
		"peRatio": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("peRatio needs needed 1 arg, got %d", len(args))
			}

			date, err := time.Parse(time.DateOnly, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.PeRatio(db, symbol, date)
		},
	}
}

type expressionResult struct {
	Value  float64
	Reason formulaDebugger
}

func evaluateFactorExpression(
	ctx context.Context,
	db *sql.DB,
	pr *l1_service.PriceCache,
	expression string,
	symbol string,
	factorMetricsHandler factorMetricCalculations,
	date time.Time, // expressions are evaluated on the given date
) (*expressionResult, error) {
	eval := goval.NewEvaluator()
	variables := map[string]interface{}{
		"currentDate": date.Format(time.DateOnly),
	}

	debug := formulaDebugger{}
	functions := constructFunctionMap(ctx, db, pr, symbol, factorMetricsHandler, debug, date)
	result, err := eval.Evaluate(expression, variables, functions)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate factor expression: %w", err)
	}

	// TODO - if it's a dry-run, we're not computing real results
	// and should allow any value to be processed here
	r, ok := result.(float64)
	if !ok {
		return nil, fmt.Errorf("failed to convert to float")
	} else if math.IsNaN(r) {
		return nil, fmt.Errorf("calculated NaN as expression result")
	} else if math.IsInf(r, 0) {
		return nil, fmt.Errorf("calculated infinity as expression result")
	}

	return &expressionResult{
		Value:  r,
		Reason: debug,
	}, nil
}

type formulaDebugger map[string][]float64

func (f formulaDebugger) Add(fieldName string, value float64) {
	if _, ok := f[fieldName]; !ok {
		f[fieldName] = make([]float64, 0)
	}
	f[fieldName] = append(f[fieldName], value)
}

type factorMetricsMissingDataError struct {
	Err error
}

func (e factorMetricsMissingDataError) Error() string {
	return e.Err.Error()
}

type factorMetricCalculations interface {
	Price(pr *l1_service.PriceCache, symbol string, date time.Time) (float64, error)
	PricePercentChange(pr *l1_service.PriceCache, symbol string, start, end time.Time) (float64, error)
	AnnualizedStdevOfDailyReturns(ctx context.Context, pr *l1_service.PriceCache, symbol string, start, end time.Time) (float64, error)
	MarketCap(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
	PeRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
	PbRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
}

type factorMetricsHandler struct {
	// both dependencies should be wrapped in some mds service
	AdjustedPriceRepository     repository.AdjustedPriceRepository
	AssetFundamentalsRepository repository.AssetFundamentalsRepository
}

func NewFactorMetricsHandler(adjPriceRepository repository.AdjustedPriceRepository, afRepository repository.AssetFundamentalsRepository) factorMetricCalculations {
	return factorMetricsHandler{
		AdjustedPriceRepository:     adjPriceRepository,
		AssetFundamentalsRepository: afRepository,
	}
}

type DryRunFactorMetricsHandler struct {
	// these may contain duplicates
	Prices []l1_service.LoadPriceCacheInput
	Stdevs []l1_service.LoadStdevCacheInput
}

func (h *DryRunFactorMetricsHandler) Price(pr *l1_service.PriceCache, symbol string, date time.Time) (float64, error) {
	h.Prices = append(h.Prices, l1_service.LoadPriceCacheInput{
		Date:   date,
		Symbol: symbol,
	})
	return 0, nil
}

func (h *DryRunFactorMetricsHandler) PricePercentChange(pr *l1_service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	h.Prices = append(h.Prices, l1_service.LoadPriceCacheInput{
		Date:   start,
		Symbol: symbol,
	})
	h.Prices = append(h.Prices, l1_service.LoadPriceCacheInput{
		Date:   end,
		Symbol: symbol,
	})

	return 1, nil
}

func (h *DryRunFactorMetricsHandler) AnnualizedStdevOfDailyReturns(ctx context.Context, pr *l1_service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	h.Stdevs = append(h.Stdevs, l1_service.LoadStdevCacheInput{
		Start:  start,
		End:    end,
		Symbol: symbol,
	})
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) MarketCap(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) PeRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) PbRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h factorMetricsHandler) Price(pr *l1_service.PriceCache, symbol string, date time.Time) (float64, error) {
	return pr.Get(symbol, date)
}

func (h factorMetricsHandler) PricePercentChange(pr *l1_service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	startPrice, err := pr.Get(symbol, start)
	if err != nil {
		return 0, err
	}

	endPrice, err := pr.Get(symbol, end)
	if err != nil {
		return 0, err
	}

	return percentChange(endPrice, startPrice), nil
}

func percentChange(end, start float64) float64 {
	return ((end - start) / end) * 100
}

func (h factorMetricsHandler) AnnualizedStdevOfDailyReturns(ctx context.Context, pr *l1_service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	return pr.GetStdev(ctx, symbol, start, end)
}

func (h factorMetricsHandler) MarketCap(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	out, err := h.AssetFundamentalsRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, factorMetricsMissingDataError{err}
	}

	price, err := h.AdjustedPriceRepository.Get(symbol, date)
	if err != nil {
		return 0, err
	}
	if out.SharesOutstandingBasic == nil {
		return 0, factorMetricsMissingDataError{fmt.Errorf("%s does not have # shares outstanding on %v", symbol, date)}
	}

	return *out.SharesOutstandingBasic * price.InexactFloat64(), nil
}

func (h factorMetricsHandler) PeRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	price, err := h.AdjustedPriceRepository.Get(symbol, date)
	if err != nil {
		return 0, err
	}

	out, err := h.AssetFundamentalsRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, factorMetricsMissingDataError{err}
	}
	if out.EpsBasic == nil {
		return 0, factorMetricsMissingDataError{fmt.Errorf("%s does not have eps on %v", symbol, date)}
	}

	return price.InexactFloat64() / *out.EpsBasic, nil

}

func (h factorMetricsHandler) PbRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	price, err := h.AdjustedPriceRepository.Get(symbol, date)
	if err != nil {
		return 0, err
	}

	out, err := h.AssetFundamentalsRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, factorMetricsMissingDataError{err}
	}

	if out.TotalAssets == nil {
		return 0, factorMetricsMissingDataError{fmt.Errorf("missing total assets")}
	}
	if out.TotalLiabilities == nil {
		return 0, factorMetricsMissingDataError{fmt.Errorf("missing total liabilities")}
	}
	if out.SharesOutstandingBasic == nil {
		return 0, factorMetricsMissingDataError{fmt.Errorf("missing shares outstanding")}
	}

	return price.InexactFloat64() / ((*out.TotalAssets - *out.TotalLiabilities) / *out.SharesOutstandingBasic), nil
}
