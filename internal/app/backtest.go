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

type ComputeTargetPortfolioInput struct {
	RoTx             *sql.Tx
	PriceMap         map[string]float64
	Date             time.Time
	PortfolioValue   float64
	FactorIntensity  float64
	FactorExpression string
	UniverseSymbols  []string
	AssetOptions     internal.AssetSelectionOptions
}

type ComputeTargetPortfolioResponse struct {
	TargetPortfolio *domain.Portfolio
	AssetWeights    map[string]float64
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

	factorScoreBySymbol := map[string]float64{}
	for _, symbol := range symbols {
		result, err := internal.EvaluateFactorExpression(
			in.RoTx,
			in.FactorExpression,
			symbol,
			h.FactorMetricsHandler,
			in.Date,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate expression: %w", err)
		}
		factorScoreBySymbol[symbol] = result.Value
	}

	if len(factorScoreBySymbol) != len(symbols) {
		return nil, fmt.Errorf("received %d symbols but calculated %d factor scores", len(symbols), len(factorScoreBySymbol))
	}

	computeTargetInput := internal.CalculateTargetAssetWeightsInput{
		Tx:                    in.RoTx,
		Date:                  in.Date,
		FactorScoresBySymbol:  factorScoreBySymbol,
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

	return &ComputeTargetPortfolioResponse{
		TargetPortfolio: targetPortfolio,
		AssetWeights:    newWeights,
		TotalValue:      in.PortfolioValue,
	}, nil
}

type BacktestSample struct {
	Date           time.Time
	EndPortfolio   domain.Portfolio
	TotalValue     float64
	ProposedTrades []domain.ProposedTrade
	AssetWeights   map[string]float64
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

	priceMap := map[string]float64{}

	for _, symbol := range universeSymbols {
		price, err := h.PriceRepository.Get(in.RoTx, symbol, in.BacktestStart)
		if err != nil {
			return nil, err
		}
		priceMap[symbol] = price
	}

	anchorPortfolioWeights := map[string]float64{}
	sum := 0.0
	for symbol, quantity := range in.AnchorPortfolioQuantities {
		sum += priceMap[symbol] * quantity
	}
	for symbol, weight := range in.AnchorPortfolioQuantities {
		anchorPortfolioWeights[symbol] = priceMap[symbol] * weight / sum
	}
	in.AssetOptions.AnchorPortfolioWeights = anchorPortfolioWeights

	startValue, err := in.StartPortfolio.TotalValue(priceMap)
	if err != nil {
		return nil, err
	} else if startValue == 0 {
		return nil, fmt.Errorf("cannot backtest portfolio with 0 total value")
	}

	currentPortfolio := *in.StartPortfolio.DeepCopy()
	currentTime := in.BacktestStart
	out := []BacktestSample{}
	for currentTime.Unix() <= in.BacktestEnd.Unix() {
		// should work on weekends too

		// kinda pre-optimizing, but we use current price
		// of assets so much that it kinda makes sense to
		// just get everything and let everyone figure it out
		// this is also premature optimization
		priceMap := map[string]float64{}

		for _, symbol := range universeSymbols {
			price, err := h.PriceRepository.Get(in.RoTx, symbol, currentTime)
			if err != nil {
				return nil, err
			}
			priceMap[symbol] = price
		}

		currentPortfolioValue, err := currentPortfolio.TotalValue(priceMap)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate portfolio value on %v: %w", currentTime, err)
		}

		computeTargetPortfolioResponse, err := h.ComputeTargetPortfolio(ComputeTargetPortfolioInput{
			RoTx:             in.RoTx,
			Date:             currentTime,
			FactorIntensity:  in.FactorOptions.Intensity,
			FactorExpression: in.FactorOptions.Expression,
			AssetOptions:     in.AssetOptions,
			PortfolioValue:   currentPortfolioValue,
			PriceMap:         priceMap,
			UniverseSymbols:  universeSymbols,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to compute target portfolio in backtest on %v: %w", currentTime, err)
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
			Date:           currentTime,
			EndPortfolio:   *computeTargetPortfolioResponse.TargetPortfolio,
			ProposedTrades: trades,
			TotalValue:     computeTargetPortfolioResponse.TotalValue,
			AssetWeights:   computeTargetPortfolioResponse.AssetWeights,
		})
		currentPortfolio = *computeTargetPortfolioResponse.TargetPortfolio.DeepCopy()
		currentTime = currentTime.Add(in.SamplingInterval)
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
