package service

import (
	"testing"

	"factorbacktest/internal/domain"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestValidateBrokerSellCapacity(t *testing.T) {
	trades := []*domain.ProposedTrade{{
		Symbol: "INTC", ExactQuantity: decimal.RequireFromString("-1.19749006"),
	}}
	positions := []alpaca.Position{{
		Symbol: "INTC", QtyAvailable: decimal.RequireFromString("0.764168329"),
	}}
	require.Error(t, validateBrokerSellCapacity(trades, positions))
}

func TestValidateBrokerSellCapacityPassesWhenSufficient(t *testing.T) {
	trades := []*domain.ProposedTrade{{
		Symbol: "INTC", ExactQuantity: decimal.RequireFromString("-0.5"),
	}}
	positions := []alpaca.Position{{
		Symbol: "INTC", QtyAvailable: decimal.RequireFromString("0.764168329"),
	}}
	require.NoError(t, validateBrokerSellCapacity(trades, positions))
}
