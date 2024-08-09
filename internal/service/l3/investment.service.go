package l3_service

import (
	"context"
	"database/sql"
	"encoding/json"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	l1_service "factorbacktest/internal/service/l1"
	l2_service "factorbacktest/internal/service/l2"
	"factorbacktest/internal/util"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// InvestmentService is responsible for the logic around creating
// investments into strategies, and maintaing those investments stay
// on trajectory. It maintains the concept of the aggregate investment
// account and calculates how to dice it up among all investments
type InvestmentService interface {
	Add(ctx context.Context, userAccountID uuid.UUID, strategyID uuid.UUID, amount int) error
	GetStats(investmentID uuid.UUID) (*GetStatsResponse, error)
	Reconcile(ctx context.Context) error
	Rebalance(ctx context.Context) error
}

type investmentServiceHandler struct {
	Db                            *sql.DB
	InvestmentRepository          repository.InvestmentRepository
	HoldingsRepository            repository.InvestmentHoldingsRepository
	UniverseRepository            repository.AssetUniverseRepository
	StrategyRepository            repository.StrategyRepository
	FactorExpressionService       l2_service.FactorExpressionService
	TickerRepository              repository.TickerRepository
	RebalancerRunRepository       repository.RebalancerRunRepository
	HoldingsVersionRepository     repository.InvestmentHoldingsVersionRepository
	InvestmentTradeRepository     repository.InvestmentTradeRepository
	BacktestHandler               BacktestHandler // lol wtf, if we're importing from the same service package, the layered approach makes no sense
	AlpacaRepository              repository.AlpacaRepository
	TradingService                l1_service.TradeService
	InvestmentRebalanceRepository repository.InvestmentRebalanceRepository
	PriceRepository               repository.AdjustedPriceRepository
	RebalancePriceRepository      repository.RebalancePriceRepository
}

func NewInvestmentService(
	db *sql.DB,
	strategyInvestmentRepository repository.InvestmentRepository,
	holdingsRepository repository.InvestmentHoldingsRepository,
	universeRepository repository.AssetUniverseRepository,
	strategyRepository repository.StrategyRepository,
	factorExpressionService l2_service.FactorExpressionService,
	tickerRepository repository.TickerRepository,
	rebalancerRunRepository repository.RebalancerRunRepository,
	holdingsVersionRepository repository.InvestmentHoldingsVersionRepository,
	investmentTradeRepository repository.InvestmentTradeRepository,
	backtestHandler BacktestHandler,
	alpacaRepository repository.AlpacaRepository,
	tradeService l1_service.TradeService,
	investmentRebalanceRepository repository.InvestmentRebalanceRepository,
	priceRepository repository.AdjustedPriceRepository,
	rebalancePriceRepository repository.RebalancePriceRepository,
) InvestmentService {
	return investmentServiceHandler{
		Db:                            db,
		InvestmentRepository:          strategyInvestmentRepository,
		HoldingsRepository:            holdingsRepository,
		UniverseRepository:            universeRepository,
		StrategyRepository:            strategyRepository,
		FactorExpressionService:       factorExpressionService,
		TickerRepository:              tickerRepository,
		RebalancerRunRepository:       rebalancerRunRepository,
		HoldingsVersionRepository:     holdingsVersionRepository,
		InvestmentTradeRepository:     investmentTradeRepository,
		BacktestHandler:               backtestHandler,
		AlpacaRepository:              alpacaRepository,
		TradingService:                tradeService,
		InvestmentRebalanceRepository: investmentRebalanceRepository,
		PriceRepository:               priceRepository,
		RebalancePriceRepository:      rebalancePriceRepository,
	}
}

func (h investmentServiceHandler) Add(ctx context.Context, userAccountID uuid.UUID, strategyID uuid.UUID, amount int) error {
	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	date := time.Now().UTC()
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
		StrategyID:    strategyID,
		UserAccountID: userAccountID,
		AmountDollars: int32(amount),
		StartDate:     date,
		PausedAt:      util.TimePointer(time.Now().UTC()),
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

type GetStatsResponse struct {
	Holdings              []domain.Position
	OriginalAmount        int32
	StartDate             time.Time
	PercentReturnFraction decimal.Decimal
	CurrentValue          decimal.Decimal
	CompletedTrades       []domain.FilledTrade
	Strategy              model.Strategy
}

func (h investmentServiceHandler) GetStats(investmentID uuid.UUID) (*GetStatsResponse, error) {
	investment, err := h.InvestmentRepository.Get(investmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get investment %s: %w", investmentID.String(), err)
	}
	strategy, err := h.StrategyRepository.Get(investment.StrategyID)
	if err != nil {
		return nil, err
	}

	currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(nil, investmentID)
	if err != nil {
		return nil, err
	}
	heldSymbols := currentHoldings.HeldSymbols()

	latestPrices := map[string]decimal.Decimal{}
	open, err := h.AlpacaRepository.IsMarketOpen()
	if err != nil {
		return nil, fmt.Errorf("failed to get market open: %w", err)
	}
	if open {
		latestPrices, err = h.AlpacaRepository.GetLatestPrices(heldSymbols)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest prices from Alpaca: %w", err)
		}
	} else {
		prices, err := h.PriceRepository.LatestPrices(heldSymbols)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest prices: %w", err)
		}
		for _, p := range prices {
			latestPrices[p.Symbol] = p.Price
		}
	}

	totalValue, err := currentHoldings.TotalValue(latestPrices)
	if err != nil {
		return nil, fmt.Errorf("failed to compute total value: %w", err)
	}

	startValue := decimal.NewFromInt32(investment.AmountDollars)
	returnFraction := (totalValue.Sub(startValue)).Div(startValue)

	positions := []domain.Position{}
	for _, p := range currentHoldings.Positions {
		p.Value = util.DecimalPointer(p.ExactQuantity.Mul(latestPrices[p.Symbol]))
		positions = append(positions, *p)
	}

	allTradesWithStatus, err := h.InvestmentTradeRepository.List(nil, repository.InvestmentTradeListFilter{
		InvestmentID: &investmentID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get trades: %w", err)
	}

	completedTrades := []domain.FilledTrade{}
	for _, t := range allTradesWithStatus {
		if t.Status != nil && *t.Status == model.TradeOrderStatus_Completed {
			completedTrades = append(completedTrades, domain.FilledTrade{
				Symbol:    *t.Symbol,
				TickerID:  *t.TickerID,
				Quantity:  *t.Quantity,
				FillPrice: *t.FilledPrice,
				FilledAt:  *t.FilledAt,
			})
		}
	}

	return &GetStatsResponse{
		Holdings:              positions,
		StartDate:             investment.StartDate,
		CurrentValue:          totalValue,
		PercentReturnFraction: returnFraction,
		CompletedTrades:       completedTrades,
		Strategy:              *strategy,
		OriginalAmount:        investment.AmountDollars,
	}, nil
}

// listForRebalance retrieves all investments that should be
// rebalanced right now
// todo - fix so that it looks at rebalance interval
func (h investmentServiceHandler) listForRebalance(ctx context.Context) ([]model.Investment, error) {
	log := logger.FromContext(ctx)
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}

	investmentsToRebalance := []model.Investment{}
	for _, investment := range investments {
		log.Infof("rebalancing %s", investment.InvestmentID.String())
		tradeOrders, err := h.InvestmentTradeRepository.List(nil, repository.InvestmentTradeListFilter{
			InvestmentID: &investment.InvestmentID,
		})
		if err != nil {
			return nil, err
		}
		pendingInvestmentTradeID := uuid.Nil
		for _, t := range tradeOrders {
			if t.Status != nil && *t.Status == model.TradeOrderStatus_Pending {
				pendingInvestmentTradeID = *t.InvestmentTradeID
			}
		}

		if pendingInvestmentTradeID == uuid.Nil {
			investmentsToRebalance = append(investmentsToRebalance, investment)
		} else {
			log.Infof("skipping rebalancing investment id %s: has pending investment trade %s\n", investment.InvestmentID, pendingInvestmentTradeID)
		}
	}

	return investmentsToRebalance, nil
}

func (h investmentServiceHandler) getTargetPortfolio(
	ctx context.Context,
	investment model.Investment,
	date time.Time,
	portfolioValue decimal.Decimal,
	pm map[string]decimal.Decimal,
	tickerIDMap map[string]uuid.UUID,
) (*ComputeTargetPortfolioResponse, error) {
	// figure out what the strategy should hold if we rebalance
	// now
	strategy, err := h.StrategyRepository.Get(investment.StrategyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy with id %s: %w", investment.StrategyID.String(), err)
	}
	universe, err := h.UniverseRepository.GetAssets(strategy.AssetUniverse)
	if err != nil {
		return nil, err
	}
	factorScoresOnLatestDay, err := h.FactorExpressionService.CalculateFactorScoresOnDay(ctx, date, universe, strategy.FactorExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}
	computeTargetPortfolioResponse, err := ComputeTargetPortfolio(ComputeTargetPortfolioInput{
		Date:             date,
		TargetNumTickers: int(strategy.NumAssets),
		FactorScores:     factorScoresOnLatestDay.SymbolScores,
		PortfolioValue:   portfolioValue,
		PriceMap:         pm,
		TickerIDMap:      tickerIDMap,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to compute target portfolio: %w", err)
	}

	return computeTargetPortfolioResponse, nil
}

type rebalanceInvestmentResponse struct {
	ProposedTrades           []*domain.ProposedTrade
	InsertedInvestmentTrades []model.InvestmentTrade
}

// rebalanceInvestment creates the InvestmentRebalance entry
// and figures out the trades to rebalance
func (h investmentServiceHandler) rebalanceInvestment(
	ctx context.Context,
	tx *sql.Tx,
	investment model.Investment,
	rebalancerRun model.RebalancerRun,
	pm map[string]decimal.Decimal,
	tickerIDMap map[string]uuid.UUID,
) (*rebalanceInvestmentResponse, error) {
	log := logger.FromContext(ctx).With(
		"investmentID", investment.InvestmentID.String(),
	)
	ctx = context.WithValue(ctx, logger.ContextKey, log)

	// should add this to the latest view and remove this call
	versionID, err := h.HoldingsVersionRepository.GetLatestVersionID(investment.InvestmentID)
	if err != nil {
		return nil, err
	}

	initialPortfolio, err := h.HoldingsRepository.GetLatestHoldings(nil, investment.InvestmentID)
	if err != nil {
		return nil, err
	}

	currentHoldingsValue, err := initialPortfolio.TotalValue(pm)
	if err != nil {
		return nil, err
	}

	if currentHoldingsValue.Equal(decimal.Zero) {
		return nil, fmt.Errorf("holdings have no value")
	}

	fmt.Println(rebalancerRun.Date)

	computeTargetPortfolioResponse, err := h.getTargetPortfolio(
		ctx,
		investment,
		rebalancerRun.Date,
		currentHoldingsValue,
		pm,
		tickerIDMap,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get target portfolio: %w", err)
	}

	proposedTrades, err := transitionToTarget(ctx, *initialPortfolio, *computeTargetPortfolioResponse.TargetPortfolio, pm)
	if err != nil {
		return nil, err
	}

	startingPortfolioJson, err := portfolioToJson(initialPortfolio)
	if err != nil {
		return nil, err
	}

	targetPortfolioJson, err := targetPortfolioToJson(*computeTargetPortfolioResponse)
	if err != nil {
		return nil, err
	}

	investmentRebalance, err := h.InvestmentRebalanceRepository.Add(tx, model.InvestmentRebalance{
		RebalancerRunID:           rebalancerRun.RebalancerRunID,
		InvestmentID:              investment.InvestmentID,
		State:                     model.RebalancerRunState_Pending,
		StartingHoldingsVersionID: *versionID,
		StartingPortfolio:         string(startingPortfolioJson),
		TargetPortfolio:           string(targetPortfolioJson),
	})
	if err != nil {
		return nil, err
	}

	investmentTrades := proposedTradesToInvestmentTradeModels(
		proposedTrades,
		investmentRebalance.InvestmentRebalanceID,
	)

	insertedInvestmentTrades, err := h.InvestmentTradeRepository.AddMany(tx, investmentTrades)
	if err != nil {
		return nil, err
	}

	// these two should be enough to only include what we
	// just inserted, but it may not be
	insertedTradesStatus, err := h.InvestmentTradeRepository.List(tx, repository.InvestmentTradeListFilter{
		InvestmentID:    &investment.InvestmentID,
		RebalancerRunID: &rebalancerRun.RebalancerRunID,
	})
	if err != nil {
		return nil, err
	}

	for i, t := range insertedTradesStatus {
		insertedTradesStatus[i].FilledPrice = util.DecimalPointer(pm[*t.Symbol])
	}

	// add a lil recon
	// newPortfolio := l1_service.AddTradesToPortfolio(insertedTradesStatus, initialPortfolio)

	// epsilon := decimal.NewFromFloat(0.01)
	// if match, reason := comparePortfolios(newPortfolio, computeTargetPortfolioResponse.TargetPortfolio, epsilon); !match {
	// 	return nil, fmt.Errorf("portfolios don't match: %s", reason)
	// }

	return &rebalanceInvestmentResponse{
		ProposedTrades:           proposedTrades,
		InsertedInvestmentTrades: insertedInvestmentTrades,
	}, nil
}

// transition to target attempts to produce
// the target portfolio given the current
// holdings. however it applies a $0.01 min order
// size rule, so it may deviate
func transitionToTarget(
	ctx context.Context,
	currentPortfolio domain.Portfolio,
	targetPortfolio domain.Portfolio,
	priceMap map[string]decimal.Decimal,
) ([]*domain.ProposedTrade, error) {
	log := logger.FromContext(ctx)
	trades := []*domain.ProposedTrade{}
	prevPositions := currentPortfolio.Positions
	targetPositions := targetPortfolio.Positions

	for symbol, position := range targetPositions {
		diff := position.ExactQuantity
		prevPosition, ok := prevPositions[symbol]
		if ok {
			diff = position.ExactQuantity.Sub(prevPosition.ExactQuantity)
		}
		if !diff.Equal(decimal.Zero) {
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

	// we know that any buys should trigger sells (even
	// if we're selling cash), and vice-versa
	// so if any sells are below $0.01, take it out from buys
	filteredTrades := filterLowVolumeTrades(trades, decimal.NewFromFloat(0.01))

	if len(filteredTrades) != len(trades) {
		log.Warnf("dropped %d trades due to low volume", len(trades)-len(filteredTrades))
	}

	return trades, nil
}

func filterLowVolumeTrades(trades []*domain.ProposedTrade, amountThreshold decimal.Decimal) []*domain.ProposedTrade {
	buys := []*domain.ProposedTrade{}
	sells := []*domain.ProposedTrade{}
	for _, t := range trades {
		if t.ExactQuantity.GreaterThan(decimal.Zero) {
			buys = append(buys, t)
		} else {
			sells = append(sells, t)
		}
	}

	sortBuys := func() {
		sort.Slice(buys, func(i, j int) bool {
			return buys[i].ExpectedAmount().LessThan(buys[j].ExpectedAmount())
		})
	}
	sortSells := func() {
		sort.Slice(sells, func(i, j int) bool {
			return sells[i].ExpectedAmount().LessThan(sells[i].ExpectedAmount())
		})
	}
	sortBuys()
	sortSells()

	for _, t := range sells {
		if t.ExpectedAmount().LessThan(amountThreshold) {
			// goal is to wipe this out
			remaining := t.ExpectedAmount()
			i := 0
			for i < len(buys) && remaining.GreaterThan(decimal.Zero) {
				buyAmount := buys[i].ExpectedAmount()
				remainingBuyAmount := buyAmount.Sub(remaining)
				remaining = decimal.Zero
				if remainingBuyAmount.LessThan(decimal.Zero) {
					remaining = remainingBuyAmount.Abs()
					remainingBuyAmount = decimal.Zero
				}
				buys[i].ExactQuantity = remainingBuyAmount.Div(buys[i].ExpectedPrice)
				i++
			}
			sortBuys()
			t.ExactQuantity = remaining.Div(t.ExpectedPrice)
		}
	}
	sortSells()

	// i think we're fine not handling buys because we'll
	// just cut out whatever buys were insufficient, which
	// should turn into cash
	// TODO - think about how to handle
	// also do we need to ledger this

	out := []*domain.ProposedTrade{}
	for _, t := range buys {
		if !t.ExpectedAmount().LessThan(amountThreshold) {
			out = append(out, t)
		}
	}
	for _, t := range sells {
		if !t.ExpectedAmount().LessThan(amountThreshold) {
			out = append(out, t)
		}
	}

	return out
}

func (h investmentServiceHandler) Rebalance(ctx context.Context) error {
	log := logger.FromContext(ctx)

	// since this function is executing trades, we
	// need this to be today. only in testing can
	// we allow another function to take over
	date := time.Now()
	// sry i'm too lazy to add this somewhere else
	var err error
	if strings.EqualFold(os.Getenv("ALPHA_ENV"), "test") {
		fmt.Println("overriding date")
		date, err = time.Parse(time.DateOnly, "2020-12-31")
		if err != nil {
			return err
		}
	} else {
		fmt.Println("here", os.Getenv("ALPHA_ENV"))
	}

	// get all assets
	// we could probably clean this up
	// by getting assets on the fly idk
	assets, err := h.TickerRepository.List()
	if err != nil {
		return err
	}
	symbols := []string{}
	tickerIDMap := map[string]uuid.UUID{}
	for _, s := range assets {
		if s.Symbol != ":CASH" {
			symbols = append(symbols, s.Symbol)
			tickerIDMap[s.Symbol] = s.TickerID
		}
	}
	pm, err := h.AlpacaRepository.GetLatestPrices(symbols)
	if err != nil {
		return fmt.Errorf("failed to get latest prices: %w", err)
	}

	// note - assumes everything is due for rebalance when run, i.e. rebalances everything
	investmentsToRebalance, err := h.listForRebalance(ctx)
	if err != nil {
		return err
	}

	log.Infof("found %d investments to rebalance", len(investmentsToRebalance))

	rebalancerRun, err := h.RebalancerRunRepository.Add(nil, model.RebalancerRun{
		Date:                    date,
		RebalancerRunType:       model.RebalancerRunType_ManualInvestmentRebalance,
		RebalancerRunState:      model.RebalancerRunState_Error,
		NumInvestmentsAttempted: int32(len(investmentsToRebalance)),
	})
	if err != nil {
		return err
	}

	// before generating trades, let's store the price map so we can
	// re-construct the entire rebalancer run later
	err = h.RebalancePriceRepository.AddMany(pm, tickerIDMap, rebalancerRun.RebalancerRunID)
	if err != nil {
		return fmt.Errorf("failed to add rebalance prices: %w", err)
	}

	proposedTrades := []*domain.ProposedTrade{}
	investmentTrades := []model.InvestmentTrade{}

	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// todo - break this up so one investment doesn't cause all rebalances
	// to fail
	for _, investment := range investmentsToRebalance {
		result, err := h.rebalanceInvestment(
			ctx,
			tx,
			investment,
			*rebalancerRun,
			pm,
			tickerIDMap,
		)
		if err != nil {
			return fmt.Errorf("failed to generate results for investment %s: %w", investment.InvestmentID.String(), err)
		}

		proposedTrades = append(proposedTrades, result.ProposedTrades...)
		investmentTrades = append(investmentTrades, result.InsertedInvestmentTrades...)
	}

	log.Infof("generated %d investment trades", len(investmentTrades))

	rebalancerRun.RebalancerRunState = model.RebalancerRunState_Pending
	if len(investmentsToRebalance) == 0 {
		rebalancerRun.RebalancerRunState = model.RebalancerRunState_Completed
		rebalancerRun.Notes = util.StringPointer("no investments to rebalance")
	} else if len(investmentTrades) == 0 {
		rebalancerRun.RebalancerRunState = model.RebalancerRunState_Completed
		rebalancerRun.Notes = util.StringPointer("no investment trades generated")
	}

	_, err = h.RebalancerRunRepository.Update(tx, rebalancerRun, []postgres.Column{
		table.RebalancerRun.RebalancerRunState,
		table.RebalancerRun.Notes,
	})
	if err != nil {
		return err
	}

	if len(investmentTrades) == 0 || len(investmentsToRebalance) == 0 {
		return nil
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// until we have some fancier math for reconciling completed trades,
	// treat any failure here as fatal
	// TODO - improve reconciliation + partial trade completion
	executedTrades, tradeExecutionErr := h.TradingService.ExecuteBlock(ctx, proposedTrades, rebalancerRun.RebalancerRunID)

	updateInvesmtentTradeErrors := []error{}
	for _, tradeOrder := range executedTrades {
		for _, investmentTrade := range investmentTrades {
			if tradeOrder.TickerID == investmentTrade.TickerID {
				investmentTrade.TradeOrderID = &tradeOrder.TradeOrderID
				_, err = h.InvestmentTradeRepository.Update(
					nil,
					investmentTrade,
					[]postgres.Column{
						table.InvestmentTrade.TradeOrderID,
					},
				)
				if err != nil {
					updateInvesmtentTradeErrors = append(updateInvesmtentTradeErrors, err)
				}
			}
		}
	}

	if len(updateInvesmtentTradeErrors) > 0 && tradeExecutionErr != nil {
		return fmt.Errorf("failed to execute trades AND update %d investment trade status. trade err: %w | first update err: %w", len(updateInvesmtentTradeErrors), tradeExecutionErr, updateInvesmtentTradeErrors[0])
	}
	if tradeExecutionErr != nil {
		return fmt.Errorf("failure on executing orders for rebalance run %s: %w\n", rebalancerRun.RebalancerRunID.String(), tradeExecutionErr)
	}
	if len(updateInvesmtentTradeErrors) > 0 {
		return fmt.Errorf("failed to update %d investment trade status. first update err: %w", len(updateInvesmtentTradeErrors), updateInvesmtentTradeErrors[0])
	}

	if len(executedTrades) == 0 {
		rebalancerRun.RebalancerRunState = model.RebalancerRunState_Completed
		rebalancerRun.Notes = util.StringPointer("no trade orders generated - investment trades must have cancelled out")
		_, err = h.RebalancerRunRepository.Update(nil, rebalancerRun, []postgres.Column{
			table.RebalancerRun.RebalancerRunState,
			table.RebalancerRun.Notes,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func proposedTradesToInvestmentTradeModels(trades []*domain.ProposedTrade, investmentRebalanceID uuid.UUID) []*model.InvestmentTrade {
	out := []*model.InvestmentTrade{}
	for _, t := range trades {
		side := model.TradeOrderSide_Buy
		if t.ExactQuantity.LessThan(decimal.Zero) {
			side = model.TradeOrderSide_Sell
		}
		out = append(out, &model.InvestmentTrade{
			TickerID:              t.TickerID,
			Side:                  side,
			Quantity:              t.ExactQuantity.Abs(),
			TradeOrderID:          nil, // need to update and set this
			InvestmentRebalanceID: investmentRebalanceID,
			ExpectedPrice:         t.ExpectedPrice,
		})
	}
	return out
}

func portfolioToJson(p *domain.Portfolio) ([]byte, error) {
	type position struct {
		Quantity float64 `json:"quantity"`
		Value    float64 `json:"value,omitempty"`
	}
	type portfolio struct {
		Cash      float64            `json:"cash"`
		Positions map[string]float64 `json:"positions"`
	}

	out := portfolio{
		Positions: map[string]float64{},
	}
	out.Cash = p.Cash.InexactFloat64()
	for symbol, pos := range p.Positions {
		out.Positions[symbol] = pos.ExactQuantity.InexactFloat64()
	}

	bytes, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func targetPortfolioToJson(c ComputeTargetPortfolioResponse) ([]byte, error) {
	type details struct {
		Quantity    float64 `json:"quantity"`
		Weight      float64 `json:"weight"`
		FactorScore float64 `json:"factorScore"`
	}

	type out struct {
		Cash      float64            `json:"cash"`
		Positions map[string]details `json:"positions"`
	}

	o := &out{
		Cash:      c.TargetPortfolio.Cash.InexactFloat64(),
		Positions: map[string]details{},
	}

	for symbol, position := range c.TargetPortfolio.Positions {
		o.Positions[symbol] = details{
			Quantity:    position.ExactQuantity.InexactFloat64(),
			Weight:      c.AssetWeights[symbol],
			FactorScore: c.FactorScores[symbol],
		}
	}

	bytes, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

type ReconErr struct {
	Message      string
	InvestmentID *uuid.UUID
}

func (h investmentServiceHandler) reconcileInvestment(ctx context.Context, investmentID uuid.UUID) ([]ReconErr, error) {
	lg := logger.FromContext(ctx).With(
		"investmentID", investmentID.String(),
	)
	ctx = context.WithValue(ctx, logger.ContextKey, lg)

	lg.Info("running investment reconciliation")
	// check deviance from backtested result
	// check that positions are > 0
	// are we planning to flag when trades executed at varying
	// prices
	// maybe check for trades in error states
	investment, err := h.InvestmentRepository.Get(investmentID)
	if err != nil {
		return nil, err
	}
	strategy, err := h.StrategyRepository.Get(investment.StrategyID)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	// since we're dealing with weights from the last rebalance,
	// we can't compare with quantity we're holding rn. best we
	// can do is take the current weights, and figure out what
	// their value was when we rebalanced maybe
	currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(nil, investmentID)
	if err != nil {
		return nil, err
	}

	reconErrors := []ReconErr{}

	if currentHoldings.Cash.LessThan(decimal.NewFromInt(-1)) {
		reconErrors = append(reconErrors, ReconErr{
			Message:      fmt.Sprintf("investment %s is holding %f cash", investmentID.String(), currentHoldings.Cash.InexactFloat64()),
			InvestmentID: &investmentID,
		})
	}

	for _, position := range currentHoldings.Positions {
		if position.ExactQuantity.LessThan(decimal.Zero) {
			reconErrors = append(reconErrors, ReconErr{
				InvestmentID: &investmentID,
				Message:      fmt.Sprintf("investment %s has %f of %s", investmentID.String(), position.ExactQuantity.InexactFloat64(), position.Symbol),
			})
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
					reconErrors = append(reconErrors, ReconErr{
						InvestmentID: &investmentID,
						Message:      fmt.Sprintf("investment %s expected to hold %s, but is not", investmentID.String(), k),
					})
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
					reconErrors = append(reconErrors, ReconErr{
						InvestmentID: &investmentID,
						Message:      fmt.Sprintf("investment %s holding %s, but is not expected to", investmentID.String(), p.Symbol),
					})
				}
			}
		}
	}

	return reconErrors, nil
}

func (h investmentServiceHandler) reconcileAggregatePortfolio() ([]ReconErr, error) {
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}
	totalHoldings := domain.NewPortfolio()
	for _, i := range investments {
		holdings, err := h.HoldingsRepository.GetLatestHoldings(nil, i.InvestmentID)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	reconErrors := []ReconErr{}

	if account.Cash.LessThan(*totalHoldings.Cash) {
		reconErrors = append(reconErrors, ReconErr{
			Message: fmt.Sprintf("alpaca account holding insufficient cash: aggregate portfolio %f vs alpaca %f", totalHoldings.Cash.InexactFloat64(), account.Cash.InexactFloat64()),
		})
	}

	excessHoldingThreshold := decimal.NewFromInt(2)

	actuallyHeld, err := h.AlpacaRepository.GetPositions()
	if err != nil {
		return nil, err
	}
	epsilonZero := decimal.NewFromFloat(1e-6)
	for _, p := range totalHoldings.Positions {
		for _, a := range actuallyHeld {
			if a.Symbol == p.Symbol {
				if a.Qty.LessThan(p.ExactQuantity.Sub(epsilonZero)) {
					reconErrors = append(reconErrors, ReconErr{
						Message: fmt.Sprintf("alpaca account holding insufficient %s: aggregate portfolio %f vs alpaca %f (%f)", a.Symbol, p.ExactQuantity.InexactFloat64(), a.Qty.InexactFloat64(), a.Qty.Sub(p.ExactQuantity).InexactFloat64()),
					})
				} else if a.Qty.GreaterThan(p.ExactQuantity.Add(excessHoldingThreshold)) {
					reconErrors = append(reconErrors, ReconErr{
						Message: fmt.Sprintf("alpaca account holding excess %s: aggregate portfolio %f vs alpaca %f", a.Symbol, p.ExactQuantity.InexactFloat64(), a.Qty.InexactFloat64()),
					})
				}
			}
		}
	}

	return reconErrors, nil
}

func (h investmentServiceHandler) Reconcile(ctx context.Context) error {
	log := logger.FromContext(ctx)
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return err
	}
	for _, i := range investments {
		reconErrors, err := h.reconcileInvestment(ctx, i.InvestmentID)
		if err != nil {
			return err
		}
		for _, err := range reconErrors {
			log.Warnf("recon err on investment %s: %s", err.InvestmentID.String(), err.Message)
		}

		reconErr, err := h.reconcileTrades(i.InvestmentID)
		if err != nil {
			return err
		} else if reconErr != nil {
			log.Warnf("trade recon err on investment %s: %s", reconErr.InvestmentID.String(), reconErr.Message)
		}
	}
	reconErrors, err := h.reconcileAggregatePortfolio()
	if err != nil {
		return err
	}
	for _, err := range reconErrors {
		log.Warnf("recon err on aggregate portfolio %s: %s", err.InvestmentID.String(), err.Message)
	}

	return nil
}

func (h investmentServiceHandler) reconcileTrades(investmentID uuid.UUID) (*ReconErr, error) {
	initialVersionID, err := h.HoldingsVersionRepository.GetEarliestVersionID(investmentID)
	if err != nil {
		return nil, err
	}

	initialHoldings, err := h.HoldingsRepository.Get(*initialVersionID)
	if err != nil {
		return nil, err
	}

	status := model.TradeOrderStatus_Completed
	trades, err := h.InvestmentTradeRepository.List(nil, repository.InvestmentTradeListFilter{
		InvestmentID: &investmentID,
		Status:       &status,
	})
	if err != nil {
		return nil, err
	}

	newPortfolio := l1_service.AddTradesToPortfolio(trades, initialHoldings)

	currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(nil, investmentID)
	if err != nil {
		return nil, err
	}

	epsilon := decimal.NewFromFloat(0.00001)
	if portfoliosMatch, reason := comparePortfolios(newPortfolio, currentHoldings, epsilon); !portfoliosMatch {
		return &ReconErr{
			InvestmentID: &investmentID,
			Message:      reason,
		}, nil
	}

	return nil, nil
}

func comparePortfolios(portfolioAfterTrades, targetPortfolio *domain.Portfolio, epsilon decimal.Decimal) (bool, string) {
	// Check if cash values are within epsilon
	if !portfolioAfterTrades.Cash.Sub(*targetPortfolio.Cash).Abs().LessThan(epsilon) {
		return false, fmt.Sprintf("Cash values differ: portfolioAfterTrades = %s, targetPortfolio = %s", portfolioAfterTrades.Cash.String(), targetPortfolio.Cash.String())
	}

	// Check if the same symbols are held in both portfolios
	if len(portfolioAfterTrades.Positions) != len(targetPortfolio.Positions) {
		return false, "The number of positions differs between the two portfolios"
	}

	for symbol, pos1 := range portfolioAfterTrades.Positions {
		pos2, exists := targetPortfolio.Positions[symbol]
		if !exists {
			return false, fmt.Sprintf("Symbol %s is missing in the second portfolio", symbol)
		}

		// Check if ExactQuantity values are within epsilon
		if !pos1.ExactQuantity.Sub(pos2.ExactQuantity).Abs().LessThan(epsilon) {
			return false, fmt.Sprintf("ExactQuantities for symbol %s differ: portfolioAfterTrades = %s, targetPortfolio = %s", symbol, pos1.ExactQuantity.String(), pos2.ExactQuantity.String())
		}
	}

	return true, ""
}
