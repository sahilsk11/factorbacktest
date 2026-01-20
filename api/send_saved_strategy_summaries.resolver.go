package api

import (
	"context"
	apimodels "factorbacktest/api/models"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"fmt"

	"github.com/gin-gonic/gin"
)

// sendSavedStrategySummaryEmailsHandler is the API endpoint that EventBridge will trigger
// via Lambda/API Gateway. It delegates to the StrategySummaryApp to handle
// the orchestration logic.
func (m ApiHandler) sendSavedStrategfySummaryEmails(c *gin.Context) {
	lg := logger.FromContext(c)
	ctx := c.Request.Context()

	// Create performance profile (required by FactorExpressionService)
	profile, endProfile := domain.NewProfile()
	defer endProfile()
	ctx = context.WithValue(ctx, domain.ContextProfileKey, profile)

	// Add logger to context
	ctx = context.WithValue(ctx, logger.ContextKey, lg)

	// Call the app layer to process and send daily strategy summaries
	err := m.StrategySummaryApp.SendSavedStrategySummaryEmails(ctx)
	if err != nil {
		lg.Errorf("failed to send daily strategy summaries: %v", err)
		c.JSON(500, apimodels.SendSavedStrategySummaryEmailsResponse{
			Message: fmt.Sprintf("Failed to process saved strategy summary emails: %v", err),
		})
		return
	}

	lg.Info("saved strategy summary emails processing completed successfully")
	c.JSON(200, apimodels.SendSavedStrategySummaryEmailsResponse{
		Message: "Saved strategy summary emails processing completed",
	})
}
