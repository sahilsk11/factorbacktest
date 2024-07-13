package service

import (
	"factorbacktest/internal/domain"

	"github.com/google/uuid"
)

// figure out all the target portfolios
// and compare them to current portfolio

type TradingService interface {
	Calculate(strategyInvestmentID uuid.UUID) ([]domain.ProposedTrade, error)
}

func 
