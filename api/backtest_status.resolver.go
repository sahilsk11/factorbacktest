package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h ApiHandler) backtestStatus(c *gin.Context) {
	backtestJobIDStr := c.Query("backtest_job_id")
	if backtestJobIDStr == "" {
		returnErrorJson(fmt.Errorf("backtest_job_id is required"), c)
		return
	}

	backtestJobID, err := uuid.Parse(backtestJobIDStr)
	if err != nil {
		returnErrorJson(fmt.Errorf("invalid backtest_job_id format: %w", err), c)
		return
	}

	job, err := h.BacktestJobRepository.Get(backtestJobID)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to get backtest job: %w", err), c)
		return
	}

	if job == nil {
		returnErrorJson(fmt.Errorf("backtest job not found"), c)
		return
	}

	c.JSON(200, gin.H{
		"backtest_job_id": job.BacktestJobID.String(),
		"status":          string(job.Status),
		"current_stage":    job.CurrentStage,
		"progress_pct":     job.ProgressPct,
		"error_message":    job.ErrorMessage,
		"result":           string(job.Result),
	})
}
