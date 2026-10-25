package data

// Adjusted-price ingest assumptions:
//   - Yahoo Finance adjclose is the source of truth for adjusted prices,
//     including spin-offs and other special cases.
//   - Adjusted values are only comparable within one fetch, so any real
//     corporate action requires a full re-pull of that symbol.
//   - Ticker is the identifier; ticker reuse is not handled.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"

	"github.com/shopspring/decimal"
)

const (
	adjPriceLookback    = 14 * 24 * time.Hour
	adjPriceRelTol      = 0.0001 // ~0.01%
	adjPriceUpsertChunk = 2000
)

type UnexplainedChange struct {
	Symbol string          `json:"symbol"`
	Date   string          `json:"date"`
	Stored decimal.Decimal `json:"stored"`
	Yahoo  decimal.Decimal `json:"yahoo"`
}

type AdjustedPriceUpdateResult struct {
	Appended           int                 `json:"appended"`
	FullRepulled       int                 `json:"fullRepulled"`
	UnexplainedChanges []UnexplainedChange `json:"unexplainedChanges"`
	FailedSymbols      []string            `json:"failedSymbols"`
}

type AdjustedPriceService interface {
	UpdateAdjustedPrices(ctx context.Context, symbols []string, dryRun bool) (AdjustedPriceUpdateResult, error)
}

type adjCloseFunc func(context.Context, string, time.Time, time.Time) ([]DailyPricePoint, error)

type adjustedPriceService struct {
	prices repository.AdjustedPriceRepository
	cas    repository.CorporateActionRepository
	ingest CorporateActionService
	fetch  adjCloseFunc
}

func NewAdjustedPriceService(prices repository.AdjustedPriceRepository, cas repository.CorporateActionRepository, ingest CorporateActionService) AdjustedPriceService {
	return adjustedPriceService{prices: prices, cas: cas, ingest: ingest, fetch: fetchYahooAdjCloses}
}

func (s adjustedPriceService) UpdateAdjustedPrices(ctx context.Context, symbols []string, dryRun bool) (AdjustedPriceUpdateResult, error) {
	out := AdjustedPriceUpdateResult{UnexplainedChanges: []UnexplainedChange{}, FailedSymbols: []string{}}
	symbols = uniqueSymbols(symbols)
	if len(symbols) == 0 {
		return out, fmt.Errorf("no symbols supplied for adjusted price update")
	}
	now := time.Now().UTC()
	windowStart := now.Add(-adjPriceLookback)
	log := logger.FromContext(ctx)
	log.Infof("updating adjusted prices for %d symbols (dryRun=%v)", len(symbols), dryRun)

	if !dryRun {
		if s.ingest == nil {
			return out, fmt.Errorf("corporate action service is not configured")
		}
		if _, err := s.ingest.IngestCorporateActions(ctx, symbols, &windowStart, &now, ""); err != nil {
			return out, err
		}
	}
	latest, err := s.prices.LatestPriceDates(symbols)
	if err != nil {
		return out, err
	}

	n := priceUpdateWorkers
	if n > len(symbols) {
		n = len(symbols)
	}
	in := make(chan string)
	ch := make(chan adjPriceOutcome, len(symbols))
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			for symbol := range in {
				ch <- s.updateSymbol(ctx, symbol, latest[symbol], windowStart, now, dryRun)
			}
		}()
	}
	go func() {
		defer close(in)
		for _, symbol := range symbols {
			select {
			case <-ctx.Done():
				return
			case in <- symbol:
			}
		}
	}()
	go func() { wg.Wait(); close(ch) }()

	for got := range ch {
		switch {
		case got.failed != "":
			out.FailedSymbols = append(out.FailedSymbols, got.failed)
		case got.unexplained != nil:
			out.UnexplainedChanges = append(out.UnexplainedChanges, *got.unexplained)
		case got.fullRepulled:
			out.FullRepulled++
		case got.appended:
			out.Appended++
		}
	}
	sort.Strings(out.FailedSymbols)
	sort.Slice(out.UnexplainedChanges, func(i, j int) bool { return out.UnexplainedChanges[i].Symbol < out.UnexplainedChanges[j].Symbol })
	log.Infof("adjusted prices: appended=%d fullRepulled=%d unexplained=%d failed=%d",
		out.Appended, out.FullRepulled, len(out.UnexplainedChanges), len(out.FailedSymbols))
	return out, ctx.Err()
}

type adjPriceOutcome struct {
	appended, fullRepulled bool
	unexplained            *UnexplainedChange
	failed                 string
}

func (s adjustedPriceService) updateSymbol(ctx context.Context, symbol string, latest, windowStart, now time.Time, dryRun bool) (out adjPriceOutcome) {
	log := logger.FromContext(ctx)
	fail := func(err error) adjPriceOutcome {
		log.Warnf("failed to update adjusted prices for %s: %s", symbol, err)
		out.failed = symbol
		return out
	}
	if latest.IsZero() {
		if err := s.fullRepull(ctx, symbol, now, dryRun); err != nil {
			return fail(err)
		}
		out.fullRepulled = true
		return out
	}

	yahoo, err := s.fetch(ctx, symbol, windowStart, now)
	if err != nil || len(yahoo) == 0 {
		if err == nil {
			err = fmt.Errorf("empty adjusted close series")
		}
		return fail(err)
	}
	storedRows, err := s.prices.List([]string{symbol}, windowStart, now)
	if err != nil {
		return fail(err)
	}
	stored := map[string]decimal.Decimal{}
	for _, row := range storedRows {
		stored[utcDate(row.Date).Format(time.DateOnly)] = row.Price
	}

	if m := firstAdjMismatch(symbol, stored, yahoo); m != nil {
		actions, err := s.cas.ListBySymbolRange(symbol, windowStart, now)
		if err != nil {
			return fail(err)
		}
		if len(actions) == 0 {
			log.Errorf("unexplained adjusted price change for %s on %s: stored=%s yahoo=%s", symbol, m.Date, m.Stored, m.Yahoo)
			out.unexplained = m
			return out
		}
		if err := s.fullRepull(ctx, symbol, now, dryRun); err != nil {
			return fail(err)
		}
		out.fullRepulled = true
		return out
	}

	newDays := newAdjDays(stored, yahoo)
	if len(newDays) == 0 {
		return out
	}
	if err := s.write(dryRun, adjCloseModels(symbol, newDays, now)); err != nil {
		return fail(err)
	}
	out.appended = true
	return out
}

func (s adjustedPriceService) fullRepull(ctx context.Context, symbol string, now time.Time, dryRun bool) error {
	yahoo, err := s.fetch(ctx, symbol, time.Time{}, now) // start zero => Yahoo range=max
	if err != nil || len(yahoo) == 0 {
		if err == nil {
			err = fmt.Errorf("empty adjusted close series")
		}
		return err
	}
	return s.write(dryRun, adjCloseModels(symbol, yahoo, now))
}

func (s adjustedPriceService) write(dryRun bool, rows []model.AdjustedPrice) error {
	if dryRun || len(rows) == 0 {
		return nil
	}
	for i := 0; i < len(rows); i += adjPriceUpsertChunk {
		j := i + adjPriceUpsertChunk
		if j > len(rows) {
			j = len(rows)
		}
		if err := s.prices.Add(nil, rows[i:j]); err != nil {
			return err
		}
	}
	return nil
}

func adjPricesMatch(stored, yahoo decimal.Decimal) bool {
	if stored.Equal(yahoo) {
		return true
	}
	if stored.IsZero() {
		return yahoo.IsZero()
	}
	return yahoo.Sub(stored).Abs().Div(stored.Abs()).LessThanOrEqual(decimal.NewFromFloat(adjPriceRelTol))
}

func firstAdjMismatch(symbol string, stored map[string]decimal.Decimal, yahoo []DailyPricePoint) *UnexplainedChange {
	for _, pt := range yahoo {
		day := utcDate(pt.Date).Format(time.DateOnly)
		have, ok := stored[day]
		if ok && !adjPricesMatch(have, pt.Price) {
			return &UnexplainedChange{Symbol: symbol, Date: day, Stored: have, Yahoo: pt.Price}
		}
	}
	return nil
}

func newAdjDays(stored map[string]decimal.Decimal, yahoo []DailyPricePoint) []DailyPricePoint {
	var out []DailyPricePoint
	for _, pt := range yahoo {
		if _, ok := stored[utcDate(pt.Date).Format(time.DateOnly)]; !ok {
			out = append(out, pt)
		}
	}
	return out
}

func adjCloseModels(symbol string, points []DailyPricePoint, createdAt time.Time) []model.AdjustedPrice {
	out := make([]model.AdjustedPrice, 0, len(points))
	for _, pt := range points {
		out = append(out, model.AdjustedPrice{Symbol: symbol, Date: utcDate(pt.Date), Price: pt.Price, CreatedAt: createdAt})
	}
	return out
}

func fetchYahooAdjCloses(ctx context.Context, symbol string, start, end time.Time) ([]DailyPricePoint, error) {
	symbol = yahooChartSymbol(symbol)
	u := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=max&interval=1d", url.PathEscape(symbol))
	if !start.IsZero() {
		p2 := end.Unix()
		if end.Equal(utcDate(end)) {
			p2 = end.Add(24 * time.Hour).Unix()
		}
		u = fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d",
			url.PathEscape(symbol), utcDate(start).Unix(), p2)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	body, status, err := httpGetRetry(ctx, yahooHTTP, req)
	if err != nil {
		return nil, fmt.Errorf("yahoo adjclose for %s: %w", symbol, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("yahoo adjclose for %s: status %d", symbol, status)
	}
	var parsed struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Adjclose []struct {
						Adjclose []*float64 `json:"adjclose"`
					} `json:"adjclose"`
				} `json:"indicators"`
			} `json:"result"`
			Error *struct {
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("yahoo adjclose for %s: %w", symbol, err)
	}
	if parsed.Chart.Error != nil && parsed.Chart.Error.Description != "" {
		return nil, fmt.Errorf("yahoo adjclose for %s: %s", symbol, parsed.Chart.Error.Description)
	}
	if len(parsed.Chart.Result) == 0 || len(parsed.Chart.Result[0].Indicators.Adjclose) == 0 {
		return nil, fmt.Errorf("yahoo adjclose for %s: empty chart", symbol)
	}
	res := parsed.Chart.Result[0]
	closes := res.Indicators.Adjclose[0].Adjclose
	out := make([]DailyPricePoint, 0, len(res.Timestamp))
	for i, ts := range res.Timestamp {
		if i >= len(closes) || closes[i] == nil {
			continue
		}
		out = append(out, DailyPricePoint{Date: time.Unix(ts, 0).UTC(), Price: decimal.NewFromFloat(*closes[i])})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("yahoo adjclose for %s: empty chart", symbol)
	}
	return out, nil
}
