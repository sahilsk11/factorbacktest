package api

import (
	"context"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) rebalance(ctx *gin.Context) {
	profile, endProfile := domain.NewProfile()
	defer endProfile()
	c := context.WithValue(ctx, domain.ContextProfileKey, profile)

	result, err := m.InvestmentService.Rebalance(c)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to rebalance: %w", err), ctx)
		return
	}
	if result == nil {
		result = &service.RebalanceResult{}
	}
	if result.DriftIssues == nil {
		result.DriftIssues = []service.ReconIssue{}
	}

	ctx.JSON(200, gin.H{
		"success":     "true",
		"driftIssues": result.DriftIssues,
	})
}
