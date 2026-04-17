package data

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// QuoteProvider fetches latest quotes for one or more symbols.
//
// This is intentionally separate from historical pricing / cache-loading logic.
// It allows swapping quote integrations (Yahoo, Alpaca, hybrid, etc.) without
// changing callers.
type QuoteProvider interface {
	ProviderName() string
	GetLatestQuotes(ctx context.Context, symbols []string) (*QuoteResponse, error)
	GetDailyAdjCloses(ctx context.Context, symbol string, start, end time.Time) ([]DailyPricePoint, error)
}
type Quote struct {
	Symbol string
	Price  decimal.Decimal
	AsOf   time.Time
}
type DailyPricePoint struct {
	Date  time.Time
	Price decimal.Decimal
}

// QuoteResponse is designed to make partial success explicit.
// Callers can decide whether missing symbols are fatal.
type QuoteResponse struct {
	Provider string
	Quotes   map[string]Quote
	Missing  []string
}
