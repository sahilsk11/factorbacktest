package api

import (
	"errors"
	"factorbacktest/internal/db/models/postgres/public/model"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type getPublishedStrategiesResponse struct {
	PublishedStrategyID uuid.UUID `json:"publishedStrategyID"`
	SavedStrategyID     uuid.UUID `json:"savedStrategyID"`
	StrategyName        string    `json:"strategyName"`
	RebalanceInterval   string    `json:"rebalanceInterval"`
	CreatedAt           time.Time `json:"createdAt"`
	FactorExpression    string    `json:"factorExpression"`
	NumAssets           int32     `json:"numAssets"`
	AssetUniverse       string    `json:"assetUniverse"`
	OneYearReturn       *float64  `json:"oneYearReturn"`
	TwoYearReturn       *float64  `json:"twoYearReturn"`
	FiveYearReturn      *float64  `json:"fiveYearReturn"`
	Diversification     *float64  `json:"diversification"`
	SharpeRatio         *float64  `json:"sharpeRatio"`
	AnnualizedReturn    *float64  `json:"annualizedReturn"`
	AnnualizedStdev     *float64  `json:"annualizedStandardDeviation"`
}

func (m ApiHandler) getPublishedStrategies(c *gin.Context) {
	publishedStrategies, err := m.PublishedStrategiesRepository.List()
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := []getPublishedStrategiesResponse{}

	for _, ps := range publishedStrategies {
		stats, err := m.PublishedStrategiesRepository.GetLatestStats(ps.PublishedStrategyID)
		if err != nil && errors.Is(err, qrm.ErrNoRows) {
			stats = &model.PublishedStrategyStats{}
		} else if err != nil {
			returnErrorJson(err, c)
			return
		}
		ss, err := m.SavedStrategyRepository.Get(ps.SavedStrategyID)
		if err != nil {
			returnErrorJson(err, c)
			return
		}

		out = append(out, getPublishedStrategiesResponse{
			SavedStrategyID:     ps.SavedStrategyID,
			PublishedStrategyID: ps.PublishedStrategyID,
			StrategyName:        ss.StrategyName,
			RebalanceInterval:   ss.RebalanceInterval,
			CreatedAt:           ps.CreatedAt,
			FactorExpression:    ss.FactorExpression,
			NumAssets:           ss.NumAssets,
			AssetUniverse:       ss.AssetUniverse,
			OneYearReturn:       stats.OneYearReturn,
			TwoYearReturn:       stats.TwoYearReturn,
			FiveYearReturn:      stats.FiveYearReturn,
			Diversification:     stats.Diversification,
			SharpeRatio:         stats.SharpeRatio,
			AnnualizedReturn:    stats.AnnualizedReturn,
			AnnualizedStdev:     stats.AnnualizedStdev,
		})
	}

	c.JSON(200, out)
}
