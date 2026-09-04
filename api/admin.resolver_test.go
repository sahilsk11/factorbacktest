package api

import (
	"context"
	"encoding/json"
	"factorbacktest/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type checkQuantityMismatchServiceStub struct {
	service.InvestmentService
	checkQuantityMismatch func(context.Context) (*service.QuantityMismatchCheckResult, error)
}

func (s checkQuantityMismatchServiceStub) CheckQuantityMismatch(ctx context.Context) (*service.QuantityMismatchCheckResult, error) {
	return s.checkQuantityMismatch(ctx)
}

func newAdminRouter(t *testing.T, handler ApiHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	admin := engine.Group("/internal/admin")
	admin.Use(handler.requireAdminApiKey)
	admin.POST("/checks/quantity-mismatch", handler.checkQuantityMismatch)
	return engine
}

func TestCheckQuantityMismatchRequiresAdminApiKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")

	handler := ApiHandler{
		InvestmentService: checkQuantityMismatchServiceStub{
			checkQuantityMismatch: func(context.Context) (*service.QuantityMismatchCheckResult, error) {
				return &service.QuantityMismatchCheckResult{
					Status:    "OK",
					CheckedAt: time.Now().UTC(),
				}, nil
			},
		},
	}
	engine := newAdminRouter(t, handler)

	t.Run("forbidden without key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/admin/checks/quantity-mismatch", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("forbidden with wrong key", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/admin/checks/quantity-mismatch", nil)
		req.Header.Set("X-Admin-Api-Key", "wrong-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})
}

func TestCheckQuantityMismatchOK(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")
	checkedAt := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)

	handler := ApiHandler{
		InvestmentService: checkQuantityMismatchServiceStub{
			checkQuantityMismatch: func(context.Context) (*service.QuantityMismatchCheckResult, error) {
				return &service.QuantityMismatchCheckResult{
					Status:     "OK",
					CheckedAt:  checkedAt,
					Mismatches: []service.QuantityMismatch{},
				}, nil
			},
		},
	}
	engine := newAdminRouter(t, handler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/admin/checks/quantity-mismatch", nil)
	req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
	engine.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response service.QuantityMismatchCheckResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "OK", response.Status)
	require.Empty(t, response.Mismatches)
}

func TestCheckQuantityMismatchINTCShortage(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")

	handler := ApiHandler{
		InvestmentService: checkQuantityMismatchServiceStub{
			checkQuantityMismatch: func(context.Context) (*service.QuantityMismatchCheckResult, error) {
				return &service.QuantityMismatchCheckResult{
					Status:    "MISMATCH",
					CheckedAt: time.Now().UTC(),
					Mismatches: []service.QuantityMismatch{
						{
							Symbol:    "INTC",
							LedgerQty: 1.197,
							BrokerQty: 0.764,
							Delta:     -0.433,
							Kind:      service.QuantityMismatchShortage,
						},
					},
				}, nil
			},
		},
	}
	engine := newAdminRouter(t, handler)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/admin/checks/quantity-mismatch", nil)
	req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
	engine.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response service.QuantityMismatchCheckResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "MISMATCH", response.Status)
	require.Len(t, response.Mismatches, 1)
	require.Equal(t, "INTC", response.Mismatches[0].Symbol)
	require.Equal(t, service.QuantityMismatchShortage, response.Mismatches[0].Kind)
	require.InDelta(t, 1.197, response.Mismatches[0].LedgerQty, 1e-9)
	require.InDelta(t, 0.764, response.Mismatches[0].BrokerQty, 1e-9)
	require.InDelta(t, -0.433, response.Mismatches[0].Delta, 1e-9)
}
