package l3_service

import (
	"context"
	"database/sql"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	l2_service "factorbacktest/internal/service/l2"
	"fmt"
	"strings"
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
		pm map[string]decimal.Decimal,
		tickerIDMap map[string]uuid.UUID,
	) (*domain.Portfolio, []*domain.ProposedTrade, error)
	Reconcile(ctx context.Context, investmentID uuid.UUID) error
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
	// also since price is stale, it could be just under $1
	// also we need to ledger these somewhere, as excess that
	// I own
	for _, t := range trades {
		if t.ExactQuantity.GreaterThan(decimal.Zero) && t.ExactQuantity.Mul(t.ExpectedPrice).LessThan(decimal.NewFromInt(1)) {
			t.ExactQuantity = (decimal.NewFromInt(2).Div(t.ExpectedPrice))
		}
	}

	return result
}

type investmentServiceHandler struct {
	Db                        *sql.DB
	InvestmentRepository      repository.InvestmentRepository
	HoldingsRepository        repository.InvestmentHoldingsRepository
	UniverseRepository        repository.AssetUniverseRepository
	SavedStrategyRepository   repository.SavedStrategyRepository
	FactorExpressionService   l2_service.FactorExpressionService
	TickerRepository          repository.TickerRepository
	RebalancerRunRepository   repository.RebalancerRunRepository
	HoldingsVersionRepository repository.InvestmentHoldingsVersionRepository
	InvestmentTradeRepository repository.InvestmentTradeRepository
	BacktestHandler           BacktestHandler
	AlpacaRepository          repository.AlpacaRepository
}

func NewInvestmentService(
	db *sql.DB,
	strategyInvestmentRepository repository.InvestmentRepository,
	holdingsRepository repository.InvestmentHoldingsRepository,
	universeRepository repository.AssetUniverseRepository,
	savedStrategyRepository repository.SavedStrategyRepository,
	factorExpressionService l2_service.FactorExpressionService,
	tickerRepository repository.TickerRepository,
	rebalancerRunRepository repository.RebalancerRunRepository,
	holdingsVersionRepository repository.InvestmentHoldingsVersionRepository,
	investmentTradeRepository repository.InvestmentTradeRepository,
	backtestHandler BacktestHandler,
) InvestmentService {
	return investmentServiceHandler{
		Db:                        db,
		InvestmentRepository:      strategyInvestmentRepository,
		HoldingsRepository:        holdingsRepository,
		UniverseRepository:        universeRepository,
		SavedStrategyRepository:   savedStrategyRepository,
		FactorExpressionService:   factorExpressionService,
		TickerRepository:          tickerRepository,
		RebalancerRunRepository:   rebalancerRunRepository,
		HoldingsVersionRepository: holdingsVersionRepository,
		InvestmentTradeRepository: investmentTradeRepository,
		BacktestHandler:           backtestHandler,
	}
}

func (h investmentServiceHandler) ListForRebalance() ([]model.Investment, error) {
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}

	investmentsToRebalance := []model.Investment{}
	for _, investment := range investments {
		tradeOrders, err := h.InvestmentTradeRepository.List(nil, repository.InvestmentTradeListFilter{
			InvestmentID: &investment.InvestmentID,
		})
		if err != nil {
			return nil, err
		}
		pendingInvestmentTradeID := uuid.Nil
		for _, t := range tradeOrders {
			if *t.Status == model.TradeOrderStatus_Pending {
				pendingInvestmentTradeID = *t.InvestmentTradeID
			}
		}

		if pendingInvestmentTradeID == uuid.Nil {
			investmentsToRebalance = append(investmentsToRebalance, investment)
		} else {
			logger.Info("skipping rebalancing investment id %s: has pending investment trade %s\n", investment.InvestmentID, pendingInvestmentTradeID)
		}
	}

	return investmentsToRebalance, nil
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
	acceptableDelta := 30 * time.Second
	if mostRecentTime.Add(acceptableDelta).After(date) {
		return fmt.Errorf("can only create 1 investment every 30s")
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
	version, err := h.HoldingsVersionRepository.Add(tx, model.InvestmentHoldingsVersion{
		InvestmentID: newStrategyInvestment.InvestmentID,
	})
	if err != nil {
		return err
	}

	// create new holdings, with just cash
	_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
		InvestmentID:                newStrategyInvestment.InvestmentID,
		TickerID:                    cashTicker.TickerID,
		Quantity:                    decimal.NewFromInt(int64(amount)),
		InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
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

func (h investmentServiceHandler) getTargetPortfolio(
	ctx context.Context,
	strategyInvestment model.Investment,
	date time.Time,
	portfolioValue decimal.Decimal,
	pm map[string]decimal.Decimal,
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
		PortfolioValue:   portfolioValue,
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
	pm map[string]decimal.Decimal, tickerIDMap map[string]uuid.UUID,
) (*domain.Portfolio, []*domain.ProposedTrade, error) {
	// get current holdings to figure out what the
	// total investment is worth
	currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(nil, strategyInvestment.InvestmentID)
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
		currentHoldingsValue,
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
	priceMap map[string]decimal.Decimal,
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

func (h investmentServiceHandler) Reconcile(ctx context.Context, investmentID uuid.UUID) error {
	// check deviance from backtested result
	// check that positions are > 0
	// are we planning to flag when trades executed at varying
	// prices
	// maybe check for trades in error states
	investment, err := h.InvestmentRepository.Get(investmentID)
	if err != nil {
		return err
	}
	strategy, err := h.SavedStrategyRepository.Get(investment.SavedStragyID)
	if err != nil {
		return err
	}

	targetWeights := map[string]float64{}

	interval := time.Hour * 24
	// if strings.EqualFold(strategy.RebalanceInterval, "weekly") {
	// 	interval *= 7
	// } else if strings.EqualFold(strategy.RebalanceInterval, "monthly") {
	// 	interval *= 30
	// } else if strings.EqualFold(strategy.RebalanceInterval, "yearly") {
	// 	interval *= 365
	// }

	// todo - figure out how to call the backtest
	backtestInput := BacktestInput{
		FactorExpression:  strategy.FactorExpression,
		BacktestStart:     investment.StartDate,
		BacktestEnd:       time.Now().UTC(),
		RebalanceInterval: interval,
		StartingCash:      float64(investment.AmountDollars),
		NumTickers:        int(strategy.NumAssets),
		AssetUniverse:     strategy.AssetUniverse,
	}

	backtestResponse, err := h.BacktestHandler.Backtest(ctx, backtestInput)
	if err != nil && strings.Contains(err.Error(), "no calculated trading days in given range") {
		backtestResponse = &BacktestResponse{
			Snapshots: map[string]BacktestSnapshot{},
		}
	} else if err != nil {
		return err
	}

	// since we're dealing with weights from the last rebalance,
	// we can't compare with quantity we're holding rn. best we
	// can do is take the current weights, and figure out what
	// their value was when we rebalanced maybe
	currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(nil, investmentID)
	if err != nil {
		return err
	}

	if currentHoldings.Cash.LessThan(decimal.NewFromInt(-1)) {
		logger.Warn("investment %s is holding %f cash", investmentID.String(), currentHoldings.Cash.InexactFloat64())
	}

	for _, position := range currentHoldings.Positions {
		if position.ExactQuantity.LessThan(decimal.Zero) {
			logger.Error(fmt.Errorf("investment %s has %f of %s", investmentID.String(), position.ExactQuantity.InexactFloat64(), position.Symbol))
		}
	}

	if len(backtestResponse.Snapshots) > 0 {
		latestResult := ""
		for k := range backtestResponse.Snapshots {
			if k > latestResult {
				latestResult = k
			}
		}
		latestSnapshot := backtestResponse.Snapshots[latestResult]
		for symbol, metrics := range latestSnapshot.AssetMetrics {
			targetWeights[symbol] = metrics.AssetWeight
		}

		// tbh just see if the assets line up for now, figure out weights
		// later maybe
		for k := range targetWeights {
			found := false
			for _, p := range currentHoldings.Positions {
				if p.Symbol == k {
					found = true
				}
			}
			if !found {
				logger.Error(fmt.Errorf("investment %s expected to hold %s, but is not", investmentID.String(), k))
			}
		}
		for _, p := range currentHoldings.Positions {
			found := false
			for k := range targetWeights {
				if p.Symbol == k {
					found = true
				}
			}
			if !found {
				logger.Error(fmt.Errorf("investment %s holding %s, but is not expected to", investmentID.String(), p.Symbol))
			}
		}
	}

	return nil
}

func (h investmentServiceHandler) ReconcileAggregatePortfolio() error {
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return err
	}
	totalHoldings := domain.NewPortfolio()
	for _, i := range investments {
		holdings, err := h.HoldingsRepository.GetLatestHoldings(nil, i.InvestmentID)
		if err != nil {
			return err
		}
		totalHoldings.SetCash(totalHoldings.Cash.Add(*holdings.Cash))
		for _, p := range holdings.Positions {
			if _, ok := totalHoldings.Positions[p.Symbol]; !ok {
				totalHoldings.Positions[p.Symbol] = &domain.Position{
					Symbol:        p.Symbol,
					Quantity:      0,
					ExactQuantity: decimal.Zero,
					TickerID:      p.TickerID,
				}
			}
			totalHoldings.Positions[p.Symbol].Quantity += p.Quantity
			totalHoldings.Positions[p.Symbol].ExactQuantity = totalHoldings.Positions[p.Symbol].ExactQuantity.Add(p.ExactQuantity)
		}
	}

	account, err := h.AlpacaRepository.GetAccount()
	if err != nil {
		return err
	}
	if account.Cash.LessThan(*totalHoldings.Cash) {
		logger.Error(fmt.Errorf("alpaca account holding insufficient cash: aggregate portfolio %f vs alpaca %f", totalHoldings.Cash.InexactFloat64(), account.Cash.InexactFloat64()))
	}

	excessHoldingThreshold := decimal.NewFromInt(2)

	actuallyHeld, err := h.AlpacaRepository.GetPositions()
	if err != nil {
		return err
	}
	for _, p := range totalHoldings.Positions {
		for _, a := range actuallyHeld {
			if a.Symbol == p.Symbol {
				if a.Qty.LessThan(p.ExactQuantity) {
					logger.Error(fmt.Errorf("alpaca account holding insufficient %s: aggregate portfolio %f vs alpaca %f", a.Symbol, p.ExactQuantity.InexactFloat64(), a.Qty.InexactFloat64()))
				} else if a.Qty.GreaterThan(p.ExactQuantity.Add(excessHoldingThreshold)) {
					logger.Warn("alpaca account holding excess %s: aggregate portfolio %f vs alpaca %f", a.Symbol, p.ExactQuantity.InexactFloat64(), a.Qty.InexactFloat64())
				}
			}
		}
	}

	return nil
}
