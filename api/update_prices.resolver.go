package api

import (
	"factorbacktest/internal/repository"
	"github.com/gin-gonic/gin"
)

type UpdatePricesResponse struct {
	NumUpdatedAssets int      `json:"numUpdatedAssets"`
	FailedSymbols    []string `json:"failedSymbols"`
}

func (m ApiHandler) updatePrices(c *gin.Context) {
	assets, err := m.AssetUniverseRepository.GetAssets("ALL")
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	symbols := make([]string, 0, len(assets)+1)
	for _, asset := range assets {
		if asset.Symbol != repository.CASH_SYMBOL {
			symbols = append(symbols, asset.Symbol)
		}
	}
	// SPY is the benchmark and is intentionally ingested even if no strategy
	// universe happens to contain it.
	symbols = append(symbols, "SPY")

	result, err := m.PriceService.UpdatePrices(c, symbols, m.PriceRepository)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := UpdatePricesResponse{
		NumUpdatedAssets: len(result.UpdatedSymbols),
		FailedSymbols:    result.FailedSymbols,
	}

	c.JSON(200, out)
}
