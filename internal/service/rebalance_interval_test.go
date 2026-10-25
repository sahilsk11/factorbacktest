package service

import (
	"context"
	"testing"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"
	"factorbacktest/internal/util"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDueForRebalance(t *testing.T) {
	now := time.Date(2026, 10, 25, 15, 4, 0, 0, time.UTC)
	lastToday := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	lastYesterday := time.Date(2026, 10, 24, 16, 0, 0, 0, time.UTC)
	lastWeek := time.Date(2026, 10, 18, 15, 4, 0, 0, time.UTC)
	lastSameMonth := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	lastPrevMonth := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	lastThisYear := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	lastPrevYear := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)

	t.Run("never rebalanced is always due", func(t *testing.T) {
		require.True(t, dueForRebalance("monthly", nil, now))
		require.True(t, dueForRebalance("DAILY", nil, now))
	})

	t.Run("daily skips the same UTC day and runs the next", func(t *testing.T) {
		require.False(t, dueForRebalance("daily", &lastToday, now))
		require.False(t, dueForRebalance("DAILY", &lastToday, now))
		require.True(t, dueForRebalance("daily", &lastYesterday, now))
	})

	t.Run("weekly requires 7 days", func(t *testing.T) {
		sixDays := lastToday.AddDate(0, 0, -6)
		require.False(t, dueForRebalance("weekly", &sixDays, now))
		require.True(t, dueForRebalance("weekly", &lastWeek, now))
	})

	t.Run("monthly uses the next UTC calendar month", func(t *testing.T) {
		require.False(t, dueForRebalance("monthly", &lastSameMonth, now))
		require.False(t, dueForRebalance("MONTHLY", &lastToday, now))
		require.True(t, dueForRebalance("monthly", &lastPrevMonth, now))
	})

	t.Run("yearly uses the next UTC calendar year", func(t *testing.T) {
		require.False(t, dueForRebalance("yearly", &lastThisYear, now))
		require.True(t, dueForRebalance("yearly", &lastPrevYear, now))
	})
}

func TestListForRebalanceInterval(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepo := mock_repository.NewMockInvestmentRepository(ctrl)
	strategyRepo := mock_repository.NewMockStrategyRepository(ctrl)
	tradeRepo := mock_repository.NewMockInvestmentTradeRepository(ctrl)
	rebalanceRepo := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)

	handler := investmentServiceHandler{
		InvestmentRepository:          investmentRepo,
		StrategyRepository:            strategyRepo,
		InvestmentTradeRepository:     tradeRepo,
		InvestmentRebalanceRepository: rebalanceRepo,
	}

	freshID := uuid.New()
	monthlyID := uuid.New()
	dueID := uuid.New()
	liqID := uuid.New()
	strategyID := uuid.New()
	now := time.Now().UTC()
	lastThisMonth := time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, time.UTC)
	lastPrevMonth := lastThisMonth.AddDate(0, -1, 0)

	investments := []model.Investment{
		{InvestmentID: freshID, StrategyID: strategyID},
		{InvestmentID: monthlyID, StrategyID: strategyID},
		{InvestmentID: dueID, StrategyID: strategyID},
		{InvestmentID: liqID, StrategyID: strategyID, LiquidationRequestedAt: util.TimePointer(now)},
	}
	investmentRepo.EXPECT().List(repository.StrategyInvestmentListFilter{}).Return(investments, nil)
	tradeRepo.EXPECT().List(nil, gomock.Any()).Return(nil, nil).Times(4)
	strategyRepo.EXPECT().Get(strategyID).Return(&model.Strategy{RebalanceInterval: "MONTHLY"}, nil).Times(3)
	rebalanceRepo.EXPECT().GetLatestPlaced(freshID).Return(nil, nil)
	rebalanceRepo.EXPECT().GetLatestPlaced(monthlyID).Return(&model.InvestmentRebalance{CreatedAt: lastThisMonth}, nil)
	rebalanceRepo.EXPECT().GetLatestPlaced(dueID).Return(&model.InvestmentRebalance{CreatedAt: lastPrevMonth}, nil)

	got, err := handler.listForRebalance(context.Background())
	require.NoError(t, err)
	gotIDs := []uuid.UUID{}
	for _, inv := range got {
		gotIDs = append(gotIDs, inv.InvestmentID)
	}
	require.Equal(t, []uuid.UUID{freshID, dueID, liqID}, gotIDs)
}
