package service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/calculator"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// should eventually be folded into strategy service
// because you backtest a strategy.

type BacktestHandler struct {
	PriceRepository         repository.AdjustedPriceRepository
	AssetUniverseRepository repository.AssetUniverseRepository

	Db                      *sql.DB
	PriceService            data.PriceService
	FactorExpressionService calculator.FactorExpressionService
}

type BacktestResult struct {
	Date       time.Time
	Portfolio  domain.Portfolio
	TotalValue float64
	// might be less memory to join these in one map, but
	// it's also cleaner to have these seperated so i don't
	// need to define another struct for this, and because
	// these can be computed seperately without needing
	// to join them back together
	AssetWeights                 map[string]float64
	FactorScores                 map[string]float64
	PriceChangeTilNextResampling map[string]float64
}

type BacktestSnapshot struct {
	// presumably from last rebalance
	ValuePercentChange float64                         `json:"valuePercentChange"`
	Value              float64                         `json:"value"`
	Date               string                          `json:"date"`
	AssetMetrics       map[string]SnapshotAssetMetrics `json:"assetMetrics"`
}

type SnapshotAssetMetrics struct {
	AssetWeight                  float64  `json:"assetWeight"`
	FactorScore                  float64  `json:"factorScore"`
	PriceChangeTilNextResampling *float64 `json:"priceChangeTilNextResampling"`
}

type BacktestInput struct {
	FactorExpression  string
	BacktestStart     time.Time
	BacktestEnd       time.Time
	RebalanceInterval time.Duration
	StartingCash      float64
	NumTickers        int
	AssetUniverse     string
}

type BacktestResponse struct {
	Results        []BacktestResult
	Snapshots      map[string]BacktestSnapshot
	LatestHoldings LatestHoldings
}

func (h BacktestHandler) Backtest(ctx context.Context, in BacktestInput) (*BacktestResponse, error) {
	profile, endProfile := domain.GetProfile(ctx) // used for profiling API performance
	defer endProfile()

	_, endSpan := profile.StartNewSpan("setting up backtest")
	tickers, err := h.AssetUniverseRepository.GetAssets(in.AssetUniverse)
	if err != nil {
		return nil, err
	} else if len(tickers) == 0 {
		return nil, fmt.Errorf("no tickers found")
	}
	universeSymbols := []string{}
	for _, u := range tickers {
		universeSymbols = append(universeSymbols, u.Symbol)
	}

	// all trading days within the selected window that we need to run a calculation on
	// this will only contain days that we actually have data for, so if data is old, it
	// will not include recent days
	tradingDays, err := h.calculateRelevantTradingDays(in.BacktestStart, in.BacktestEnd, in.RebalanceInterval)
	if err != nil {
		return nil, err
	}
	if len(tradingDays) == 0 {
		return nil, fmt.Errorf("failed to backtest: no calculated trading days in given range")
	}

	endSpan()

	span, endSpan := profile.StartNewSpan("calculating factor scores")
	factorScoresByDay, err := h.FactorExpressionService.CalculateFactorScores(domain.NewCtxWithSubProfile(ctx, span), tradingDays, tickers, in.FactorExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}
	endSpan()

	startValue := decimal.NewFromFloat(in.StartingCash)

	currentPortfolio := domain.NewPortfolio()
	currentPortfolio.SetCash(startValue)

	out := []BacktestResult{}

	const errThreshold = 0.1
	backtestErrors := []error{}

	priceMap := map[string]map[string]decimal.Decimal{}

	_, endSpan = profile.StartNewSpan("daily calcs")
	for _, t := range tradingDays {
		// should work on weekends too

		// kinda pre-optimizing, but we use current price
		// of assets so much that it kinda makes sense to
		// just get everything and let everyone figure it out
		// this is also premature optimization
		pm, err := h.PriceRepository.GetManyOnDay(universeSymbols, t)
		if err != nil {
			return nil, fmt.Errorf("failed to get prices on day %v: %w", t, err)
		}
		priceMap[t.Format(time.DateOnly)] = pm

		currentPortfolioValue, err := currentPortfolio.TotalValue(pm)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate portfolio value on %v: %w", t, err)
		}

		valuesFromDay, ok := factorScoresByDay[t]
		if !ok {
			return nil, fmt.Errorf("failed to retrieve factor score data from %s", t.Format(time.DateOnly))
		}
		// scoringErrors := valuesFromDay.errors
		// backtestErrors = append(backtestErrors, scoringErrors...)

		// TODO - find a better place for this function to live
		computeTargetPortfolioResponse, err := calculator.ComputeTargetPortfolio(calculator.ComputeTargetPortfolioInput{
			Date:             t,
			TargetNumTickers: in.NumTickers,
			FactorScores:     valuesFromDay.SymbolScores,
			PortfolioValue:   currentPortfolioValue,
			PriceMap:         pm,
		})
		if err != nil {
			// TODO figure out what to do here. should
			// include something in the response that says
			// we couldn't rebalance here
			backtestErrors = append(backtestErrors, err)
			continue
			// return nil, fmt.Errorf("failed to compute target portfolio in backtest on %s: %w", t.Format(time.DateOnly), err)
		}

		out = append(out, BacktestResult{
			Date:         t,
			Portfolio:    *computeTargetPortfolioResponse.TargetPortfolio,
			TotalValue:   currentPortfolioValue.InexactFloat64(),
			AssetWeights: computeTargetPortfolioResponse.AssetWeights,
			FactorScores: computeTargetPortfolioResponse.FactorScores,
		})
		currentPortfolio = computeTargetPortfolioResponse.TargetPortfolio.DeepCopy()
	}
	endSpan()

	if float64(len(backtestErrors))/float64(len(tradingDays)) >= errThreshold {
		numErrors := 3
		if len(backtestErrors) < 3 {
			numErrors = len(backtestErrors)
		}
		return nil, fmt.Errorf("too many backtest errors (%d %%). first %d: %v", int(100*float64(len(backtestErrors))/float64(len(tradingDays))), numErrors, backtestErrors[:numErrors])
	}

	// todo - snapshots and latest holdings
	// need to be moved out

	_, endSpan = profile.StartNewSpan("creating snapshots")
	snapshots, err := toSnapshots(out, priceMap)
	if err != nil {
		return nil, fmt.Errorf("failed to compute snapshots: %w", err)
	}
	endSpan()

	latestHoldings, err := getLatestHoldings(ctx, h, universeSymbols, tickers, in)
	if err != nil {
		return nil, err
	}

	return &BacktestResponse{
		Results:        out,
		Snapshots:      snapshots,
		LatestHoldings: *latestHoldings,
	}, nil
}

type LatestHoldings struct {
	Date   time.Time
	Assets map[string]SnapshotAssetMetrics
}

func getLatestHoldings(ctx context.Context, h BacktestHandler, universeSymbols []string, tickers []model.Ticker, in BacktestInput) (*LatestHoldings, error) {
	latestTradingDay, err := h.PriceRepository.LatestTradingDay()
	if err != nil {
		return nil, err
	}

	pm, err := h.PriceRepository.GetManyOnDay(universeSymbols, *latestTradingDay)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices on day %v: %w", latestTradingDay, err)
	}
	factorScoresOnLatestDay, err := h.FactorExpressionService.CalculateFactorScores(ctx, []time.Time{*latestTradingDay}, tickers, in.FactorExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}
	scoreResults := factorScoresOnLatestDay[*latestTradingDay]
	computeTargetPortfolioResponse, err := calculator.ComputeTargetPortfolio(calculator.ComputeTargetPortfolioInput{
		Date:             *latestTradingDay,
		TargetNumTickers: in.NumTickers,
		FactorScores:     scoreResults.SymbolScores,
		PortfolioValue:   decimal.NewFromInt(1000),
		PriceMap:         pm,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to calculate target portfolio")
	}

	out := LatestHoldings{
		Date:   *latestTradingDay,
		Assets: map[string]SnapshotAssetMetrics{},
	}

	for symbol := range computeTargetPortfolioResponse.FactorScores {
		out.Assets[symbol] = SnapshotAssetMetrics{
			AssetWeight: computeTargetPortfolioResponse.AssetWeights[symbol],
			FactorScore: computeTargetPortfolioResponse.FactorScores[symbol],
		}
	}

	return &out, nil
}

func toSnapshots(result []BacktestResult, priceMap map[string]map[string]decimal.Decimal) (map[string]BacktestSnapshot, error) {
	snapshots := map[string]BacktestSnapshot{}

	for i, r := range result {
		pc := 0.0
		if i != 0 {
			pc = 100 * (r.TotalValue - result[0].TotalValue) / result[0].TotalValue
		}
		priceChangeTilNextResampling := map[string]float64{}

		if i < len(result)-1 {
			nextResamplingDate := result[i+1].Date
			for symbol := range r.AssetWeights {
				startPrice, ok := priceMap[r.Date.Format(time.DateOnly)][symbol]
				if !ok {
					return nil, fmt.Errorf("failed to get start price from cache: %s, %v", symbol, r.Date)
				}
				endPrice, ok := priceMap[nextResamplingDate.Format(time.DateOnly)][symbol]
				if !ok {
					return nil, fmt.Errorf("failed to get end price from cache: %s, %v", symbol, r.Date)
				}
				priceChangeTilNextResampling[symbol] = decimal.NewFromInt(100).Mul((endPrice.Sub(startPrice)).Div(startPrice)).InexactFloat64()
			}
		}

		snapshots[r.Date.Format(time.DateOnly)] = BacktestSnapshot{
			ValuePercentChange: pc,
			Value:              r.TotalValue,
			Date:               r.Date.Format(time.DateOnly),
			AssetMetrics:       joinAssetMetrics(r.AssetWeights, r.FactorScores, priceChangeTilNextResampling),
		}
	}

	return snapshots, nil
}

func joinAssetMetrics(
	weights map[string]float64,
	factorScores map[string]float64,
	priceChangeTilNextResampling map[string]float64,
) map[string]SnapshotAssetMetrics {
	assetMetrics := map[string]*SnapshotAssetMetrics{}
	for k, v := range weights {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &SnapshotAssetMetrics{}
		}
		assetMetrics[k].AssetWeight = v
	}
	for k, v := range factorScores {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &SnapshotAssetMetrics{}
		}
		assetMetrics[k].FactorScore = v
	}
	for k, v := range priceChangeTilNextResampling {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &SnapshotAssetMetrics{}
		}
		x := v // lol pointer math
		assetMetrics[k].PriceChangeTilNextResampling = &x
	}

	out := map[string]SnapshotAssetMetrics{}
	for k := range assetMetrics {
		out[k] = *assetMetrics[k]
	}

	return out
}

func (h BacktestHandler) calculateRelevantTradingDays(
	start, end time.Time,
	interval time.Duration,
) ([]time.Time, error) {
	allTradingDays, err := h.PriceRepository.ListTradingDays(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate trading days: %w", err)
	}
	if len(allTradingDays) == 0 {
		return []time.Time{}, nil
	}

	allTradingDaysSet := map[time.Time]bool{}
	for _, t := range allTradingDays {
		allTradingDaysSet[t] = true
	}

	tradingDays := []time.Time{}
	currentTime := allTradingDays[0]
	for currentTime.Unix() <= end.Unix() {
		if _, ok := allTradingDaysSet[currentTime]; ok {
			tradingDays = append(tradingDays, currentTime)
			currentTime = currentTime.Add(interval)
		} else {
			currentTime = currentTime.Add(time.Hour * 24)
		}
	}

	return tradingDays, nil
}
