package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"factorbacktest/internal/calculator"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newRefreshPublishedStrategiesRouter(t *testing.T, handler ApiHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	cron := engine.Group("/internal/cron")
	cron.Use(handler.requireCronSecret)
	cron.POST("/refreshPublishedStrategies", handler.refreshPublishedStrategies)
	return engine
}

func TestRefreshPublishedStrategiesCronRouteRequiresSecret(t *testing.T) {
	t.Setenv("CRON_SECRET", "test-cron-secret")
	engine := newRefreshPublishedStrategiesRouter(t, ApiHandler{})

	t.Run("forbidden without secret", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cron/refreshPublishedStrategies", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("forbidden with wrong secret", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cron/refreshPublishedStrategies", nil)
		req.Header.Set("X-Cron-Secret", "wrong-secret")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("starts with valid secret", func(t *testing.T) {
		listed := make(chan struct{})
		handler := ApiHandler{
			StrategyRepository: strategyRepoStub{
				list: func(repository.StrategyListFilter) ([]model.Strategy, error) {
					close(listed)
					return nil, nil
				},
			},
		}
		engine := newRefreshPublishedStrategiesRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cron/refreshPublishedStrategies", nil)
		req.Header.Set("X-Cron-Secret", "test-cron-secret")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)

		var out refreshPublishedStrategiesResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &out))
		require.Equal(t, "started", out.Status)

		select {
		case <-listed:
		case <-time.After(2 * time.Second):
			t.Fatal("refresh goroutine did not list published strategies")
		}
	})
}

func TestRefreshPublishedStrategyRunsEmptyList(t *testing.T) {
	handler := ApiHandler{
		StrategyRepository: strategyRepoStub{
			list: func(filter repository.StrategyListFilter) ([]model.Strategy, error) {
				require.NotNil(t, filter.Published)
				require.True(t, *filter.Published)
				return []model.Strategy{}, nil
			},
			addRun: func(model.StrategyRun) (*model.StrategyRun, error) {
				t.Fatal("AddRun should not run when there are no published strategies")
				return nil, nil
			},
		},
	}

	out := handler.refreshPublishedStrategyRuns(t.Context())
	require.Equal(t, "ok", out.Status)
	require.Equal(t, 0, out.Refreshed)
	require.Empty(t, out.Failed)
}

func TestPersistPublishedStrategyRunStoresResultJSON(t *testing.T) {
	strategyID := uuid.New()
	var got model.StrategyRun
	handler := ApiHandler{
		StrategyRepository: strategyRepoStub{
			addRun: func(m model.StrategyRun) (*model.StrategyRun, error) {
				got = m
				return &m, nil
			},
		},
	}

	sharpe := 1.1
	resp := &BacktestResponse{
		FactorName:  "Value",
		StrategyID:  strategyID,
		Snapshots:   map[string]service.BacktestSnapshot{"2026-01-02": {Value: 10100, Date: "2026-01-02"}},
		SharpeRatio: &sharpe,
	}
	metrics := &calculator.CalculateMetricsResult{SharpeRatio: 1.1, AnnualizedReturn: 0.2, AnnualizedStdev: 0.1}
	start := time.Date(2023, 9, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	err := handler.persistPublishedStrategyRun(model.Strategy{StrategyID: strategyID}, start, end, resp, metrics)
	require.NoError(t, err)
	require.Equal(t, strategyID, got.StrategyID)
	require.Equal(t, start, got.StartDate)
	require.Equal(t, end, got.EndDate)
	require.NotNil(t, got.Result)
	require.NotNil(t, got.SharpeRatio)
	require.InDelta(t, 1.1, *got.SharpeRatio, 1e-9)

	unmarshaled, err := unmarshalBacktestResult(got.Result)
	require.NoError(t, err)
	require.NotNil(t, unmarshaled)
	require.Equal(t, "Value", unmarshaled.FactorName)
	require.Equal(t, 10100.0, unmarshaled.Snapshots["2026-01-02"].Value)
}
