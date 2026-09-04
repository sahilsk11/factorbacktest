package service

import (
	"context"
	"errors"
	"fmt"
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

type AdminReplaceHoldingsPosition struct {
	Symbol   string  `json:"symbol"`
	Quantity float64 `json:"quantity"`
}

type AdminReplaceHoldingsRequest struct {
	Note      string                         `json:"note"`
	Cash      *float64                       `json:"cash"`
	Positions []AdminReplaceHoldingsPosition `json:"positions"`
}

type AdminReplaceHoldingsResponse struct {
	VersionID uuid.UUID                      `json:"versionId"`
	Holdings  AdminReplaceHoldingsSnapshot   `json:"holdings"`
}

type AdminReplaceHoldingsSnapshot struct {
	Cash      float64                        `json:"cash"`
	Positions []AdminReplaceHoldingsPosition `json:"positions"`
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

	cashTicker, err := h.TickerRepository.GetCashTicker()
	if err != nil {
		return nil, err
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

	cash := decimal.NewFromFloat(*req.Cash)
	_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
		TickerID:                    cashTicker.TickerID,
		Quantity:                    cash,
		InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
	})
	if err != nil {
		return nil, err
	}

	responsePositions := make([]AdminReplaceHoldingsPosition, 0, len(positions))
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
		responsePositions = append(responsePositions, AdminReplaceHoldingsPosition{
			Symbol:   position.Symbol,
			Quantity: position.Quantity,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &AdminReplaceHoldingsResponse{
		VersionID: version.InvestmentHoldingsVersionID,
		Holdings: AdminReplaceHoldingsSnapshot{
			Cash:      *req.Cash,
			Positions: responsePositions,
		},
	}, nil
}
