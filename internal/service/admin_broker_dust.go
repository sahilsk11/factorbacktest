package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrAdminSellBrokerDustInvalidRequest = errors.New("invalid broker dust sell request")
	ErrAdminSellBrokerDustRefused        = errors.New("broker dust sell refused")
)

// AdminDustSellHardCeilingDollars is the maximum per-position notional this
// tool will sell, matching reconcile's DUST cutoff. The request may set a
// lower cap; it cannot raise this ceiling.
var AdminDustSellHardCeilingDollars = reconDustDollarThreshold

type AdminSellBrokerDustRequest struct {
	Symbols               []string `json:"symbols"`
	Note                  string   `json:"note"`
	Actor                 string   `json:"actor"`
	DryRun                *bool    `json:"dryRun"`
	Confirm               bool     `json:"confirm"`
	MaxDollarsPerPosition *float64 `json:"maxDollarsPerPosition"`
}

type AdminSellBrokerDustSale struct {
	Symbol        string  `json:"symbol"`
	Quantity      string  `json:"quantity"`
	EstDollars    float64 `json:"estDollars"`
	AlpacaOrderID *string `json:"alpacaOrderId,omitempty"`
	Status        string  `json:"status"`
	Error         *string `json:"error,omitempty"`
}

type AdminSellBrokerDustResponse struct {
	DryRun     bool                      `json:"dryRun"`
	Executed   bool                      `json:"executed"`
	CapDollars float64                   `json:"capDollars"`
	Actor      string                    `json:"actor"`
	Note       string                    `json:"note,omitempty"`
	Sales      []AdminSellBrokerDustSale `json:"sales"`
	AuditID    *uuid.UUID                `json:"auditId,omitempty"`
}

func (r AdminSellBrokerDustRequest) dryRun() bool {
	if r.DryRun == nil {
		return true
	}
	return *r.DryRun
}

func normalizeDustSymbols(symbols []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	sort.Strings(out)
	return out
}

func dustSellQty(p alpaca.Position) decimal.Decimal {
	if p.QtyAvailable.GreaterThan(decimal.Zero) {
		return p.QtyAvailable
	}
	return p.Qty
}

func dustSellDollars(p alpaca.Position, qty decimal.Decimal) decimal.Decimal {
	if p.CurrentPrice != nil && p.CurrentPrice.GreaterThan(decimal.Zero) {
		return qty.Mul(*p.CurrentPrice)
	}
	if p.Qty.GreaterThan(decimal.Zero) && p.MarketValue != nil && !p.MarketValue.IsZero() {
		return p.MarketValue.Mul(qty).Div(p.Qty)
	}
	return decimal.Zero
}

func (h investmentServiceHandler) ledgerHolders() (active, all map[string][]uuid.UUID, err error) {
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{
		IncludePaused: true,
		IncludeEnded:  true,
	})
	if err != nil {
		return nil, nil, err
	}
	active = map[string][]uuid.UUID{}
	all = map[string][]uuid.UUID{}
	for _, inv := range investments {
		holdings, err := h.HoldingsRepository.GetLatestHoldings(nil, inv.InvestmentID)
		if err != nil {
			return nil, nil, err
		}
		if holdings == nil {
			continue
		}
		for _, pos := range holdings.Positions {
			if pos == nil || isEffectivelyZero(pos.ExactQuantity) {
				continue
			}
			symbol := strings.ToUpper(pos.Symbol)
			all[symbol] = append(all[symbol], inv.InvestmentID)
			if inv.EndDate == nil {
				active[symbol] = append(active[symbol], inv.InvestmentID)
			}
		}
	}
	return active, all, nil
}

func (h investmentServiceHandler) AdminSellBrokerDust(ctx context.Context, req AdminSellBrokerDustRequest) (*AdminSellBrokerDustResponse, error) {
	if h.AlpacaRepository == nil {
		return nil, fmt.Errorf("%w: broker client is not configured", ErrAdminSellBrokerDustInvalidRequest)
	}

	symbols := normalizeDustSymbols(req.Symbols)
	if len(symbols) == 0 {
		return nil, fmt.Errorf("%w: symbols is required (explicit list; there is no sell-everything default)", ErrAdminSellBrokerDustInvalidRequest)
	}

	cap := AdminDustSellHardCeilingDollars
	if req.MaxDollarsPerPosition != nil {
		requested := decimal.NewFromFloat(*req.MaxDollarsPerPosition)
		if requested.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("%w: maxDollarsPerPosition must be > 0", ErrAdminSellBrokerDustInvalidRequest)
		}
		if requested.GreaterThan(AdminDustSellHardCeilingDollars) {
			return nil, fmt.Errorf("%w: maxDollarsPerPosition %s exceeds hard ceiling $%s", ErrAdminSellBrokerDustInvalidRequest, requested.String(), AdminDustSellHardCeilingDollars.StringFixed(0))
		}
		cap = requested
	}

	dryRun := req.dryRun()
	note := strings.TrimSpace(req.Note)
	actor := adminActor(req.Actor)
	if !dryRun {
		if !req.Confirm {
			return nil, fmt.Errorf("%w: confirm=true is required to execute (dryRun defaults to true)", ErrAdminSellBrokerDustInvalidRequest)
		}
		if note == "" {
			return nil, fmt.Errorf("%w: note is required to execute", ErrAdminSellBrokerDustInvalidRequest)
		}
	}

	positions, err := h.AlpacaRepository.GetPositions()
	if err != nil {
		return nil, err
	}
	bySymbol := map[string]alpaca.Position{}
	for _, p := range positions {
		bySymbol[strings.ToUpper(p.Symbol)] = p
	}

	activeHolders, allHolders, err := h.ledgerHolders()
	if err != nil {
		return nil, err
	}

	var refusals []string
	sales := make([]AdminSellBrokerDustSale, 0, len(symbols))
	for _, symbol := range symbols {
		pos, ok := bySymbol[symbol]
		if !ok {
			refusals = append(refusals, fmt.Sprintf("%s is not held at the broker", symbol))
			continue
		}
		if ids := activeHolders[symbol]; len(ids) > 0 {
			refusals = append(refusals, fmt.Sprintf("%s is held in non-ended investment %s", symbol, ids[0].String()))
			continue
		}
		if ids := allHolders[symbol]; len(ids) > 0 {
			refusals = append(refusals, fmt.Sprintf("%s is still on the ledger for investment %s (not UNTRACKED; write off the ledger first)", symbol, ids[0].String()))
			continue
		}
		qty := dustSellQty(pos)
		if isEffectivelyZero(qty) {
			refusals = append(refusals, fmt.Sprintf("%s broker qty is zero", symbol))
			continue
		}
		dollars := dustSellDollars(pos, qty)
		if dollars.LessThanOrEqual(decimal.Zero) {
			refusals = append(refusals, fmt.Sprintf("%s has no usable price/market value", symbol))
			continue
		}
		if dollars.GreaterThanOrEqual(cap) {
			refusals = append(refusals, fmt.Sprintf("%s estimated $%s is at or above the $%s per-position cap", symbol, dollars.StringFixed(2), cap.StringFixed(2)))
			continue
		}
		sales = append(sales, AdminSellBrokerDustSale{
			Symbol:     symbol,
			Quantity:   qty.String(),
			EstDollars: dollars.InexactFloat64(),
			Status:     "planned",
		})
	}
	if len(refusals) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrAdminSellBrokerDustRefused, strings.Join(refusals, "; "))
	}

	resp := &AdminSellBrokerDustResponse{
		DryRun:     dryRun,
		Executed:   false,
		CapDollars: cap.InexactFloat64(),
		Actor:      actor,
		Note:       note,
		Sales:      sales,
	}
	if dryRun {
		return resp, nil
	}

	open, err := h.AlpacaRepository.IsMarketOpen()
	if err != nil {
		return nil, err
	}
	if !open {
		return nil, fmt.Errorf("%w: market is closed", ErrAdminSellBrokerDustRefused)
	}

	for i := range resp.Sales {
		sale := &resp.Sales[i]
		qty, err := decimal.NewFromString(sale.Quantity)
		if err != nil {
			msg := err.Error()
			sale.Status = "error"
			sale.Error = &msg
			continue
		}
		price := decimal.NewFromFloat(sale.EstDollars).Div(qty)
		limit := price.Mul(decimal.NewFromFloat(0.95)).Round(2)
		if limit.LessThanOrEqual(decimal.Zero) {
			msg := "limit price rounded to 0"
			sale.Status = "error"
			sale.Error = &msg
			continue
		}
		order, err := h.AlpacaRepository.PlaceOrder(repository.AlpacaPlaceOrderRequest{
			TradeOrderID: uuid.New(),
			Quantity:     qty,
			Symbol:       sale.Symbol,
			Side:         alpaca.Sell,
			LimitPrice:   &limit,
		})
		if err != nil {
			msg := err.Error()
			sale.Status = "error"
			sale.Error = &msg
			continue
		}
		sale.Status = "submitted"
		id := order.ID
		sale.AlpacaOrderID = &id
	}

	resp.Executed = true
	resp.DryRun = false
	if h.AdminMutationAuditRepository != nil {
		payload, err := json.Marshal(map[string]any{
			"source":     adminMutationSource,
			"symbols":    symbols,
			"capDollars": cap.InexactFloat64(),
			"sales":      resp.Sales,
			"result":     "applied",
		})
		if err != nil {
			return nil, err
		}
		audit, err := h.AdminMutationAuditRepository.Add(nil, model.AdminMutationAudit{
			Actor:   actor,
			Kind:    repository.AdminMutationKindBrokerDustSell,
			Note:    note,
			DryRun:  false,
			Payload: string(payload),
		})
		if err != nil {
			return nil, err
		}
		resp.AuditID = &audit.AdminMutationAuditID
	}
	return resp, nil
}
