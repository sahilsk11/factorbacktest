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

type endInvestmentServiceStub struct {
	service.InvestmentService
	end func(context.Context, uuid.UUID, uuid.UUID) error
}

func (s endInvestmentServiceStub) End(ctx context.Context, userAccountID, investmentID uuid.UUID) error {
	return s.end(ctx, userAccountID, investmentID)
}

func TestEndInvestment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userAccountID := uuid.New()
	investmentID := uuid.New()

	t.Run("queues an owned investment for liquidation", func(t *testing.T) {
		called := false
		handler := ApiHandler{InvestmentService: endInvestmentServiceStub{
			end: func(_ context.Context, gotUserAccountID, gotInvestmentID uuid.UUID) error {
				called = true
				require.Equal(t, userAccountID, gotUserAccountID)
				require.Equal(t, investmentID, gotInvestmentID)
				return nil
			},
		}}
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/investments/"+investmentID.String()+"/end", nil)
		ctx.Params = gin.Params{{Key: "investmentID", Value: investmentID.String()}}
		ctx.Set("userAccountID", userAccountID.String())

		handler.endInvestment(ctx)

		require.True(t, called)
		require.Equal(t, http.StatusAccepted, recorder.Code)
	})

	t.Run("does not reveal an unowned investment", func(t *testing.T) {
		handler := ApiHandler{InvestmentService: endInvestmentServiceStub{
			end: func(context.Context, uuid.UUID, uuid.UUID) error {
				return repository.ErrInvestmentNotFound
			},
		}}
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/investments/"+investmentID.String()+"/end", nil)
		ctx.Params = gin.Params{{Key: "investmentID", Value: investmentID.String()}}
		ctx.Set("userAccountID", userAccountID.String())

		handler.endInvestment(ctx)

		require.Equal(t, http.StatusNotFound, recorder.Code)
	})
}
