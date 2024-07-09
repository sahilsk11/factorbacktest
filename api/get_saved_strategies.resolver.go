package api

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type getSavedStrategiesResponse struct {
	SavedStragyID     uuid.UUID `json:"savedStrategyID"`
	StrategyName      string    `json:"strategyName"`
	RebalanceInterval string    `json:"rebalanceInterval"`
	Bookmarked        bool      `json:"bookmarked"`
	CreatedAt         time.Time `json:"createdAt"`
	FactorExpression  string    `json:"factorExpression"`
	// ModifiedAt        time.Time

	BacktestStart time.Time `json:"backtestStart"`
	BacktestEnd   time.Time `json:"backtestEnd"`
	NumAssets     int32     `json:"numAssets"`
	AssetUniverse string    `json:"assetUniverse"`
}

func (m ApiHandler) getSavedStrategies(c *gin.Context) {
	ginUserAccountID, ok := c.Get("userAccountID")
	if !ok {
		returnErrorJson(fmt.Errorf("must be logged in to view saved strategy"), c)
		return
	}
	userAccountIDStr, ok := ginUserAccountID.(string)
	if !ok {
		returnErrorJson(fmt.Errorf("misformatted user account id"), c)
		return
	}

	userAccountID, err := uuid.Parse(userAccountIDStr)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	savedStrategies, err := m.SavedStrategyRepository.List(userAccountID)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := []getSavedStrategiesResponse{}
	for _, s := range savedStrategies {
		out = append(out, getSavedStrategiesResponse{
			SavedStragyID:     s.SavedStragyID,
			StrategyName:      s.StrategyName,
			RebalanceInterval: s.RebalanceInterval,
			Bookmarked:        s.Bookmarked,
			CreatedAt:         s.CreatedAt,
			FactorExpression:  s.FactorExpression,
			BacktestStart:     s.BacktestStart,
			BacktestEnd:       s.BacktestEnd,
			NumAssets:         s.NumAssets,
			AssetUniverse:     s.AssetUniverse,
		})
	}

	c.JSON(200, out)
}
