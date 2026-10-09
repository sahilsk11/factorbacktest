package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultRequestTimeout = 2 * time.Minute

	auditInsertTimeout = 5 * time.Second
	auditUpdateTimeout = 5 * time.Second

	healthDBPingTimeout = 3 * time.Second
)

// requestTimeout returns the server-side deadline for a route. Long-running cron
// jobs and backtests get higher budgets; liveness paths stay short.
func requestTimeout(method, path string) time.Duration {
	switch {
	case method == http.MethodGet && path == "/":
		return 10 * time.Second
	case method == http.MethodGet && path == "/health":
		return 10 * time.Second
	case path == "/backtest/stream":
		return 3 * time.Hour
	case path == "/backtest" || path == "/benchmark":
		return 45 * time.Minute
	case path == "/internal/cron/rebalance" || path == "/internal/admin/rebalance":
		return 2 * time.Hour
	case path == "/internal/cron/updatePrices" || path == "/internal/admin/updatePrices":
		return 45 * time.Minute
	case path == "/internal/cron/sendSavedStrategySummaryEmails":
		return 45 * time.Minute
	case path == "/internal/cron/updateOrders" || path == "/internal/admin/updateOrders":
		return 12 * time.Minute
	default:
		return defaultRequestTimeout
	}
}

func requestTimeoutMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		timeout := requestTimeout(c.Request.Method, c.Request.URL.Path)
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if ctx.Err() == context.DeadlineExceeded && !c.IsAborted() && c.Writer.Status() == http.StatusOK {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "request timed out",
			})
		}
	}
}
