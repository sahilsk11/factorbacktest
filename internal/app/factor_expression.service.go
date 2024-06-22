package app

import (
	"context"
	"database/sql"
	"errors"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
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
	Db                   *sql.DB
	FactorMetricsHandler internal.FactorMetricCalculations
	PriceService         service.PriceService
}

func NewFactorExpressionService(db *sql.DB, factorMetricsHandler internal.FactorMetricCalculations) FactorExpressionService {
	return factorExpressionServiceHandler{
		Db:                   db,
		FactorMetricsHandler: factorMetricsHandler,
	}
}

type workInput struct {
	Symbol           string
	Date             time.Time
	FactorExpression string
}

// calculateFactorScores asynchronously processes factor expression calculations for every relevant day in the backtest
// using the list of workInputs, it spawns workers to calculate what the score for a particular asset would be on that day
// despite using workers, this is still the slowest part of the flow
func (h factorExpressionServiceHandler) CalculateFactorScores(ctx context.Context, tradingDays []time.Time, tickers []model.Ticker, factorExpression string) (map[time.Time]*ScoresResultsOnDay, error) {
	profile := domain.GetPerformanceProfile(ctx) // used for profiling API performance

	inputs := []workInput{}
	for _, tradingDay := range tradingDays {
		for _, ticker := range tickers {
			inputs = append(inputs, workInput{
				Symbol:           ticker.Symbol,
				Date:             tradingDay,
				FactorExpression: factorExpression,
			})
		}
	}

	cache, err := h.preloadData(ctx, inputs)
	if err != nil {
		return nil, err
	}

	profile.Add("finished preloading prices")

	numGoroutines := 10

	type result struct {
		Date             time.Time
		Symbol           string
		ExpressionResult *internal.ExpressionResult
		Err              error
	}

	inputCh := make(chan workInput, len(inputs))
	resultCh := make(chan result, len(inputs))

	var wg sync.WaitGroup
	for _, f := range inputs {
		wg.Add(1)
		inputCh <- f
	}
	close(inputCh)

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
					res, err := internal.EvaluateFactorExpression(
						ctx,
						h.Db,
						cache,
						input.FactorExpression,
						input.Symbol,
						h.FactorMetricsHandler,
						input.Date,
					)
					if err != nil {
						err = fmt.Errorf("failed to compute factor score for %s on %s: %w", input.Symbol, input.Date.Format(time.DateOnly), err)
					}
					resultCh <- result{
						ExpressionResult: res,
						Symbol:           input.Symbol,
						Date:             input.Date,
						Err:              err,
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

	results := []result{}
	for res := range resultCh {
		results = append(results, res)
	}

	out := map[time.Time]*ScoresResultsOnDay{}
	for _, res := range results {
		if _, ok := out[res.Date]; !ok {
			out[res.Date] = &ScoresResultsOnDay{
				SymbolScores: map[string]*float64{},
				Errors:       []error{},
			}
		}
		if res.Err != nil && !errors.As(res.Err, &internal.FactorMetricsMissingDataError{}) {
			out[res.Date].Errors = append(out[res.Date].Errors, res.Err)
		} else if res.Err == nil {
			out[res.Date].SymbolScores[res.Symbol] = &res.ExpressionResult.Value
		}
	}

	return out, nil
}

// preloadData "dry-runs" the factor expression to determine which dates are needed
// then loads them into a price cache. it has no concept of trading days, so it
// may produce cache misses on holidays
func (h factorExpressionServiceHandler) preloadData(ctx context.Context, in []workInput) (*service.PriceCache, error) {
	dataHandler := internal.DryRunFactorMetricsHandler{
		Data: map[string]service.LoadPriceCacheInput{},
	}
	for _, n := range in {
		_, err := internal.EvaluateFactorExpression(ctx, nil, nil, n.FactorExpression, n.Symbol, &dataHandler, n.Date)
		if err != nil {
			return nil, err
		}
	}

	dataValues := []service.LoadPriceCacheInput{}
	for _, v := range dataHandler.Data {
		dataValues = append(dataValues, v)
	}

	priceCache, err := h.PriceService.LoadCache(dataValues)
	if err != nil {
		return nil, fmt.Errorf("failed to populate price cache: %w", err)
	}

	return priceCache, nil
}
