package domain

import (
	"time"

	"github.com/google/uuid"
)

type BacktestJobStatus string

const (
	BacktestJobStatusPending   BacktestJobStatus = "pending"
	BacktestJobStatusRunning   BacktestJobStatus = "running"
	BacktestJobStatusCompleted BacktestJobStatus = "completed"
	BacktestJobStatusFailed    BacktestJobStatus = "failed"
)

type BacktestJob struct {
	BacktestJobID uuid.UUID         `json:"backtestJobId"`
	StrategyID    uuid.UUID         `json:"strategyId"`
	Status       BacktestJobStatus `json:"status"`
	CurrentStage string             `json:"currentStage"`
	ProgressPct  int                `json:"progressPct"`
	Result       string    `json:"result,omitempty"`
	ErrorMessage *string           `json:"errorMessage,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// Stage constants for progress tracking
const (
	StageInitializing           = "initializing"
	StageLoadingPriceData       = "loading_price_data"
	StageCalculatingFactorScore = "calculating_factor_scores"
	StageRunningBacktest        = "running_backtest"
	StageCreatingSnapshots      = "creating_snapshots"
	StageDone                   = "done"
)

func StageLabel(stage string) string {
	switch stage {
	case StageInitializing:
		return "Setting up backtest..."
	case StageLoadingPriceData:
		return "Loading price data..."
	case StageCalculatingFactorScore:
		return "Calculating factor scores..."
	case StageRunningBacktest:
		return "Running backtest..."
	case StageCreatingSnapshots:
		return "Building results..."
	case StageDone:
		return "Done!"
	default:
		return stage
	}
}
