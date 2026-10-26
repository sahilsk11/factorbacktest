package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newPublishedBacktestRouter(t *testing.T, handler ApiHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/publishedStrategies/:strategyID/backtest", handler.getPublishedStrategyBacktest)
	return engine
}

func TestGetPublishedStrategyBacktest(t *testing.T) {
	strategyID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	runCreatedAt := time.Date(2026, 9, 28, 13, 30, 0, 0, time.UTC)

	published := &model.Strategy{
		StrategyID:        strategyID,
		StrategyName:      "Momentum",
		FactorExpression:  "momentumScore",
		RebalanceInterval: "weekly",
		NumAssets:         10,
		AssetUniverse:     "SPY_TOP_80",
		Published:         true,
		CreatedAt:         createdAt,
	}

	t.Run("400 on invalid id", func(t *testing.T) {
		engine := newPublishedBacktestRouter(t, ApiHandler{})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/publishedStrategies/not-a-uuid/backtest", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("404 when strategy is missing", func(t *testing.T) {
		handler := ApiHandler{
			StrategyRepository: strategyRepoStub{
				get: func(uuid.UUID) (*model.Strategy, error) {
					return nil, qrm.ErrNoRows
				},
			},
		}
		engine := newPublishedBacktestRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/publishedStrategies/"+strategyID.String()+"/backtest", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("404 when strategy is not published", func(t *testing.T) {
		unpublished := *published
		unpublished.Published = false
		handler := ApiHandler{
			StrategyRepository: strategyRepoStub{
				get: func(uuid.UUID) (*model.Strategy, error) {
					return &unpublished, nil
				},
			},
		}
		engine := newPublishedBacktestRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/publishedStrategies/"+strategyID.String()+"/backtest", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("200 with null result when no cached payload", func(t *testing.T) {
		handler := ApiHandler{
			StrategyRepository: strategyRepoStub{
				get: func(id uuid.UUID) (*model.Strategy, error) {
					require.Equal(t, strategyID, id)
					return published, nil
				},
				getLatestPublishedRun: func(id uuid.UUID) (*model.StrategyRun, error) {
					require.Equal(t, strategyID, id)
					return &model.StrategyRun{
						StrategyID: strategyID,
						StartDate:  time.Date(2023, 9, 28, 0, 0, 0, 0, time.UTC),
						EndDate:    time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC),
						CreatedAt:  runCreatedAt,
					}, nil
				},
			},
		}
		engine := newPublishedBacktestRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/publishedStrategies/"+strategyID.String()+"/backtest", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)

		var out getPublishedStrategyBacktestResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &out))
		require.Equal(t, "Momentum", out.StrategyName)
		require.Equal(t, "2023-09-28", out.BacktestStart)
		require.Equal(t, "2026-09-28", out.BacktestEnd)
		require.Nil(t, out.Result)
	})

	t.Run("200 with cached backtest payload", func(t *testing.T) {
		sharpe := 1.25
		cached := BacktestResponse{
			FactorName:       "Momentum",
			StrategyID:       strategyID,
			Snapshots:        map[string]service.BacktestSnapshot{"2026-01-02": {Date: "2026-01-02", Value: 11000}},
			SharpeRatio:      &sharpe,
			AnnualizedReturn: &sharpe,
		}
		raw, err := marshalBacktestResult(&cached)
		require.NoError(t, err)

		handler := ApiHandler{
			StrategyRepository: strategyRepoStub{
				get: func(uuid.UUID) (*model.Strategy, error) {
					return published, nil
				},
				getLatestPublishedRun: func(uuid.UUID) (*model.StrategyRun, error) {
					return &model.StrategyRun{
						StrategyID: strategyID,
						StartDate:  time.Date(2023, 9, 28, 0, 0, 0, 0, time.UTC),
						EndDate:    time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC),
						CreatedAt:  runCreatedAt,
						Result:     raw,
					}, nil
				},
			},
		}
		engine := newPublishedBacktestRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/publishedStrategies/"+strategyID.String()+"/backtest", nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)

		var out getPublishedStrategyBacktestResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &out))
		require.NotNil(t, out.Result)
		require.Equal(t, strategyID, out.Result.StrategyID)
		require.Equal(t, "Momentum", out.Result.FactorName)
		require.Equal(t, 11000.0, out.Result.Snapshots["2026-01-02"].Value)
		require.NotNil(t, out.Result.SharpeRatio)
		require.InDelta(t, 1.25, *out.Result.SharpeRatio, 1e-9)
	})
}
