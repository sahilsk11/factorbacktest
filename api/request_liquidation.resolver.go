package api

import (
	"errors"
	"factorbacktest/internal/repository"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (m ApiHandler) endInvestment(c *gin.Context) {
	userAccountIDValue, ok := c.Get("userAccountID")
	if !ok {
		returnErrorJsonCode(fmt.Errorf("must be logged in to end an investment"), c, http.StatusUnauthorized)
		return
	}

	userAccountID, err := uuid.Parse(fmt.Sprint(userAccountIDValue))
	if err != nil {
		returnErrorJsonCode(fmt.Errorf("misformatted user account id"), c, http.StatusUnauthorized)
		return
	}
	investmentID, err := uuid.Parse(c.Param("investmentID"))
	if err != nil {
		returnErrorJsonCode(fmt.Errorf("invalid investment id"), c, http.StatusBadRequest)
		return
	}

	err = m.InvestmentService.End(c.Request.Context(), userAccountID, investmentID)
	if errors.Is(err, repository.ErrInvestmentNotFound) {
		returnErrorJsonCode(repository.ErrInvestmentNotFound, c, http.StatusNotFound)
		return
	}
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"success": true})
}
