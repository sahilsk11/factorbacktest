package app

import (
	"context"
	"database/sql"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	l1_service "factorbacktest/internal/service/l1"
	l3_service "factorbacktest/internal/service/l3"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/internal/jet"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type RebalancerHandler struct {
	Db                        *sql.DB
	InvestmentService         l3_service.InvestmentService
	TradingService            l1_service.TradeService
	RebalancerRunRepository   repository.RebalancerRunRepository
	PriceRepository           repository.AdjustedPriceRepository
	TickerRepository          repository.TickerRepository
	InvestmentTradeRepository repository.InvestmentTradeRepository
	HoldingsRepository        repository.InvestmentHoldingsRepository
}

// Rebalance retrieves the latest proposed trades for the aggregate
// trading account, then calls the trading service to execute them
// Trade execution is non-blocking, so we will need to kick off an
// event that checks status after submission
//
// TODO - add some sort of reconciliation that figures out what
// everything got executed at.
// Also consider how we should store/link virtual trades with
// the actual executed trades
// TODO - add idempotency around runs and somehow invalidate any
// old runs
func (h RebalancerHandler) Rebalance(ctx context.Context) error {
	date := time.Now().UTC()

	// figure out most recent trading day from date
	tradingDays, err := h.PriceRepository.ListTradingDays(
		date.AddDate(0, -6, 0),
		date,
	)
	if err != nil {
		return fmt.Errorf("failed to get trading days")
	}
	if len(tradingDays) == 0 {
		return fmt.Errorf("failed to get trading days")
	}
	tradingDay := tradingDays[len(tradingDays)-1]
	for i, td := range tradingDays[:len(tradingDays)-1] {
		if tradingDays[i+1].After(date) {
			tradingDay = td
			break
		}
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
		symbols = append(symbols, s.Symbol)
		tickerIDMap[s.Symbol] = s.TickerID
	}
	pm, err := h.PriceRepository.GetManyOnDay(symbols, tradingDay)
	if err != nil {
		return fmt.Errorf("failed to get prices on day %v: %w", tradingDay, err)
	}

	// officially start the rebalancer run
	rebalancerRun, err := h.RebalancerRunRepository.Add(nil, model.RebalancerRun{
		Date:              date,
		RebalancerRunType: model.RebalancerRunType_ManualInvestmentRebalance,
	})
	if err != nil {
		return err
	}
	// if it exits for any unhandled reason, mark the
	// run as an error
	defer func() {
		_, err = h.RebalancerRunRepository.Update(nil, model.RebalancerRun{}, []jet.ColumnExpression{})
		if err != nil {
			fmt.Printf("failed to update rebalancer run to failed: %s: %v\n", rebalancerRun.RebalancerRunID, err.Error())
		}
	}()

	// note - assumes everything is due for rebalance when run, i.e. rebalances everything
	investmentsToRebalance, err := h.InvestmentService.ListForRebalance()
	if err != nil {
		return err
	}

	investmentTrades := []*domain.ProposedTrade{}
	// keyed by investment id
	mappedPortfolios := map[uuid.UUID]*domain.Portfolio{}
	for _, investment := range investmentsToRebalance {
		err = h.generateTrades(
			ctx,
			investment,
			rebalancerRun,
			pm,
			tickerIDMap,
			investmentTrades,
			mappedPortfolios, // what should we do with this
		)
		if err != nil {
			// return returnValue
		}

	}

	proposedTrades := l3_service.AggregateAndFormatTrades(investmentTrades)

	// TODO - verify proposed trades
	// by checking mapped portfolios

	internal.Pprint(proposedTrades)

	for _, t := range proposedTrades {
		if t.ExactQuantity.GreaterThan(decimal.Zero) {
			_, err = h.TradingService.Buy(l1_service.BuyInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				Quantity:        t.ExactQuantity,
				ExpectedPrice:   t.ExpectedPrice,
				RebalancerRunID: rebalancerRun.RebalancerRunID,
			})
		} else {
			_, err = h.TradingService.Sell(l1_service.SellInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				Quantity:        t.ExactQuantity,
				ExpectedPrice:   t.ExpectedPrice,
				RebalancerRunID: rebalancerRun.RebalancerRunID,
			})
		}
		if err != nil {
			return err
		}
	}

	for strategyInvestmentID, portfolio := range mappedPortfolios {
		for _, position := range portfolio.Positions {
			_, err = h.HoldingsRepository.Add(nil, model.InvestmentHoldings{
				InvestmentID:    strategyInvestmentID,
				TickerID:        position.TickerID,
				Quantity:        position.ExactQuantity,
				RebalancerRunID: rebalancerRun.RebalancerRunID,
			})
			if err != nil {
				return err
			}

		}
		cashTicker, err := h.TickerRepository.GetCashTicker()
		if err != nil {
			return err
		}

		if portfolio.Cash.GreaterThan(decimal.Zero) {
			_, err = h.HoldingsRepository.Add(nil, model.InvestmentHoldings{
				InvestmentID:    strategyInvestmentID,
				TickerID:        cashTicker.TickerID,
				Quantity:        portfolio.Cash,
				RebalancerRunID: rebalancerRun.RebalancerRunID,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (h RebalancerHandler) generateTrades(
	ctx context.Context,
	investment model.Investment,
	rebalancerRun *model.RebalancerRun,
	pm map[string]decimal.Decimal,
	tickerIDMap map[string]uuid.UUID,
	investmentTrades []*domain.ProposedTrade,
	mappedPortfolios map[uuid.UUID]*domain.Portfolio,
) error {
	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			fmt.Printf("failed to rollback transaction: %v\n", err.Error())
		}
	}()

	portfolio, trades, err := h.InvestmentService.GenerateRebalanceResults(
		ctx,
		investment,
		rebalancerRun.Date,
		pm,
		tickerIDMap,
	)
	if err != nil {
		return err
	}

	investmentTrades = append(investmentTrades, trades...)
	mappedPortfolios[investment.InvestmentID] = portfolio

	// create the "nominal" trades for the investment
	for _, t := range trades {
		side := model.TradeOrderSide_Buy
		if t.ExactQuantity.LessThan(decimal.Zero) {
			side = model.TradeOrderSide_Sell
		}
		_, err = h.InvestmentTradeRepository.Add(tx, model.InvestmentTrade{
			TickerID:        t.TickerID,
			Side:            side,
			InvestmentID:    investment.InvestmentID,
			RebalancerRunID: rebalancerRun.RebalancerRunID,
			Quantity:        t.ExactQuantity,
		})
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
