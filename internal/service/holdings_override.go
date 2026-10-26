package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrAdminReplaceHoldingsInvestmentNotFound = errors.New("investment not found")
	ErrAdminReplaceHoldingsInvalidRequest     = errors.New("invalid replace holdings request")
)

const adminMutationSource = "admin-api"

type AdminReplaceHoldingsPosition struct {
	Symbol   string  `json:"symbol"`
	Quantity float64 `json:"quantity"`
}

type AdminReplaceHoldingsRequest struct {
	Note      string                         `json:"note"`
	Actor     string                         `json:"actor"`
	Cash      *float64                       `json:"cash"`
	Positions []AdminReplaceHoldingsPosition `json:"positions"`
}

type AdminAdjustHoldingsRequest struct {
	Note           string   `json:"note"`
	Actor          string   `json:"actor"`
	ZeroOutSymbols []string `json:"zeroOutSymbols"`
	Cash           *float64 `json:"cash"`
}

type AdminReplaceHoldingsResponse struct {
	VersionID      uuid.UUID                    `json:"versionId"`
	AuditID        *uuid.UUID                   `json:"auditId,omitempty"`
	Actor          string                       `json:"actor"`
	Note           string                       `json:"note"`
	Before         AdminReplaceHoldingsSnapshot `json:"before"`
	Holdings       AdminReplaceHoldingsSnapshot `json:"holdings"`
	SkippedSymbols []string                     `json:"skippedSymbols,omitempty"`
}

type AdminReplaceHoldingsSnapshot struct {
	Cash      float64                        `json:"cash"`
	Positions []AdminReplaceHoldingsPosition `json:"positions"`
}

func adminActor(actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return "admin"
	}
	return actor
}

func snapshotFromPortfolio(p *domain.Portfolio) AdminReplaceHoldingsSnapshot {
	out := AdminReplaceHoldingsSnapshot{Positions: []AdminReplaceHoldingsPosition{}}
	if p == nil {
		return out
	}
	if p.Cash != nil {
		out.Cash = p.Cash.InexactFloat64()
	}
	symbols := make([]string, 0, len(p.Positions))
	for symbol := range p.Positions {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	for _, symbol := range symbols {
		pos := p.Positions[symbol]
		if pos == nil || pos.ExactQuantity.IsZero() {
			continue
		}
		out.Positions = append(out.Positions, AdminReplaceHoldingsPosition{
			Symbol:   symbol,
			Quantity: pos.ExactQuantity.InexactFloat64(),
		})
	}
	return out
}

func (h investmentServiceHandler) currentHoldingsSnapshot(investmentID uuid.UUID) (*domain.Portfolio, AdminReplaceHoldingsSnapshot, error) {
	holdings, err := h.HoldingsRepository.GetLatestHoldings(nil, investmentID)
	if err != nil {
		return nil, AdminReplaceHoldingsSnapshot{}, err
	}
	return holdings, snapshotFromPortfolio(holdings), nil
}

func (h investmentServiceHandler) appendNotedHoldings(
	investmentID uuid.UUID,
	actor, note, mode string,
	cash decimal.Decimal,
	positions []domain.Position,
	before AdminReplaceHoldingsSnapshot,
	extra map[string]any,
) (*AdminReplaceHoldingsResponse, error) {
	cashTicker, err := h.TickerRepository.GetCashTicker()
	if err != nil {
		return nil, err
	}

	tx, err := h.Db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	version, err := h.HoldingsVersionRepository.Add(tx, model.InvestmentHoldingsVersion{
		InvestmentID: investmentID,
		Note:         &note,
	})
	if err != nil {
		return nil, err
	}

	_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
		TickerID:                    cashTicker.TickerID,
		Quantity:                    cash,
		InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
	})
	if err != nil {
		return nil, err
	}

	afterPositions := make([]AdminReplaceHoldingsPosition, 0, len(positions))
	for _, position := range positions {
		if position.ExactQuantity.IsZero() {
			continue
		}
		_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
			TickerID:                    position.TickerID,
			Quantity:                    position.ExactQuantity,
			InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
		})
		if err != nil {
			return nil, err
		}
		afterPositions = append(afterPositions, AdminReplaceHoldingsPosition{
			Symbol:   position.Symbol,
			Quantity: position.ExactQuantity.InexactFloat64(),
		})
	}
	sort.Slice(afterPositions, func(i, j int) bool {
		return afterPositions[i].Symbol < afterPositions[j].Symbol
	})

	after := AdminReplaceHoldingsSnapshot{Cash: cash.InexactFloat64(), Positions: afterPositions}
	payload := map[string]any{
		"mode":   mode,
		"source": adminMutationSource,
		"before": before,
		"after":  after,
		"result": "applied",
	}
	for k, v := range extra {
		payload[k] = v
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var auditID *uuid.UUID
	if h.AdminMutationAuditRepository != nil {
		audit, err := h.AdminMutationAuditRepository.Add(tx, model.AdminMutationAudit{
			Actor:        actor,
			Kind:         repository.AdminMutationKindLedgerAdjust,
			InvestmentID: &investmentID,
			Note:         note,
			DryRun:       false,
			Payload:      string(payloadJSON),
		})
		if err != nil {
			return nil, err
		}
		auditID = &audit.AdminMutationAuditID
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &AdminReplaceHoldingsResponse{
		VersionID: version.InvestmentHoldingsVersionID,
		AuditID:   auditID,
		Actor:     actor,
		Note:      note,
		Before:    before,
		Holdings:  after,
	}, nil
}

func (h investmentServiceHandler) AdminReplaceHoldings(ctx context.Context, investmentID uuid.UUID, req AdminReplaceHoldingsRequest) (*AdminReplaceHoldingsResponse, error) {
	if _, err := h.InvestmentRepository.Get(investmentID); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAdminReplaceHoldingsInvestmentNotFound, err)
	}

	note := strings.TrimSpace(req.Note)
	if note == "" {
		return nil, fmt.Errorf("%w: note is required", ErrAdminReplaceHoldingsInvalidRequest)
	}
	if req.Cash == nil {
		return nil, fmt.Errorf("%w: cash is required", ErrAdminReplaceHoldingsInvalidRequest)
	}
	if *req.Cash < 0 {
		return nil, fmt.Errorf("%w: cash must be >= 0", ErrAdminReplaceHoldingsInvalidRequest)
	}

	positions := make([]domain.Position, 0, len(req.Positions))
	for _, position := range req.Positions {
		if position.Quantity < 0 {
			return nil, fmt.Errorf("%w: quantity for %s must be >= 0", ErrAdminReplaceHoldingsInvalidRequest, position.Symbol)
		}
		ticker, err := h.TickerRepository.GetBySymbol(position.Symbol)
		if err != nil {
			if errors.Is(err, repository.ErrTickerNotFound) {
				return nil, fmt.Errorf("%w: unknown symbol: %s", ErrAdminReplaceHoldingsInvalidRequest, position.Symbol)
			}
			return nil, err
		}
		positions = append(positions, domain.Position{
			Symbol:        position.Symbol,
			Quantity:      position.Quantity,
			ExactQuantity: decimal.NewFromFloat(position.Quantity),
			TickerID:      ticker.TickerID,
		})
	}

	_, before, err := h.currentHoldingsSnapshot(investmentID)
	if err != nil {
		return nil, err
	}

	return h.appendNotedHoldings(
		investmentID,
		adminActor(req.Actor),
		note,
		"replace",
		decimal.NewFromFloat(*req.Cash),
		positions,
		before,
		nil,
	)
}

func (h investmentServiceHandler) AdminAdjustHoldings(ctx context.Context, investmentID uuid.UUID, req AdminAdjustHoldingsRequest) (*AdminReplaceHoldingsResponse, error) {
	if _, err := h.InvestmentRepository.Get(investmentID); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrAdminReplaceHoldingsInvestmentNotFound, err)
	}

	note := strings.TrimSpace(req.Note)
	if note == "" {
		return nil, fmt.Errorf("%w: note is required", ErrAdminReplaceHoldingsInvalidRequest)
	}
	if req.Cash != nil && *req.Cash < 0 {
		return nil, fmt.Errorf("%w: cash must be >= 0", ErrAdminReplaceHoldingsInvalidRequest)
	}

	zeroSet := map[string]struct{}{}
	zeroOut := make([]string, 0)
	for _, symbol := range req.ZeroOutSymbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			continue
		}
		if _, ok := zeroSet[symbol]; ok {
			continue
		}
		zeroSet[symbol] = struct{}{}
		zeroOut = append(zeroOut, symbol)
	}
	if len(zeroOut) == 0 {
		return nil, fmt.Errorf("%w: zeroOutSymbols is required", ErrAdminReplaceHoldingsInvalidRequest)
	}

	current, before, err := h.currentHoldingsSnapshot(investmentID)
	if err != nil {
		return nil, err
	}

	positions := make([]domain.Position, 0, len(current.Positions))
	skipped := make([]string, 0)
	changed := false
	for symbol, pos := range current.Positions {
		if pos == nil || pos.ExactQuantity.IsZero() {
			continue
		}
		if _, ok := zeroSet[strings.ToUpper(symbol)]; ok {
			changed = true
			continue
		}
		positions = append(positions, *pos)
	}
	for _, symbol := range zeroOut {
		held := false
		for _, pos := range before.Positions {
			if strings.EqualFold(pos.Symbol, symbol) {
				held = true
				break
			}
		}
		if !held {
			skipped = append(skipped, symbol)
		}
	}
	if !changed {
		return nil, fmt.Errorf("%w: none of the requested symbols are currently held", ErrAdminReplaceHoldingsInvalidRequest)
	}

	cash := decimal.Zero
	if current != nil && current.Cash != nil {
		cash = *current.Cash
	}
	if req.Cash != nil {
		cash = decimal.NewFromFloat(*req.Cash)
	}

	actor := adminActor(req.Actor)
	storedNote := fmt.Sprintf("[manual-adjustment] actor=%s source=%s | %s", actor, adminMutationSource, note)

	result, err := h.appendNotedHoldings(
		investmentID,
		actor,
		storedNote,
		"zero_out",
		cash,
		positions,
		before,
		map[string]any{"zeroOutSymbols": zeroOut, "skippedSymbols": skipped},
	)
	if err != nil {
		return nil, err
	}
	result.SkippedSymbols = skipped
	return result, nil
}
