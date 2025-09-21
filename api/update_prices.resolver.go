package api

import (
	"factorbacktest/internal/data"

	"github.com/gin-gonic/gin"
)

type UpdatePricesResponse struct {
	NumUpdatedAssets int `json:"numUpdatedAssets"`
}

func (m ApiHandler) updatePrices(c *gin.Context) {
	tx, err := m.Db.Begin()
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	numUpdatedAssets, err := data.UpdateUniversePrices(c, tx, m.TickerRepository, m.PriceRepository)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	err = tx.Commit()
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := UpdatePricesResponse{
		NumUpdatedAssets: numUpdatedAssets,
	}

	c.JSON(200, out)
}
