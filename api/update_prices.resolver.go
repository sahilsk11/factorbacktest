package api

import (
	"factorbacktest/internal/data"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) updatePrices(c *gin.Context) {
	tx, err := m.Db.Begin()
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	err = data.UpdateUniversePrices(tx, m.TickerRepository, m.PriceRepository)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	err = tx.Commit()
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := map[string]string{
		"message": "ok",
	}

	c.JSON(200, out)
}
