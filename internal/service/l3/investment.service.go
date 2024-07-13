package l3_service

import (
	"context"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	l2_service "factorbacktest/internal/service/l2"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type InvestmentService interface {
	GetAggregrateTargetPortfolio(date time.Time) (*domain.Portfolio, error)
}

type investmentServiceHandler struct {
	PriceRepository         repository.AdjustedPriceRepository
	UniverseRepository      repository.AssetUniverseRepository
	StrategyInvestment      repository.StrategyInvestmentRepository
	SavedStrategyRepository repository.SavedStrategyRepository
	FactorExpressionService l2_service.FactorExpressionService
}

func (h investmentServiceHandler) GetTargetPortfolio(ctx context.Context, strategyInvestmentID uuid.UUID, date time.Time) (*domain.Portfolio, error) {
	investmentDetails, err := h.StrategyInvestment.Get(strategyInvestmentID)
	if err != nil {
		return nil, err
	}

	// TODO - get current value of holdings? or do we want to return as
	// percent allocations here

	savedStrategyDetails, err := h.SavedStrategyRepository.Get(investmentDetails.SavedStragyID)
	if err != nil {
		return nil, err
	}

	universe, err := h.UniverseRepository.GetAssets(savedStrategyDetails.AssetUniverse)
	if err != nil {
		return nil, err
	}
	universeSymbols := []string{}
	for _, u := range universe {
		universeSymbols = append(universeSymbols, u.Symbol)
	}

	pm, err := h.PriceRepository.GetManyOnDay(universeSymbols, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices on day %v: %w", date, err)
	}

	factorScoresOnLatestDay, err := h.FactorExpressionService.CalculateFactorScoresOnDay(ctx, date, universe, savedStrategyDetails.FactorExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}

	computeTargetPortfolioResponse, err := ComputeTargetPortfolio(ComputeTargetPortfolioInput{
		Date:             date,
		TargetNumTickers: int(savedStrategyDetails.NumAssets),
		FactorScores:     factorScoresOnLatestDay.SymbolScores,
		PortfolioValue:   float64(investmentDetails.AmountDollars), // should get latest portfolio value
		PriceMap:         pm,
	})
	if err != nil {
		return nil, err
	}

	return computeTargetPortfolioResponse.TargetPortfolio, nil
}
