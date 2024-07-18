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

	rebalancerRun, err := h.RebalancerRunRepository.Add(nil, model.RebalancerRun{
		Date:              date,
		RebalancerRunType: model.RebalancerRunType_ManualInvestmentRebalance,
	})
	if err != nil {
		return err
	}

	// note - assumes everything is due for rebalance when run, i.e. rebalances everything
	investmentsToRebalance, err := h.InvestmentService.ListForRebalance()
	if err != nil {
		return err
	}

	proposedTrades := []*domain.ProposedTrade{}
	investmentTrades := []*model.InvestmentTrade{}
	// keyed by investment id
	mappedPortfolios := map[uuid.UUID]*domain.Portfolio{}

	for _, investment := range investmentsToRebalance {
		portfolio, trades, err := h.InvestmentService.GenerateRebalanceResults(
			ctx,
			investment,
			rebalancerRun.Date,
			pm,
			tickerIDMap,
		)
		if err != nil {
			return fmt.Errorf("failed to rebalance: failed to generate results for investment %s: %w", investment.InvestmentID.String(), err)
		}

		mappedPortfolios[investment.InvestmentID] = portfolio

		proposedTrades = append(proposedTrades, trades...)
		investmentTrades = append(investmentTrades,
			proposedTradesToInvestmentTradeModels(
				proposedTrades,
				investment.InvestmentID,
				rebalancerRun.RebalancerRunID,
			)...)
	}

	// until we have some fancier math for reconciling completed trades,
	// treat any failure here as fatal
	// TODO - improve reconciliation + partial trade completion
	_, err = h.aggregateAndExecuteTradeOrders(proposedTrades, rebalancerRun.RebalancerRunID)
	if err != nil {
		return fmt.Errorf("failure on executing orders for rebalance run %s: %v\n", rebalancerRun.RebalancerRunID.String(), err.Error())
	}

	// todo - link trade and investment orders

	// update positions to match whatever we just traded to get
	// them to
	// todo - when we improve reconciliation, handle partial failures
	// from this
	_, err = h.updateHoldings(mappedPortfolios, err, rebalancerRun)
	if err != nil {
		return err
	}

	return nil
}

func (h RebalancerHandler) updateHoldings(mappedPortfolios map[uuid.UUID]*domain.Portfolio, err error, rebalancerRun *model.RebalancerRun) (map[uuid.UUID][]error, error) {
	cashTicker, err := h.TickerRepository.GetCashTicker()
	if err != nil {
		return nil, err
	}

	errorsByInvestment := map[uuid.UUID][]error{}
	for investmentID, portfolio := range mappedPortfolios {
		for _, position := range portfolio.Positions {
			_, err = h.HoldingsRepository.Add(nil, model.InvestmentHoldings{
				InvestmentID:    investmentID,
				TickerID:        position.TickerID,
				Quantity:        position.ExactQuantity,
				RebalancerRunID: rebalancerRun.RebalancerRunID,
			})
			if err != nil {
				errorsByInvestment[investmentID] = append(errorsByInvestment[investmentID], err)
			}
		}

		if portfolio.Cash.GreaterThan(decimal.Zero) {
			_, err = h.HoldingsRepository.Add(nil, model.InvestmentHoldings{
				InvestmentID:    investmentID,
				TickerID:        cashTicker.TickerID,
				Quantity:        portfolio.Cash,
				RebalancerRunID: rebalancerRun.RebalancerRunID,
			})
			if err != nil {
				errorsByInvestment[investmentID] = append(errorsByInvestment[investmentID], err)
			}
		}
	}

	return errorsByInvestment, nil
}

func proposedTradesToInvestmentTradeModels(trades []*domain.ProposedTrade, investmentID, rebalancerRunID uuid.UUID) []*model.InvestmentTrade {
	out := []*model.InvestmentTrade{}
	for _, t := range trades {
		side := model.TradeOrderSide_Buy
		if t.ExactQuantity.LessThan(decimal.Zero) {
			side = model.TradeOrderSide_Sell
		}
		out = append(out, &model.InvestmentTrade{
			TickerID:        t.TickerID,
			Side:            side,
			InvestmentID:    investmentID,
			RebalancerRunID: rebalancerRunID,
			Quantity:        t.ExactQuantity,
			TradeOrderID:    nil, // need to update and set this
		})
	}
	return out
}

func (h RebalancerHandler) aggregateAndExecuteTradeOrders(proposedTrades []*domain.ProposedTrade, rebalancerRunID uuid.UUID) ([]model.TradeOrder, error) {
	aggregatedTrades := l3_service.AggregateAndFormatTrades(proposedTrades)
	internal.Pprint(aggregatedTrades)

	generatedOrders := []model.TradeOrder{}

	// TODO - we may have to optimize the way we submit trades
	// i don't think alpaca has block orders, but ideally we submit
	// this in one chunk, in an all-or-none fashion. if we don't have
	// that, we should research proper techniques. for example, we might
	// want to submit all sell orders first to avoid overexceeded cash
	// limit. for now, what should we do if one of these fails?

	// do a simple two pass to run all trades first

	for _, t := range aggregatedTrades {
		if t.ExactQuantity.LessThan(decimal.Zero) {
			order, err := h.TradingService.Sell(l1_service.SellInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				Quantity:        t.ExactQuantity,
				ExpectedPrice:   t.ExpectedPrice,
				RebalancerRunID: rebalancerRunID,
			})
			if err != nil {
				return nil, err
			}
			generatedOrders = append(generatedOrders, *order)
		}
	}

	for _, t := range aggregatedTrades {
		if t.ExactQuantity.GreaterThan(decimal.Zero) {
			order, err := h.TradingService.Buy(l1_service.BuyInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				Quantity:        t.ExactQuantity,
				ExpectedPrice:   t.ExpectedPrice,
				RebalancerRunID: rebalancerRunID,
			})
			if err != nil {
				return nil, err
			}
			generatedOrders = append(generatedOrders, *order)
		}
	}

	return generatedOrders, nil
}
