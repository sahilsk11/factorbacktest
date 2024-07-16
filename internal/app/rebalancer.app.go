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
	// todo - use latest trading date or something idk
	date := time.Now().UTC()

	// figure out most recent trading day from date
	// super wide window bc i haven't update prices on local
	// in a long time lol
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

	rebalancerRun, err := h.RebalancerRunRepository.Add(nil, model.RebalancerRun{
		Date: tradingDay,
	})
	if err != nil {
		return err
	}

	// note - assumes everything is due for rebalance when run, i.e. rebalances everything
	investments, err := h.InvestmentService.ListForRebalance()
	if err != nil {
		return err
	}

	allTrades := []*domain.ProposedTrade{}
	mappedPortfolios := map[uuid.UUID]*domain.Portfolio{}
	for _, investment := range investments {
		tx, err := h.Db.Begin()
		if err != nil {
			return err
		}

		defer tx.Rollback()

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
		allTrades = append(allTrades, trades...)

		mappedPortfolios[investment.InvestmentID] = portfolio

		for _, t := range trades {
			side := model.TradeOrderSide_Buy
			if t.ExactQuantity.LessThan(decimal.Zero) {
				side = model.TradeOrderSide_Sell
			}
			h.InvestmentTradeRepository.Add(tx, model.InvestmentTrade{
				TickerID:        t.TickerID,
				AmountInDollars: t.ExactQuantity.Mul(decimal.NewFromFloat(t.ExpectedPrice)),
				Side:            side,
				CreatedAt:       time.Time{},
				InvestmentID:    investment.InvestmentID,
				RebalancerRunID: rebalancerRun.RebalancerRunID,
			})
		}
		err = tx.Commit()
		if err != nil {
			return err
		}

	}

	proposedTrades := l3_service.AggregateAndFormatTrades(allTrades)

	// TODO - verify proposed trades
	// by checking mapped portfolios

	internal.Pprint(proposedTrades)

	for _, t := range proposedTrades {
		// TODO - optimize this amount math
		if t.ExactQuantity.GreaterThan(decimal.Zero) {
			err = h.TradingService.Buy(l1_service.BuyInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				AmountInDollars: t.ExactQuantity.Abs().Mul(decimal.NewFromFloat(t.ExpectedPrice)).Round(2),
			})
		} else {
			err = h.TradingService.Sell(l1_service.SellInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				AmountInDollars: t.ExactQuantity.Mul(decimal.NewFromFloat(t.ExpectedPrice)).Round(2),
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
				Ticker:          position.TickerID,
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

		if portfolio.Cash > 0 {
			_, err = h.HoldingsRepository.Add(nil, model.InvestmentHoldings{
				InvestmentID:    strategyInvestmentID,
				Ticker:          cashTicker.TickerID,
				Quantity:        decimal.NewFromFloat(portfolio.Cash),
				RebalancerRunID: rebalancerRun.RebalancerRunID,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
