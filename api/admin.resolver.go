package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) checkQuantityMismatch(ctx *gin.Context) {
	result, err := m.InvestmentService.CheckQuantityMismatch(ctx.Request.Context())
	if err != nil {
		returnErrorJson(err, ctx)
		return
	}

	ctx.JSON(http.StatusOK, result)
}
