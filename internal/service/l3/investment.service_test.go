package l3_service

import (
	"context"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	mock_repository "factorbacktest/internal/repository/mocks"
	l2_service "factorbacktest/internal/service/l2"
	mock_l2_service "factorbacktest/internal/service/l2/mocks"
	"factorbacktest/internal/util"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func Test_rebalanceInvestment(t *testing.T) {
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

		handler := investmentServiceHandler{
			HoldingsRepository:            holdingsRepository,
			SavedStrategyRepository:       ssRepo,
			UniverseRepository:            universeRepository,
			FactorExpressionService:       feService,
			InvestmentRebalanceRepository: investmentRebalanceRepository,
			InvestmentTradeRepository:     investmentTradeRepository,
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
		tickerIDMap := map[string]uuid.UUID{}

		latestVersionID := uuid.New()
		holdingsRepository.EXPECT().
			GetLatestVersionID(investment.InvestmentID).
			Return(&latestVersionID, nil)

		holdingsRepository.EXPECT().
			GetLatestHoldings(nil, investment.InvestmentID).
			Return(&domain.Portfolio{
				Positions: map[string]*domain.Position{
					"AAPL": {
						Quantity:      1,
						ExactQuantity: decimal.NewFromInt(100),
						TickerID:      uuid.New(),
					},
					"GOOG": {
						Quantity:      1,
						ExactQuantity: decimal.NewFromInt(100),
						TickerID:      uuid.New(),
					},
					"MSFT": {
						Quantity:      1,
						ExactQuantity: decimal.NewFromInt(100),
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
			AddMany(gomock.Any(), gomock.Any()).
			Return([]model.InvestmentTrade{}, nil)

		response, err := handler.rebalanceInvestment(context.Background(), tx, investment, rebalancerRun, priceMap, tickerIDMap)
		require.NoError(t, err)

		require.NotEmpty(t, response)
	})
}
