package service

import (
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
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
	FactorExpressionService Fac
}

func (h investmentServiceHandler) GetTargetPortfolio(strategyInvestmentID uuid.UUID, date time.Time) (*domain.Portfolio, error) {
	strategyDetails, err := h.StrategyInvestment.Get(strategyInvestmentID)
	if err != nil {
		return nil, err
	}

	savedStrategyDetails, err := h.SavedStrategyRepository.Get(strategyDetails.SavedStragyID)
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

	factorScoresOnLatestDay, err := h.FactorExpressionService.CalculateFactorScores(ctx, []time.Time{date}, tickers, in.FactorExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}
	scoreResults := factorScoresOnLatestDay[*latestTradingDay]
	computeTargetPortfolioResponse, err := h.ComputeTargetPortfolio(ComputeTargetPortfolioInput{
		Date:             *latestTradingDay,
		TargetNumTickers: in.NumTickers,
		FactorScores:     scoreResults.SymbolScores,
		PortfolioValue:   1000,
		PriceMap:         pm,
	})
}
