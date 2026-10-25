package data

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"

	"github.com/shopspring/decimal"
)

// ComputeAdjustedSeries builds total-return prices: adjusted = raw close * factor(date).
// factor(date) is the product of every split/dividend with effective date AFTER date and
// on/before the last raw date: split -> 1/ratio; dividend -> (1 - cash / prior raw close).
// Latest date has factor 1. Acquired/delisted history is truncated, not adjusted.
type AdjustedPoint struct {
	Date   time.Time
	Raw    decimal.Decimal
	Factor decimal.Decimal
	Price  decimal.Decimal
}

func ComputeAdjustedSeries(raw []model.RawPrice, actions []model.CorporateAction) ([]AdjustedPoint, []string) {
	bars := make([]model.RawPrice, 0, len(raw))
	for _, row := range raw {
		if row.Close.GreaterThan(decimal.Zero) {
			row.Date = utcDate(row.Date)
			bars = append(bars, row)
		}
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date.Before(bars[j].Date) })

	var cutoff time.Time
	keep := make([]model.CorporateAction, 0, len(actions))
	for _, a := range actions {
		a.EffectiveDate = utcDate(a.EffectiveDate)
		if a.Type == "acquired" || a.Type == "delisted" {
			if cutoff.IsZero() || a.EffectiveDate.Before(cutoff) {
				cutoff = a.EffectiveDate
			}
			continue
		}
		keep = append(keep, a)
	}
	if !cutoff.IsZero() {
		n := 0
		for _, row := range bars {
			if row.Date.Before(cutoff) {
				bars[n] = row
				n++
			}
		}
		bars = bars[:n]
	}
	if len(bars) == 0 {
		return nil, nil
	}

	last := bars[len(bars)-1].Date
	usable := keep[:0]
	for _, a := range keep {
		if !a.EffectiveDate.After(last) {
			usable = append(usable, a)
		}
	}

	var warnings []string
	for _, a := range usable {
		if a.Type == "dividend" && priorClose(bars, a.EffectiveDate) == nil {
			warnings = append(warnings, fmt.Sprintf("%s dividend on %s skipped: no prior raw close", a.Symbol, a.EffectiveDate.Format(time.DateOnly)))
		}
	}

	one := decimal.NewFromInt(1)
	factor := one
	out := make([]AdjustedPoint, len(bars))
	for i := len(bars) - 1; i >= 0; i-- {
		out[i] = AdjustedPoint{Date: bars[i].Date, Raw: bars[i].Close, Factor: factor, Price: bars[i].Close.Mul(factor)}
		if i == 0 {
			break
		}
		prev, cur := bars[i-1].Date, bars[i].Date
		for _, a := range usable {
			if ex := a.EffectiveDate; ex.After(prev) && !ex.After(cur) {
				mult, warn := actionMultiplier(a, priorClose(bars, ex))
				if warn != "" {
					warnings = append(warnings, warn)
				}
				factor = factor.Mul(mult)
			}
		}
	}
	return out, warnings
}

func priorClose(bars []model.RawPrice, ex time.Time) *decimal.Decimal {
	for i := len(bars) - 1; i >= 0; i-- {
		if bars[i].Date.Before(ex) {
			c := bars[i].Close
			return &c
		}
	}
	return nil
}

func actionMultiplier(a model.CorporateAction, prior *decimal.Decimal) (decimal.Decimal, string) {
	one := decimal.NewFromInt(1)
	day := a.EffectiveDate.Format(time.DateOnly)
	switch a.Type {
	case "split":
		if a.Ratio == nil || !a.Ratio.GreaterThan(decimal.Zero) {
			return one, fmt.Sprintf("%s split on %s skipped: missing ratio", a.Symbol, day)
		}
		return one.Div(*a.Ratio), ""
	case "dividend":
		if a.CashAmount == nil || !a.CashAmount.GreaterThan(decimal.Zero) {
			return one, fmt.Sprintf("%s dividend on %s skipped: missing cash", a.Symbol, day)
		}
		if prior == nil || !prior.GreaterThan(decimal.Zero) {
			return one, ""
		}
		mult := one.Sub(a.CashAmount.Div(*prior))
		if !mult.GreaterThan(decimal.Zero) {
			return one, fmt.Sprintf("%s dividend on %s skipped: factor <= 0", a.Symbol, day)
		}
		return mult, ""
	default:
		return one, ""
	}
}

var DefaultVerifySymbols = []string{"AAPL", "NVDA", "MSFT", "GOOGL", "AMZN", "SPY"}

var priceHistoryStart = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

type DerivedUpdateInput struct {
	Symbols []string
	Start   *time.Time
	End     *time.Time
	Write   bool
	Ingest  bool
	Full    bool
}

type DerivedUpdateResult struct {
	Raw                *RawPriceIngestResult
	CorporateActions   *CorporateActionIngestResult
	CorporateActionErr string
	FullRecompute      []string
	Appended           []string
	FailedSymbols      []string
	Wrote              bool
	NumRows            int
}

type AdjustedDiff struct {
	Symbol         string
	ComputedPoints int
	Overlap        int
	MaxRelDiff     decimal.Decimal
	MaxRelDiffDate *time.Time
	Latest         *AdjustedPoint
	LatestStored   *decimal.Decimal
	Err            string
	Warnings       []string
}

type DerivedPriceService interface {
	UpdateDerivedPrices(ctx context.Context, in DerivedUpdateInput) (DerivedUpdateResult, error)
	VerifyAdjustedPrices(ctx context.Context, symbols []string) ([]AdjustedDiff, error)
}

type derivedPriceService struct {
	db   *sql.DB
	raw  repository.RawPriceRepository
	ca   repository.CorporateActionRepository
	adj  repository.AdjustedPriceRepository
	rawS RawPriceService
	caS  CorporateActionService
}

func NewDerivedPriceService(db *sql.DB, raw repository.RawPriceRepository, ca repository.CorporateActionRepository, adj repository.AdjustedPriceRepository, rawS RawPriceService, caS CorporateActionService) DerivedPriceService {
	return derivedPriceService{db: db, raw: raw, ca: ca, adj: adj, rawS: rawS, caS: caS}
}

func (s derivedPriceService) UpdateDerivedPrices(ctx context.Context, in DerivedUpdateInput) (DerivedUpdateResult, error) {
	log := logger.FromContext(ctx)
	out := DerivedUpdateResult{Wrote: in.Write}
	symbols := uniqueSymbols(in.Symbols)
	if len(symbols) == 0 {
		return out, fmt.Errorf("no symbols supplied for derived price update")
	}
	now := time.Now().UTC()
	from, to := now.Add(-priceRefreshOverlap), now
	if in.Start != nil {
		from = *in.Start
	}
	if in.End != nil {
		to = *in.End
	}
	if in.Ingest {
		rawResult, err := s.rawS.IngestRawPrices(ctx, symbols, in.Start)
		if err != nil {
			return out, err
		}
		out.Raw = &rawResult
		caResult, err := s.caS.IngestCorporateActions(ctx, symbols, in.Start, in.End)
		if err != nil {
			out.CorporateActionErr = err.Error()
			log.Warnf("corporate action ingest skipped: %s", err)
		} else {
			out.CorporateActions = &caResult
		}
	}

	fullSet := map[string]bool{}
	if in.Full {
		for _, symbol := range symbols {
			fullSet[symbol] = true
		}
	} else if err := s.markFullRecompute(symbols, from, to, fullSet); err != nil {
		return out, err
	}

	createdAt := time.Now().UTC()
	for _, symbol := range symbols {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		n, warnings, err := s.applySymbol(ctx, symbol, createdAt, in.Write, fullSet[symbol])
		if err != nil {
			log.Warnf("derived price %s: %s", symbol, err)
			out.FailedSymbols = append(out.FailedSymbols, symbol)
			continue
		}
		for _, w := range warnings {
			log.Warn(w)
		}
		out.NumRows += n
		if fullSet[symbol] {
			out.FullRecompute = append(out.FullRecompute, symbol)
		} else {
			out.Appended = append(out.Appended, symbol)
		}
	}
	return out, nil
}

func (s derivedPriceService) markFullRecompute(symbols []string, from, to time.Time, fullSet map[string]bool) error {
	need, err := s.ca.SymbolsNeedingRecompute(from, to)
	if err != nil {
		return err
	}
	requested := map[string]bool{}
	for _, symbol := range symbols {
		requested[symbol] = true
	}
	for _, symbol := range need {
		if requested[symbol] {
			fullSet[symbol] = true
		}
	}
	latest, err := s.adj.LatestPriceDates(symbols)
	if err != nil {
		return err
	}
	for _, symbol := range symbols {
		if _, ok := latest[symbol]; !ok {
			fullSet[symbol] = true
		}
	}
	return nil
}

func (s derivedPriceService) VerifyAdjustedPrices(ctx context.Context, symbols []string) ([]AdjustedDiff, error) {
	symbols = uniqueSymbols(symbols)
	if len(symbols) == 0 {
		symbols = append([]string{}, DefaultVerifySymbols...)
	}
	end := time.Now().UTC().Add(24 * time.Hour)
	out := make([]AdjustedDiff, 0, len(symbols))
	for _, symbol := range symbols {
		diff := AdjustedDiff{Symbol: symbol}
		points, warnings, err := s.computeSymbol(symbol)
		diff.Warnings = warnings
		if err != nil {
			diff.Err = err.Error()
			out = append(out, diff)
			continue
		}
		diff.ComputedPoints = len(points)
		if len(points) > 0 {
			latest := points[len(points)-1]
			diff.Latest = &latest
		}
		stored, err := s.adj.List([]string{symbol}, priceHistoryStart, end)
		if err != nil {
			diff.Err = err.Error()
			out = append(out, diff)
			continue
		}
		byDate := map[string]decimal.Decimal{}
		for _, p := range stored {
			byDate[utcDate(p.Date).Format(time.DateOnly)] = p.Price
		}
		for _, p := range points {
			got, ok := byDate[p.Date.Format(time.DateOnly)]
			if !ok {
				continue
			}
			diff.Overlap++
			rel := relativeDiff(p.Price, got)
			if rel.GreaterThan(diff.MaxRelDiff) {
				diff.MaxRelDiff = rel
				d := p.Date
				diff.MaxRelDiffDate = &d
			}
		}
		if diff.Latest != nil {
			if got, ok := byDate[diff.Latest.Date.Format(time.DateOnly)]; ok {
				g := got
				diff.LatestStored = &g
			}
		}
		out = append(out, diff)
	}
	return out, ctx.Err()
}

func (s derivedPriceService) computeSymbol(symbol string) ([]AdjustedPoint, []string, error) {
	end := time.Now().UTC().Add(24 * time.Hour)
	raw, err := s.raw.ListBySymbolRange(symbol, priceHistoryStart, end)
	if err != nil {
		return nil, nil, err
	}
	actions, err := s.ca.ListBySymbolRange(symbol, priceHistoryStart, end)
	if err != nil {
		return nil, nil, err
	}
	points, warnings := ComputeAdjustedSeries(raw, actions)
	return points, warnings, nil
}

func (s derivedPriceService) applySymbol(ctx context.Context, symbol string, createdAt time.Time, write, replace bool) (int, []string, error) {
	points, warnings, err := s.computeSymbol(symbol)
	if err != nil {
		return 0, warnings, err
	}
	if len(points) == 0 {
		if write && replace {
			return 0, warnings, s.adj.DeleteBySymbol(nil, symbol)
		}
		return 0, warnings, nil
	}
	n := len(points)
	if !replace {
		n = 1
		points = points[len(points)-1:]
	}
	if !write {
		return n, warnings, nil
	}
	rows := make([]model.AdjustedPrice, len(points))
	for i, p := range points {
		rows[i] = model.AdjustedPrice{Symbol: symbol, Date: p.Date, Price: p.Price, CreatedAt: createdAt}
	}
	if !replace {
		return n, warnings, s.adj.Add(nil, rows)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, warnings, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.adj.DeleteBySymbol(tx, symbol); err != nil {
		return 0, warnings, err
	}
	if err := s.adj.Add(tx, rows); err != nil {
		return 0, warnings, err
	}
	return n, warnings, tx.Commit()
}

func relativeDiff(computed, stored decimal.Decimal) decimal.Decimal {
	if stored.IsZero() {
		if computed.IsZero() {
			return decimal.Zero
		}
		return decimal.NewFromInt(1)
	}
	return computed.Sub(stored).Abs().Div(stored)
}
