package api

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type investInStrategyRequest struct {
	StrategyID string `json:"strategyID"`
	Amount     int    `json:"amountDollars"`
}

func (m ApiHandler) investInStrategy(c *gin.Context) {
	ctx := context.Background()

	var requestBody investInStrategyRequest
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	strategyID, err := uuid.Parse(requestBody.StrategyID)
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

	err = m.StrategyService.Save(strategyID)
	if !ok {
		returnErrorJson(fmt.Errorf("failed to save strategy: %w", err), c)
		return
	}

	err = m.InvestmentService.Add(ctx, userAccountID, strategyID, requestBody.Amount)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := map[string]bool{
		"success": true,
	}

	c.JSON(200, out)
}
