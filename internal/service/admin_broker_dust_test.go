package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type recordingAudit struct {
	rows []model.AdminMutationAudit
}

func (r *recordingAudit) Add(_ *sql.Tx, row model.AdminMutationAudit) (*model.AdminMutationAudit, error) {
	row.AdminMutationAuditID = uuid.New()
	r.rows = append(r.rows, row)
	return &row, nil
}

func dustPosition(symbol, qty, price string) alpaca.Position {
	q := decimal.RequireFromString(qty)
	p := decimal.RequireFromString(price)
	mv := q.Mul(p)
	return alpaca.Position{
		Symbol:       symbol,
		Qty:          q,
		QtyAvailable: q,
		CurrentPrice: &p,
		MarketValue:  &mv,
	}
}

func TestAdminSellBrokerDustDryRunDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	alpacaRepo := mock_repository.NewMockAlpacaRepository(ctrl)
	investmentRepo := mock_repository.NewMockInvestmentRepository(ctrl)
	holdingsRepo := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)

	alpacaRepo.EXPECT().GetPositions().Return([]alpaca.Position{
		dustPosition("XYZ", "0.1", "10"),
	}, nil)
	investmentRepo.EXPECT().
		List(repository.StrategyInvestmentListFilter{IncludePaused: true, IncludeEnded: true}).
		Return([]model.Investment{}, nil)

	handler := investmentServiceHandler{
		AlpacaRepository:     alpacaRepo,
		InvestmentRepository: investmentRepo,
		HoldingsRepository:   holdingsRepo,
	}

	result, err := handler.AdminSellBrokerDust(context.Background(), AdminSellBrokerDustRequest{
		Symbols: []string{"xyz"},
	})
	require.NoError(t, err)
	require.True(t, result.DryRun)
	require.False(t, result.Executed)
	require.Len(t, result.Sales, 1)
	require.Equal(t, "XYZ", result.Sales[0].Symbol)
	require.Equal(t, "0.1", result.Sales[0].Quantity)
	require.InDelta(t, 1.0, result.Sales[0].EstDollars, 0.0001)
	require.Equal(t, "planned", result.Sales[0].Status)
	require.Nil(t, result.AuditID)
}

func TestAdminSellBrokerDustRefusesHeldSymbol(t *testing.T) {
	ctrl := gomock.NewController(t)
	alpacaRepo := mock_repository.NewMockAlpacaRepository(ctrl)
	investmentRepo := mock_repository.NewMockInvestmentRepository(ctrl)
	holdingsRepo := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	investmentID := uuid.New()
	cash := decimal.Zero

	alpacaRepo.EXPECT().GetPositions().Return([]alpaca.Position{
		dustPosition("AAPL", "0.2", "10"),
	}, nil)
	investmentRepo.EXPECT().
		List(gomock.Any()).
		Return([]model.Investment{{InvestmentID: investmentID}}, nil)
	holdingsRepo.EXPECT().
		GetLatestHoldings(nil, investmentID).
		Return(&domain.Portfolio{
			Cash: &cash,
			Positions: map[string]*domain.Position{
				"AAPL": {Symbol: "AAPL", ExactQuantity: decimal.RequireFromString("0.2")},
			},
		}, nil)

	handler := investmentServiceHandler{
		AlpacaRepository:     alpacaRepo,
		InvestmentRepository: investmentRepo,
		HoldingsRepository:   holdingsRepo,
	}

	_, err := handler.AdminSellBrokerDust(context.Background(), AdminSellBrokerDustRequest{
		Symbols: []string{"AAPL"},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAdminSellBrokerDustRefused))
	require.Contains(t, err.Error(), "non-ended investment")
}

func TestAdminSellBrokerDustRefusesOverCap(t *testing.T) {
	ctrl := gomock.NewController(t)
	alpacaRepo := mock_repository.NewMockAlpacaRepository(ctrl)
	investmentRepo := mock_repository.NewMockInvestmentRepository(ctrl)

	alpacaRepo.EXPECT().GetPositions().Return([]alpaca.Position{
		dustPosition("BIG", "1", "60"),
	}, nil)
	investmentRepo.EXPECT().List(gomock.Any()).Return([]model.Investment{}, nil)

	handler := investmentServiceHandler{
		AlpacaRepository:     alpacaRepo,
		InvestmentRepository: investmentRepo,
	}

	_, err := handler.AdminSellBrokerDust(context.Background(), AdminSellBrokerDustRequest{
		Symbols: []string{"BIG"},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAdminSellBrokerDustRefused))
	require.Contains(t, err.Error(), "per-position cap")
}

func TestAdminSellBrokerDustRefusesMissingConfirm(t *testing.T) {
	ctrl := gomock.NewController(t)
	alpacaRepo := mock_repository.NewMockAlpacaRepository(ctrl)
	dryRun := false

	handler := investmentServiceHandler{AlpacaRepository: alpacaRepo}
	_, err := handler.AdminSellBrokerDust(context.Background(), AdminSellBrokerDustRequest{
		Symbols: []string{"XYZ"},
		Note:    "sell dust",
		DryRun:  &dryRun,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAdminSellBrokerDustInvalidRequest))
	require.Contains(t, err.Error(), "confirm=true")
}

func TestAdminSellBrokerDustExecuteUsesBroker(t *testing.T) {
	ctrl := gomock.NewController(t)
	alpacaRepo := mock_repository.NewMockAlpacaRepository(ctrl)
	investmentRepo := mock_repository.NewMockInvestmentRepository(ctrl)
	audit := &recordingAudit{}
	dryRun := false

	alpacaRepo.EXPECT().GetPositions().Return([]alpaca.Position{
		dustPosition("XYZ", "0.1", "10"),
	}, nil)
	investmentRepo.EXPECT().List(gomock.Any()).Return([]model.Investment{}, nil)
	alpacaRepo.EXPECT().IsMarketOpen().Return(true, nil)
	alpacaRepo.EXPECT().PlaceOrder(gomock.Any()).DoAndReturn(func(req repository.AlpacaPlaceOrderRequest) (*alpaca.Order, error) {
		require.Equal(t, "XYZ", req.Symbol)
		require.Equal(t, alpaca.Sell, req.Side)
		require.True(t, req.Quantity.Equal(decimal.RequireFromString("0.1")))
		require.NotNil(t, req.LimitPrice)
		return &alpaca.Order{ID: "alpaca-order-1"}, nil
	})

	handler := investmentServiceHandler{
		AlpacaRepository:             alpacaRepo,
		InvestmentRepository:         investmentRepo,
		AdminMutationAuditRepository: audit,
	}

	result, err := handler.AdminSellBrokerDust(context.Background(), AdminSellBrokerDustRequest{
		Symbols: []string{"XYZ"},
		Note:    "sell untracked dust",
		Actor:   "sahil",
		DryRun:  &dryRun,
		Confirm: true,
	})
	require.NoError(t, err)
	require.True(t, result.Executed)
	require.False(t, result.DryRun)
	require.Equal(t, "submitted", result.Sales[0].Status)
	require.Equal(t, "alpaca-order-1", *result.Sales[0].AlpacaOrderID)
	require.Len(t, audit.rows, 1)
	require.Equal(t, "broker_dust_sell", audit.rows[0].Kind)
	require.Equal(t, "sahil", audit.rows[0].Actor)
	require.False(t, audit.rows[0].DryRun)
}
