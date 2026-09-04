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

func TestDetectQuantityMismatchINTCShortage(t *testing.T) {
	ledgerQty := decimal.RequireFromString("1.197")
	brokerQty := decimal.RequireFromString("0.764")

	totalHoldings := domain.NewPortfolio()
	totalHoldings.Positions["INTC"] = &domain.Position{
		Symbol:        "INTC",
		ExactQuantity: ledgerQty,
	}

	account := &alpaca.Account{Cash: decimal.NewFromInt(100)}
	positions := []alpaca.Position{
		{Symbol: "INTC", Qty: brokerQty},
	}

	mismatches := detectQuantityMismatches(totalHoldings, account, positions)
	require.Len(t, mismatches, 1)
	require.Equal(t, "INTC", mismatches[0].Symbol)
	require.Equal(t, QuantityMismatchShortage, mismatches[0].Kind)
	require.InDelta(t, 1.197, mismatches[0].LedgerQty, 1e-9)
	require.InDelta(t, 0.764, mismatches[0].BrokerQty, 1e-9)
	require.InDelta(t, -0.433, mismatches[0].Delta, 1e-9)
}

func TestDetectQuantityMismatchOKWithinEpsilon(t *testing.T) {
	ledgerQty := decimal.RequireFromString("1.197")
	brokerQty := decimal.RequireFromString("1.1970005")

	totalHoldings := domain.NewPortfolio()
	totalHoldings.Positions["INTC"] = &domain.Position{
		Symbol:        "INTC",
		ExactQuantity: ledgerQty,
	}

	account := &alpaca.Account{Cash: decimal.NewFromInt(100)}
	positions := []alpaca.Position{
		{Symbol: "INTC", Qty: brokerQty},
	}

	mismatches := detectQuantityMismatches(totalHoldings, account, positions)
	require.Empty(t, mismatches)
}

func TestCheckQuantityMismatchService(t *testing.T) {
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

	result, err := handler.CheckQuantityMismatch(context.Background())
	require.NoError(t, err)
	require.Equal(t, "MISMATCH", result.Status)
	require.Len(t, result.Mismatches, 1)
	require.Equal(t, "INTC", result.Mismatches[0].Symbol)
	require.Equal(t, QuantityMismatchShortage, result.Mismatches[0].Kind)
}

func TestCheckQuantityMismatchServiceOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	investmentID := uuid.New()
	qty := decimal.RequireFromString("10")
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
				"AAPL": {
					Symbol:        "AAPL",
					ExactQuantity: qty,
				},
			},
		}, nil)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: cash}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{
		{Symbol: "AAPL", Qty: qty},
	}, nil)

	result, err := handler.CheckQuantityMismatch(context.Background())
	require.NoError(t, err)
	require.Equal(t, "OK", result.Status)
	require.Empty(t, result.Mismatches)
}
