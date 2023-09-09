package app

import (
	"alpha/internal"
	"alpha/internal/domain"
	"alpha/internal/repository"
	"database/sql"
	"fmt"
	"time"
)

type BacktestHandler struct {
	PriceRepository      repository.AdjustedPriceRepository
	FactorMetricsHandler internal.FactorMetricCalculations
	UniverseRepository   repository.UniverseRepository
}

type workInput struct {
	Symbol           string
	Date             time.Time
	FactorExpression string
}

func (h BacktestHandler) CalculateFactorScores(tx *sql.Tx, in []workInput) (map[string]map[string]float64, error) {
	out := map[string]map[string]float64{}
	for _, x := range in {
		fmt.Println("starting", x.Symbol, x.Date)
		result, err := internal.EvaluateFactorExpression(
			tx,
			x.FactorExpression,
			x.Symbol,
			h.FactorMetricsHandler,
			x.Date,
		)
		if err != nil {
			return nil, err
		}
		fmt.Println("finished", x.Symbol, x.Date)
		dateStr := x.Date.Format("2006-01-02")
		if _, ok := out[dateStr]; !ok {
			out[dateStr] = map[string]float64{}
		}
		out[dateStr][x.Symbol] = result.Value
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
	FactorScores    map[string]float64
}

type ComputeTargetPortfolioResponse struct {
	TargetPortfolio *domain.Portfolio
	AssetWeights    map[string]float64
	FactorScores    map[string]float64
	TotalValue      float64
}

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

	if len(in.FactorScores) != len(symbols) {
		return nil, fmt.Errorf("received %d symbols but calculated %d factor scores", len(symbols), len(in.FactorScores))
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
		selectedAssetFactorScores[asset.Symbol] = in.FactorScores[asset.Symbol]
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

func (h BacktestHandler) Backtest(in BacktestInput) ([]BacktestSample, error) {
	universe, err := h.UniverseRepository.List(in.RoTx)
	if err != nil {
		return nil, err
	}
	universeSymbols := []string{}
	for _, u := range universe {
		universeSymbols = append(universeSymbols, u.Symbol)
	}

	backtestStartPriceMap := map[string]float64{}

	for _, symbol := range universeSymbols {
		price, err := h.PriceRepository.Get(in.RoTx, symbol, in.BacktestStart)
		if err != nil {
			return nil, err
		}
		backtestStartPriceMap[symbol] = price
	}

	allTradingDays, err := h.PriceRepository.ListTradingDays(in.RoTx, in.BacktestStart, in.BacktestEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate trading days: %w", err)
	}
	tradingDays := []time.Time{}
	currentTime := allTradingDays[0]
	for currentTime.Unix() <= in.BacktestEnd.Unix() {
		tradingDays = append(tradingDays, currentTime)
		currentTime = currentTime.Add(in.SamplingInterval)
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

	factorScoresByDay, err := h.CalculateFactorScores(in.RoTx, inputs)
	if err != nil {
		return nil, err
	}

	fmt.Println("done")

	anchorPortfolioWeights, err := h.calculateAnchorPortfolioWeights(in.RoTx, in.BacktestStart, in.AnchorPortfolioQuantities, backtestStartPriceMap)
	if err != nil {
		return nil, err
	}

	in.AssetOptions.AnchorPortfolioWeights = anchorPortfolioWeights

	startValue, err := in.StartPortfolio.TotalValue(backtestStartPriceMap)
	if err != nil {
		return nil, err
	} else if startValue == 0 {
		return nil, fmt.Errorf("cannot backtest portfolio with 0 total value")
	}

	currentPortfolio := *in.StartPortfolio.DeepCopy()
	out := []BacktestSample{}
	for _, t := range tradingDays {
		// should work on weekends too

		// kinda pre-optimizing, but we use current price
		// of assets so much that it kinda makes sense to
		// just get everything and let everyone figure it out
		// this is also premature optimization
		priceMap := map[string]float64{}

		for _, symbol := range universeSymbols {
			price, err := h.PriceRepository.Get(in.RoTx, symbol, t)
			if err != nil {
				return nil, err
			}
			priceMap[symbol] = price
		}

		currentPortfolioValue, err := currentPortfolio.TotalValue(priceMap)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate portfolio value on %v: %w", t, err)
		}

		computeTargetPortfolioResponse, err := h.ComputeTargetPortfolio(ComputeTargetPortfolioInput{
			RoTx:            in.RoTx,
			Date:            t,
			FactorIntensity: in.FactorOptions.Intensity,
			FactorScores:    factorScoresByDay[t.Format("2006-01-02")],
			AssetOptions:    in.AssetOptions,
			PortfolioValue:  currentPortfolioValue,
			PriceMap:        priceMap,
			UniverseSymbols: universeSymbols,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to compute target portfolio in backtest on %v: %w", t, err)
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
	tx *sql.Tx,
	backtestStart time.Time,
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
