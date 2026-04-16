package api

import (
	"github.com/gin-gonic/gin"
)

func (m ApiHandler) updateOrders(c *gin.Context) {
	err := m.TradingService.UpdateAllPendingOrders(c.Request.Context())
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	c.JSON(200, gin.H{"success": "true"})
}
