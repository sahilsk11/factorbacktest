package l3_service

import (
	"context"
	"database/sql"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	l2_service "factorbacktest/internal/service/l2"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// InvestmentService is responsible for the logic around creating
// investments into strategies, and maintaing those investments stay
// on trajectory. It maintains the concept of the aggregate investment
// account and calculates how to dice it up among all investments
type InvestmentService interface {
	// ledgers a new request to invest in a strategy
	AddStrategyInvestment(ctx context.Context, userAccountID uuid.UUID, savedStrategyID uuid.UUID, amount int) error
	// creates a set of trades that should be executed to rebalance all strategy investments
	// note - assumes everything is due for rebalance when run, i.e. rebalances everything
	GenerateProposedTrades(ctx context.Context, date time.Time) ([]*domain.ProposedTrade, error)
	AddStrategyTrades()
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
	AlpacaRepository             repository.AlpacaRepository
}

func NewInvestmentService(
	db *sql.DB,
	strategyInvestmentRepository repository.StrategyInvestmentRepository,
	holdingsRepository repository.StrategyInvestmentHoldingsRepository,
	priceRepository repository.AdjustedPriceRepository,
	universeRepository repository.AssetUniverseRepository,
	savedStrategyRepository repository.SavedStrategyRepository,
	factorExpressionService l2_service.FactorExpressionService,
	tickerRepository repository.TickerRepository,
	alpacaRepository repository.AlpacaRepository,
) InvestmentService {
	return investmentServiceHandler{
		Db:                           db,
		StrategyInvestmentRepository: strategyInvestmentRepository,
		HoldingsRepository:           holdingsRepository,
		PriceRepository:              priceRepository,
		UniverseRepository:           universeRepository,
		SavedStrategyRepository:      savedStrategyRepository,
		FactorExpressionService:      factorExpressionService,
		TickerRepository:             tickerRepository,
		AlpacaRepository:             alpacaRepository,
	}
}

func (h investmentServiceHandler) AddStrategyInvestment(ctx context.Context, userAccountID uuid.UUID, savedStrategyID uuid.UUID, amount int) error {
	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	date := time.Now().UTC()

	// ensure we don't double record an entry
	prevInvestments, err := h.StrategyInvestmentRepository.List(repository.StrategyInvestmentListFilter{
		UserAccountIDs: []uuid.UUID{userAccountID},
	})
	if err != nil {
		return err
	}
	mostRecentTime := time.Time{}
	for _, p := range prevInvestments {
		if p.CreatedAt.After(mostRecentTime) {
			mostRecentTime = p.CreatedAt
		}
	}
	acceptableDelta := time.Minute
	if mostRecentTime.Add(acceptableDelta).After(date) {
		return fmt.Errorf("can only create 1 investment per minute")
	}

	newStrategyInvestment, err := h.StrategyInvestmentRepository.Add(tx, model.StrategyInvestment{
		SavedStragyID: savedStrategyID,
		UserAccountID: userAccountID,
		AmountDollars: int32(amount),
		StartDate:     date,
	})
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

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

type ComputeTargetPortfolioInput struct {
	PriceMap         map[string]float64
	Date             time.Time
	PortfolioValue   float64
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

		tickerID := uuid.Nil
		if in.TickerIDMap != nil {
			if id, ok := in.TickerIDMap[symbol]; ok {
				tickerID = id
			}
		}
		quantity := dollarsOfSymbol / price
		targetPortfolio.Positions[symbol] = &domain.Position{
			Symbol:   symbol,
			Quantity: quantity,
			TickerID: tickerID, // TODO - find out how to get ticker here
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

func (h investmentServiceHandler) getTargetPortfolio(ctx context.Context, strategyInvestmentID uuid.UUID, date time.Time, pm map[string]float64, tickerIDMap map[string]uuid.UUID) (*domain.Portfolio, error) {
	investmentDetails, err := h.StrategyInvestmentRepository.Get(strategyInvestmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy investment with id %s: %w", strategyInvestmentID.String(), err)
	}

	currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(strategyInvestmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get holdings from saved strategy %s: %w", strategyInvestmentID.String(), err)
	}

	// we need to get this in decimal and potentially use a different
	// set of prices? should we use live pricing from Alpaca?
	currentHoldingsValue, err := currentHoldings.TotalValue(pm)
	if err != nil {
		return nil, err
	}

	savedStrategyDetails, err := h.SavedStrategyRepository.Get(investmentDetails.SavedStragyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved strategy with id %s: %w", investmentDetails.SavedStragyID.String(), err)
	}

	universe, err := h.UniverseRepository.GetAssets(savedStrategyDetails.AssetUniverse)
	if err != nil {
		return nil, err
	}

	// assumes every asset is rebalancing today, which is not true. can simplify for now
	// but this should only pull the assets which need to rebalance today

	factorScoresOnLatestDay, err := h.FactorExpressionService.CalculateFactorScoresOnDay(ctx, date, universe, savedStrategyDetails.FactorExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}

	computeTargetPortfolioResponse, err := ComputeTargetPortfolio(ComputeTargetPortfolioInput{
		Date:             date,
		TargetNumTickers: int(savedStrategyDetails.NumAssets),
		FactorScores:     factorScoresOnLatestDay.SymbolScores,
		PortfolioValue:   currentHoldingsValue,
		PriceMap:         pm,
		TickerIDMap:      tickerIDMap,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to compute target portfolio: %w", err)
	}

	return computeTargetPortfolioResponse.TargetPortfolio, nil
}

func (h investmentServiceHandler) getAggregrateTargetPortfolio(ctx context.Context, date time.Time, pm map[string]float64, tickerIDMap map[string]uuid.UUID) (*domain.Portfolio, error) {
	// get all active investments
	investments, err := h.StrategyInvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investments: %w", err)
	}

	aggregatePortfolio := domain.NewPortfolio()
	for _, i := range investments {
		strategyPortfolio, err := h.getTargetPortfolio(ctx, i.StrategyInvestmentID, date, pm, tickerIDMap)
		if err != nil {
			return nil, fmt.Errorf("failed to get target portfolio for strategy investment %s: %w", i.StrategyInvestmentID.String(), err)
		}
		for symbol, position := range strategyPortfolio.Positions {
			if _, ok := aggregatePortfolio.Positions[symbol]; !ok {
				aggregatePortfolio.Positions[symbol] = &domain.Position{
					Symbol:        symbol,
					Quantity:      0,
					ExactQuantity: decimal.Zero,
					TickerID:      position.TickerID,
				}
			}
			currentQuantity := aggregatePortfolio.Positions[symbol].ExactQuantity
			newPositionQuantity := decimal.NewFromFloat(position.Quantity)
			aggregatePortfolio.Positions[symbol].ExactQuantity = currentQuantity.Add(newPositionQuantity)
		}

		// definitely shouldn't be the case, but we don't
		// do anything with cash, so we
		// should check this for errors
		if strategyPortfolio.Cash > 0 {
			return nil, fmt.Errorf("portfolio %s generated %f cash", i.StrategyInvestmentID, strategyPortfolio.Cash)
		}
	}

	return aggregatePortfolio, nil
}

func (h investmentServiceHandler) getCurrentAggregatePortfolio(tickerIDMap map[string]uuid.UUID) (*domain.Portfolio, error) {
	positions, err := h.AlpacaRepository.GetPositions()
	if err != nil {
		return nil, err
	}
	portfolio := domain.NewPortfolio()
	for _, p := range positions {
		tickerID, ok := tickerIDMap[p.Symbol]
		if !ok {
			return nil, fmt.Errorf("missing ticker id for %s from ticker id map", p.Symbol)
		}
		portfolio.Positions[p.Symbol] = &domain.Position{
			Symbol:        p.Symbol,
			ExactQuantity: p.Qty,
			Quantity:      p.AvgEntryPrice.InexactFloat64(),
			TickerID:      tickerID,
		}
	}

	return portfolio, nil
}

func transitionToTarget(
	currentPortfolio domain.Portfolio,
	targetPortfolio domain.Portfolio,
	priceMap map[string]float64,
) ([]*domain.ProposedTrade, error) {
	trades := []*domain.ProposedTrade{}
	prevPositions := currentPortfolio.Positions
	targetPositions := targetPortfolio.Positions

	for symbol, position := range targetPositions {
		diff := position.ExactQuantity
		prevPosition, ok := prevPositions[symbol]
		if ok {
			diff = position.ExactQuantity.Sub(prevPosition.ExactQuantity)
		}
		if diff.GreaterThan(decimal.Zero) {
			trades = append(trades, &domain.ProposedTrade{
				Symbol:        symbol,
				TickerID:      position.TickerID,
				ExactQuantity: diff,
				ExpectedPrice: priceMap[symbol],
			})
		}
	}
	for symbol, position := range prevPositions {
		if _, ok := targetPositions[symbol]; !ok {
			trades = append(trades, &domain.ProposedTrade{
				Symbol:        symbol,
				TickerID:      position.TickerID,
				ExactQuantity: position.ExactQuantity.Neg(),
				ExpectedPrice: priceMap[symbol],
			})
		}
	}

	// we could round all trades up to $1 but
	// if they have tons of little trades, that
	// could get expensive
	// round all buy orders to $1
	// TODO - i think we should use market value
	// and figure out whether to round up or down
	for _, t := range trades {
		if t.ExactQuantity.GreaterThan(decimal.Zero) && t.ExactQuantity.Mul(decimal.NewFromFloat(t.ExpectedPrice)).LessThan(decimal.NewFromInt(1)) {
			t.ExactQuantity = (decimal.NewFromInt(1).Div(decimal.NewFromFloat(t.ExpectedPrice)))
		}
	}

	return trades, nil
}

// the way this is set up, i think date would be like, the last
// trading day? it definitely needs to be a day we have prices
// for
func (h investmentServiceHandler) GenerateProposedTrades(ctx context.Context, date time.Time) ([]*domain.ProposedTrade, error) {
	assets, err := h.TickerRepository.List()
	if err != nil {
		return nil, err
	}
	symbols := []string{}
	tickerIDMap := map[string]uuid.UUID{}
	for _, s := range assets {
		symbols = append(symbols, s.Symbol)
		tickerIDMap[s.Symbol] = s.TickerID
	}

	// figure out most recent trading day from date
	// super wide window bc i haven't update prices on local
	// in a long time lol
	tradingDays, err := h.PriceRepository.ListTradingDays(
		date.AddDate(0, -6, 0),
		date,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get trading days")
	}

	if len(tradingDays) == 0 {
		return nil, fmt.Errorf("failed to get trading days")
	}
	tradingDay := tradingDays[len(tradingDays)-1]
	for i, td := range tradingDays[:len(tradingDays)-1] {
		if tradingDays[i+1].After(date) {
			tradingDay = td
			break
		}
	}

	pm, err := h.PriceRepository.GetManyOnDay(symbols, tradingDay)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices on day %v: %w", tradingDay, err)
	}

	// TODO - ensure both these functions have ticker IDs
	// populated
	currentPortfolio, err := h.getCurrentAggregatePortfolio(tickerIDMap)
	if err != nil {
		return nil, err
	}
	targetPortfolio, err := h.getAggregrateTargetPortfolio(ctx, tradingDay, pm, tickerIDMap)
	if err != nil {
		return nil, err
	}
	proposedTrades, err := transitionToTarget(*currentPortfolio, *targetPortfolio, pm)
	if err != nil {
		return nil, err
	}

	// TODO - verify that after all trades, the accounts
	// line up with what they should be
	// ensure we don't generate trades on cash

	return proposedTrades, nil
}

func (h investmentServiceHandler) AddStrategyTrades() {}
