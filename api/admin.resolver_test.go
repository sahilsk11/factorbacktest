package api

import (
	"context"
	"encoding/json"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type runReconcileServiceStub struct {
	service.InvestmentService
	runReconcile func(context.Context) (*service.ReconcileResult, error)
}

func (s runReconcileServiceStub) RunReconcile(ctx context.Context) (*service.ReconcileResult, error) {
	return s.runReconcile(ctx)
}

func newAdminRouter(t *testing.T, handler ApiHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	admin := engine.Group("/internal/admin")
	admin.Use(handler.requireAdminApiKey)
	admin.POST("/reconcile", handler.adminReconcile)
	admin.POST("/updatePrices", handler.updatePrices)
	admin.POST("/rebalance", handler.rebalance)
	admin.POST("/updateOrders", handler.updateOrders)
	return engine
}

func TestAdminReconcileRequiresApiKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")

	handler := ApiHandler{
		InvestmentService: runReconcileServiceStub{
			runReconcile: func(context.Context) (*service.ReconcileResult, error) {
				return &service.ReconcileResult{Status: "OK", CheckedAt: time.Now().UTC()}, nil
			},
		},
	}
	engine := newAdminRouter(t, handler)

	t.Run("forbidden without key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/admin/reconcile", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("forbidden with wrong key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/admin/reconcile", nil)
		req.Header.Set("X-Admin-Api-Key", "wrong-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})
}

func TestAdminReconcileReturnsStructuredIssues(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")
	checkedAt := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)

	handler := ApiHandler{
		InvestmentService: runReconcileServiceStub{
			runReconcile: func(context.Context) (*service.ReconcileResult, error) {
				return &service.ReconcileResult{
					Status:    "ISSUES",
					CheckedAt: checkedAt,
					Issues: []service.ReconIssue{
						{
							Message: "alpaca account holding insufficient INTC: aggregate portfolio 1.197000 vs alpaca 0.764000 (-0.433000)",
						},
					},
				}, nil
			},
		},
	}
	engine := newAdminRouter(t, handler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/admin/reconcile", nil)
	req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
	engine.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response service.ReconcileResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "ISSUES", response.Status)
	require.Len(t, response.Issues, 1)
	require.Nil(t, response.Issues[0].InvestmentID)
	require.Contains(t, response.Issues[0].Message, "INTC")
}

func TestAdminUpdatePricesRequiresApiKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")

	handler := ApiHandler{
		AssetUniverseRepository: updatePricesAssetUniverseStub{
			getAssets: func(string) ([]model.Ticker, error) {
				return []model.Ticker{{Symbol: "AAPL"}}, nil
			},
		},
		PriceService: updatePricesServiceStub{
			updatePrices: func(context.Context, []string, repository.AdjustedPriceRepository) (data.PriceUpdateResult, error) {
				return data.PriceUpdateResult{UpdatedSymbols: []string{"AAPL"}}, nil
			},
		},
	}
	engine := newAdminRouter(t, handler)

	t.Run("forbidden without key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/admin/updatePrices", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("runs shared handler with valid key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/admin/updatePrices", nil)
		req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)

		var response UpdatePricesResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		require.Equal(t, 1, response.NumUpdatedAssets)
	})
}

type adminRebalanceServiceStub struct {
	service.InvestmentService
	rebalance func(context.Context) error
}

func (s adminRebalanceServiceStub) Rebalance(ctx context.Context) error {
	return s.rebalance(ctx)
}

func TestAdminRebalanceRequiresApiKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")

	called := false
	handler := ApiHandler{
		InvestmentService: adminRebalanceServiceStub{
			rebalance: func(context.Context) error {
				called = true
				return nil
			},
		},
	}
	engine := newAdminRouter(t, handler)

	t.Run("forbidden without key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/admin/rebalance", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("runs shared handler with valid key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/admin/rebalance", nil)
		req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.True(t, called)
	})
}
