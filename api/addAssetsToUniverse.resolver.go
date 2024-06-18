package api

import (
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

type asset struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

type addAssetsToUniverseRequest struct {
	UniverseName string  `json:"universeName"`
	Assets       []asset `json:"assets"`
}

func (m ApiHandler) addAssetsToUniverse(c *gin.Context) {
	tx, err := m.Db.Begin()
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	defer tx.Rollback()

	var requestBody addAssetsToUniverseRequest
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	universe, err := m.AssetUniverseRepository.GetOrCreate(tx, requestBody.UniverseName)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	tickers := []model.Ticker{}
	for _, asset := range requestBody.Assets {
		ticker, err := m.TickerRepository.GetOrCreate(tx, model.Ticker{
			Symbol: asset.Symbol,
			Name:   asset.Name,
		})
		if err != nil {
			returnErrorJson(err, c)
			return
		}
		err = service.IngestPrices(tx, ticker.Symbol, m.PriceRepository, nil)
		if err != nil {
			returnErrorJson(fmt.Errorf("failed to ingest prices: %w", err), c)
			return
		}
		tickers = append(tickers, *ticker)
	}

	err = m.AssetUniverseRepository.AddAssets(tx, *universe, tickers)
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
