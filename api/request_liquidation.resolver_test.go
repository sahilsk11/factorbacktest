package api

import (
	"context"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type requestLiquidationServiceStub struct {
	service.InvestmentService
	requestLiquidation func(context.Context, uuid.UUID, uuid.UUID) error
}

func (s requestLiquidationServiceStub) RequestLiquidation(ctx context.Context, userAccountID, investmentID uuid.UUID) error {
	return s.requestLiquidation(ctx, userAccountID, investmentID)
}

func TestRequestLiquidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userAccountID := uuid.New()
	investmentID := uuid.New()

	t.Run("queues an owned investment for liquidation", func(t *testing.T) {
		called := false
		handler := ApiHandler{InvestmentService: requestLiquidationServiceStub{
			requestLiquidation: func(_ context.Context, gotUserAccountID, gotInvestmentID uuid.UUID) error {
				called = true
				require.Equal(t, userAccountID, gotUserAccountID)
				require.Equal(t, investmentID, gotInvestmentID)
				return nil
			},
		}}
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/investments/"+investmentID.String()+"/request-liquidation", nil)
		ctx.Params = gin.Params{{Key: "investmentID", Value: investmentID.String()}}
		ctx.Set("userAccountID", userAccountID.String())

		handler.requestLiquidation(ctx)

		require.True(t, called)
		require.Equal(t, http.StatusAccepted, recorder.Code)
	})

	t.Run("does not reveal an unowned investment", func(t *testing.T) {
		handler := ApiHandler{InvestmentService: requestLiquidationServiceStub{
			requestLiquidation: func(context.Context, uuid.UUID, uuid.UUID) error {
				return repository.ErrInvestmentNotFound
			},
		}}
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/investments/"+investmentID.String()+"/request-liquidation", nil)
		ctx.Params = gin.Params{{Key: "investmentID", Value: investmentID.String()}}
		ctx.Set("userAccountID", userAccountID.String())

		handler.requestLiquidation(ctx)

		require.Equal(t, http.StatusNotFound, recorder.Code)
	})
}
