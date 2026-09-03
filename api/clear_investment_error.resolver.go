package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (m ApiHandler) clearInvestmentError(c *gin.Context) {
	investmentID, err := uuid.Parse(c.Param("investmentID"))
	if err != nil {
		returnErrorJsonCode(errors.New("invalid investment id"), c, http.StatusBadRequest)
		return
	}
	if err := m.InvestmentService.ClearInvestmentError(c.Request.Context(), investmentID); err != nil {
		returnErrorJson(err, c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
