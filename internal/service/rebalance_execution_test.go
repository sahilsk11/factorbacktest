package service

import (
	"testing"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"
	"factorbacktest/internal/util"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_investmentServiceHandler_finalizeRebalancePlan(t *testing.T) {
	t.Run("marks investment rebalance completed when all trades completed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		investmentRebalanceRepository := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)
		investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)

		handler := investmentServiceHandler{
			InvestmentRebalanceRepository: investmentRebalanceRepository,
			InvestmentRepository:          investmentRepository,
		}

		rebalanceID := uuid.New()
		investmentID := uuid.New()
		tradeOrderID := uuid.New()

		investmentRebalanceRepository.EXPECT().
			Update(nil, model.InvestmentRebalance{
				InvestmentRebalanceID: rebalanceID,
				State:                 model.RebalancerRunState_Completed,
			}, postgres.ColumnList{table.InvestmentRebalance.State}).
			Return(&model.InvestmentRebalance{}, nil)

		err := handler.finalizeRebalancePlan(
			[]rebalancePlanEntry{
				{
					InvestmentID:          investmentID,
					InvestmentRebalanceID: rebalanceID,
					InvestmentTrades: []model.InvestmentTrade{
						{TradeOrderID: &tradeOrderID},
					},
				},
			},
			[]model.TradeOrder{
				{
					TradeOrderID: tradeOrderID,
					Status:       model.TradeOrderStatus_Completed,
				},
			},
			nil,
		)
		require.NoError(t, err)
	})

	t.Run("leaves investment rebalance pending when trades still pending", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		investmentRebalanceRepository := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)

		handler := investmentServiceHandler{
			InvestmentRebalanceRepository: investmentRebalanceRepository,
		}

		rebalanceID := uuid.New()
		tradeOrderID := uuid.New()

		err := handler.finalizeRebalancePlan(
			[]rebalancePlanEntry{
				{
					InvestmentRebalanceID: rebalanceID,
					InvestmentTrades: []model.InvestmentTrade{
						{TradeOrderID: &tradeOrderID},
					},
				},
			},
			[]model.TradeOrder{
				{
					TradeOrderID: tradeOrderID,
					Status:       model.TradeOrderStatus_Pending,
				},
			},
			nil,
		)
		require.NoError(t, err)
	})

	t.Run("marks investment rebalance error when trade failed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		investmentRebalanceRepository := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)
		investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)

		handler := investmentServiceHandler{
			InvestmentRebalanceRepository: investmentRebalanceRepository,
			InvestmentRepository:          investmentRepository,
		}

		rebalanceID := uuid.New()
		investmentID := uuid.New()
		tradeOrderID := uuid.New()

		investmentRebalanceRepository.EXPECT().
			Update(nil, model.InvestmentRebalance{
				InvestmentRebalanceID: rebalanceID,
				State:                 model.RebalancerRunState_Error,
			}, postgres.ColumnList{table.InvestmentRebalance.State}).
			Return(&model.InvestmentRebalance{}, nil)
		investmentRepository.EXPECT().
			SetErrorAt(nil, investmentID, "rebalance trade execution failed").
			Return(nil)

		err := handler.finalizeRebalancePlan(
			[]rebalancePlanEntry{
				{
					InvestmentID:          investmentID,
					InvestmentRebalanceID: rebalanceID,
					InvestmentTrades: []model.InvestmentTrade{
						{TradeOrderID: &tradeOrderID},
					},
				},
			},
			[]model.TradeOrder{
				{
					TradeOrderID: tradeOrderID,
					Status:       model.TradeOrderStatus_Error,
				},
			},
			nil,
		)
		require.NoError(t, err)
	})

	t.Run("marks empty-trade investment rebalance completed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		investmentRebalanceRepository := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)

		handler := investmentServiceHandler{
			InvestmentRebalanceRepository: investmentRebalanceRepository,
		}

		rebalanceID := uuid.New()

		investmentRebalanceRepository.EXPECT().
			Update(nil, model.InvestmentRebalance{
				InvestmentRebalanceID: rebalanceID,
				State:                 model.RebalancerRunState_Completed,
			}, postgres.ColumnList{table.InvestmentRebalance.State}).
			Return(&model.InvestmentRebalance{}, nil)

		err := handler.finalizeRebalancePlan(
			[]rebalancePlanEntry{
				{
					InvestmentRebalanceID: rebalanceID,
					InvestmentTrades:      nil,
				},
			},
			nil,
			nil,
		)
		require.NoError(t, err)
	})
}

func Test_completeInvestmentRebalancesForRun(t *testing.T) {
	t.Run("marks pending investment rebalance completed when all trades completed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db, err := util.NewTestDb()
		require.NoError(t, err)

		investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)
		investmentRebalanceRepository := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)

		rebalancerRunID := uuid.New()
		investmentID := uuid.New()
		rebalanceID := uuid.New()
		tradeOrderID := uuid.New()
		completedStatus := model.TradeOrderStatus_Completed

		investmentRebalanceRepository.EXPECT().
			ListByRebalancerRunID(gomock.Any(), rebalancerRunID).
			Return([]model.InvestmentRebalance{
				{
					InvestmentRebalanceID: rebalanceID,
					RebalancerRunID:       rebalancerRunID,
					InvestmentID:          investmentID,
					State:                 model.RebalancerRunState_Pending,
				},
			}, nil)

		investmentTradeRepository.EXPECT().
			List(gomock.Any(), repository.InvestmentTradeListFilter{
				RebalancerRunID: &rebalancerRunID,
				InvestmentID:    &investmentID,
			}).
			Return([]*model.InvestmentTradeStatus{
				{
					InvestmentID:    &investmentID,
					Status:          &completedStatus,
					RebalancerRunID: &rebalancerRunID,
					TradeOrderID:    &tradeOrderID,
				},
			}, nil)

		investmentRebalanceRepository.EXPECT().
			Update(gomock.Any(), model.InvestmentRebalance{
				InvestmentRebalanceID: rebalanceID,
				State:                 model.RebalancerRunState_Completed,
			}, postgres.ColumnList{table.InvestmentRebalance.State}).
			Return(&model.InvestmentRebalance{}, nil)

		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = completeInvestmentRebalancesForRun(
			tx,
			investmentRebalanceRepository,
			investmentTradeRepository,
			rebalancerRunID,
		)
		require.NoError(t, err)
	})

	t.Run("leaves investment rebalance pending when trades still pending", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db, err := util.NewTestDb()
		require.NoError(t, err)

		investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)
		investmentRebalanceRepository := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)

		rebalancerRunID := uuid.New()
		investmentID := uuid.New()
		rebalanceID := uuid.New()
		pendingStatus := model.TradeOrderStatus_Pending

		investmentRebalanceRepository.EXPECT().
			ListByRebalancerRunID(gomock.Any(), rebalancerRunID).
			Return([]model.InvestmentRebalance{
				{
					InvestmentRebalanceID: rebalanceID,
					RebalancerRunID:       rebalancerRunID,
					InvestmentID:          investmentID,
					State:                 model.RebalancerRunState_Pending,
				},
			}, nil)

		investmentTradeRepository.EXPECT().
			List(gomock.Any(), repository.InvestmentTradeListFilter{
				RebalancerRunID: &rebalancerRunID,
				InvestmentID:    &investmentID,
			}).
			Return([]*model.InvestmentTradeStatus{
				{
					InvestmentID:    &investmentID,
					Status:          &pendingStatus,
					RebalancerRunID: &rebalancerRunID,
				},
			}, nil)

		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = completeInvestmentRebalancesForRun(
			tx,
			investmentRebalanceRepository,
			investmentTradeRepository,
			rebalancerRunID,
		)
		require.NoError(t, err)
	})
}

func Test_investmentTradeStatusesAllCompleted(t *testing.T) {
	completed := model.TradeOrderStatus_Completed
	pending := model.TradeOrderStatus_Pending

	require.True(t, investmentTradeStatusesAllCompleted(nil))
	require.True(t, investmentTradeStatusesAllCompleted([]*model.InvestmentTradeStatus{
		{Status: &completed},
	}))
	require.False(t, investmentTradeStatusesAllCompleted([]*model.InvestmentTradeStatus{
		{Status: &pending},
	}))
}
