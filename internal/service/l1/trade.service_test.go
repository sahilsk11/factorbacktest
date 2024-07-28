package l1_service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"
	"factorbacktest/internal/util"
	"fmt"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_updatePortfoliosFromTrades(t *testing.T) {
	t.Run("ensure sells work", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
		holdingsVersionRepository := mock_repository.NewMockInvestmentHoldingsVersionRepository(ctrl)

		handler := tradeServiceHandler{
			HoldingsRepository:        holdingsRepository,
			HoldingsVersionRepository: holdingsVersionRepository,
		}

		db, err := util.NewTestDb()
		require.NoError(t, err)

		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		cashTickerID := uuid.New()
		investmentID := uuid.New()
		aaplTickerID := uuid.New()
		completedTradesByInvestment := map[uuid.UUID][]*model.InvestmentTradeStatus{
			investmentID: {
				newInvestmentTradeStatus(
					model.TradeOrderSide_Sell,
					"AAPL",
					model.TradeOrderStatus_Completed,
					decimal.NewFromInt(100),
				),
			},
		}

		holdingsRepository.EXPECT().
			GetLatestHoldings(tx, investmentID).
			Return(&domain.Portfolio{
				Positions: map[string]*domain.Position{
					"AAPL": {
						ExactQuantity: decimal.NewFromInt(100),
						TickerID:      aaplTickerID,
					},
				},
				Cash: util.DecimalPointer(decimal.Zero),
			}, nil)

		versionID := uuid.New()
		holdingsVersionRepository.EXPECT().
			Add(tx, model.InvestmentHoldingsVersion{
				InvestmentID: investmentID,
			}).Return(
			&model.InvestmentHoldingsVersion{
				InvestmentHoldingsVersionID: versionID,
			}, nil,
		)

		holdingsRepository.EXPECT().
			Add(tx, model.InvestmentHoldings{
				InvestmentID:                investmentID,
				TickerID:                    cashTickerID,
				Quantity:                    *util.DecimalPointer(decimal.NewFromInt(10000)),
				InvestmentHoldingsVersionID: versionID,
			}).Return(
			nil, nil,
		)

		err = handler.updatePortfoliosFromTrades(tx, completedTradesByInvestment, cashTickerID)
		require.NoError(t, err)
	})
}

func newInvestmentTradeStatus(
	side model.TradeOrderSide,
	symbol string,
	status model.TradeOrderStatus,
	quantity decimal.Decimal,
) *model.InvestmentTradeStatus {
	return &model.InvestmentTradeStatus{
		Side:         &side,
		Symbol:       &symbol,
		Status:       &status,
		Quantity:     &quantity,
		FilledPrice:  util.DecimalPointer(decimal.NewFromInt(100)),
		FilledAmount: &quantity,
		FilledAt:     util.TimePointer(time.Now()),
		// RebalancerRunID: &[16]byte{},
		// InvestmentID:    &[16]byte{},
		// TradeOrderID:    &[16]byte{},
		// TickerID:        &[16]byte{},
	}
}

func TestAddTradesToPortfolio(t *testing.T) {
	t.Run("add a few trades to portfolio with positions", func(t *testing.T) {
		startPortfolio := &domain.Portfolio{
			Positions: map[string]*domain.Position{
				"AAPL": {
					ExactQuantity: decimal.NewFromInt(100),
				},
				"MSFT": {
					ExactQuantity: decimal.NewFromInt(100),
				},
				"GOOG": {
					ExactQuantity: decimal.NewFromInt(100),
				},
			},
			Cash: util.DecimalPointer(decimal.Zero),
		}
		trades := []*model.InvestmentTradeStatus{
			{
				Symbol:       util.StringPointer("AAPL"),
				Quantity:     util.DecimalPointer(decimal.NewFromInt(100)),
				Side:         util.TradeOrderSidePointer(model.TradeOrderSide_Buy),
				FilledPrice:  util.DecimalPointer(decimal.NewFromInt(100)),
				TradeOrderID: util.UUIDPointer(uuid.New()),
			},
			{
				Symbol:       util.StringPointer("GOOG"),
				Quantity:     util.DecimalPointer(decimal.NewFromInt(100)),
				Side:         util.TradeOrderSidePointer(model.TradeOrderSide_Sell),
				FilledPrice:  util.DecimalPointer(decimal.NewFromInt(50)),
				TradeOrderID: util.UUIDPointer(uuid.New()),
			},
		}

		newPortfolio := AddTradesToPortfolio(trades, startPortfolio)

		require.Equal(t, decimal.NewFromInt(200), newPortfolio.Positions["AAPL"].ExactQuantity)

		_, ok := newPortfolio.Positions["GOOG"]
		require.True(t, !ok, fmt.Sprintf("GOOG was found in portfolio positions: %v", newPortfolio.Positions["GOOG"]))
		require.Equal(t, decimal.NewFromInt(-5_000), *newPortfolio.Cash)
	})
}

func Test_tradeServiceHandler_ExecuteBlock(t *testing.T) {
	t.Run("simple buys", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db, err := util.NewTestDb()
		require.NoError(t, err)
		alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)
		tradeOrderRepository := mock_repository.NewMockTradeOrderRepository(ctrl)
		handler := tradeServiceHandler{
			Db:                   db,
			AlpacaRepository:     alpacaRepository,
			TradeOrderRepository: tradeOrderRepository,
		}

		// misc setup
		tickerIDs := map[string]uuid.UUID{
			"AAPL": uuid.New(),
			"GOOG": uuid.New(),
		}

		ctx := context.Background()
		rawTrades := []*domain.ProposedTrade{
			{
				Symbol:        "AAPL",
				TickerID:      tickerIDs["AAPL"],
				ExactQuantity: decimal.NewFromInt(100),
				ExpectedPrice: decimal.NewFromInt(100),
			},
			{
				Symbol:        "AAPL",
				TickerID:      tickerIDs["AAPL"],
				ExactQuantity: decimal.NewFromInt(100),
				ExpectedPrice: decimal.NewFromInt(100),
			},
			{
				Symbol:        "GOOG",
				TickerID:      tickerIDs["GOOG"],
				ExactQuantity: decimal.NewFromInt(100),
				ExpectedPrice: decimal.NewFromInt(100),
			},
		}
		rebalancerRunID := uuid.New()

		// mocks
		var expectedAaplOrder, expectedGoogOrder model.TradeOrder
		{
			// say we hold an arbitrarily large amount
			// so we don't breach limits
			alpacaRepository.EXPECT().
				GetPositions().
				Return([]alpaca.Position{
					{
						Symbol:       "AAPL",
						Qty:          decimal.NewFromInt(5000),
						QtyAvailable: decimal.NewFromInt(5000),
					},
					{
						Symbol:       "GOOG",
						Qty:          decimal.NewFromInt(5000),
						QtyAvailable: decimal.NewFromInt(5000),
					},
				}, nil)

			expectedAaplOrder = mockPlaceOrder(
				t,
				tradeOrderRepository,
				alpacaRepository,
				tickerIDs["AAPL"],
				"AAPL",
				nil,
				decimal.NewFromInt(200),
				model.TradeOrderSide_Buy,
				alpaca.Buy,
				rebalancerRunID,
				decimal.NewFromInt(100),
			)

			expectedGoogOrder = mockPlaceOrder(
				t,
				tradeOrderRepository,
				alpacaRepository,
				tickerIDs["GOOG"],
				"GOOG",
				nil,
				decimal.NewFromInt(100),
				model.TradeOrderSide_Buy,
				alpaca.Buy,
				rebalancerRunID,
				decimal.NewFromInt(100),
			)
		}

		executedOrders, err := handler.ExecuteBlock(
			ctx,
			rawTrades,
			rebalancerRunID,
		)

		require.NoError(t, err)
		// kind of useless bc these are the same models
		// you're mocking in the return
		require.Equal(t, "", cmp.Diff(
			[]model.TradeOrder{
				expectedAaplOrder,
				expectedGoogOrder,
			},
			executedOrders,
			cmpopts.SortSlices(func(i, j model.TradeOrder) bool {
				return i.TickerID.String() > j.TickerID.String()
			}),
		))
	})

	t.Run("track excess", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		db, err := util.NewTestDb()
		require.NoError(t, err)
		alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)
		tradeOrderRepository := mock_repository.NewMockTradeOrderRepository(ctrl)
		excessTradeVolumeRepository := mock_repository.NewMockExcessTradeVolumeRepository(ctrl)
		handler := tradeServiceHandler{
			Db:                          db,
			AlpacaRepository:            alpacaRepository,
			TradeOrderRepository:        tradeOrderRepository,
			ExcessTradeVolumeRepository: excessTradeVolumeRepository,
		}

		// misc setup
		tickerIDs := map[string]uuid.UUID{
			"AAPL": uuid.New(),
		}

		ctx := context.Background()
		rawTrades := []*domain.ProposedTrade{
			{
				Symbol:        "AAPL",
				TickerID:      tickerIDs["AAPL"],
				ExactQuantity: decimal.NewFromFloat(1),
				ExpectedPrice: decimal.NewFromInt(1),
			},
		}
		rebalancerRunID := uuid.New()

		// mocks
		var expectedAaplOrder model.TradeOrder
		{
			// say we hold an arbitrarily large amount
			// so we don't breach limits
			alpacaRepository.EXPECT().
				GetPositions().
				Return([]alpaca.Position{
					{
						Symbol:       "AAPL",
						Qty:          decimal.NewFromInt(5000),
						QtyAvailable: decimal.NewFromInt(5000),
					},
				}, nil)

			excessVolumeID := uuid.New()
			excessTradeVolumeRepository.EXPECT().
				Add(gomock.Any(), gomock.Any()).
				DoAndReturn(func(tx *sql.Tx, m model.ExcessTradeVolume) (*model.ExcessTradeVolume, error) {
					require.Equal(t, "", cmp.Diff(
						model.ExcessTradeVolume{
							TickerID:        tickerIDs["AAPL"],
							Quantity:        decimal.NewFromFloat(0.5),
							RebalancerRunID: rebalancerRunID,
						},
						m,
					))
					return &model.ExcessTradeVolume{
						ExcessTradeVolumeID: excessVolumeID,
						TickerID:            tickerIDs["AAPL"],
						Quantity:            decimal.NewFromFloat(0.5),
						RebalancerRunID:     rebalancerRunID,
					}, nil
				})

			expectedAaplOrder = mockPlaceOrder(
				t,
				tradeOrderRepository,
				alpacaRepository,
				tickerIDs["AAPL"],
				"AAPL",
				nil,
				decimal.NewFromFloat(1.5),
				model.TradeOrderSide_Buy,
				alpaca.Buy,
				rebalancerRunID,
				decimal.NewFromInt(1),
			)

			excessTradeVolumeRepository.EXPECT().
				Update(gomock.Any(), gomock.Any(), postgres.ColumnList{
					table.ExcessTradeVolume.TradeOrderID,
				}).
				DoAndReturn(func(tx *sql.Tx, m model.ExcessTradeVolume, columns postgres.ColumnList) (*model.ExcessTradeVolume, error) {
					require.Equal(t, "", cmp.Diff(
						model.ExcessTradeVolume{
							TradeOrderID:        &expectedAaplOrder.TradeOrderID,
							ExcessTradeVolumeID: excessVolumeID,
							TickerID:            tickerIDs["AAPL"],
							Quantity:            decimal.NewFromFloat(0.5),
							RebalancerRunID:     rebalancerRunID,
						},
						m,
						cmp.Comparer(func(i, j uuid.UUID) bool {
							return i.String() == j.String()
						}),
					))
					return nil, nil
				})

		}

		executedOrders, err := handler.ExecuteBlock(
			ctx,
			rawTrades,
			rebalancerRunID,
		)

		require.NoError(t, err)
		// kind of useless bc these are the same models
		// you're mocking in the return
		require.Equal(t, "", cmp.Diff(
			[]model.TradeOrder{
				expectedAaplOrder,
			},
			executedOrders,
			cmpopts.SortSlices(func(i, j model.TradeOrder) bool {
				return i.TickerID.String() > j.TickerID.String()
			}),
			cmp.Comparer(func(i, j uuid.UUID) bool {
				return i.String() == j.String()
			}),
		))
	})

}

func mockPlaceOrder(
	t *testing.T,
	tradeOrderRepository *mock_repository.MockTradeOrderRepository,
	alpacaRepository *mock_repository.MockAlpacaRepository,
	tickerID uuid.UUID,
	symbol string,
	notes *string,
	quantity decimal.Decimal,
	dbSide model.TradeOrderSide,
	alpacaSide alpaca.Side,
	rebalancerRunID uuid.UUID,
	expectedPrice decimal.Decimal,
) model.TradeOrder {
	tradeOrderID := uuid.New()

	expectedOrder := model.TradeOrder{
		TickerID:          tickerID,
		Side:              dbSide,
		Status:            model.TradeOrderStatus_Error,
		FilledQuantity:    decimal.Zero,
		FilledPrice:       nil,
		FilledAt:          nil,
		Notes:             notes,
		RebalancerRunID:   rebalancerRunID,
		RequestedQuantity: quantity,
		ExpectedPrice:     expectedPrice,
	}
	tradeOrderRepository.EXPECT().
		Add(
			nil,
			// expectedOrder,
			gomock.Any(),
		).
		DoAndReturn(func(tx *sql.Tx, to model.TradeOrder) (*model.TradeOrder, error) {
			require.Equal(t, "", cmp.Diff(
				expectedOrder,
				to,
				cmp.Comparer(func(i, j uuid.UUID) bool {
					return i.String() == j.String()
				}),
			))

			return &model.TradeOrder{
				TradeOrderID: tradeOrderID,
			}, nil
		})

	orderID := uuid.New()
	expectedAlpacaReq := repository.AlpacaPlaceOrderRequest{
		TradeOrderID: tradeOrderID,
		Quantity:     quantity,
		Symbol:       symbol,
		Side:         alpacaSide,
	}
	alpacaRepository.EXPECT().
		PlaceOrder(
			gomock.Any(),
		).
		DoAndReturn(func(req repository.AlpacaPlaceOrderRequest) (*alpaca.Order, error) {
			require.Equal(t, "", cmp.Diff(
				expectedAlpacaReq,
				req,
			))
			return &alpaca.Order{
				ID: orderID.String(),
			}, nil
		})

	outputOrder := model.TradeOrder{
		TradeOrderID:      tradeOrderID,
		ProviderID:        &orderID,
		TickerID:          tickerID,
		Side:              dbSide,
		Status:            model.TradeOrderStatus_Pending,
		FilledQuantity:    quantity,
		FilledPrice:       nil,
		FilledAt:          nil,
		CreatedAt:         time.Time{},
		ModifiedAt:        time.Time{},
		Notes:             notes,
		RebalancerRunID:   rebalancerRunID,
		RequestedQuantity: quantity,
		ExpectedPrice:     expectedPrice,
	}
	tradeOrderRepository.EXPECT().
		Update(nil, tradeOrderID, gomock.Any(), postgres.ColumnList{
			table.TradeOrder.Status,
			table.TradeOrder.ProviderID,
			table.TradeOrder.FilledQuantity,
			table.TradeOrder.FilledPrice,
			table.TradeOrder.FilledAt,
		}).DoAndReturn(func(tx *sql.Tx, tradeOrderID uuid.UUID, to model.TradeOrder, columns postgres.ColumnList) (*model.TradeOrder, error) {
		expectedModel := model.TradeOrder{
			Status:         model.TradeOrderStatus_Pending,
			ProviderID:     &orderID,
			FilledQuantity: decimal.Zero, // will probably be 0
			FilledPrice:    nil,          // will probably be nil
			FilledAt:       nil,          // will probably be nil
		}

		require.Equal(t, "", cmp.Diff(
			expectedModel,
			to,
			cmp.Comparer(func(i, j uuid.UUID) bool {
				return i.String() == j.String()
			}),
		))

		return &outputOrder, nil
	})

	return outputOrder
}
