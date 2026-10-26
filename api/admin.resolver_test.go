package api

import (
	"context"
	"encoding/json"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"factorbacktest/internal/testseed"
	"factorbacktest/internal/util"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

type runReconcileServiceStub struct {
	service.InvestmentService
	runReconcile         func(context.Context) (*service.ReconcileResult, error)
	adminReplaceHoldings func(context.Context, uuid.UUID, service.AdminReplaceHoldingsRequest) (*service.AdminReplaceHoldingsResponse, error)
	adminUnpause         func(context.Context, uuid.UUID) (*service.AdminUnpauseResponse, error)
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

func (s runReconcileServiceStub) AdminUnpause(ctx context.Context, investmentID uuid.UUID) (*service.AdminUnpauseResponse, error) {
	if s.adminUnpause != nil {
		return s.adminUnpause(ctx, investmentID)
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
	admin.POST("/investments/:investmentID/unpause", handler.adminUnpauseInvestment)
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

func TestAdminUnpauseInvestment(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")
	investmentID := uuid.New()
	path := "/internal/admin/investments/" + investmentID.String() + "/unpause"

	t.Run("forbidden without key", func(t *testing.T) {
		handler := ApiHandler{InvestmentService: runReconcileServiceStub{
			adminUnpause: func(context.Context, uuid.UUID) (*service.AdminUnpauseResponse, error) {
				t.Fatal("unpause must not run without a valid admin key")
				return nil, nil
			},
		}}
		engine := newAdminRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("forbidden with wrong key", func(t *testing.T) {
		handler := ApiHandler{InvestmentService: runReconcileServiceStub{
			adminUnpause: func(context.Context, uuid.UUID) (*service.AdminUnpauseResponse, error) {
				t.Fatal("unpause must not run with a wrong admin key")
				return nil, nil
			},
		}}
		engine := newAdminRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-Admin-Api-Key", "wrong-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("success", func(t *testing.T) {
		handler := ApiHandler{InvestmentService: runReconcileServiceStub{
			adminUnpause: func(_ context.Context, got uuid.UUID) (*service.AdminUnpauseResponse, error) {
				require.Equal(t, investmentID, got)
				return &service.AdminUnpauseResponse{
					InvestmentID:  investmentID,
					Paused:        false,
					AlreadyActive: false,
				}, nil
			},
		}}
		engine := newAdminRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)

		var response service.AdminUnpauseResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		require.Equal(t, investmentID, response.InvestmentID)
		require.False(t, response.Paused)
		require.False(t, response.AlreadyActive)
	})

	t.Run("not found", func(t *testing.T) {
		handler := ApiHandler{InvestmentService: runReconcileServiceStub{
			adminUnpause: func(context.Context, uuid.UUID) (*service.AdminUnpauseResponse, error) {
				return nil, service.ErrAdminUnpauseInvestmentNotFound
			},
		}}
		engine := newAdminRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("already active is idempotent", func(t *testing.T) {
		handler := ApiHandler{InvestmentService: runReconcileServiceStub{
			adminUnpause: func(context.Context, uuid.UUID) (*service.AdminUnpauseResponse, error) {
				return &service.AdminUnpauseResponse{
					InvestmentID:  investmentID,
					Paused:        false,
					AlreadyActive: true,
				}, nil
			},
		}}
		engine := newAdminRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)

		var response service.AdminUnpauseResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		require.True(t, response.AlreadyActive)
		require.False(t, response.Paused)
	})

	t.Run("terminal state", func(t *testing.T) {
		handler := ApiHandler{InvestmentService: runReconcileServiceStub{
			adminUnpause: func(context.Context, uuid.UUID) (*service.AdminUnpauseResponse, error) {
				return nil, service.ErrAdminUnpauseTerminal
			},
		}}
		engine := newAdminRouter(t, handler)
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
		engine.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusConflict, recorder.Code)
	})
}

func TestAdminUnpauseInvestmentHTTP_RealDB(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-admin-api-key")
	db, err := util.NewTestDb()
	require.NoError(t, err)

	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: uuid.NewString() + "@example.com"})
	universeName := "ADMIN_UNPAUSE_HTTP_" + uuid.NewString()
	testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: universeName})
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "admin-unpause-http",
		UserAccountID:     user.UserAccountID,
		AssetUniverse:     universeName,
		NumAssets:         3,
		RebalanceInterval: "DAILY",
		FactorExpression:  "price()",
	})
	investment := testseed.CreateInvestment(db, testseed.InvestmentOpts{
		StrategyID:    strategy.StrategyID,
		UserAccountID: user.UserAccountID,
		AmountDollars: 100,
	})
	_, err = db.Exec(`UPDATE investment SET paused_at = now() WHERE investment_id = $1`, investment.InvestmentID)
	require.NoError(t, err)

	repo := repository.NewInvestmentRepository(db)
	handler := ApiHandler{
		InvestmentService: service.NewInvestmentService(
			db,
			repo,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			service.BacktestHandler{},
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		),
	}
	engine := newAdminRouter(t, handler)
	path := "/internal/admin/investments/" + investment.InvestmentID.String() + "/unpause"

	listed, err := repo.List(repository.StrategyInvestmentListFilter{UserAccountIDs: []uuid.UUID{user.UserAccountID}})
	require.NoError(t, err)
	require.Empty(t, listed)

	var tradeOrdersBefore, investmentTradesBefore int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM trade_order`).Scan(&tradeOrdersBefore))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM investment_trade`).Scan(&investmentTradesBefore))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("X-Admin-Api-Key", "test-admin-api-key")
	engine.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response service.AdminUnpauseResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, investment.InvestmentID, response.InvestmentID)
	require.False(t, response.Paused)
	require.False(t, response.AlreadyActive)

	listed, err = repo.List(repository.StrategyInvestmentListFilter{UserAccountIDs: []uuid.UUID{user.UserAccountID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, investment.InvestmentID, listed[0].InvestmentID)
	require.Nil(t, listed[0].PausedAt)

	var tradeOrdersAfter, investmentTradesAfter int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM trade_order`).Scan(&tradeOrdersAfter))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM investment_trade`).Scan(&investmentTradesAfter))
	require.Equal(t, tradeOrdersBefore, tradeOrdersAfter)
	require.Equal(t, investmentTradesBefore, investmentTradesAfter)
}
