package service

import (
	"context"
	"errors"
	"fmt"

	"factorbacktest/internal/repository"

	"github.com/google/uuid"
)

var (
	ErrAdminUnpauseInvestmentNotFound = errors.New("investment not found")
	ErrAdminUnpauseTerminal           = errors.New("investment is in a terminal state")
)

type AdminUnpauseResponse struct {
	InvestmentID  uuid.UUID `json:"investmentId"`
	Paused        bool      `json:"paused"`
	AlreadyActive bool      `json:"alreadyActive"`
}

// AdminUnpause clears paused_at so the next rebalance candidate listing
// includes the investment. It does not place orders, trigger a rebalance,
// or change cash/holdings.
func (h investmentServiceHandler) AdminUnpause(ctx context.Context, investmentID uuid.UUID) (*AdminUnpauseResponse, error) {
	result, err := h.InvestmentRepository.Unpause(investmentID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvestmentNotFound):
			return nil, fmt.Errorf("%w: %w", ErrAdminUnpauseInvestmentNotFound, err)
		case errors.Is(err, repository.ErrInvestmentEnded), errors.Is(err, repository.ErrInvestmentInLiquidation):
			return nil, fmt.Errorf("%w: %w", ErrAdminUnpauseTerminal, err)
		default:
			return nil, err
		}
	}
	return &AdminUnpauseResponse{
		InvestmentID:  result.Investment.InvestmentID,
		Paused:        result.Investment.PausedAt != nil,
		AlreadyActive: result.AlreadyActive,
	}, nil
}
