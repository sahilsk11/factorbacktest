package api

import (
	"context"
	"database/sql"
	"factorbacktest/internal"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type backtestBondPortfolioRequest struct {
	BacktestStart string  `json:"backtestStart"`
	BacktestEnd   string  `json:"backtestEnd"`
	Durations     []int   `json:"durations"`
	StartCash     float64 `json:"startCash"`

	UserID *string `json:"userID"`
}

type backtestBondPortfolioResponse struct {
	FactorName string                      `json:"factorName"`
	Snapshots  map[string]backtestSnapshot `json:"backtestSnapshots"`
}

func (h ApiHandler) backtestBondPortfolio(c *gin.Context) {
	ctx := context.Background()
	tx, err := h.Db.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
			ReadOnly:  true,
		},
	)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	defer tx.Rollback()

	var requestBody backtestBondPortfolioRequest

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	backtestStartDate, err := time.Parse("2006-01-02", requestBody.BacktestStart)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	backtestEndDate, err := time.Parse("2006-01-02", requestBody.BacktestEnd)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	result, err := internal.BacktestBondPortfolio(
		requestBody.Durations,
		requestBody.StartCash,
		backtestStartDate,
		backtestEndDate,
	)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to run backtest: %w", err), c)
		return
	}

	c.JSON(200, result)
}
