package api

import (
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

type bookmarkStrategyRequest struct {
	Expression        string `json:"expression"`
	Name              string `json:"name"`
	RebalanceInterval string `json:"rebalanceInterval"`
	NumAssets         int    `json:"numAssets"`
	AssetUniverse     string `json:"assetUniverse"`
	// whether to save or unsave strategy
	// if false but not found, will err
	Bookmark bool `json:"bookmark"`
}

func (m ApiHandler) isStrategyBookmarked(c *gin.Context) {
	var requestBody bookmarkStrategyRequest
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	ginUserAccountID, ok := c.Get("userAccountID")
	if !ok {
		returnErrorJson(fmt.Errorf("must be logged in to save strategy"), c)
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

	newModel := model.Strategy{
		FactorExpression:  requestBody.Expression,
		RebalanceInterval: requestBody.RebalanceInterval,
		NumAssets:         int32(requestBody.NumAssets),
		AssetUniverse:     requestBody.AssetUniverse,
		UserAccountID:     userAccountID,
	}

	existing, err := m.StrategyRepository.GetIfBookmarked(newModel)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	saved := false
	name := ""
	if existing != nil {
		name = existing.StrategyName
		saved = true
	}

	out := map[string]interface{}{
		"name":         name,
		"isBookmarked": saved,
	}

	c.JSON(200, out)
}

func (m ApiHandler) bookmarkStrategy(c *gin.Context) {
	var requestBody bookmarkStrategyRequest
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	ginUserAccountID, ok := c.Get("userAccountID")
	if !ok {
		returnErrorJson(fmt.Errorf("must be logged in to save strategy"), c)
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

	newModel := model.Strategy{
		StrategyName:      requestBody.Name,
		FactorExpression:  requestBody.Expression,
		RebalanceInterval: requestBody.RebalanceInterval,
		NumAssets:         int32(requestBody.NumAssets),
		AssetUniverse:     requestBody.AssetUniverse,
		UserAccountID:     userAccountID,
		Saved:             requestBody.Bookmark,
	}

	existing, err := m.StrategyRepository.GetIfBookmarked(newModel)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	var strategyID uuid.UUID

	if existing != nil && requestBody.Bookmark {
		strategyID = existing.StrategyID
	} else if existing != nil {
		strategyID = existing.StrategyID
		newModel.StrategyID = existing.StrategyID
		_, err = m.StrategyRepository.Update(newModel, []postgres.Column{
			table.Strategy.Saved,
		})
		if err != nil {
			returnErrorJson(err, c)
			return
		}
	} else if requestBody.Bookmark {
		m, err := m.StrategyRepository.Add(newModel)
		if err != nil {
			returnErrorJson(err, c)
			return
		}
		strategyID = m.StrategyID
	}

	out := map[string]string{
		"message":         "ok",
		"savedStrategyID": strategyID.String(),
	}

	c.JSON(200, out)
}
