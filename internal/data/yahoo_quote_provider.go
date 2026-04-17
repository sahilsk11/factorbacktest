package data

import (
	"context"
	"fmt"
	"time"

	"factorbacktest/internal/logger"

	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
	"github.com/shopspring/decimal"
)

type YahooQuoteProvider struct {
	// LookbackDays is how far back we request daily bars in order to find a
	// recent close. Defaults to 4 if unset/<=0.
	LookbackDays int
}

func NewYahooQuoteProvider() *YahooQuoteProvider {
	return &YahooQuoteProvider{LookbackDays: 4}
}

func (p *YahooQuoteProvider) ProviderName() string {
	return "yahoo_finance"
}

// GetLatestQuotes uses Yahoo Finance daily bars and returns the most recent
// AdjClose as the "latest" price.
//
// Behavior matches the existing pattern in PriceService.GetLatestPrices:
// - per-symbol failures are logged and treated as missing
// - if all symbols fail, returns a non-nil error
func (p *YahooQuoteProvider) GetLatestQuotes(ctx context.Context, symbols []string) (*QuoteResponse, error) {
	log := logger.FromContext(ctx)

	out := &QuoteResponse{
		Provider: p.ProviderName(),
		Quotes:   map[string]Quote{},
		Missing:  []string{},
	}
	if len(symbols) == 0 {
		return out, nil
	}

	lookback := p.LookbackDays
	if lookback <= 0 {
		lookback = 4
	}

	var lastErr error

	for _, symbol := range symbols {
		approxStart := time.Now().AddDate(0, 0, -lookback)
		start := time.Date(approxStart.Year(), approxStart.Month(), approxStart.Day(), 0, 0, 0, 0, time.UTC)
		now := time.Now()

		params := &chart.Params{
			Start:    datetime.New(&start),
			End:      datetime.New(&now),
			Symbol:   symbol,
			Interval: datetime.OneDay,
		}

		iter := chart.Get(params)

		var (
			lastPrice decimal.Decimal
			lastTs    time.Time
			seen      bool
		)

		for iter.Next() {
			bar := iter.Bar()
			lastPrice = bar.AdjClose
			lastTs = time.Unix(int64(bar.Timestamp), 0).UTC()
			seen = true
		}

		if err := iter.Err(); err != nil {
			lastErr = fmt.Errorf("failed to get prices for %s: %w", symbol, err)
			log.Warnf("Failed to get prices for %s: %v", symbol, err)
			out.Missing = append(out.Missing, symbol)
			continue
		}
		if !seen || lastPrice.IsZero() {
			lastErr = fmt.Errorf("failed to get price for %s", symbol)
			log.Warnf("Failed to get price for %s: no prices returned", symbol)
			out.Missing = append(out.Missing, symbol)
			continue
		}

		out.Quotes[symbol] = Quote{
			Symbol: symbol,
			Price:  lastPrice,
			AsOf:   lastTs,
		}
	}

	if len(out.Quotes) == 0 && lastErr != nil {
		return nil, lastErr
	}

	return out, nil
}

func (p *YahooQuoteProvider) GetDailyAdjCloses(ctx context.Context, symbol string, start, end time.Time) ([]DailyPricePoint, error) {
	_ = logger.FromContext(ctx) // keep consistent behavior; caller likely already has a logger in ctx

	s := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	e := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	params := &chart.Params{
		Start:    datetime.New(&s),
		End:      datetime.New(&e),
		Symbol:   symbol,
		Interval: datetime.OneDay,
	}
	iter := chart.Get(params)

	out := []DailyPricePoint{}
	for iter.Next() {
		bar := iter.Bar()
		out = append(out, DailyPricePoint{
			Date:  time.Unix(int64(bar.Timestamp), 0).UTC(),
			Price: bar.AdjClose,
		})
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to get prices for %s: %w", symbol, err)
	}
	return out, nil
}

