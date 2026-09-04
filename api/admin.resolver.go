package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) adminReconcile(ctx *gin.Context) {
	result, err := m.InvestmentService.RunReconcile(ctx.Request.Context())
	if err != nil {
		returnErrorJson(err, ctx)
		return
	}

	ctx.JSON(http.StatusOK, result)
}
