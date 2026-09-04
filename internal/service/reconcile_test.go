package service

import (
	"context"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	mock_repository "factorbacktest/internal/repository/mocks"
	"testing"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReconcileAggregatePortfolioINTCShortage(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	investmentID := uuid.New()
	ledgerQty := decimal.RequireFromString("1.197")
	brokerQty := decimal.RequireFromString("0.764")
	cash := decimal.NewFromInt(100)

	handler := investmentServiceHandler{
		InvestmentRepository: investmentRepository,
		HoldingsRepository:   holdingsRepository,
		AlpacaRepository:     alpacaRepository,
	}

	investmentRepository.EXPECT().
		List(gomock.Any()).
		Return([]model.Investment{{InvestmentID: investmentID}}, nil)
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, investmentID).
		Return(&domain.Portfolio{
			Cash: &cash,
			Positions: map[string]*domain.Position{
				"INTC": {
					Symbol:        "INTC",
					ExactQuantity: ledgerQty,
				},
			},
		}, nil)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: cash}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{
		{Symbol: "INTC", Qty: brokerQty},
	}, nil)

	reconErrors, err := handler.reconcileAggregatePortfolio()
	require.NoError(t, err)
	require.Len(t, reconErrors, 1)
	require.Nil(t, reconErrors[0].InvestmentID)
	require.Contains(t, reconErrors[0].Message, "INTC")
	require.Contains(t, reconErrors[0].Message, "1.197")
	require.Contains(t, reconErrors[0].Message, "0.764")
}

func TestReconErrToIssueAggregateScope(t *testing.T) {
	issue := reconErrToIssue(ReconErr{
		Message: "alpaca account holding insufficient INTC: aggregate portfolio 1.197000 vs alpaca 0.764000 (-0.433000)",
	})
	require.Nil(t, issue.InvestmentID)
	require.Contains(t, issue.Message, "INTC")
}

func TestRunReconcileNoInvestments(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	handler := investmentServiceHandler{
		InvestmentRepository: investmentRepository,
		AlpacaRepository:     alpacaRepository,
	}

	investmentRepository.EXPECT().
		List(gomock.Any()).
		Return([]model.Investment{}, nil).
		Times(2)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: decimal.Zero}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{}, nil)

	result, err := handler.RunReconcile(context.Background())
	require.NoError(t, err)
	require.Equal(t, "OK", result.Status)
	require.Empty(t, result.Issues)
}

func TestReconcileNoInvestmentsDoesNotPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	handler := investmentServiceHandler{
		InvestmentRepository: investmentRepository,
		AlpacaRepository:     alpacaRepository,
	}

	investmentRepository.EXPECT().
		List(gomock.Any()).
		Return([]model.Investment{}, nil).
		Times(2)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: decimal.Zero}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{}, nil)

	require.NotPanics(t, func() {
		err := handler.Reconcile(context.Background())
		require.NoError(t, err)
	})
}
