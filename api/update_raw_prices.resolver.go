package api

import (
	"fmt"
	"strings"
	"time"

	"factorbacktest/internal/repository"

	"github.com/gin-gonic/gin"
)

type UpdateRawPricesResponse struct {
	NumUpdatedAssets int      `json:"numUpdatedAssets"`
	FailedSymbols    []string `json:"failedSymbols"`
	Source           string   `json:"source"`
}

func (m ApiHandler) updateRawPrices(c *gin.Context) {
	if m.RawPriceService == nil {
		returnErrorJson(fmt.Errorf("raw price service is not configured"), c)
		return
	}

	start, err := parseOptionalDateQuery(c.Query("start"))
	if err != nil {
		returnErrorJsonCode(err, c, 400)
		return
	}

	symbols, err := m.rawPriceSymbols(c.Query("symbols"))
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	result, err := m.RawPriceService.IngestRawPrices(c, symbols, start)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	c.JSON(200, UpdateRawPricesResponse{
		NumUpdatedAssets: len(result.UpdatedSymbols),
		FailedSymbols:    result.FailedSymbols,
		Source:           result.Source,
	})
}

func (m ApiHandler) rawPriceSymbols(symbolsQuery string) ([]string, error) {
	if symbolsQuery != "" {
		parts := strings.Split(symbolsQuery, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			symbol := strings.TrimSpace(strings.ToUpper(part))
			if symbol != "" && symbol != repository.CASH_SYMBOL {
				out = append(out, symbol)
			}
		}
		return out, nil
	}

	assets, err := m.AssetUniverseRepository.GetAssets("ALL")
	if err != nil {
		return nil, err
	}
	symbols := make([]string, 0, len(assets)+1)
	for _, asset := range assets {
		if asset.Symbol != repository.CASH_SYMBOL {
			symbols = append(symbols, asset.Symbol)
		}
	}
	symbols = append(symbols, "SPY")
	return symbols, nil
}

func parseOptionalDateQuery(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("invalid start date %q: want YYYY-MM-DD", value)
	}
	utc := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &utc, nil
}
