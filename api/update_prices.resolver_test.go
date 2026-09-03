package api

import (
	"context"
	"encoding/json"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type updatePricesAssetUniverseStub struct {
	repository.AssetUniverseRepository
	getAssets func(string) ([]model.Ticker, error)
}

func (s updatePricesAssetUniverseStub) GetAssets(assetUniverseName string) ([]model.Ticker, error) {
	return s.getAssets(assetUniverseName)
}

type updatePricesServiceStub struct {
	data.PriceService
	updatePrices func(context.Context, []string, repository.AdjustedPriceRepository) (data.PriceUpdateResult, error)
}

func (s updatePricesServiceStub) UpdatePrices(
	ctx context.Context,
	symbols []string,
	adjPricesRepository repository.AdjustedPriceRepository,
) (data.PriceUpdateResult, error) {
	return s.updatePrices(ctx, symbols, adjPricesRepository)
}

func newUpdatePricesCronRouter(t *testing.T, handler ApiHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	cron := engine.Group("/internal/cron")
	cron.Use(handler.requireCronSecret)
	cron.POST("/updatePrices", handler.updatePrices)
	return engine
}

func TestUpdatePricesCronRouteRequiresSecret(t *testing.T) {
	t.Setenv("CRON_SECRET", "test-cron-secret")

	handler := ApiHandler{
		AssetUniverseRepository: updatePricesAssetUniverseStub{
			getAssets: func(string) ([]model.Ticker, error) {
				return []model.Ticker{{Symbol: "AAPL"}}, nil
			},
		},
		PriceService: updatePricesServiceStub{
			updatePrices: func(context.Context, []string, repository.AdjustedPriceRepository) (data.PriceUpdateResult, error) {
				return data.PriceUpdateResult{UpdatedSymbols: []string{"AAPL", "SPY"}}, nil
			},
		},
	}
	engine := newUpdatePricesCronRouter(t, handler)

	t.Run("forbidden without secret", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cron/updatePrices", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("forbidden with wrong secret", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cron/updatePrices", nil)
		req.Header.Set("X-Cron-Secret", "wrong-secret")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("ingests universe symbols and SPY with valid secret", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cron/updatePrices", nil)
		req.Header.Set("X-Cron-Secret", "test-cron-secret")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)

		var response UpdatePricesResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		require.Equal(t, 2, response.NumUpdatedAssets)
		require.Empty(t, response.FailedSymbols)
	})
}

func TestUpdatePricesBuildsSymbolList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotSymbols []string
	handler := ApiHandler{
		AssetUniverseRepository: updatePricesAssetUniverseStub{
			getAssets: func(assetUniverseName string) ([]model.Ticker, error) {
				require.Equal(t, "ALL", assetUniverseName)
				return []model.Ticker{
					{Symbol: "AAPL"},
					{Symbol: repository.CASH_SYMBOL},
					{Symbol: "MSFT"},
				}, nil
			},
		},
		PriceService: updatePricesServiceStub{
			updatePrices: func(_ context.Context, symbols []string, _ repository.AdjustedPriceRepository) (data.PriceUpdateResult, error) {
				gotSymbols = symbols
				return data.PriceUpdateResult{UpdatedSymbols: symbols}, nil
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/internal/cron/updatePrices", nil)

	handler.updatePrices(ctx)

	require.Equal(t, []string{"AAPL", "MSFT", "SPY"}, gotSymbols)
	require.Equal(t, http.StatusOK, recorder.Code)
}
