package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"

	"github.com/shopspring/decimal"
)

const (
	CorporateActionSourceAlpaca = "alpaca"
	alpacaCorporateActionsPath  = "/v1/corporate-actions"
	alpacaCATypes               = "reverse_split,forward_split,cash_dividend,cash_merger,spin_off"
	alpacaCAPageLimit           = "1000"
	alpacaCASymbolBatch         = 50
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

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return alpacaCAPage{}, fmt.Errorf("alpaca corporate-actions: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
	if err != nil {
		return alpacaCAPage{}, fmt.Errorf("alpaca corporate-actions: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return alpacaCAPage{}, fmt.Errorf("alpaca corporate-actions: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
		out = appendCA(out, e.Symbol, e.ExDate, "split", "forward_split", ratio(e.NewRate, e.OldRate), nil, nil, ingestedAt)
	}
	for _, e := range g.ReverseSplits {
		out = appendCA(out, e.Symbol, e.ExDate, "split", "reverse_split", ratio(e.NewRate, e.OldRate), nil, optStr(e.NewSymbol), ingestedAt)
	}
	for _, e := range g.CashDividends {
		out = appendCA(out, e.Symbol, e.ExDate, "dividend", "cash_dividend", nil, dec(e.Rate), nil, ingestedAt)
	}
	for _, e := range g.CashMergers {
		out = appendCA(out, e.AcquireeSymbol, e.EffectiveDate, "acquired", "cash_merger", nil, dec(e.Rate), nil, ingestedAt)
	}
	for _, e := range g.SpinOffs {
		out = appendCA(out, e.SourceSymbol, e.ExDate, "other", "spin_off", ratio(e.NewRate, e.SourceRate), nil, optStr(e.NewSymbol), ingestedAt)
	}
	return out
}

func appendCA(out []model.CorporateAction, symbol, date, typ, note string, ratio, cash *decimal.Decimal, newSymbol *string, ingestedAt time.Time) []model.CorporateAction {
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
		Source: CorporateActionSourceAlpaca, Notes: optStr(note), IngestedAt: ingestedAt,
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

type CorporateActionIngestResult struct {
	NumRows       int
	FailedSymbols []string
	Source        string
}

type CorporateActionService interface {
	IngestCorporateActions(ctx context.Context, symbols []string, start, end *time.Time) (CorporateActionIngestResult, error)
}

type corporateActionService struct {
	repo     repository.CorporateActionRepository
	provider CorporateActionProvider
}

func NewCorporateActionService(repo repository.CorporateActionRepository, provider CorporateActionProvider) CorporateActionService {
	return corporateActionService{repo: repo, provider: provider}
}

func (s corporateActionService) IngestCorporateActions(ctx context.Context, symbols []string, start, end *time.Time) (CorporateActionIngestResult, error) {
	result := CorporateActionIngestResult{}
	if s.provider == nil {
		return result, fmt.Errorf("alpaca corporate actions require real data API credentials (this environment has placeholder Alpaca keys)")
	}
	result.Source = s.provider.Source()
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

	var rows []model.CorporateAction
	for i := 0; i < len(symbols); i += alpacaCASymbolBatch {
		j := i + alpacaCASymbolBatch
		if j > len(symbols) {
			j = len(symbols)
		}
		batch := symbols[i:j]
		got, err := s.provider.Fetch(ctx, batch, from, to)
		if err != nil {
			log.Warnf("failed to ingest corporate actions for %s: %s", strings.Join(batch, ","), err)
			result.FailedSymbols = append(result.FailedSymbols, batch...)
			continue
		}
		rows = append(rows, got...)
	}
	if err := s.repo.UpsertMany(rows); err != nil {
		return result, err
	}
	result.NumRows = len(rows)
	if result.NumRows == 0 && len(result.FailedSymbols) > 0 {
		return result, fmt.Errorf("failed to ingest corporate actions for every symbol")
	}
	return result, ctx.Err()
}
