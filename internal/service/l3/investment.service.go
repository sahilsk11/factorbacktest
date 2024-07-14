package l3_service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	l2_service "factorbacktest/internal/service/l2"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type InvestmentService interface {
	// GetAggregrateTargetPortfolio(date time.Time) (*domain.Portfolio, error)
	AddStrategyInvestment(userAccountID uuid.UUID, savedStrategyID uuid.UUID, amount int) error
}

type investmentServiceHandler struct {
	Db                           *sql.DB
	StrategyInvestmentRepository repository.StrategyInvestmentRepository
	HoldingsRepository           repository.StrategyInvestmentHoldingsRepository
	PriceRepository              repository.AdjustedPriceRepository
	UniverseRepository           repository.AssetUniverseRepository
	SavedStrategyRepository      repository.SavedStrategyRepository
	FactorExpressionService      l2_service.FactorExpressionService
	TickerRepository             repository.TickerRepository
}

func (h investmentServiceHandler) AddStrategyInvestment(ctx context.Context, userAccountID uuid.UUID, savedStrategyID uuid.UUID, amount int) error {
	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	date := time.Now().UTC()

	// TODO - put a timeout on this so we don't duplicate
	newStrategyInvestment, err := h.StrategyInvestmentRepository.Add(tx, model.StrategyInvestment{
		SavedStragyID: savedStrategyID,
		UserAccountID: userAccountID,
		AmountDollars: int32(amount),
		StartDate:     date,
	})
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	cashTicker, err := h.TickerRepository.GetCashTicker()
	if err != nil {
		return err
	}

	// create new holdings, with just cash
	_, err = h.HoldingsRepository.Add(tx, model.StrategyInvestmentHoldings{
		StrategyInvestmentID: newStrategyInvestment.StrategyInvestmentID,
		Date:                 date,
		Ticker:               cashTicker.TickerID,
		Quantity:             decimal.NewFromInt(int64(amount)),
	})
	if err != nil {
		return err
	}

	return nil
}

func (h investmentServiceHandler) getTargetPortfolio(ctx context.Context, strategyInvestmentID uuid.UUID, date time.Time, pm map[string]float64) (*domain.Portfolio, error) {
	investmentDetails, err := h.StrategyInvestmentRepository.Get(strategyInvestmentID)
	if err != nil {
		return nil, err
	}

	// TODO - get current value of holdings? or do we want to return as
	// percent allocations here
	// we should definitely get current holdings and figure out
	// value from that

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

	return computeTargetPortfolioResponse.TargetPortfolio, nil
}

func (h investmentServiceHandler) getAggregrateTargetPortfolio(ctx context.Context, date time.Time, pm map[string]float64) (*domain.Portfolio, error) {
	// get all active investments
	investments, err := h.StrategyInvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}

	aggregatePortfolio := domain.NewPortfolio()
	for _, i := range investments {
		strategyPortfolio, err := h.getTargetPortfolio(ctx, i.StrategyInvestmentID, date, pm)
		if err != nil {
			return nil, err
		}
		for symbol, position := range strategyPortfolio.Positions {
			if _, ok := aggregatePortfolio.Positions[symbol]; !ok {
				aggregatePortfolio.Positions[symbol] = &domain.Position{
					Symbol:   symbol,
					Quantity: 0,
				}
			}
			aggregatePortfolio.Positions[symbol].Quantity += position.Quantity
		}

		// definitely shouldn't be the case, but throw in cash
		// just in case. we don't do anything with cash, so we
		// should check this for errors
		if strategyPortfolio.Cash > 0 {
			return nil, fmt.Errorf("portfolio %s generated %f cash", i.StrategyInvestmentID, strategyPortfolio.Cash)
		}
		// aggregatePortfolio.Cash += strategyPortfolio.Cash
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

// the way this is set up, i think date would be like, the last
// trading day? it definitely needs to be a day we have prices
// for
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

	// TODO - verify that after all trades, the accounts
	// line up with what they should be

	return proposedTrades, nil
}