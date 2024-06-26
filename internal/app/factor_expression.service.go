package app

import (
	"context"
	"database/sql"
	"errors"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"

	"fmt"
	"sync"
	"time"
)

type ScoresResultsOnDay struct {
	SymbolScores map[string]*float64
	Errors       []error
}

type FactorExpressionService interface {
	CalculateFactorScores(ctx context.Context, tradingDays []time.Time, tickers []model.Ticker, factorExpression string) (map[time.Time]*ScoresResultsOnDay, error)
}

type factorExpressionServiceHandler struct {
	Db                    *sql.DB
	FactorMetricsHandler  internal.FactorMetricCalculations
	PriceService          service.PriceService
	FactorScoreRepository repository.FactorScoreRepository
}

func NewFactorExpressionService(
	db *sql.DB,
	factorMetricsHandler internal.FactorMetricCalculations,
	priceService service.PriceService,
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
	ExpressionResult *internal.ExpressionResult
	Err              error
	elapsedMs        int64
	span             *domain.Span
}

// calculateFactorScores asynchronously processes factor expression calculations for every relevant day in the backtest
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
	_, endSpan := profile.StartNewSpan("load price cache")
	cache, err := h.loadPriceCache(ctx, inputs)
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
					res, err := internal.EvaluateFactorExpression(
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

		if res.Err != nil && !errors.As(res.Err, &internal.FactorMetricsMissingDataError{}) {
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
func (h factorExpressionServiceHandler) loadPriceCache(ctx context.Context, in []workInput) (*service.PriceCache, error) {
	dataHandler := internal.DryRunFactorMetricsHandler{
		Prices: []service.LoadPriceCacheInput{},
		Stdevs: []service.LoadStdevCacheInput{},
	}
	for _, n := range in {
		_, err := internal.EvaluateFactorExpression(
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

	priceCache, err := h.PriceService.LoadPriceCache(dataHandler.Prices, dataHandler.Stdevs)
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
