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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type runReconcileServiceStub struct {
	service.InvestmentService
	runReconcile         func(context.Context) (*service.ReconcileResult, error)
	adminReplaceHoldings func(context.Context, uuid.UUID, service.AdminReplaceHoldingsRequest) (*service.AdminReplaceHoldingsResponse, error)
	adminAdjustHoldings  func(context.Context, uuid.UUID, service.AdminAdjustHoldingsRequest) (*service.AdminReplaceHoldingsResponse, error)
	adminSellBrokerDust  func(context.Context, service.AdminSellBrokerDustRequest) (*service.AdminSellBrokerDustResponse, error)
}

func (s runReconcileServiceStub) RunReconcile(ctx context.Context) (*service.ReconcileResult, error) {
	return s.runReconcile(ctx)
}

func (s runReconcileServiceStub) AdminReplaceHoldings(ctx context.Context, investmentID uuid.UUID, req service.AdminReplaceHoldingsRequest) (*service.AdminReplaceHoldingsResponse, error) {
	if s.adminReplaceHoldings != nil {
		return s.adminReplaceHoldings(ctx, investmentID, req)
	}
	return nil, nil
}

func (s runReconcileServiceStub) AdminAdjustHoldings(ctx context.Context, investmentID uuid.UUID, req service.AdminAdjustHoldingsRequest) (*service.AdminReplaceHoldingsResponse, error) {
	if s.adminAdjustHoldings != nil {
		return s.adminAdjustHoldings(ctx, investmentID, req)
	}
	return nil, nil
}

func (s runReconcileServiceStub) AdminSellBrokerDust(ctx context.Context, req service.AdminSellBrokerDustRequest) (*service.AdminSellBrokerDustResponse, error) {
	if s.adminSellBrokerDust != nil {
		return s.adminSellBrokerDust(ctx, req)
	}
	return nil, nil
}

func newAdminRouter(t *testing.T, handler ApiHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	admin := engine.Group("/internal/admin")
	admin.Use(handler.requireAdminApiKey)
	admin.POST("/reconcile", handler.adminReconcile)
	admin.POST("/investments/:investmentID/holdings", handler.adminReplaceHoldings)
	admin.POST("/investments/:investmentID/holdings/adjust", handler.adminAdjustHoldings)
	admin.POST("/broker/dust-sells", handler.adminSellBrokerDust)
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
				ledgerQty := 1.197
				alpacaQty := 0.764
				diff := alpacaQty - ledgerQty
				dollar := diff * 20
				return &service.ReconcileResult{
					Status:    service.ReconStatusDustOnly,
					CheckedAt: checkedAt,
					Issues: []service.ReconIssue{
						{
							Type:        service.ReconIssueQuantityMismatch,
							Severity:    service.ReconSeverityDust,
							Symbol:      "INTC",
							LedgerQty:   &ledgerQty,
							AlpacaQty:   &alpacaQty,
							Diff:        &diff,
							DollarValue: &dollar,
							Message:     "QUANTITY_MISMATCH INTC: ledger 1.197 vs alpaca 0.764 (diff -0.433) (~$8.66)",
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
	require.Equal(t, service.ReconStatusDustOnly, response.Status)
	require.Len(t, response.Issues, 1)
	require.Nil(t, response.Issues[0].InvestmentID)
	require.Equal(t, service.ReconIssueQuantityMismatch, response.Issues[0].Type)
	require.Equal(t, service.ReconSeverityDust, response.Issues[0].Severity)
	require.Equal(t, "INTC", response.Issues[0].Symbol)
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
	rebalance func(context.Context) (*service.RebalanceResult, error)
}

func (s adminRebalanceServiceStub) Rebalance(ctx context.Context) (*service.RebalanceResult, error) {
	return s.rebalance(ctx)
}

func TestAdminRebalanceRequiresApiKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")

	called := false
	handler := ApiHandler{
		InvestmentService: adminRebalanceServiceStub{
			rebalance: func(context.Context) (*service.RebalanceResult, error) {
				called = true
				return &service.RebalanceResult{
					DriftIssues: []service.ReconIssue{
						{
							Type:     service.ReconIssuePhantomPosition,
							Severity: service.ReconSeverityDrift,
							Symbol:   "VST",
							Message:  "PHANTOM_POSITION VST",
						},
					},
				}, nil
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

		var response struct {
			Success     string               `json:"success"`
			DriftIssues []service.ReconIssue `json:"driftIssues"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		require.Equal(t, "true", response.Success)
		require.Len(t, response.DriftIssues, 1)
		require.Equal(t, service.ReconIssuePhantomPosition, response.DriftIssues[0].Type)
		require.Equal(t, service.ReconSeverityDrift, response.DriftIssues[0].Severity)
		require.Equal(t, "VST", response.DriftIssues[0].Symbol)
	})
}

func TestAdminReplaceHoldingsRequiresApiKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")
	investmentID := uuid.New()

	handler := ApiHandler{
		InvestmentService: runReconcileServiceStub{
			adminReplaceHoldings: func(context.Context, uuid.UUID, service.AdminReplaceHoldingsRequest) (*service.AdminReplaceHoldingsResponse, error) {
				return &service.AdminReplaceHoldingsResponse{VersionID: uuid.New()}, nil
			},
		},
	}
	engine := newAdminRouter(t, handler)
	path := "/internal/admin/investments/" + investmentID.String() + "/holdings"

	t.Run("forbidden without key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"note":"x","cash":1,"positions":[]}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("forbidden with wrong key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"note":"x","cash":1,"positions":[]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Admin-Api-Key", "wrong-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})
}

func TestAdminAdjustAndDustSellRequireApiKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")
	investmentID := uuid.New()
	engine := newAdminRouter(t, ApiHandler{
		InvestmentService: runReconcileServiceStub{
			adminAdjustHoldings: func(context.Context, uuid.UUID, service.AdminAdjustHoldingsRequest) (*service.AdminReplaceHoldingsResponse, error) {
				t.Fatal("should not be called without a valid key")
				return nil, nil
			},
			adminSellBrokerDust: func(context.Context, service.AdminSellBrokerDustRequest) (*service.AdminSellBrokerDustResponse, error) {
				t.Fatal("should not be called without a valid key")
				return nil, nil
			},
		},
	})

	adjustPath := "/internal/admin/investments/" + investmentID.String() + "/holdings/adjust"
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, adjustPath, strings.NewReader(`{"note":"x","zeroOutSymbols":["VST"]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)

	recorder = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/internal/admin/broker/dust-sells", strings.NewReader(`{"symbols":["XYZ"]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
