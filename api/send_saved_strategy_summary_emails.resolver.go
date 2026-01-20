package api

import (
	"context"
	apimodels "factorbacktest/api/models"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"fmt"

	"github.com/gin-gonic/gin"
)


func (m ApiHandler) sendSavedStrategySummaryEmails(c *gin.Context) {
	lg := logger.FromContext(c)
	ctx := c.Request.Context()

	// Create performance profile (required by some services)
	profile, endProfile := domain.NewProfile()
	defer endProfile()
	ctx = context.WithValue(ctx, domain.ContextProfileKey, profile)

	// Add logger to context
	ctx = context.WithValue(ctx, logger.ContextKey, lg)

	// TODO: Implement handler logic

	lg.Info("handler completed successfully")
	c.JSON(200, apimodels.SendSavedStrategySummaryEmailsResponse{
		// TODO: Populate response
	})
}
