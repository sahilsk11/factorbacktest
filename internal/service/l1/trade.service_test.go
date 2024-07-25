package l1_service

import (
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	mock_repository "factorbacktest/internal/repository/mocks"
	"factorbacktest/internal/util"
	"fmt"
	"testing"
	"time"

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
