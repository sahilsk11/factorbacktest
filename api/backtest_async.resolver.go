package api

import (
	"context"
	"encoding/json"
	"factorbacktest/internal/calculator"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/service"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

type backtestAsyncRequest struct {
	FactorExpression string  `json:"factor_expression"`
	FactorName       string  `json:"factor_name"`
	BacktestStart     string  `json:"backtest_start"`
	BacktestEnd       string  `json:"backtest_end"`
	SamplingInterval  string  `json:"sampling_interval_unit"`
	StartCash         float64 `json:"start_cash"`
	AssetUniverse     string  `json:"asset_universe"`
	NumSymbols        int     `json:"num_symbols"`
	UserID            *string `json:"user_id"`
}

func (h ApiHandler) backtestAsync(c *gin.Context) {
	lg := logger.FromContext(c)

	var requestBody backtestAsyncRequest
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	backtestStartDate, err := time.Parse("2006-01-02", requestBody.BacktestStart)
	if err != nil {
		returnErrorJson(fmt.Errorf("invalid backtest_start format: %w", err), c)
		return
	}
	backtestEndDate, err := time.Parse("2006-01-02", requestBody.BacktestEnd)
	if err != nil {
		returnErrorJson(fmt.Errorf("invalid backtest_end format: %w", err), c)
		return
	}

	if backtestEndDate.Before(backtestStartDate) {
		returnErrorJson(fmt.Errorf("end date cannot be before start date"), c)
		return
	}

	assetUniverse := "SPY_TOP_80"
	if requestBody.AssetUniverse != "" {
		assetUniverse = requestBody.AssetUniverse
	}

	samplingInterval := time.Hour * 24
	if strings.EqualFold(requestBody.SamplingInterval, "weekly") {
		samplingInterval *= 7
	} else if strings.EqualFold(requestBody.SamplingInterval, "monthly") {
		samplingInterval *= 30
	} else if strings.EqualFold(requestBody.SamplingInterval, "yearly") {
		samplingInterval *= 365
	}

	// Create strategy first (reuse existing logic)
	insertedStrategy, err := h.addNewStrategy(c, requestBody.FactorName, requestBody.FactorExpression, requestBody.SamplingInterval, assetUniverse, requestBody.NumSymbols)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	// Create backtest job record
	backtestJob := domain.BacktestJob{
		BacktestJobID: uuid.New(),
		StrategyID:    insertedStrategy.StrategyID,
		Status:        domain.BacktestJobStatusPending,
		CurrentStage:  domain.StageInitializing,
		ProgressPct:   0,
	}
	createdJob, err := h.BacktestJobRepository.Create(backtestJob)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to create backtest job: %w", err), c)
		return
	}

	// Update stage to running
	h.BacktestJobRepository.UpdateStage(createdJob.BacktestJobID, domain.StageLoadingPriceData, 5)

	// Run backtest in background goroutine
	go h.runBacktestAsync(createdJob.BacktestJobID, insertedStrategy.StrategyID, requestBody, backtestStartDate, backtestEndDate, samplingInterval)

	lg.Info("async backtest job started")
	c.JSON(200, gin.H{
		"backtest_job_id": createdJob.BacktestJobID.String(),
		"strategy_id":     createdJob.StrategyID.String(),
	})
}

func (h ApiHandler) runBacktestAsync(
	jobID uuid.UUID,
	strategyID uuid.UUID,
	req backtestAsyncRequest,
	startDate, endDate time.Time,
	samplingInterval time.Duration,
) {
	ctx := context.Background()
	profile, endProfile := domain.NewProfile()
	ctx = context.WithValue(ctx, domain.ContextProfileKey, profile)
	defer endProfile()

	lg := logger.New()

	// Stage: loading price data
	h.BacktestJobRepository.UpdateStage(jobID, domain.StageLoadingPriceData, 5)

	assetUniverse := "SPY_TOP_80"
	if req.AssetUniverse != "" {
		assetUniverse = req.AssetUniverse
	}

	// Stage: calculating factor scores
	h.BacktestJobRepository.UpdateStage(jobID, domain.StageCalculatingFactorScore, 15)

	backtestInput := service.BacktestInput{
		FactorExpression:  req.FactorExpression,
		BacktestStart:     startDate,
		BacktestEnd:       endDate,
		RebalanceInterval: samplingInterval,
		StartingCash:      req.StartCash,
		NumTickers:        req.NumSymbols,
		AssetUniverse:     assetUniverse,
	}

	result, err := h.BacktestHandler.Backtest(ctx, backtestInput)
	if err != nil {
		h.BacktestJobRepository.MarkFailed(jobID, fmt.Sprintf("backtest failed: %v", err))
		return
	}

	// Stage: running backtest (we're already done here since BacktestHandler runs the full loop)
	h.BacktestJobRepository.UpdateStage(jobID, domain.StageRunningBacktest, 80)

	// Calculate metrics
	metrics, err := h.StrategyService.CalculateMetrics(ctx, strategyID, result.Results)
	if err != nil {
		lg.Errorf("failed to calculate metrics: %v", err)
		metrics = &calculator.CalculateMetricsResult{}
	}

	newRunModel := model.StrategyRun{
		StrategyID: strategyID,
		StartDate:  startDate,
		EndDate:    endDate,
	}
	if metrics != nil {
		newRunModel.SharpeRatio = &metrics.SharpeRatio
		newRunModel.AnnualizedReturn = &metrics.AnnualizedReturn
		newRunModel.AnnualuzedStdev = &metrics.AnnualizedStdev
	}

	_, err = h.StrategyRepository.AddRun(newRunModel)
	if err != nil {
		lg.Errorf("failed to add strategy run: %v", err)
	}

	// Stage: creating snapshots
	h.BacktestJobRepository.UpdateStage(jobID, domain.StageCreatingSnapshots, 90)

	responseJson := BacktestResponse{
		StrategyID: strategyID,
		FactorName: req.FactorName,
		Snapshots:  result.Snapshots,
		LatestHoldings: LatestHoldings{
			Date:   result.LatestHoldings.Date,
			Assets: result.LatestHoldings.Assets,
		},
		AnnualizedReturn: &metrics.AnnualizedReturn,
		SharpeRatio:      &metrics.SharpeRatio,
		AnnualizedStdev:  &metrics.AnnualizedStdev,
	}

	resultBytes, err := json.Marshal(responseJson)
	if err != nil {
		h.BacktestJobRepository.MarkFailed(jobID, fmt.Sprintf("failed to marshal result: %v", err))
		return
	}

	err = h.BacktestJobRepository.MarkCompleted(jobID, resultBytes)
	if err != nil {
		lg.Errorf("failed to mark job completed: %v", err)
	}
}
