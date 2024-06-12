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
	UniverseRepository   repository.UniverseRepository

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

	tx, err := h.Db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	dataValues := []service.LoadPriceCacheInput{}
	for _, v := range dataHandler.Data {
		dataValues = append(dataValues, v)
	}

	priceCache, err := h.PriceService.LoadCache(tx, dataValues)
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
		tx, err := h.Db.BeginTx(ctx, &sql.TxOptions{
			ReadOnly:  true,
			Isolation: sql.LevelReadCommitted,
		})
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
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
						tx,
						pr,
						input.FactorExpression,
						input.Symbol,
						h.FactorMetricsHandler,
						input.Date,
					)
					if err != nil {
						err = fmt.Errorf("failed to compute factor score for %s on %s: %w", input.Symbol, input.Date.Format("2006-01-02"), err)
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
	RoTx            *sql.Tx
	PriceMap        map[string]float64
	Date            time.Time
	PortfolioValue  float64
	FactorIntensity float64
	UniverseSymbols []string
	AssetOptions    internal.AssetSelectionOptions
	FactorScores    map[string]*float64
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
	symbols := []string{}
	if in.AssetOptions.Mode == internal.AssetSelectionMode_AnchorPortfolio {
		for symbol := range in.AssetOptions.AnchorPortfolioWeights {
			symbols = append(symbols, symbol)
		}
	} else {
		symbols = in.UniverseSymbols
	}

	if len(symbols) == 0 {
		return nil, fmt.Errorf("cannot compute target portfolio with 0 asset universe")
	}

	computeTargetInput := internal.CalculateTargetAssetWeightsInput{
		Tx:                    in.RoTx,
		Date:                  in.Date,
		FactorScoresBySymbol:  in.FactorScores,
		FactorIntensity:       in.FactorIntensity,
		AssetSelectionOptions: in.AssetOptions,
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

type FactorOptions struct {
	Expression string
	Intensity  float64
	Name       string
}

type BacktestInput struct {
	RoTx             *sql.Tx
	FactorOptions    FactorOptions
	BacktestStart    time.Time
	BacktestEnd      time.Time
	SamplingInterval time.Duration
	StartPortfolio   domain.Portfolio

	AnchorPortfolioQuantities map[string]float64
	AssetOptions              internal.AssetSelectionOptions
}

func (h BacktestHandler) Backtest(ctx context.Context, in BacktestInput) ([]BacktestSample, error) {
	profile := domain.GetPerformanceProfile(ctx) // used for profiling API performance

	universe, err := h.UniverseRepository.List(in.RoTx)
	if err != nil {
		return nil, err
	}
	universeSymbols := []string{}
	for _, u := range universe {
		universeSymbols = append(universeSymbols, u.Symbol)
	}

	updatePricesTx, err := h.Db.Begin()
	if err != nil {
		return nil, err
	}
	defer updatePricesTx.Rollback()
	err = h.PriceService.UpdatePricesIfNeeded(ctx, updatePricesTx, universeSymbols)
	if err != nil {
		return nil, err
	}
	err = updatePricesTx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit update prices changes: %w", err)
	}
	profile.Add("finished updating prices (if needed)")

	// all trading days within the selected window that we need to run a calculation on
	// this will only contain days that we actually have data for, so if data is old, it
	// will not include recent days
	tradingDays, err := h.calculateRelevantTradingDays(in.RoTx, in.BacktestStart, in.BacktestEnd, in.SamplingInterval)
	if err != nil {
		return nil, err
	}

	inputs := []workInput{}
	for _, tradingDay := range tradingDays {
		for _, symbol := range universeSymbols {
			inputs = append(inputs, workInput{
				Symbol:           symbol,
				Date:             tradingDay,
				FactorExpression: in.FactorOptions.Expression,
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

	// get price on first day? consider removing
	backtestStartPriceMap, err := h.PriceRepository.GetMany(in.RoTx, universeSymbols, in.BacktestStart)
	if err != nil {
		return nil, err
	}

	anchorPortfolioWeights, err := h.calculateAnchorPortfolioWeights(in.AnchorPortfolioQuantities, backtestStartPriceMap)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate anchor portfolio weights: %w", err)
	}

	in.AssetOptions.AnchorPortfolioWeights = anchorPortfolioWeights

	startValue, err := in.StartPortfolio.TotalValue(backtestStartPriceMap)
	if err != nil {
		return nil, fmt.Errorf("failed to compute start value: %w", err)
	} else if startValue == 0 {
		return nil, fmt.Errorf("cannot backtest portfolio with 0 total value")
	}

	currentPortfolio := *in.StartPortfolio.DeepCopy()
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
		priceMap, err := h.PriceRepository.GetMany(in.RoTx, universeSymbols, t)
		if err != nil {
			return nil, fmt.Errorf("failed to get prices on day %v: %w", t, err)
		}

		currentPortfolioValue, err := currentPortfolio.TotalValue(priceMap)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate portfolio value on %v: %w", t, err)
		}

		computeTargetPortfolioResponse, err := h.ComputeTargetPortfolio(ComputeTargetPortfolioInput{
			RoTx:            in.RoTx,
			Date:            t,
			FactorIntensity: in.FactorOptions.Intensity,
			FactorScores:    factorScoresByDay[t],
			AssetOptions:    in.AssetOptions,
			PortfolioValue:  currentPortfolioValue,
			PriceMap:        priceMap,
			UniverseSymbols: universeSymbols,
		})
		if err != nil {
			// TODO figure out what to do here. should
			// include something in the response that says
			// we couldn't rebalance here
			backtestErrors = append(backtestErrors, err)
			continue
			// return nil, fmt.Errorf("failed to compute target portfolio in backtest on %s: %w", t.Format("2006-01-02"), err)
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

	return out, nil
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
	tx *sql.Tx,
	start, end time.Time,
	interval time.Duration,
) ([]time.Time, error) {
	allTradingDays, err := h.PriceRepository.ListTradingDays(tx, start, end)
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
