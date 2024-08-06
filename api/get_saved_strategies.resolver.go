package api

import (
	"factorbacktest/internal/repository"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type getSavedStrategiesResponse struct {
	StrategyID        uuid.UUID `json:"strategyID"`
	StrategyName      string    `json:"strategyName"`
	RebalanceInterval string    `json:"rebalanceInterval"`
	Bookmarked        bool      `json:"bookmarked"`
	CreatedAt         time.Time `json:"createdAt"`
	FactorExpression  string    `json:"factorExpression"`
	NumAssets         int32     `json:"numAssets"`
	AssetUniverse     string    `json:"assetUniverse"`
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

	savedStrategies, err := m.StrategyRepository.List(repository.StrategyListFilter{
		SavedByUser: &userAccountID,
	})
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := []getSavedStrategiesResponse{}
	for _, s := range savedStrategies {
		out = append(out, getSavedStrategiesResponse{
			StrategyID:        s.StrategyID,
			StrategyName:      s.StrategyName,
			RebalanceInterval: s.RebalanceInterval,
			CreatedAt:         s.CreatedAt,
			FactorExpression:  s.FactorExpression,
			NumAssets:         s.NumAssets,
			AssetUniverse:     s.AssetUniverse,
		})
	}

	c.JSON(200, out)
}
