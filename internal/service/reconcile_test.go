package service

import (
	"context"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	mock_repository "factorbacktest/internal/repository/mocks"
	"testing"
	"time"

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

func TestRunReconcileDoesNotPanicWithoutProfileInContext(t *testing.T) {
	ctrl := gomock.NewController(t)

	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	strategyRepository := mock_repository.NewMockStrategyRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	holdingsVersionRepository := mock_repository.NewMockInvestmentHoldingsVersionRepository(ctrl)
	investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)
	universeRepository := mock_repository.NewMockAssetUniverseRepository(ctrl)
	priceRepository := mock_repository.NewMockAdjustedPriceRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	investmentID := uuid.New()
	strategyID := uuid.New()
	versionID := uuid.New()
	cash := decimal.NewFromInt(100)
	portfolio := &domain.Portfolio{
		Cash:      &cash,
		Positions: map[string]*domain.Position{},
	}
	investment := model.Investment{
		InvestmentID:  investmentID,
		StrategyID:    strategyID,
		StartDate:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		AmountDollars: 1000,
	}
	strategy := model.Strategy{
		StrategyID:       strategyID,
		FactorExpression: "close",
		NumAssets:        1,
		AssetUniverse:    "sp500",
	}

	handler := investmentServiceHandler{
		InvestmentRepository:          investmentRepository,
		StrategyRepository:            strategyRepository,
		HoldingsRepository:            holdingsRepository,
		HoldingsVersionRepository:     holdingsVersionRepository,
		InvestmentTradeRepository:     investmentTradeRepository,
		UniverseRepository:            universeRepository,
		AlpacaRepository:              alpacaRepository,
		BacktestHandler: BacktestHandler{
			AssetUniverseRepository: universeRepository,
			PriceRepository:         priceRepository,
		},
	}

	investmentRepository.EXPECT().
		List(gomock.Any()).
		Return([]model.Investment{investment}, nil).
		Times(2)
	investmentRepository.EXPECT().
		Get(investmentID).
		Return(&investment, nil)
	strategyRepository.EXPECT().
		Get(strategyID).
		Return(&strategy, nil)
	universeRepository.EXPECT().
		GetAssets("sp500").
		Return([]model.Ticker{{Symbol: "AAPL"}}, nil)
	priceRepository.EXPECT().
		ListTradingDays(gomock.Any(), gomock.Any()).
		Return([]time.Time{}, nil)
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, investmentID).
		Return(portfolio, nil).
		Times(3)
	holdingsVersionRepository.EXPECT().
		GetLatestNotedVersion(investmentID).
		Return(nil, nil)
	holdingsVersionRepository.EXPECT().
		GetEarliestVersionID(investmentID).
		Return(&versionID, nil)
	holdingsRepository.EXPECT().
		Get(versionID).
		Return(portfolio, nil)
	investmentTradeRepository.EXPECT().
		List(nil, gomock.Any()).
		Return([]*model.InvestmentTradeStatus{}, nil)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: cash}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{}, nil)

	require.NotPanics(t, func() {
		result, err := handler.RunReconcile(context.Background())
		require.NoError(t, err)
		require.Equal(t, "OK", result.Status)
	})
}
