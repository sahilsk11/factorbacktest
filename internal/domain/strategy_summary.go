package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// StrategySummaryResult represents the computed result for a single strategy
// on a given date - what assets it would buy and their weights/scores
type StrategySummaryResult struct {
	StrategyID          uuid.UUID
	StrategyName        string
	Date                time.Time
	Assets              []StrategySummaryAsset // Assets the strategy would buy
	TotalPortfolioValue decimal.Decimal        // Reference value used for calculations
	Error               error
}

// StrategySummaryAsset represents a single asset that a strategy would buy
type StrategySummaryAsset struct {
	Symbol      string
	Weight      float64 // Percentage allocation (0-1)
	FactorScore float64
	LastPrice   decimal.Decimal
}
