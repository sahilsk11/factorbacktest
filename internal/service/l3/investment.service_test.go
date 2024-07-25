package l3_service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"
	l2_service "factorbacktest/internal/service/l2"
	mock_l2_service "factorbacktest/internal/service/l2/mocks"
	"factorbacktest/internal/util"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_investmentServiceHandler_rebalanceInvestment(t *testing.T) {
	t.Run("rebalance from existing holdings", func(t *testing.T) {
		db, err := util.NewTestDb()
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
		universeRepository := mock_repository.NewMockAssetUniverseRepository(ctrl)
		ssRepo := mock_repository.NewMockSavedStrategyRepository(ctrl)
		feService := mock_l2_service.NewMockFactorExpressionService(ctrl)
		investmentRebalanceRepository := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)
		investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)
		holdingsVersionRepository := mock_repository.NewMockInvestmentHoldingsVersionRepository(ctrl)

		handler := investmentServiceHandler{
			HoldingsRepository:            holdingsRepository,
			SavedStrategyRepository:       ssRepo,
			UniverseRepository:            universeRepository,
			FactorExpressionService:       feService,
			InvestmentRebalanceRepository: investmentRebalanceRepository,
			InvestmentTradeRepository:     investmentTradeRepository,
			HoldingsVersionRepository:     holdingsVersionRepository,
		}

		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		investment := model.Investment{
			InvestmentID:  uuid.New(),
			AmountDollars: 10,
			StartDate:     time.Now(),
			SavedStragyID: uuid.New(),
			UserAccountID: uuid.New(),
			CreatedAt:     time.Now(),
			ModifiedAt:    time.Now(),
			EndDate:       nil,
		}
		rebalancerRun := model.RebalancerRun{
			RebalancerRunID:         uuid.New(),
			Date:                    time.Now(),
			CreatedAt:               time.Now(),
			RebalancerRunType:       model.RebalancerRunType_ManualInvestmentRebalance,
			RebalancerRunState:      model.RebalancerRunState_Error, // i think it starts as error
			ModifiedAt:              time.Now(),
			NumInvestmentsAttempted: 1,
			Notes:                   nil,
		}
		priceMap := map[string]decimal.Decimal{
			"AAPL": decimal.NewFromInt(100),
			"GOOG": decimal.NewFromInt(100),
			"MSFT": decimal.NewFromInt(100),
		}
		tickerIDMap := map[string]uuid.UUID{
			"AAPL": uuid.New(),
			"GOOG": uuid.New(),
			"MSFT": uuid.New(),
		}

		latestVersionID := uuid.New()
		holdingsVersionRepository.EXPECT().
			GetLatestVersionID(investment.InvestmentID).
			Return(&latestVersionID, nil)

		holdingsRepository.EXPECT().
			GetLatestHoldings(nil, investment.InvestmentID).
			Return(&domain.Portfolio{
				Positions: map[string]*domain.Position{
					"AAPL": {
						Quantity:      1,
						ExactQuantity: decimal.NewFromInt(1),
						TickerID:      uuid.New(),
					},
					"GOOG": {
						Quantity:      1,
						ExactQuantity: decimal.NewFromInt(1),
						TickerID:      uuid.New(),
					},
					"MSFT": {
						Quantity:      1,
						ExactQuantity: decimal.NewFromInt(1),
						TickerID:      uuid.New(),
					},
				},
				Cash: util.DecimalPointer(decimal.Zero),
			}, nil)

		ssRepo.EXPECT().
			Get(gomock.Any()).
			Return(&model.SavedStrategy{
				AssetUniverse: "universe",
				NumAssets:     3,
			}, nil)

		universeRepository.EXPECT().
			GetAssets("universe").
			Return([]model.Ticker{}, nil)

		feService.EXPECT().
			CalculateFactorScoresOnDay(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(
				&l2_service.ScoresResultsOnDay{
					SymbolScores: map[string]*float64{
						"AAPL": util.FloatPointer(100),
						"GOOG": util.FloatPointer(200),
						"MSFT": util.FloatPointer(300),
					},
					Errors: []error{},
				}, nil,
			)

		investmentRebalanceRepository.EXPECT().
			Add(gomock.Any(), gomock.Any()).
			Return(&model.InvestmentRebalance{}, nil)

		investmentTradeRepository.EXPECT().
			AddMany(tx, gomock.Any()).
			DoAndReturn(func(tx *sql.Tx, investmentTrades []*model.InvestmentTrade) ([]model.InvestmentTrade, error) {
				require.Equal(t, "", cmp.Diff(
					[]*model.InvestmentTrade{
						{
							Side:                  model.TradeOrderSide_Buy,
							TickerID:              tickerIDMap["MSFT"],
							Quantity:              decimal.NewFromFloat(0.999),
							ModifiedAt:            time.Time{},
							InvestmentRebalanceID: [16]byte{},
						},
						{
							Side:                  model.TradeOrderSide_Sell,
							TickerID:              tickerIDMap["AAPL"],
							Quantity:              decimal.NewFromFloat(0.999),
							ModifiedAt:            time.Time{},
							InvestmentRebalanceID: [16]byte{},
						},
					},
					investmentTrades,
					cmp.Comparer(func(i, j uuid.UUID) bool {
						return i.String() == j.String()
					}),
					cmpopts.SortSlices(func(i, j *model.InvestmentTrade) bool {
						return i.TickerID.String() < j.TickerID.String()
					}),
				))
				out := []model.InvestmentTrade{}
				for _, t := range investmentTrades {
					out = append(out, *t)
				}
				return out, nil
			})

		investmentTradeRepository.EXPECT().
			List(tx, repository.InvestmentTradeListFilter{
				InvestmentID:    &investment.InvestmentID,
				RebalancerRunID: &rebalancerRun.RebalancerRunID,
			}).
			Return([]model.InvestmentTradeStatus{
				{
					Symbol:   util.StringPointer("MSFT"),
					Quantity: util.DecimalPointer(decimal.NewFromFloat(0.999)),
					Side:     util.TradeOrderSidePointer(model.TradeOrderSide_Buy),
				},
			}, nil)

		response, err := handler.rebalanceInvestment(context.Background(), tx, investment, rebalancerRun, priceMap, tickerIDMap)
		require.NoError(t, err)

		require.NotEmpty(t, response)
	})
}

func Test_transitionToTarget(t *testing.T) {
	t.Run("idk", func(t *testing.T) {
		startingPortfolio := domain.Portfolio{
			Positions: map[string]*domain.Position{
				"AAPL": {
					ExactQuantity: decimal.NewFromInt(1),
				},
				"GOOG": {
					ExactQuantity: decimal.NewFromInt(1),
				},
				"MSFT": {
					ExactQuantity: decimal.NewFromInt(1),
				},
			},
			Cash: util.DecimalPointer(decimal.Zero),
		}
		targetPortfolio := domain.Portfolio{
			Positions: map[string]*domain.Position{
				"AAPL": {
					ExactQuantity: decimal.NewFromFloat(0.001),
				},
				"GOOG": {
					ExactQuantity: decimal.NewFromInt(1),
				},
				"MSFT": {
					ExactQuantity: decimal.NewFromFloat(1.999),
				},
			},
			Cash: util.DecimalPointer(decimal.Zero),
		}
		priceMap := map[string]decimal.Decimal{}
		trades, err := transitionToTarget(startingPortfolio, targetPortfolio, priceMap)
		require.NoError(t, err)

		require.Equal(t, "", cmp.Diff(
			[]*domain.ProposedTrade{
				{
					Symbol:        "MSFT",
					ExactQuantity: decimal.NewFromFloat(0.999),
				},
				{
					Symbol:        "AAPL",
					ExactQuantity: decimal.NewFromFloat(-0.999),
				},
			},
			trades,
		))
	})
}
