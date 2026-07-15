package api

import (
	"errors"
	"factorbacktest/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (m ApiHandler) previewReconciliation(c *gin.Context) {
	preview, err := m.InvestmentService.PreviewReconciliation(c.Request.Context())
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	c.JSON(http.StatusCreated, preview)
}

func (m ApiHandler) applyReconciliation(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("reconciliationRunID"))
	if err != nil {
		returnErrorJsonCode(errors.New("invalid reconciliation run id"), c, http.StatusBadRequest)
		return
	}
	if err := m.InvestmentService.ApplyReconciliation(c.Request.Context(), runID); err != nil {
		if errors.Is(err, service.ErrStaleReconciliation) {
			returnErrorJsonCode(err, c, http.StatusConflict)
			return
		}
		returnErrorJson(err, c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
