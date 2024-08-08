package api

import (
	"context"
	"factorbacktest/internal/domain"
	"fmt"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) rebalance(ctx *gin.Context) {
	profile, endProfile := domain.NewProfile()
	defer endProfile()
	c := context.WithValue(ctx, domain.ContextProfileKey, profile)

	err := m.InvestmentService.Rebalance(c)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to rebalance: %w", err), ctx)
		return
	}

	out := map[string]string{
		"success": "true",
	}
	ctx.JSON(200, out)
}
