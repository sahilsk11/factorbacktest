package service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/calculator"
	mock_calculator "factorbacktest/internal/calculator/mocks"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	mock_repository "factorbacktest/internal/repository/mocks"
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
		ssRepo := mock_repository.NewMockStrategyRepository(ctrl)
		feService := mock_calculator.NewMockFactorExpressionService(ctrl)
		investmentRebalanceRepository := mock_repository.NewMockInvestmentRebalanceRepository(ctrl)
		investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)
		holdingsVersionRepository := mock_repository.NewMockInvestmentHoldingsVersionRepository(ctrl)

		handler := investmentServiceHandler{
			HoldingsRepository:            holdingsRepository,
			StrategyRepository:            ssRepo,
			UniverseRepository:            universeRepository,
			FactorExpressionService:       feService,
			InvestmentRebalanceRepository: investmentRebalanceRepository,
			InvestmentTradeRepository:     investmentTradeRepository,
			HoldingsVersionRepository:     holdingsVersionRepository,
		}

		// inputs to func
		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()
		investment := model.Investment{
			InvestmentID:  uuid.New(),
			AmountDollars: 10,
			StartDate:     time.Now(),
			StrategyID:    uuid.New(),
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

		// mocked values
		startPortfolio := &domain.Portfolio{
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
		}
		scoresOnDay := &calculator.ScoresResultsOnDay{
			SymbolScores: map[string]*float64{
				"AAPL": util.FloatPointer(100),
				"GOOG": util.FloatPointer(200),
				"MSFT": util.FloatPointer(300),
			},
			Errors: []error{},
		}
		expectedTradesStatus := []*model.InvestmentTradeStatus{
			{
				Symbol:        util.StringPointer("MSFT"),
				Quantity:      util.DecimalPointer(decimal.NewFromFloat(0.999)),
				Side:          util.TradeOrderSidePointer(model.TradeOrderSide_Buy),
				ExpectedPrice: util.DecimalPointer(decimal.NewFromInt(100)),
			},
			{
				Side:          util.TradeOrderSidePointer(model.TradeOrderSide_Sell),
				Symbol:        util.StringPointer("AAPL"),
				Quantity:      util.DecimalPointer(decimal.NewFromFloat(0.999)),
				ExpectedPrice: util.DecimalPointer(decimal.NewFromInt(100)),
			},
		}

		// mocks
		{
			latestVersionID := uuid.New()
			holdingsVersionRepository.EXPECT().
				GetLatestVersionID(investment.InvestmentID).
				Return(&latestVersionID, nil)

			holdingsRepository.EXPECT().
				GetLatestHoldings(nil, investment.InvestmentID).
				Return(startPortfolio, nil)

			ssRepo.EXPECT().
				Get(gomock.Any()).
				Return(&model.Strategy{
					AssetUniverse: "universe",
					NumAssets:     3,
				}, nil)

			universeRepository.EXPECT().
				GetAssets("universe").
				Return([]model.Ticker{}, nil)

			feService.EXPECT().
				CalculateLatestFactorScores(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(
					scoresOnDay, nil,
				)

			investmentRebalanceRepository.EXPECT().
				Add(gomock.Any(), gomock.Any()).
				Return(&model.InvestmentRebalance{}, nil)

			expectedInvestmentTrades := []*model.InvestmentTrade{}
			for _, t := range expectedTradesStatus {
				expectedInvestmentTrades = append(expectedInvestmentTrades, &model.InvestmentTrade{
					Side:          *t.Side,
					TickerID:      tickerIDMap[*t.Symbol],
					Quantity:      *t.Quantity,
					ExpectedPrice: *t.ExpectedPrice,
				})
			}

			investmentTradeRepository.EXPECT().
				AddMany(tx, gomock.Any()).
				DoAndReturn(func(tx *sql.Tx, investmentTrades []*model.InvestmentTrade) ([]model.InvestmentTrade, error) {
					require.Equal(t, "", cmp.Diff(
						expectedInvestmentTrades,
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

			// investmentTradeRepository.EXPECT().
			// 	List(tx, repository.InvestmentTradeListFilter{
			// 		InvestmentID:    &investment.InvestmentID,
			// 		RebalancerRunID: &rebalancerRun.RebalancerRunID,
			// 	}).
			// 	Return(expectedTradesStatus, nil)
		}

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
		priceMap := map[string]decimal.Decimal{
			"MSFT": decimal.NewFromInt(100),
			"AAPL": decimal.NewFromInt(100),
		}
		trades, err := transitionToTarget(context.Background(), startingPortfolio, targetPortfolio, priceMap)
		require.NoError(t, err)

		require.Equal(t, "", cmp.Diff(
			[]*domain.ProposedTrade{
				{
					Symbol:        "MSFT",
					ExactQuantity: decimal.NewFromFloat(0.999),
					ExpectedPrice: decimal.NewFromInt(100),
				},
				{
					Symbol:        "AAPL",
					ExactQuantity: decimal.NewFromFloat(-0.999),
					ExpectedPrice: decimal.NewFromInt(100),
				},
			},
			trades,
			cmpopts.SortSlices(func(i, j *domain.ProposedTrade) bool {
				return i.Symbol > j.Symbol
			}),
		))
	})
}

func Test_filterLowVolumeTrades(t *testing.T) {
	t.Run("cancel out some buys", func(t *testing.T) {
		newTrades := filterLowVolumeTrades(
			[]*domain.ProposedTrade{
				{
					Symbol:        "AAPL",
					TickerID:      [16]byte{},
					ExactQuantity: decimal.NewFromFloat(2.1),
					ExpectedPrice: decimal.NewFromInt(1),
				},
				{
					Symbol:        "GOOG",
					TickerID:      [16]byte{},
					ExactQuantity: decimal.NewFromInt(2),
					ExpectedPrice: decimal.NewFromInt(1),
				},
				{
					Symbol:        "MSFT",
					TickerID:      [16]byte{},
					ExactQuantity: decimal.NewFromFloat(-1.99),
					ExpectedPrice: decimal.NewFromInt(1),
				},
			},
			decimal.NewFromInt(2),
		)

		require.Equal(t, "", cmp.Diff(
			[]*domain.ProposedTrade{
				{
					Symbol:        "AAPL",
					TickerID:      [16]byte{},
					ExactQuantity: decimal.NewFromFloat(2.1),
					ExpectedPrice: decimal.NewFromInt(1),
				},
			},
			newTrades,
			cmpopts.SortSlices(func(i, j *domain.ProposedTrade) bool {
				return i.Symbol > j.Symbol
			}),
		))
	})

	t.Run("some real trades that produced 0 quantity results", func(t *testing.T) {
		newTrades := filterLowVolumeTrades(
			[]*domain.ProposedTrade{
				{
					Symbol: "NVDA",
					// TickerID:      4690d13c-83d5-4ca0-9e9e-31403d873cac,
					ExactQuantity: decimal.NewFromFloat(0.0725128446042737),
					ExpectedPrice: decimal.NewFromFloat(104.23),
				},
				{
					Symbol: "AVGO",
					// TickerID:      80f44ca8-989b-4179-9a44-9b2ea6cd3aaf,
					ExactQuantity: decimal.NewFromFloat(0.0100321256038648),
					ExpectedPrice: decimal.NewFromFloat(143.52),
				},
				{
					Symbol: "ABBV",
					// TickerID:      09c2f210-8d30-4ef2-ad80-23b9b83ded02,
					ExactQuantity: decimal.NewFromFloat(0.0000199282582702),
					ExpectedPrice: decimal.NewFromFloat(150.54),
				},
				{
					Symbol: "AMAT",
					// TickerID:      0c2e2c97-83b3-4529-925b-3d0061ff80e8,
					ExactQuantity: decimal.NewFromFloat(-0.0000173591019558),
					ExpectedPrice: decimal.NewFromFloat(189.8),
				},
			},
			decimal.NewFromFloat(0.01),
		)

		require.Equal(t, "", cmp.Diff(
			[]*domain.ProposedTrade{
				{
					Symbol: "NVDA",
					// TickerID: 4690d13c-83d5-4ca0-9e9e-31403d873cac,
					ExactQuantity: decimal.NewFromFloat(0.0725128446042737),
					ExpectedPrice: decimal.NewFromFloat(104.23),
				},
				{
					Symbol: "AVGO",
					// TickerID: 80f44ca8-989b-4179-9a44-9b2ea6cd3aaf,
					ExactQuantity: decimal.NewFromFloat(0.0100300718305146),
					ExpectedPrice: decimal.NewFromFloat(143.52),
				},
			},
			newTrades,
			cmpopts.SortSlices(func(i, j *domain.ProposedTrade) bool {
				return i.Symbol > j.Symbol
			}),
		))
	})
}
