package service

import (
	"fmt"

	"factorbacktest/internal/domain"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/shopspring/decimal"
)

var brokerQtyEpsilon = decimal.NewFromFloat(1e-6)

func brokerAvailableQty(positions []alpaca.Position, symbol string) decimal.Decimal {
	for _, position := range positions {
		if position.Symbol == symbol {
			return position.QtyAvailable
		}
	}
	return decimal.Zero
}

func validateBrokerSellCapacity(trades []*domain.ProposedTrade, positions []alpaca.Position) error {
	for _, trade := range trades {
		if trade.ExactQuantity.GreaterThanOrEqual(decimal.Zero) {
			continue
		}
		sellQty := trade.ExactQuantity.Abs()
		available := brokerAvailableQty(positions, trade.Symbol)
		if available.Add(brokerQtyEpsilon).LessThan(sellQty) {
			return fmt.Errorf(
				"insufficient %s (broker available %s) to sell %s; reconcile ledger before rebalance",
				trade.Symbol, available, sellQty,
			)
		}
	}
	return nil
}
