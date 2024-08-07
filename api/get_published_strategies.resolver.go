package api

import (
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
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

		out = append(out, getPublishedStrategiesResponse{
			StrategyID:        r.StrategyID,
			StrategyName:      r.StrategyName,
			RebalanceInterval: r.RebalanceInterval,
			CreatedAt:         r.CreatedAt,
			FactorExpression:  r.FactorExpression,
			NumAssets:         r.NumAssets,
			AssetUniverse:     r.AssetUniverse,
			SharpeRatio:       latestRun.SharpeRatio,
			AnnualizedReturn:  latestRun.AnnualizedReturn,
			AnnualizedStdev:   latestRun.AnnualuzedStdev,
		})
	}

	c.JSON(200, out)
}
