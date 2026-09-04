package api

import (
	"errors"
	"fmt"
	"net/http"

	"factorbacktest/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (m ApiHandler) adminReconcile(ctx *gin.Context) {
	result, err := m.InvestmentService.RunReconcile(ctx.Request.Context())
	if err != nil {
		returnErrorJson(err, ctx)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (m ApiHandler) adminReplaceHoldings(ctx *gin.Context) {
	investmentID, err := uuid.Parse(ctx.Param("investmentID"))
	if err != nil {
		returnErrorJsonCode(fmt.Errorf("invalid investment id"), ctx, http.StatusBadRequest)
		return
	}

	var req service.AdminReplaceHoldingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		returnErrorJsonCode(fmt.Errorf("invalid request body: %w", err), ctx, http.StatusBadRequest)
		return
	}

	result, err := m.InvestmentService.AdminReplaceHoldings(ctx.Request.Context(), investmentID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAdminReplaceHoldingsInvestmentNotFound):
			returnErrorJsonCode(err, ctx, http.StatusNotFound)
		case errors.Is(err, service.ErrAdminReplaceHoldingsInvalidRequest):
			returnErrorJsonCode(err, ctx, http.StatusBadRequest)
		default:
			returnErrorJson(err, ctx)
		}
		return
	}

	ctx.JSON(http.StatusOK, result)
}
