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
	// GetAggregrateTargetPortfolio(date time.Time) (*domain.Portfolio, error)
}

type investmentServiceHandler struct {
	PriceRepository         repository.AdjustedPriceRepository
	UniverseRepository      repository.AssetUniverseRepository
	StrategyInvestment      repository.StrategyInvestmentRepository
	SavedStrategyRepository repository.SavedStrategyRepository
	FactorExpressionService l2_service.FactorExpressionService
	TickerRepository        repository.TickerRepository
}

func (h investmentServiceHandler) getTargetPortfolio(ctx context.Context, strategyInvestmentID uuid.UUID, date time.Time, pm map[string]float64) (map[string]*domain.Position, error) {
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

	return computeTargetPortfolioResponse.TargetPortfolio.Positions, nil
}

func (h investmentServiceHandler) getAggregrateTargetPortfolio(ctx context.Context, date time.Time, pm map[string]float64) (*domain.Portfolio, error) {
	// get all active investments
	investments, err := h.StrategyInvestment.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}

	aggregatePortfolio := domain.NewPortfolio()
	for _, i := range investments {
		strategyPortfolio, err := h.getTargetPortfolio(ctx, i.StrategyInvestmentID, date, pm)
		if err != nil {
			return nil, err
		}
		for symbol, position := range strategyPortfolio {
			if _, ok := aggregatePortfolio.Positions[symbol]; !ok {
				aggregatePortfolio.Positions[symbol] = &domain.Position{
					Symbol:   symbol,
					Quantity: 0,
				}
			}
			aggregatePortfolio.Positions[symbol].Quantity += position.Quantity
		}
	}

	return aggregatePortfolio, nil
}

func (h investmentServiceHandler) getCurrentAggregatePortfolio() (*domain.Portfolio, error) {
	return nil, fmt.Errorf("not implemented")
}

// TODO - how should we handle trades where amount < $1
func transitionToTarget(
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

	// we could round all trades up to $1 but
	// if they have tons of little trades, that
	// could get expensive

	return trades, nil
}

func (h investmentServiceHandler) generateProposedTrades(ctx context.Context, date time.Time) ([]domain.ProposedTrade, error) {
	assets, err := h.TickerRepository.List()
	if err != nil {
		return nil, err
	}
	symbols := []string{}
	for _, s := range assets {
		symbols = append(symbols, s.Symbol)
	}

	pm, err := h.PriceRepository.GetManyOnDay(symbols, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices on day %v: %w", date, err)
	}

	currentPortfolio, err := h.getCurrentAggregatePortfolio()
	if err != nil {
		return nil, err
	}
	targetPortfolio, err := h.getAggregrateTargetPortfolio(ctx, date, pm)
	if err != nil {
		return nil, err
	}
	proposedTrades, err := transitionToTarget(*currentPortfolio, *targetPortfolio, pm)
	if err != nil {
		return nil, err
	}

	return proposedTrades, nil
}
