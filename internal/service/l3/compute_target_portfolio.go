package l3_service

import (
	"factorbacktest/internal"
	"factorbacktest/internal/domain"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ComputeTargetPortfolioInput struct {
	PriceMap         map[string]decimal.Decimal
	Date             time.Time
	PortfolioValue   decimal.Decimal
	FactorScores     map[string]*float64
	TargetNumTickers int
	TickerIDMap      map[string]uuid.UUID
}

type ComputeTargetPortfolioResponse struct {
	TargetPortfolio *domain.Portfolio
	AssetWeights    map[string]float64
	FactorScores    map[string]float64
}

// Computes what the portfolio should hold on a given day, given the
// strategy (equation and universe) and value of current holdings
// TODO - find a better place for this function
func ComputeTargetPortfolio(in ComputeTargetPortfolioInput) (*ComputeTargetPortfolioResponse, error) {
	if in.PortfolioValue.LessThan(decimal.NewFromFloat(0.001)) {
		return nil, fmt.Errorf("cannot compute target portfolio with value %s", in.PortfolioValue.String())
	}
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
	targetPortfolio := domain.NewPortfolio()

	// convert weights into quantities
	for symbol, weight := range newWeights {
		price, ok := in.PriceMap[symbol]
		if !ok {
			return nil, fmt.Errorf("priceMap does not have %s", symbol)
		}

		// key line - determines how much new amount of symbol should be
		// i want to round this to something so that we can generate results
		// deterministically.

		dollarsOfSymbol := in.PortfolioValue.Mul(decimal.NewFromFloat(weight)).Round(3)
		quantity := dollarsOfSymbol.Div(price)

		tickerID := uuid.Nil
		if in.TickerIDMap != nil {
			if id, ok := in.TickerIDMap[symbol]; ok {
				tickerID = id
			}
		}

		targetPortfolio.Positions[symbol] = &domain.Position{
			Symbol:        symbol,
			ExactQuantity: quantity,
			TickerID:      tickerID,
			// if we want to switch to $ instead, add here
		}
	}

	selectedAssetFactorScores := map[string]float64{}
	for _, asset := range targetPortfolio.Positions {
		selectedAssetFactorScores[asset.Symbol] = *in.FactorScores[asset.Symbol]
	}

	return &ComputeTargetPortfolioResponse{
		TargetPortfolio: targetPortfolio,
		AssetWeights:    newWeights,
		FactorScores:    selectedAssetFactorScores,
	}, nil
}
