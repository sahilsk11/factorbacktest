package app

import (
	"context"
	"database/sql"
	"errors"
	"factorbacktest/internal"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"fmt"
	"sync"
	"time"
)

type BacktestHandler struct {
	PriceRepository      repository.AdjustedPriceRepository
	FactorMetricsHandler internal.FactorMetricCalculations
	TickerRepository     repository.TickerRepository

	Db           *sql.DB
	PriceService service.PriceService
}

type workInput struct {
	Symbol           string
	Date             time.Time
	FactorExpression string
}

// preloadData "dry-runs" the factor expression to determine which dates are needed
// then loads them into a price cache. it has no concept of trading days, so it
// may produce cache misses on holidays
func (h BacktestHandler) preloadData(ctx context.Context, in []workInput) (*service.PriceCache, error) {
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

// calculateFactorScores asynchronously processes factor expression calculations for every relevant day in the backtest
// using the list of workInputs, it spawns workers to calculate what the score for a particular asset would be on that day
// despite using workers, this is still the slowest part of the flow
func (h BacktestHandler) calculateFactorScores(ctx context.Context, pr *service.PriceCache, in []workInput) (map[time.Time]map[string]*float64, error) {
	numGoroutines := 10

	type result struct {
		Date             time.Time
		Symbol           string
		ExpressionResult *internal.ExpressionResult
		Err              error
	}

	inputCh := make(chan workInput, len(in))
	resultCh := make(chan result, len(in))

	var wg sync.WaitGroup
	for _, f := range in {
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
						pr,
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

	out := map[time.Time]map[string]*float64{}
	for _, res := range results {
		if _, ok := out[res.Date]; !ok {
			out[res.Date] = map[string]*float64{}
		}
		if res.Err != nil && !errors.As(res.Err, &internal.FactorMetricsMissingDataError{}) {
			return nil, res.Err
		} else if res.Err == nil {
			out[res.Date][res.Symbol] = &res.ExpressionResult.Value
		}
	}

	return out, nil
}

type ComputeTargetPortfolioInput struct {
	PriceMap         map[string]float64
	Date             time.Time
	PortfolioValue   float64
	FactorScores     map[string]*float64
	TargetNumTickers int
}

type ComputeTargetPortfolioResponse struct {
	TargetPortfolio *domain.Portfolio
	AssetWeights    map[string]float64
	FactorScores    map[string]float64
	TotalValue      float64
}

// Computes what the portfolio should hold on a given day, given the
// strategy (equation and universe) and value of current holdings
func (h BacktestHandler) ComputeTargetPortfolio(in ComputeTargetPortfolioInput) (*ComputeTargetPortfolioResponse, error) {
	if in.TargetNumTickers < 3 {
		return nil, fmt.Errorf("insufficient tickers: at least 3 target tickers required, got %d", in.TargetNumTickers)
	}

	computeTargetInput := internal.CalculateTargetAssetWeightsInput{
		Date:                 in.Date,
		FactorScoresBySymbol: in.FactorScores,
		NumTickers:           in.TargetNumTickers,
	}
	newWeights, err := internal.CalculateTargetAssetWeights(computeTargetInput)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate target asset weights: %w", err)
	}

	// this is where the assumption that target portfolio will not hold
	// cash comes from - the field is just not populated
	targetPortfolio := &domain.Portfolio{
		Positions: map[string]*domain.Position{},
	}

	// convert weights into quantities
	for symbol, weight := range newWeights {
		price, ok := in.PriceMap[symbol]
		if !ok {
			return nil, fmt.Errorf("priceMap does not have %s", symbol)
		}
		dollarsOfSymbol := in.PortfolioValue * weight
		// TODO - verify that priceMap[symbol] exists
		quantity := dollarsOfSymbol / price
		targetPortfolio.Positions[symbol] = &domain.Position{
			Symbol:   symbol,
			Quantity: quantity,
		}
	}

	selectedAssetFactorScores := map[string]float64{}
	for _, asset := range targetPortfolio.Positions {
		selectedAssetFactorScores[asset.Symbol] = *in.FactorScores[asset.Symbol]
	}

	return &ComputeTargetPortfolioResponse{
		TargetPortfolio: targetPortfolio,
		AssetWeights:    newWeights,
		TotalValue:      in.PortfolioValue,
		FactorScores:    selectedAssetFactorScores,
	}, nil
}

type BacktestSample struct {
	Date           time.Time
	EndPortfolio   domain.Portfolio
	TotalValue     float64
	ProposedTrades []domain.ProposedTrade
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
	ValuePercentChange float64                          `json:"valuePercentChange"`
	Value              float64                          `json:"value"`
	Date               string                           `json:"date"`
	AssetMetrics       map[string]ScnapshotAssetMetrics `json:"assetMetrics"`
}

type ScnapshotAssetMetrics struct {
	AssetWeight                  float64  `json:"assetWeight"`
	FactorScore                  float64  `json:"factorScore"`
	PriceChangeTilNextResampling *float64 `json:"priceChangeTilNextResampling"`
}

type BacktestInput struct {
	FactorName        string
	FactorExpression  string
	BacktestStart     time.Time
	BacktestEnd       time.Time
	RebalanceInterval time.Duration
	StartingCash      float64
	NumTickers        int
}

type BacktestResponse struct {
	BacktestSamples []BacktestSample
	Snapshots       map[string]BacktestSnapshot
}

func (h BacktestHandler) Backtest(ctx context.Context, in BacktestInput) (*BacktestResponse, error) {
	profile := domain.GetPerformanceProfile(ctx) // used for profiling API performance

	universe, err := h.TickerRepository.List()
	if err != nil {
		return nil, err
	}
	universeSymbols := []string{}
	for _, u := range universe {
		universeSymbols = append(universeSymbols, u.Symbol)
	}

	// all trading days within the selected window that we need to run a calculation on
	// this will only contain days that we actually have data for, so if data is old, it
	// will not include recent days
	tradingDays, err := h.calculateRelevantTradingDays(in.BacktestStart, in.BacktestEnd, in.RebalanceInterval)
	if err != nil {
		return nil, err
	}

	inputs := []workInput{}
	for _, tradingDay := range tradingDays {
		for _, symbol := range universeSymbols {
			inputs = append(inputs, workInput{
				Symbol:           symbol,
				Date:             tradingDay,
				FactorExpression: in.FactorExpression,
			})
		}
	}
	profile.Add("finished helper info")

	priceCache, err := h.preloadData(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to preload data: %w", err)
	}

	profile.Add("finished preloading prices")

	x := time.Now()
	factorScoresByDay, err := h.calculateFactorScores(ctx, priceCache, inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}
	fmt.Println("scores calculated in", time.Since(x).Seconds())
	profile.Add("finished scores")

	startValue := in.StartingCash

	currentPortfolio := domain.Portfolio{
		Cash: startValue,
	}
	out := []BacktestSample{}

	profile.Add("finished backtest setup")

	const errThreshold = 0.1
	backtestErrors := []error{}

	for _, t := range tradingDays {
		// should work on weekends too

		// kinda pre-optimizing, but we use current price
		// of assets so much that it kinda makes sense to
		// just get everything and let everyone figure it out
		// this is also premature optimization
		priceMap, err := h.PriceRepository.GetMany(universeSymbols, t)
		if err != nil {
			return nil, fmt.Errorf("failed to get prices on day %v: %w", t, err)
		}

		currentPortfolioValue, err := currentPortfolio.TotalValue(priceMap)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate portfolio value on %v: %w", t, err)
		}

		computeTargetPortfolioResponse, err := h.ComputeTargetPortfolio(ComputeTargetPortfolioInput{
			Date:             t,
			TargetNumTickers: in.NumTickers,
			FactorScores:     factorScoresByDay[t],
			PortfolioValue:   currentPortfolioValue,
			PriceMap:         priceMap,
		})
		if err != nil {
			// TODO figure out what to do here. should
			// include something in the response that says
			// we couldn't rebalance here
			backtestErrors = append(backtestErrors, err)
			continue
			// return nil, fmt.Errorf("failed to compute target portfolio in backtest on %s: %w", t.Format(time.DateOnly), err)
		}
		trades, err := h.transitionToTarget(
			currentPortfolio,
			*computeTargetPortfolioResponse.TargetPortfolio,
			priceMap,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to transition to target: %w", err)
		}

		out = append(out, BacktestSample{
			Date:           t,
			EndPortfolio:   *computeTargetPortfolioResponse.TargetPortfolio,
			ProposedTrades: trades,
			TotalValue:     computeTargetPortfolioResponse.TotalValue,
			AssetWeights:   computeTargetPortfolioResponse.AssetWeights,
			FactorScores:   computeTargetPortfolioResponse.FactorScores,
		})
		currentPortfolio = *computeTargetPortfolioResponse.TargetPortfolio.DeepCopy()
	}
	profile.Add("finished daily calcs")

	if float64(len(backtestErrors))/float64(len(tradingDays)) >= errThreshold {
		numErrors := 3
		if len(backtestErrors) < 3 {
			numErrors = len(backtestErrors)
		}
		return nil, fmt.Errorf("too many backtest errors (%d %%). first %d: %v", int(100*float64(len(backtestErrors))/float64(len(tradingDays))), numErrors, backtestErrors[:numErrors])
	}

	snapshots, err := toSnapshots(out, priceCache)
	if err != nil {
		return nil, fmt.Errorf("failed to compute snapshots: %w", err)
	}

	return &BacktestResponse{
		BacktestSamples: out,
		Snapshots:       snapshots,
	}, nil
}

func toSnapshots(result []BacktestSample, priceCache *service.PriceCache) (map[string]BacktestSnapshot, error) {
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
				startPrice, err := priceCache.Get(symbol, r.Date)
				if err != nil {
					return nil, fmt.Errorf("failed to get start price from cache: %w", err)
				}
				endPrice, err := priceCache.Get(symbol, nextResamplingDate)
				if err != nil {
					return nil, fmt.Errorf("failed to get end price from cache: %w", err)
				}
				priceChangeTilNextResampling[symbol] = 100 * (endPrice - startPrice) / startPrice
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
) map[string]ScnapshotAssetMetrics {
	assetMetrics := map[string]*ScnapshotAssetMetrics{}
	for k, v := range weights {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &ScnapshotAssetMetrics{}
		}
		assetMetrics[k].AssetWeight = v
	}
	for k, v := range factorScores {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &ScnapshotAssetMetrics{}
		}
		assetMetrics[k].FactorScore = v
	}
	for k, v := range priceChangeTilNextResampling {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &ScnapshotAssetMetrics{}
		}
		x := v // lol pointer math
		assetMetrics[k].PriceChangeTilNextResampling = &x
	}

	out := map[string]ScnapshotAssetMetrics{}
	for k := range assetMetrics {
		out[k] = *assetMetrics[k]
	}

	return out
}

type transitionToTargetResult struct {
	ProposedTrades []domain.ProposedTrade
	NewPortfolio   domain.Portfolio
	NewTotalValue  float64
}

func (h BacktestHandler) transitionToTarget(
	currentPortfolio domain.Portfolio,
	targetPortfolio domain.Portfolio,
	priceMap map[string]float64,
) ([]domain.ProposedTrade, error) {
	trades := []domain.ProposedTrade{}
	prevPositions := currentPortfolio.Positions
	targetPositions := targetPortfolio.Positions

	for symbol, position := range targetPositions {
		diff := position.Quantity
		prevPosition, ok := prevPositions[symbol]
		if ok {
			diff = position.Quantity - prevPosition.Quantity
		}
		if diff != 0 {
			trades = append(trades, domain.ProposedTrade{
				Symbol:        symbol,
				Quantity:      diff,
				ExpectedPrice: priceMap[symbol],
			})
		}
	}
	for symbol, position := range prevPositions {
		if _, ok := targetPositions[symbol]; !ok {
			trades = append(trades, domain.ProposedTrade{
				Symbol:        symbol,
				Quantity:      -position.Quantity,
				ExpectedPrice: priceMap[symbol],
			})
		}
	}

	return trades, nil
}

func (h BacktestHandler) calculateAnchorPortfolioWeights(
	anchorPortfolioQuantities map[string]float64,
	priceMap map[string]float64,
) (map[string]float64, error) {
	anchorPortfolioWeights := map[string]float64{}
	sum := 0.0
	for symbol, quantity := range anchorPortfolioQuantities {
		sum += priceMap[symbol] * quantity
	}
	for symbol, weight := range anchorPortfolioQuantities {
		anchorPortfolioWeights[symbol] = priceMap[symbol] * weight / sum
	}

	return anchorPortfolioWeights, nil
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
