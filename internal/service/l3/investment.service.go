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
	ListForRebalance() ([]model.Investment, error)
	// ledgers a new request to invest in a strategy
	AddStrategyInvestment(ctx context.Context, userAccountID uuid.UUID, savedStrategyID uuid.UUID, amount int) error
	// assumes investment should be rebalanced and determines
	// holdings and necessary trades
	GenerateRebalanceResults(
		ctx context.Context,
		strategyInvestment model.Investment,
		date time.Time,
		pm map[string]float64, tickerIDMap map[string]uuid.UUID,
	) (*domain.Portfolio, []*domain.ProposedTrade, error)
}

func AggregateAndFormatTrades(trades []*domain.ProposedTrade) []*domain.ProposedTrade {
	// Map to hold aggregated trades by symbol
	aggregatedTrades := make(map[string]*domain.ProposedTrade)

	// Aggregate trades by symbol
	for _, trade := range trades {
		if existingTrade, exists := aggregatedTrades[trade.Symbol]; exists {
			// Update the existing trade quantity
			existingTrade.ExactQuantity = existingTrade.ExactQuantity.Add(trade.ExactQuantity)
			aggregatedTrades[trade.Symbol] = existingTrade
		} else {
			// Add a new trade to the map
			aggregatedTrades[trade.Symbol] = trade
		}
	}

	// Create a slice to hold the formatted trades
	var result []*domain.ProposedTrade
	for _, trade := range aggregatedTrades {
		if !trade.ExactQuantity.IsZero() {
			result = append(result, trade)
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

	return result
}

type investmentServiceHandler struct {
	Db                      *sql.DB
	InvestmentRepository    repository.InvestmentRepository
	HoldingsRepository      repository.InvestmentHoldingsRepository
	PriceRepository         repository.AdjustedPriceRepository
	UniverseRepository      repository.AssetUniverseRepository
	SavedStrategyRepository repository.SavedStrategyRepository
	FactorExpressionService l2_service.FactorExpressionService
	TickerRepository        repository.TickerRepository
	AlpacaRepository        repository.AlpacaRepository
	RebalancerRunRepository repository.RebalancerRunRepository
}

func NewInvestmentService(
	db *sql.DB,
	strategyInvestmentRepository repository.InvestmentRepository,
	holdingsRepository repository.InvestmentHoldingsRepository,
	priceRepository repository.AdjustedPriceRepository,
	universeRepository repository.AssetUniverseRepository,
	savedStrategyRepository repository.SavedStrategyRepository,
	factorExpressionService l2_service.FactorExpressionService,
	tickerRepository repository.TickerRepository,
	alpacaRepository repository.AlpacaRepository,
	rebalancerRunRepository repository.RebalancerRunRepository,
) InvestmentService {
	return investmentServiceHandler{
		Db:                      db,
		InvestmentRepository:    strategyInvestmentRepository,
		HoldingsRepository:      holdingsRepository,
		PriceRepository:         priceRepository,
		UniverseRepository:      universeRepository,
		SavedStrategyRepository: savedStrategyRepository,
		FactorExpressionService: factorExpressionService,
		TickerRepository:        tickerRepository,
		AlpacaRepository:        alpacaRepository,
		RebalancerRunRepository: rebalancerRunRepository,
	}
}

func (h investmentServiceHandler) ListForRebalance() ([]model.Investment, error) {
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}
	return investments, nil
}

func (h investmentServiceHandler) AddStrategyInvestment(ctx context.Context, userAccountID uuid.UUID, savedStrategyID uuid.UUID, amount int) error {
	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	date := time.Now().UTC()

	// ensure we don't double record an entry
	prevInvestments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{
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

	newStrategyInvestment, err := h.InvestmentRepository.Add(tx, model.Investment{
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

	// this is super weird but just call this a rebalance lol
	rebalancerRun, err := h.RebalancerRunRepository.Add(nil, model.RebalancerRun{
		Date:              date,
		RebalancerRunType: model.RebalancerRunType_Deposit,
	})
	if err != nil {
		return err
	}

	// create new holdings, with just cash
	_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
		InvestmentID:    newStrategyInvestment.InvestmentID,
		Ticker:          cashTicker.TickerID,
		Quantity:        decimal.NewFromInt(int64(amount)),
		RebalancerRunID: rebalancerRun.RebalancerRunID,
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
	if in.PortfolioValue < 0.001 {
		return nil, fmt.Errorf("cannot compute target portfolio with value %f", in.PortfolioValue)
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
			Symbol:        symbol,
			Quantity:      quantity,
			ExactQuantity: decimal.NewFromFloat(quantity),
			TickerID:      tickerID, // TODO - find out how to get ticker here
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

func (h investmentServiceHandler) getTargetPortfolio(
	ctx context.Context,
	strategyInvestment model.Investment,
	date time.Time,
	portfolioValue decimal.Decimal,
	pm map[string]float64,
	tickerIDMap map[string]uuid.UUID,
) (*domain.Portfolio, error) {
	// figure out what the strategy should hold if we rebalance
	// now
	savedStrategyDetails, err := h.SavedStrategyRepository.Get(strategyInvestment.SavedStragyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved strategy with id %s: %w", strategyInvestment.SavedStragyID.String(), err)
	}
	universe, err := h.UniverseRepository.GetAssets(savedStrategyDetails.AssetUniverse)
	if err != nil {
		return nil, err
	}
	factorScoresOnLatestDay, err := h.FactorExpressionService.CalculateFactorScoresOnDay(ctx, date, universe, savedStrategyDetails.FactorExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}
	computeTargetPortfolioResponse, err := ComputeTargetPortfolio(ComputeTargetPortfolioInput{
		Date:             date,
		TargetNumTickers: int(savedStrategyDetails.NumAssets),
		FactorScores:     factorScoresOnLatestDay.SymbolScores,
		PortfolioValue:   portfolioValue.InexactFloat64(),
		PriceMap:         pm,
		TickerIDMap:      tickerIDMap,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to compute target portfolio: %w", err)
	}

	return computeTargetPortfolioResponse.TargetPortfolio, nil
}

func (h investmentServiceHandler) GenerateRebalanceResults(
	ctx context.Context,
	strategyInvestment model.Investment,
	date time.Time,
	pm map[string]float64, tickerIDMap map[string]uuid.UUID,
) (*domain.Portfolio, []*domain.ProposedTrade, error) {
	// get current holdings to figure out what the
	// total investment is worth
	currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(strategyInvestment.InvestmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get holdings from investment id %s: %w", strategyInvestment.InvestmentID.String(), err)
	}

	// we need to get this in decimal and potentially use a different
	// set of prices? should we use live pricing from Alpaca?
	currentHoldingsValue, err := currentHoldings.TotalValue(pm)
	if err != nil {
		return nil, nil, err
	}

	targetPortfolio, err := h.getTargetPortfolio(
		ctx,
		strategyInvestment,
		date,
		decimal.NewFromFloat(currentHoldingsValue),
		pm,
		tickerIDMap,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get target portfolio: %w", err)
	}

	proposedTrades, err := transitionToTarget(*currentHoldings, *targetPortfolio, pm)
	if err != nil {
		return nil, nil, err
	}

	return targetPortfolio, proposedTrades, nil
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

	return trades, nil
}
