package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/shopspring/decimal"
)

const (
	RawPriceSourceAlpaca = "alpaca"
	RawPriceSourceYahoo  = "yahoo"
)

type RawDailyBar struct {
	Date   time.Time
	Close  decimal.Decimal
	Volume *decimal.Decimal
}

type RawBarProvider interface {
	Source() string
	GetRawDailyBars(ctx context.Context, symbol string, start, end time.Time) ([]RawDailyBar, error)
}

func NewRawBarProvider(apiKey, apiSecret, endpoint string) RawBarProvider {
	if apiKey == "" || apiSecret == "" || apiKey == "test" || endpoint == "test" {
		return NewYahooRawBarProvider()
	}
	return NewAlpacaRawBarProvider(apiKey, apiSecret)
}

type alpacaRawBarProvider struct {
	client *marketdata.Client
}

func NewAlpacaRawBarProvider(apiKey, apiSecret string) RawBarProvider {
	return alpacaRawBarProvider{
		client: marketdata.NewClient(marketdata.ClientOpts{
			APIKey:    apiKey,
			APISecret: apiSecret,
		}),
	}
}

func (alpacaRawBarProvider) Source() string { return RawPriceSourceAlpaca }

func (p alpacaRawBarProvider) GetRawDailyBars(_ context.Context, symbol string, start, end time.Time) ([]RawDailyBar, error) {
	bars, err := p.client.GetBars(symbol, marketdata.GetBarsRequest{
		TimeFrame:  marketdata.OneDay,
		Adjustment: marketdata.Raw,
		Start:      start,
		End:        end,
	})
	if err != nil {
		return nil, fmt.Errorf("alpaca raw bars for %s: %w", symbol, err)
	}
	out := make([]RawDailyBar, 0, len(bars))
	for _, bar := range bars {
		volume := decimal.NewFromInt(int64(bar.Volume))
		out = append(out, RawDailyBar{
			Date:   bar.Timestamp.UTC(),
			Close:  decimal.NewFromFloat(bar.Close),
			Volume: &volume,
		})
	}
	return out, nil
}

type yahooSplit struct {
	Date        time.Time
	Numerator   decimal.Decimal
	Denominator decimal.Decimal
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close  []*float64 `json:"close"`
					Volume []*int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
			Events *struct {
				Splits map[string]struct {
					Date        int64   `json:"date"`
					Numerator   float64 `json:"numerator"`
					Denominator float64 `json:"denominator"`
				} `json:"splits"`
			} `json:"events"`
		} `json:"result"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

var yahooHTTP = &http.Client{Timeout: 30 * time.Second}

type yahooRawBarProvider struct{}

func NewYahooRawBarProvider() RawBarProvider {
	return yahooRawBarProvider{}
}

func (yahooRawBarProvider) Source() string { return RawPriceSourceYahoo }

func (yahooRawBarProvider) GetRawDailyBars(ctx context.Context, symbol string, start, end time.Time) ([]RawDailyBar, error) {
	p1 := utcDate(start).Unix()
	p2 := end.Unix()
	if end.Equal(utcDate(end)) {
		p2 = end.Add(24 * time.Hour).Unix()
	}
	u := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d&events=%s",
		url.PathEscape(symbol), p1, p2, url.QueryEscape("split"),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := yahooHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yahoo raw bars for %s: %w", symbol, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
	if err != nil {
		return nil, fmt.Errorf("yahoo raw bars for %s: %w", symbol, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo raw bars for %s: status %d", symbol, resp.StatusCode)
	}

	parsed := yahooChartResponse{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("yahoo raw bars for %s: %w", symbol, err)
	}
	if parsed.Chart.Error != nil && parsed.Chart.Error.Description != "" {
		return nil, fmt.Errorf("yahoo raw bars for %s: %s", symbol, parsed.Chart.Error.Description)
	}
	if len(parsed.Chart.Result) == 0 || len(parsed.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("yahoo raw bars for %s: empty chart", symbol)
	}

	result := parsed.Chart.Result[0]
	quote := result.Indicators.Quote[0]
	splits := []yahooSplit{}
	if result.Events != nil {
		for _, s := range result.Events.Splits {
			if s.Numerator > 0 && s.Denominator > 0 {
				splits = append(splits, yahooSplit{
					Date:        utcDate(time.Unix(s.Date, 0).UTC()),
					Numerator:   decimal.NewFromFloat(s.Numerator),
					Denominator: decimal.NewFromFloat(s.Denominator),
				})
			}
		}
	}

	out := make([]RawDailyBar, 0, len(result.Timestamp))
	for i, ts := range result.Timestamp {
		if i >= len(quote.Close) || quote.Close[i] == nil {
			continue
		}
		bar := RawDailyBar{
			Date:  time.Unix(ts, 0).UTC(),
			Close: decimal.NewFromFloat(*quote.Close[i]),
		}
		if i < len(quote.Volume) && quote.Volume[i] != nil {
			volume := decimal.NewFromInt(*quote.Volume[i])
			bar.Volume = &volume
		}
		out = append(out, bar)
	}
	return applyYahooSplitFactors(out, splits), nil
}

func applyYahooSplitFactors(bars []RawDailyBar, splits []yahooSplit) []RawDailyBar {
	if len(splits) == 0 {
		return bars
	}
	out := make([]RawDailyBar, len(bars))
	for i, bar := range bars {
		factor := decimal.NewFromInt(1)
		barDate := utcDate(bar.Date)
		for _, split := range splits {
			if barDate.Before(split.Date) {
				factor = factor.Mul(split.Numerator).Div(split.Denominator)
			}
		}
		bar.Close = bar.Close.Mul(factor)
		if bar.Volume != nil && !factor.IsZero() {
			volume := bar.Volume.Div(factor)
			bar.Volume = &volume
		}
		out[i] = bar
	}
	return out
}

func utcDate(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func MapRawBarsToRows(symbol, source string, bars []RawDailyBar, ingestedAt time.Time) []model.RawPrice {
	out := make([]model.RawPrice, 0, len(bars))
	for _, bar := range bars {
		if bar.Close.LessThanOrEqual(decimal.Zero) {
			continue
		}
		out = append(out, model.RawPrice{
			Symbol:     symbol,
			Date:       utcDate(bar.Date),
			Close:      bar.Close,
			Volume:     bar.Volume,
			Source:     source,
			IngestedAt: ingestedAt,
		})
	}
	return out
}

type RawPriceIngestResult struct {
	UpdatedSymbols []string
	FailedSymbols  []string
	Source         string
}

type RawPriceService interface {
	IngestRawPrices(ctx context.Context, symbols []string, start *time.Time) (RawPriceIngestResult, error)
}

type rawPriceService struct {
	repo     repository.RawPriceRepository
	provider RawBarProvider
}

func NewRawPriceService(repo repository.RawPriceRepository, provider RawBarProvider) RawPriceService {
	if provider == nil {
		provider = NewYahooRawBarProvider()
	}
	return rawPriceService{repo: repo, provider: provider}
}

func (s rawPriceService) IngestRawPrices(ctx context.Context, symbols []string, start *time.Time) (RawPriceIngestResult, error) {
	log := logger.FromContext(ctx)
	result := RawPriceIngestResult{Source: s.provider.Source()}

	symbols = uniqueSymbols(symbols)
	if len(symbols) == 0 {
		return result, fmt.Errorf("no symbols supplied for raw price update")
	}
	if start == nil {
		from := time.Now().UTC().Add(-priceRefreshOverlap)
		start = &from
	}

	log.Infof("ingesting raw prices for %d symbols via %s", len(symbols), result.Source)
	result = s.asyncIngest(ctx, symbols, start)
	log.Infof("ingested raw prices for %d symbols; %d failed", len(result.UpdatedSymbols), len(result.FailedSymbols))

	if len(result.UpdatedSymbols) == 0 && len(result.FailedSymbols) > 0 {
		return result, fmt.Errorf("failed to ingest raw prices for every symbol")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func (s rawPriceService) asyncIngest(ctx context.Context, symbols []string, start *time.Time) RawPriceIngestResult {
	log := logger.FromContext(ctx)
	type ingestResult struct {
		symbol string
		err    error
	}

	inputCh := make(chan string)
	resultCh := make(chan ingestResult, len(symbols))

	var wg sync.WaitGroup
	wg.Add(priceUpdateWorkers)
	for i := 0; i < priceUpdateWorkers; i++ {
		go func() {
			defer wg.Done()
			for symbol := range inputCh {
				err := s.ingestSymbol(ctx, symbol, start)
				resultCh <- ingestResult{symbol: symbol, err: err}
			}
		}()
	}

	go func() {
		defer close(inputCh)
		for _, symbol := range symbols {
			select {
			case <-ctx.Done():
				return
			case inputCh <- symbol:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	result := RawPriceIngestResult{Source: s.provider.Source()}
	for ingest := range resultCh {
		if ingest.err != nil {
			log.Warnf("failed to ingest raw price for %s: %s", ingest.symbol, ingest.err)
			result.FailedSymbols = append(result.FailedSymbols, ingest.symbol)
			continue
		}
		result.UpdatedSymbols = append(result.UpdatedSymbols, ingest.symbol)
	}
	return result
}

func (s rawPriceService) ingestSymbol(ctx context.Context, symbol string, start *time.Time) error {
	from := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if start != nil {
		from = *start
	}
	now := time.Now().UTC()
	bars, err := s.provider.GetRawDailyBars(ctx, symbol, from, now)
	if err != nil {
		return err
	}
	rows := MapRawBarsToRows(symbol, s.provider.Source(), bars, now)
	if len(rows) == 0 {
		return fmt.Errorf("no raw daily bars returned")
	}
	return s.repo.UpsertMany(rows)
}
