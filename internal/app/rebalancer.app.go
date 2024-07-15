package app

import (
	"context"
	l1_service "factorbacktest/internal/service/l1"
	l3_service "factorbacktest/internal/service/l3"
	"time"

	"github.com/shopspring/decimal"
)

type RebalancerHandler struct {
	InvestmentService l3_service.InvestmentService
	TradingService    l1_service.TradeService
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
func (h RebalancerHandler) Rebalance(ctx context.Context) error {
	date := time.Now().UTC()

	proposedTrades, err := h.InvestmentService.GenerateProposedTrades(ctx, date)
	if err != nil {
		return err
	}

	for _, t := range proposedTrades {
		if !t.ExactQuantity.LessThan(decimal.Zero) {
			err = h.TradingService.Buy(l1_service.BuyInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				AmountInDollars: t.ExactQuantity.Abs(),
			})
		} else {
			err = h.TradingService.Sell(l1_service.SellInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				AmountInDollars: t.ExactQuantity,
			})
		}
		if err != nil {
			return err
		}
	}

	return nil
}
