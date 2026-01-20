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
	lg := logger.FromContext(c).With("handler", "sendSavedStrategySummaryEmails")
	ctx := c.Request.Context()

	// Create performance profile (required by some services)
	profile, endProfile := domain.NewProfile()
	defer endProfile()
	ctx = context.WithValue(ctx, domain.ContextProfileKey, profile)

	// Add logger to context
	ctx = context.WithValue(ctx, logger.ContextKey, lg)

	// TODO: Implement handler logic

	err := m.StrategySummaryApp.SendSavedStrategySummaryEmails(ctx)
	if err != nil {
		lg.Errorf("failed to send saved strategy summary emails: %v", err)
		c.JSON(500, apimodels.SendSavedStrategySummaryEmailsResponse{
			Message: fmt.Sprintf("Failed to send saved strategy summary emails: %v", err),
		})
		return
	}

	lg.Info("handler completed successfully")
	c.JSON(200, apimodels.SendSavedStrategySummaryEmailsResponse{
		Message: "Saved strategy summary emails sent successfully",
	})
}
