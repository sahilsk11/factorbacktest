package api

import (
	"context"
	"factorbacktest/internal/repository"

	"github.com/gin-gonic/gin"
)

type constructFactorEquationRequest struct {
	UserInput string `json:"input"`
}

// no one will ever know
type constructFactorEquationResponse repository.ConstructFactorEquationReponse

func (h ApiHandler) constructFactorEquation(c *gin.Context) {
	ctx := context.Background()
	var requestBody constructFactorEquationRequest

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	response, err := h.GptRepository.ConstructFactorEquation(
		ctx,
		requestBody.UserInput,
	)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	c.JSON(200, response)
}
