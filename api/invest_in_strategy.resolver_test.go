package api

import (
	"bytes"
	"context"
	"encoding/json"
	"factorbacktest/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type investInStrategyServiceStub struct {
	service.InvestmentService
	add func(context.Context, uuid.UUID, uuid.UUID, int) error
}

func (s investInStrategyServiceStub) Add(ctx context.Context, userAccountID, strategyID uuid.UUID, amount int) error {
	return s.add(ctx, userAccountID, strategyID, amount)
}

type investInStrategySaveStub struct {
	service.StrategyService
}

func (investInStrategySaveStub) Save(uuid.UUID) error { return nil }

func TestInvestInStrategyRebalanceInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userAccountID := uuid.New()
	strategyID := uuid.New()

	postInvest := func(t *testing.T, handler ApiHandler) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		body, err := json.Marshal(map[string]any{
			"strategyID":    strategyID.String(),
			"amountDollars": 100,
		})
		require.NoError(t, err)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/investInStrategy", bytes.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set("userAccountID", userAccountID.String())
		handler.investInStrategy(ctx)
		return recorder
	}

	t.Run("accepts daily", func(t *testing.T) {
		called := false
		handler := ApiHandler{
			StrategyService: investInStrategySaveStub{},
			InvestmentService: investInStrategyServiceStub{
				add: func(_ context.Context, gotUser, gotStrategy uuid.UUID, amount int) error {
					called = true
					require.Equal(t, userAccountID, gotUser)
					require.Equal(t, strategyID, gotStrategy)
					require.Equal(t, 100, amount)
					return nil
				},
			},
		}
		recorder := postInvest(t, handler)
		require.True(t, called)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.JSONEq(t, `{"success":true}`, recorder.Body.String())
	})

	t.Run("rejects weekly monthly yearly", func(t *testing.T) {
		for _, interval := range []string{"weekly", "monthly", "yearly"} {
			handler := ApiHandler{
				StrategyService: investInStrategySaveStub{},
				InvestmentService: investInStrategyServiceStub{
					add: func(context.Context, uuid.UUID, uuid.UUID, int) error {
						return service.CheckNewInvestmentRebalanceInterval(interval)
					},
				},
			}
			recorder := postInvest(t, handler)
			require.Equal(t, http.StatusBadRequest, recorder.Code, interval)
			require.Contains(t, recorder.Body.String(), "only daily rebalance is currently supported", interval)
		}
	})
}
