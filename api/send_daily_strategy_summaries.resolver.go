package api

import (
	"factorbacktest/internal/logger"
	"github.com/gin-gonic/gin"
)

type SendDailyStrategySummariesResponse struct {
	Message      string `json:"message"`
	EmailsSent   int    `json:"emailsSent"`
	EmailsFailed int    `json:"emailsFailed"`
}

// sendDailyStrategySummaries is the API endpoint that EventBridge will trigger
// via Lambda/API Gateway. It orchestrates sending daily strategy summary emails
// to all users with saved strategies.
func (m ApiHandler) sendDailyStrategySummaries(c *gin.Context) {
	lg := logger.FromContext(c)

	// TODO: Implement business logic:
	// ctx := c.Request.Context()
	// 1. Call EmailService.SendDailyStrategySummaries(ctx)
	// 2. Handle errors appropriately
	// 3. Return response with summary of emails sent/failed

	// Placeholder response
	response := SendDailyStrategySummariesResponse{
		Message:      "Daily strategy summaries processing completed",
		EmailsSent:   0,
		EmailsFailed: 0,
	}

	lg.Info("daily strategy summaries endpoint called")
	c.JSON(200, response)
}
