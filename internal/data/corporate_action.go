package data

// Corporate-action ingest assumptions:
//   - Yahoo Finance chart events are the source of truth for splits and dividends.
//   - The ticker symbol is the identifier; ticker reuse is not handled.
//   - Spin-offs and other special cases are whatever Yahoo reports (chart
//     events only include splits and dividends).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"

	"github.com/shopspring/decimal"
)

const (
	CorporateActionSourceAlpaca = "alpaca"
	CorporateActionSourceYahoo  = "yahoo"
	alpacaCorporateActionsPath  = "/v1/corporate-actions"
	alpacaCATypes               = "reverse_split,forward_split,cash_dividend,cash_merger,spin_off"
	alpacaCAPageLimit           = "1000"
	alpacaCASymbolBatch         = 50
	ingestHTTPAttempts          = 5
	ingestHTTPBackoff           = 250 * time.Millisecond
)

type CorporateActionProvider interface {
	Source() string
	Fetch(ctx context.Context, symbols []string, start, end time.Time) ([]model.CorporateAction, error)
}

func NewCorporateActionProvider(apiKey, apiSecret, endpoint string) CorporateActionProvider {
	if apiKey == "" || apiSecret == "" || apiKey == "test" || endpoint == "test" {
		return nil
	}
	return NewAlpacaCorporateActionProvider(apiKey, apiSecret, "https://data.alpaca.markets")
}

type alpacaCorporateActionProvider struct {
	apiKey, apiSecret, baseURL string
	httpClient                 *http.Client
}

func NewAlpacaCorporateActionProvider(apiKey, apiSecret, baseURL string) CorporateActionProvider {
	return alpacaCorporateActionProvider{
		apiKey: apiKey, apiSecret: apiSecret,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (alpacaCorporateActionProvider) Source() string { return CorporateActionSourceAlpaca }

func (p alpacaCorporateActionProvider) Fetch(ctx context.Context, symbols []string, start, end time.Time) ([]model.CorporateAction, error) {
	ingestedAt := time.Now().UTC()
	var out []model.CorporateAction
	pageToken := ""
	for {
		page, err := p.getPage(ctx, symbols, start, end, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, MapAlpacaCorporateActions(page.CorporateActions, ingestedAt)...)
		if page.NextPageToken == nil || *page.NextPageToken == "" {
			return out, nil
		}
		pageToken = *page.NextPageToken
	}
}

type alpacaCAPage struct {
	CorporateActions alpacaCAGroups `json:"corporate_actions"`
	NextPageToken    *string        `json:"next_page_token"`
}

type alpacaCAGroups struct {
	ReverseSplits []alpacaCAEvent `json:"reverse_splits"`
	ForwardSplits []alpacaCAEvent `json:"forward_splits"`
	CashDividends []alpacaCAEvent `json:"cash_dividends"`
	CashMergers   []alpacaCAEvent `json:"cash_mergers"`
	SpinOffs      []alpacaCAEvent `json:"spin_offs"`
}

type alpacaCAEvent struct {
	Symbol         string      `json:"symbol"`
	NewSymbol      string      `json:"new_symbol"`
	SourceSymbol   string      `json:"source_symbol"`
	AcquireeSymbol string      `json:"acquiree_symbol"`
	ExDate         string      `json:"ex_date"`
	EffectiveDate  string      `json:"effective_date"`
	NewRate        json.Number `json:"new_rate"`
	OldRate        json.Number `json:"old_rate"`
	SourceRate     json.Number `json:"source_rate"`
	Rate           json.Number `json:"rate"`
}

func (p alpacaCorporateActionProvider) getPage(ctx context.Context, symbols []string, start, end time.Time, pageToken string) (alpacaCAPage, error) {
	u, err := url.Parse(p.baseURL + alpacaCorporateActionsPath)
	if err != nil {
		return alpacaCAPage{}, err
	}
	q := u.Query()
	q.Set("symbols", strings.Join(symbols, ","))
	q.Set("types", alpacaCATypes)
	q.Set("start", utcDate(start).Format("2006-01-02"))
	q.Set("end", utcDate(end).Format("2006-01-02"))
	q.Set("limit", alpacaCAPageLimit)
	if pageToken != "" {
		q.Set("page_token", pageToken)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return alpacaCAPage{}, err
	}
	req.Header.Set("APCA-API-KEY-ID", p.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", p.apiSecret)

	body, status, err := httpGetRetry(ctx, p.httpClient, req)
	if err != nil {
		return alpacaCAPage{}, fmt.Errorf("alpaca corporate-actions: %w", err)
	}
	if status != http.StatusOK {
		return alpacaCAPage{}, fmt.Errorf("alpaca corporate-actions: status %d: %s", status, strings.TrimSpace(string(body)))
	}
	var page alpacaCAPage
	if err := json.Unmarshal(body, &page); err != nil {
		return alpacaCAPage{}, fmt.Errorf("alpaca corporate-actions: %w", err)
	}
	return page, nil
}

func MapAlpacaCorporateActions(g alpacaCAGroups, ingestedAt time.Time) []model.CorporateAction {
	out := make([]model.CorporateAction, 0)
	for _, e := range g.ForwardSplits {
		out = appendCA(out, e.Symbol, e.ExDate, "split", "forward_split", ratio(e.NewRate, e.OldRate), nil, nil, ingestedAt, CorporateActionSourceAlpaca)
	}
	for _, e := range g.ReverseSplits {
		out = appendCA(out, e.Symbol, e.ExDate, "split", "reverse_split", ratio(e.NewRate, e.OldRate), nil, optStr(e.NewSymbol), ingestedAt, CorporateActionSourceAlpaca)
	}
	for _, e := range g.CashDividends {
		out = appendCA(out, e.Symbol, e.ExDate, "dividend", "cash_dividend", nil, dec(e.Rate), nil, ingestedAt, CorporateActionSourceAlpaca)
	}
	for _, e := range g.CashMergers {
		out = appendCA(out, e.AcquireeSymbol, e.EffectiveDate, "acquired", "cash_merger", nil, dec(e.Rate), nil, ingestedAt, CorporateActionSourceAlpaca)
	}
	for _, e := range g.SpinOffs {
		out = appendCA(out, e.SourceSymbol, e.ExDate, "other", "spin_off", ratio(e.NewRate, e.SourceRate), nil, optStr(e.NewSymbol), ingestedAt, CorporateActionSourceAlpaca)
	}
	return out
}

func appendCA(out []model.CorporateAction, symbol, date, typ, note string, ratio, cash *decimal.Decimal, newSymbol *string, ingestedAt time.Time, source string) []model.CorporateAction {
	if symbol == "" || date == "" {
		return out
	}
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return out
	}
	return append(out, model.CorporateAction{
		Symbol: symbol, EffectiveDate: utcDate(d), Type: typ,
		Ratio: ratio, CashAmount: cash, NewSymbol: newSymbol,
		Source: source, Notes: optStr(note), IngestedAt: ingestedAt,
	})
}

func dec(n json.Number) *decimal.Decimal {
	if n == "" {
		return nil
	}
	d, err := decimal.NewFromString(string(n))
	if err != nil {
		return nil
	}
	return &d
}

func ratio(newRate, oldRate json.Number) *decimal.Decimal {
	n, o := dec(newRate), dec(oldRate)
	if n == nil || o == nil || o.IsZero() {
		return nil
	}
	r := n.Div(*o)
	return &r
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type yahooCAEvents struct {
	Splits map[string]struct {
		Date        int64   `json:"date"`
		Numerator   float64 `json:"numerator"`
		Denominator float64 `json:"denominator"`
	} `json:"splits"`
	Dividends map[string]struct {
		Amount float64 `json:"amount"`
		Date   int64   `json:"date"`
	} `json:"dividends"`
}

type yahooCAChartResponse struct {
	Chart struct {
		Result []struct {
			Events *yahooCAEvents `json:"events"`
		} `json:"result"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

type yahooCorporateActionProvider struct {
	httpClient *http.Client
}

func NewYahooCorporateActionProvider() CorporateActionProvider {
	return yahooCorporateActionProvider{httpClient: yahooHTTP}
}

func (yahooCorporateActionProvider) Source() string { return CorporateActionSourceYahoo }

func yahooChartSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, ".", "-")
}

func (p yahooCorporateActionProvider) Fetch(ctx context.Context, symbols []string, start, end time.Time) ([]model.CorporateAction, error) {
	ingestedAt := time.Now().UTC()
	var out []model.CorporateAction
	for _, symbol := range symbols {
		rows, err := p.fetchOne(ctx, symbol, start, end, ingestedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

func (p yahooCorporateActionProvider) fetchOne(ctx context.Context, symbol string, start, end, ingestedAt time.Time) ([]model.CorporateAction, error) {
	p1 := utcDate(start).Unix()
	p2 := end.Unix()
	if end.Equal(utcDate(end)) {
		p2 = end.Add(24 * time.Hour).Unix()
	}
	u := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d&events=%s",
		url.PathEscape(yahooChartSymbol(symbol)), p1, p2, url.QueryEscape("split|div"),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := p.httpClient
	if client == nil {
		client = yahooHTTP
	}
	body, status, err := httpGetRetry(ctx, client, req)
	if err != nil {
		return nil, fmt.Errorf("yahoo corporate actions for %s: %w", symbol, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("yahoo corporate actions for %s: status %d", symbol, status)
	}

	parsed := yahooCAChartResponse{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("yahoo corporate actions for %s: %w", symbol, err)
	}
	if parsed.Chart.Error != nil && parsed.Chart.Error.Description != "" {
		return nil, fmt.Errorf("yahoo corporate actions for %s: %s", symbol, parsed.Chart.Error.Description)
	}
	if len(parsed.Chart.Result) == 0 {
		return nil, fmt.Errorf("yahoo corporate actions for %s: empty chart", symbol)
	}
	return MapYahooChartEvents(symbol, parsed.Chart.Result[0].Events, ingestedAt), nil
}

func MapYahooChartEvents(symbol string, events *yahooCAEvents, ingestedAt time.Time) []model.CorporateAction {
	if events == nil {
		return nil
	}
	out := make([]model.CorporateAction, 0, len(events.Splits)+len(events.Dividends))
	for _, s := range events.Splits {
		if s.Numerator <= 0 || s.Denominator <= 0 {
			continue
		}
		r := decimal.NewFromFloat(s.Numerator).Div(decimal.NewFromFloat(s.Denominator))
		out = appendCA(out, symbol, utcDate(time.Unix(s.Date, 0).UTC()).Format("2006-01-02"), "split", "split", &r, nil, nil, ingestedAt, CorporateActionSourceYahoo)
	}
	for _, d := range events.Dividends {
		if d.Amount == 0 {
			continue
		}
		cash := decimal.NewFromFloat(d.Amount)
		out = appendCA(out, symbol, utcDate(time.Unix(d.Date, 0).UTC()).Format("2006-01-02"), "dividend", "dividend", nil, &cash, nil, ingestedAt, CorporateActionSourceYahoo)
	}
	return out
}

func httpGetRetry(ctx context.Context, client *http.Client, req *http.Request) ([]byte, int, error) {
	if client == nil {
		client = yahooHTTP
	}
	backoff := ingestHTTPBackoff
	var lastErr error
	for attempt := 0; attempt < ingestHTTPAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
		resp, err := client.Do(req.Clone(ctx))
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}
		return body, resp.StatusCode, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("retry exhausted")
	}
	return nil, 0, lastErr
}

type CorporateActionIngestResult struct {
	NumRows       int
	FailedSymbols []string
	Source        string
}

type CorporateActionService interface {
	IngestCorporateActions(ctx context.Context, symbols []string, start, end *time.Time, source string) (CorporateActionIngestResult, error)
}

type corporateActionService struct {
	repo   repository.CorporateActionRepository
	yahoo  CorporateActionProvider
	alpaca CorporateActionProvider
}

func NewCorporateActionService(repo repository.CorporateActionRepository, provider CorporateActionProvider) CorporateActionService {
	s := corporateActionService{
		repo:  repo,
		yahoo: NewYahooCorporateActionProvider(),
	}
	if provider == nil {
		return s
	}
	if provider.Source() == CorporateActionSourceAlpaca {
		s.alpaca = provider
		return s
	}
	s.yahoo = provider
	return s
}

func (s corporateActionService) providerFor(source string) (CorporateActionProvider, error) {
	if strings.EqualFold(strings.TrimSpace(source), CorporateActionSourceAlpaca) {
		if s.alpaca == nil {
			return nil, fmt.Errorf("alpaca corporate actions require real data API credentials (this environment has placeholder Alpaca keys)")
		}
		return s.alpaca, nil
	}
	return s.yahoo, nil
}

func (s corporateActionService) IngestCorporateActions(ctx context.Context, symbols []string, start, end *time.Time, source string) (CorporateActionIngestResult, error) {
	result := CorporateActionIngestResult{}
	provider, err := s.providerFor(source)
	if err != nil {
		return result, err
	}
	result.Source = provider.Source()
	symbols = uniqueSymbols(symbols)
	if len(symbols) == 0 {
		return result, fmt.Errorf("no symbols supplied for corporate action update")
	}
	now := time.Now().UTC()
	from, to := now.Add(-priceRefreshOverlap), now
	if start != nil {
		from = *start
	}
	if end != nil {
		to = *end
	}

	log := logger.FromContext(ctx)
	log.Infof("ingesting corporate actions for %d symbols via %s", len(symbols), result.Source)

	rows, failed := s.fetchPerSymbol(ctx, provider, symbols, from, to)
	result.FailedSymbols = failed
	rows = coalesceCorporateActions(rows)
	if err := s.repo.UpsertMany(rows); err != nil {
		return result, err
	}
	result.NumRows = len(rows)
	// 0 rows is success when Yahoo had no split/div events in the window.
	// Only fail the request if every symbol's fetch failed.
	if len(result.FailedSymbols) == len(symbols) {
		return result, fmt.Errorf("failed to ingest corporate actions for every symbol")
	}
	return result, ctx.Err()
}

func (s corporateActionService) fetchPerSymbol(ctx context.Context, provider CorporateActionProvider, symbols []string, from, to time.Time) ([]model.CorporateAction, []string) {
	log := logger.FromContext(ctx)
	type one struct {
		symbol string
		rows   []model.CorporateAction
		err    error
	}

	workers := priceUpdateWorkers
	if workers > len(symbols) {
		workers = len(symbols)
	}
	inputCh := make(chan string)
	resultCh := make(chan one, len(symbols))

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for symbol := range inputCh {
				rows, err := provider.Fetch(ctx, []string{symbol}, from, to)
				resultCh <- one{symbol: symbol, rows: rows, err: err}
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

	var rows []model.CorporateAction
	var failed []string
	for got := range resultCh {
		if got.err != nil {
			log.Warnf("failed to ingest corporate actions for %s: %s", got.symbol, got.err)
			failed = append(failed, got.symbol)
			continue
		}
		rows = append(rows, got.rows...)
	}
	return rows, failed
}

// coalesceCorporateActions merges rows sharing the (symbol, effective_date, type) unique key:
// dividends are summed (e.g. regular + special on one ex-date); others keep the first row.
func coalesceCorporateActions(rows []model.CorporateAction) []model.CorporateAction {
	type key struct {
		s, t string
		d    time.Time
	}
	idx := map[key]int{}
	out := make([]model.CorporateAction, 0, len(rows))
	for _, r := range rows {
		k := key{r.Symbol, r.Type, r.EffectiveDate}
		if i, ok := idx[k]; ok {
			if r.Type == "dividend" && r.CashAmount != nil && out[i].CashAmount != nil {
				sum := out[i].CashAmount.Add(*r.CashAmount)
				out[i].CashAmount = &sum
			}
			continue
		}
		idx[k] = len(out)
		out = append(out, r)
	}
	return out
}
