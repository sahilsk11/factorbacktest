package api

import (
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"fmt"
	"time"

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
	log := logger.FromContext(c)

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

	if len(requestBody.Assets) == 0 {
		returnErrorJson(fmt.Errorf("no assets provided"), c)
		return
	}

	universe, err := m.AssetUniverseRepository.GetOrCreate(tx, requestBody.UniverseName)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	tickers := []model.Ticker{}
	symbols := []string{}
	for _, asset := range requestBody.Assets {
		ticker, err := m.TickerRepository.GetOrCreate(tx, model.Ticker{
			Symbol: asset.Symbol,
			Name:   asset.Name,
		})
		if err != nil {
			returnErrorJson(err, c)
			return
		}
		symbols = append(symbols, ticker.Symbol)
		tickers = append(tickers, *ticker)
	}

	// verify ticker is real
	for _, ticker := range tickers {
		// try getting price on some random day to check if we already track the price
		_, err = m.PriceRepository.Get(ticker.Symbol, time.Date(2024, 06, 12, 0, 0, 0, 0, time.UTC))
		if err == nil {
			log.Warnf("skipping %s", ticker.Symbol)
			continue
		}
		err = data.IngestPrices(tx, ticker.Symbol, m.PriceRepository, nil)
		if err != nil {
			returnErrorJson(fmt.Errorf("failed to ingest prices: %w", err), c)
			return
		}

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
