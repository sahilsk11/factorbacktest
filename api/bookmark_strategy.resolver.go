package api

import (
	"factorbacktest/internal/db/models/postgres/public/model"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type bookmarkStrategyRequest struct {
	Expression        string `json:"expression"`
	Name              string `json:"name"`
	BacktestStart     string `json:"backtestStart"`
	BacktestEnd       string `json:"backtestEnd"`
	RebalanceInterval string `json:"rebalanceInterval"`
	NumAssets         int    `json:"numAssets"`
	AssetUniverse     string `json:"assetUniverse"`
	// whether to save or unsave strategy
	// if false but not found, will err
	Bookmark bool `json:"bookmark"`
}

func (m ApiHandler) isStrategyBookmarked(c *gin.Context) {
	// ignores bookmark field
	var requestBody bookmarkStrategyRequest
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

	// TODO - create util for validating strategy input
	// also consider making a domain version of it
	// then re-using in backtest

	newModel := model.SavedStrategy{
		StrategyName:      requestBody.Name,
		FactorExpression:  requestBody.Expression,
		BacktestStart:     backtestStartDate,
		BacktestEnd:       backtestEndDate,
		RebalanceInterval: requestBody.RebalanceInterval,
		NumAssets:         int32(requestBody.NumAssets),
		AssetUniverse:     requestBody.AssetUniverse,
		Bookmarked:        requestBody.Bookmark,
		UserAccountID:     userAccountID,
	}

	existing, err := m.SavedStrategyRepository.ListMatchingStrategies(newModel)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	saved := false
	name := ""
	for _, ex := range existing {
		if ex.Bookmarked {
			name = ex.StrategyName
			saved = true
		}
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

	// TODO - create util for validating strategy input
	// also consider making a domain version of it
	// then re-using in backtest

	newModel := model.SavedStrategy{
		StrategyName:      requestBody.Name,
		FactorExpression:  requestBody.Expression,
		BacktestStart:     backtestStartDate,
		BacktestEnd:       backtestEndDate,
		RebalanceInterval: requestBody.RebalanceInterval,
		NumAssets:         int32(requestBody.NumAssets),
		AssetUniverse:     requestBody.AssetUniverse,
		Bookmarked:        requestBody.Bookmark,
		UserAccountID:     userAccountID,
	}

	existing, err := m.SavedStrategyRepository.ListMatchingStrategies(newModel)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	if len(existing) == 0 {
		err := m.SavedStrategyRepository.Add(newModel)
		if err != nil {
			returnErrorJson(err, c)
			return
		}
		out := map[string]string{
			"message": "ok",
		}

		c.JSON(200, out)
		return
	}
	if newModel.Bookmarked {
		// if any of the existing records are bookmarked
		// do nothing
		for _, ex := range existing {
			if ex.Bookmarked {
				out := map[string]string{
					"message": "ok",
				}

				c.JSON(200, out)
				return
			}
		}

		// just bookmark the first one
		ex := existing[0]
		err = m.SavedStrategyRepository.SetBookmarked(ex.SavedStragyID, newModel.Bookmarked)
		if err != nil {
			returnErrorJson(err, c)
			return
		}
	}
	// disable bookmarks on anything that's currently saved
	for _, ex := range existing {
		if ex.Bookmarked {
			err = m.SavedStrategyRepository.SetBookmarked(ex.SavedStragyID, newModel.Bookmarked)
			if err != nil {
				returnErrorJson(err, c)
				return
			}
		}
	}

	out := map[string]string{
		"message": "ok",
	}

	c.JSON(200, out)
}
