package app

import (
	"context"
	"factorbacktest/internal"
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
// TODO - add idempotency around runs and somehow invalidate any
// old runs
func (h RebalancerHandler) Rebalance(ctx context.Context) error {
	date := time.Now().UTC()

	proposedTrades, err := h.InvestmentService.GenerateProposedTrades(ctx, date)
	if err != nil {
		return err
	}

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

	return nil
}
