package api

import (
	"context"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) updatePrices(c *gin.Context) {
	universe, err := m.BacktestHandler.UniverseRepository.List(m.Db)
	universeSymbols := []string{}
	for _, u := range universe {
		universeSymbols = append(universeSymbols, u.Symbol)
	}

	if err != nil {
		returnErrorJson(err, c)
		return
	}

	err = m.PriceService.UpdatePricesIfNeeded(context.Background(), universeSymbols)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := map[string]string{
		"message": "ok",
	}

	c.JSON(200, out)
}
