package api

import (
	"errors"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type getPublishedStrategiesResponse struct {
	StrategyID        uuid.UUID `json:"strategyID"`
	StrategyName      string    `json:"strategyName"`
	RebalanceInterval string    `json:"rebalanceInterval"`
	CreatedAt         time.Time `json:"createdAt"`
	FactorExpression  string    `json:"factorExpression"`
	NumAssets         int32     `json:"numAssets"`
	AssetUniverse     string    `json:"assetUniverse"`
	SharpeRatio       *float64  `json:"sharpeRatio"`
	AnnualizedReturn  *float64  `json:"annualizedReturn"`
	AnnualizedStdev   *float64  `json:"annualizedStandardDeviation"`
	Description       *string   `json:"description"`
}

type getPublishedStrategyBacktestResponse struct {
	StrategyID        uuid.UUID         `json:"strategyID"`
	StrategyName      string            `json:"strategyName"`
	RebalanceInterval string            `json:"rebalanceInterval"`
	CreatedAt         time.Time         `json:"createdAt"`
	FactorExpression  string            `json:"factorExpression"`
	NumAssets         int32             `json:"numAssets"`
	AssetUniverse     string            `json:"assetUniverse"`
	Description       *string           `json:"description"`
	BacktestStart     string            `json:"backtestStart"`
	BacktestEnd       string            `json:"backtestEnd"`
	CachedAt          *time.Time        `json:"cachedAt"`
	Result            *BacktestResponse `json:"result"`
}

func (m ApiHandler) getPublishedStrategies(c *gin.Context) {
	results, err := m.StrategyRepository.List(repository.StrategyListFilter{
		Published: util.BoolPointer(true),
	})
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := []getPublishedStrategiesResponse{}
	for _, r := range results {
		latestRun, err := m.StrategyRepository.GetLatestPublishedRun(r.StrategyID)
		if err != nil {
			returnErrorJson(fmt.Errorf("failed to get strategy run details: %w", err), c)
			return
		}

		var sharpeRatio *float64
		var annualizedReturn *float64
		var annualizedStdev *float64
		if latestRun != nil {
			sharpeRatio = latestRun.SharpeRatio
			annualizedReturn = latestRun.AnnualizedReturn
			annualizedStdev = latestRun.AnnualuzedStdev
		}

		out = append(out, getPublishedStrategiesResponse{
			StrategyID:        r.StrategyID,
			StrategyName:      r.StrategyName,
			RebalanceInterval: r.RebalanceInterval,
			CreatedAt:         r.CreatedAt,
			FactorExpression:  r.FactorExpression,
			NumAssets:         r.NumAssets,
			AssetUniverse:     r.AssetUniverse,
			SharpeRatio:       sharpeRatio,
			AnnualizedReturn:  annualizedReturn,
			AnnualizedStdev:   annualizedStdev,
			Description:       r.Description,
		})
	}

	c.JSON(200, out)
}

func (m ApiHandler) getPublishedStrategyBacktest(c *gin.Context) {
	strategyID, err := uuid.Parse(c.Param("strategyID"))
	if err != nil {
		returnErrorJsonCode(fmt.Errorf("invalid strategy id"), c, http.StatusBadRequest)
		return
	}

	strategy, err := m.StrategyRepository.Get(strategyID)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			returnErrorJsonCode(fmt.Errorf("published strategy not found"), c, http.StatusNotFound)
			return
		}
		returnErrorJson(fmt.Errorf("failed to get strategy: %w", err), c)
		return
	}
	if strategy == nil || !strategy.Published {
		returnErrorJsonCode(fmt.Errorf("published strategy not found"), c, http.StatusNotFound)
		return
	}

	run, err := m.StrategyRepository.GetLatestPublishedRun(strategy.StrategyID)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to get strategy run details: %w", err), c)
		return
	}

	out := getPublishedStrategyBacktestResponse{
		StrategyID:        strategy.StrategyID,
		StrategyName:      strategy.StrategyName,
		RebalanceInterval: strategy.RebalanceInterval,
		CreatedAt:         strategy.CreatedAt,
		FactorExpression:  strategy.FactorExpression,
		NumAssets:         strategy.NumAssets,
		AssetUniverse:     strategy.AssetUniverse,
		Description:       strategy.Description,
	}
	if run != nil {
		out.BacktestStart = formatDate(run.StartDate)
		out.BacktestEnd = formatDate(run.EndDate)
		cachedAt := run.CreatedAt
		out.CachedAt = &cachedAt
		result, unmarshalErr := unmarshalBacktestResult(run.Result)
		if unmarshalErr != nil {
			logger.FromContext(c).Errorf("failed to unmarshal cached backtest result for %s: %v", strategy.StrategyID, unmarshalErr)
		} else {
			out.Result = result
		}
	}

	c.JSON(http.StatusOK, out)
}
