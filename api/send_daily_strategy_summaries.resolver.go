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
// via Lambda/API Gateway. It delegates to the StrategySummaryApp to handle
// the orchestration logic.
func (m ApiHandler) sendDailyStrategySummaries(c *gin.Context) {
	lg := logger.FromContext(c)

	// TODO: Implement:
	// ctx := c.Request.Context()
	// 1. Call m.StrategySummaryApp.SendDailyStrategySummaries(ctx)
	// 2. Handle errors appropriately
	// 3. Return response with summary of emails sent/failed
	//    (app layer should return this info, or we track it here)

	// Placeholder response
	response := SendDailyStrategySummariesResponse{
		Message:      "Daily strategy summaries processing completed",
		EmailsSent:   0,
		EmailsFailed: 0,
	}

	lg.Info("daily strategy summaries endpoint called")
	c.JSON(200, response)
}
