package api

import (
	"context"
	"database/sql"
	"factorbacktest/internal"
	"factorbacktest/internal/repository"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

func durationKeyToMonthlyDurations(durationKey int) []int {
	return map[int][]int{
		0: {1, 2, 3},
		// 1: {3, 6, 9},
		2: {12, 24, 36},
		3: {36, 60, 84},
		4: {120, 240, 360},
	}[durationKey]
}

type backtestBondPortfolioRequest struct {
	BacktestStart       string  `json:"backtestStart"`
	BacktestEnd         string  `json:"backtestEnd"`
	SelectedDurationKey int     `json:"durationKey"`
	StartCash           float64 `json:"startCash"`

	UserID *string `json:"userID"`
}

func (h ApiHandler) backtestBondPortfolio(c *gin.Context) {
	ctx := context.Background()
	tx, err := h.Db.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
			ReadOnly:  false,
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

	backtestStartDate, err := time.Parse("2006-01", requestBody.BacktestStart)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	backtestEndDate, err := time.Parse("2006-01", requestBody.BacktestEnd)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	if backtestStartDate.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		returnErrorJson(fmt.Errorf("invalid start date %s - must be after 2000-01-01", requestBody.BacktestStart), c)
		return
	}
	if backtestEndDate.Before(backtestStartDate) {
		returnErrorJson(fmt.Errorf("invalid end date %s - must be after backtest start", requestBody.BacktestEnd), c)
		return
	}
	if backtestEndDate.After(time.Now().UTC()) {
		returnErrorJson(fmt.Errorf("invalid end date %s - must be before today", requestBody.BacktestEnd), c)
		return
	}
	if backtestStartDate.After(backtestEndDate) {
		returnErrorJson(fmt.Errorf("invalid backtest start date %s - must be before end date", requestBody.BacktestStart), c)
		return
	}

	bs := internal.BondService{
		InterestRateRepository: repository.NewInterestRateRepository(h.Db),
	}

	result, err := bs.BacktestBondPortfolio(
		tx,
		durationKeyToMonthlyDurations(requestBody.SelectedDurationKey),
		requestBody.StartCash,
		backtestStartDate,
		backtestEndDate,
	)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to run backtest: %w", err), c)
		return
	}

	err = tx.Commit()
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to commit rows: %w", err), c)
		return
	}

	c.JSON(200, result)
}
