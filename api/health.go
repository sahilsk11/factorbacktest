package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthDBPingTimeout)
	defer cancel()
	if err := m.Db.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"db":     err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
