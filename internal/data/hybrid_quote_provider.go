package data

import (
	"context"
	"fmt"
	"time"

	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"

	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
	"github.com/shopspring/decimal"
)

type HybridQuoteProvider struct {
	// LookbackDays is how far back we request daily bars in order to find a
	// recent close. Defaults to 4 if unset/<=0.
	LookbackDays     int
	AlpacaRepository repository.AlpacaRepository
}

func NewHybridQuoteProvider(alpacaRepository repository.AlpacaRepository) *HybridQuoteProvider {
	return &HybridQuoteProvider{LookbackDays: 4, AlpacaRepository: alpacaRepository}
}

func (p *HybridQuoteProvider) ProviderName() string {
	return "hybrid_quote_provider"
}

// GetLatestQuotes uses Yahoo Finance daily bars and returns the most recent
// AdjClose as the "latest" price.
//
// Behavior matches the existing pattern in PriceService.GetLatestPrices:
// - per-symbol failures are logged and treated as missing
// - if all symbols fail, returns a non-nil error
func (p *HybridQuoteProvider) GetLatestQuotes(ctx context.Context, symbols []string) (*QuoteResponse, error) {
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

	for _, symbol := range symbols {
		quote, err := p.getQuoteFromYahoo(symbol, lookback)
		if err != nil || quote == nil {
			quote, err = p.getQuoteFromAlpaca(ctx, symbol)
		}

		if err != nil {
			log.Warnf("[%s] failed to get quote for %s: %v", p.ProviderName(), symbol, err)
			out.Missing = append(out.Missing, symbol)
			continue
		}

		out.Quotes[symbol] = *quote
	}

	if len(out.Quotes) == 0 && len(out.Missing) == len(symbols) {
		return nil, fmt.Errorf("[%s] failed to get quotes for all symbols", p.ProviderName())
	}

	return out, nil
}

func (p *HybridQuoteProvider) getQuoteFromYahoo(symbol string, lookback int) (*Quote, error) {
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
		return nil, fmt.Errorf("yahoo chart error for symbol %s: %w", symbol, err)
	}

	if !seen || lastPrice.IsZero() {
		return nil, fmt.Errorf("no valid prices returned in yahoo for symbol %s", symbol)
	}

	return &Quote{
		Symbol: symbol,
		Price:  lastPrice,
		AsOf:   lastTs,
	}, nil
}

func (p *HybridQuoteProvider) getQuoteFromAlpaca(ctx context.Context, symbol string) (*Quote, error) {
	prices, err := p.AlpacaRepository.GetLatestPrices(ctx, []string{symbol})
	if err != nil {
		return nil, fmt.Errorf("alpaca error for symbol %s: %w", symbol, err)
	}

	price, ok := prices[symbol]
	if !ok || price.IsZero() {
		return nil, fmt.Errorf("no price returned in alpaca for symbol %s", symbol)
	}

	return &Quote{
		Symbol: symbol,
		Price:  price,
		AsOf:   time.Now().UTC(),
	}, nil
}

func (p *HybridQuoteProvider) GetDailyAdjCloses(ctx context.Context, symbol string, start, end time.Time) ([]DailyPricePoint, error) {
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
		return nil, fmt.Errorf("[yahoo_quote_provider] failed to get prices for %s: %w", symbol, err)
	}
	return out, nil
}
