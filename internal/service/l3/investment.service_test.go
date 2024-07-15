package l3_service

import (
	"context"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"
	l2_service "factorbacktest/internal/service/l2"
	mock_l2_service "factorbacktest/internal/service/l2/mocks"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func Test_GenerateProposedTrades(t *testing.T) {
	ctx := context.Background()
	date := time.Now().UTC()
	ctrl := gomock.NewController(t)

	savedStrategyRepository := mock_repository.NewMockSavedStrategyRepository(ctrl)
	strategyInvestmentRepository := mock_repository.NewMockStrategyInvestmentRepository(ctrl)
	holdingsRepository := mock_repository.NewMockStrategyInvestmentHoldingsRepository(ctrl)
	priceRepository := mock_repository.NewMockAdjustedPriceRepository(ctrl)
	universeRepository := mock_repository.NewMockAssetUniverseRepository(ctrl)
	tickerRepository := mock_repository.NewMockTickerRepository(ctrl)

	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	factorExpressionService := mock_l2_service.NewMockFactorExpressionService(ctrl)

	handler := investmentServiceHandler{
		StrategyInvestmentRepository: strategyInvestmentRepository,
		HoldingsRepository:           holdingsRepository,
		PriceRepository:              priceRepository,
		UniverseRepository:           universeRepository,
		SavedStrategyRepository:      savedStrategyRepository,
		FactorExpressionService:      factorExpressionService,
		TickerRepository:             tickerRepository,
		AlpacaRepository:             alpacaRepository,
	}

	alpacaRepository.EXPECT().
		GetPositions().
		Return([]alpaca.Position{}, nil)

	allTickers := []model.Ticker{{Symbol: "test_ticker"}}
	tickerRepository.EXPECT().
		List().
		Return(allTickers, nil)

	priceRepository.EXPECT().
		GetManyOnDay(gomock.Any(), date).
		Return(
			map[string]float64{
				"AAPL": 100,
				"MSFT": 100,
				"NVDA": 100,
				"GOOG": 100,
			}, nil,
		)

	// mocks for getTargetPortfolio
	{
		savedStrategy := model.SavedStrategy{
			SavedStragyID:    uuid.New(),
			NumAssets:        4,
			FactorExpression: "test_expression",
			AssetUniverse:    "test_universe",
		}

		savedStrategyRepository.EXPECT().
			Get(savedStrategy.SavedStragyID).
			Return(&savedStrategy, nil)

		strategyInvestment := model.StrategyInvestment{
			StrategyInvestmentID: uuid.New(),
			SavedStragyID:        savedStrategy.SavedStragyID,
			AmountDollars:        100,
			StartDate:            date,
		}

		strategyInvestmentRepository.EXPECT().
			Get(strategyInvestment.StrategyInvestmentID).
			Return(&strategyInvestment, nil)

		strategyInvestmentRepository.EXPECT().
			List(repository.StrategyInvestmentListFilter{}).
			Return([]model.StrategyInvestment{
				strategyInvestment,
			}, nil)

		holdingsRepository.EXPECT().
			GetLatestHoldings(strategyInvestment.StrategyInvestmentID).
			Return(&domain.Portfolio{
				Cash: 1000,
			}, nil)

		universe := []model.Ticker{{}}
		universeRepository.EXPECT().
			GetAssets("test_universe").
			Return(universe, nil)

		factorExpressionService.EXPECT().
			CalculateFactorScoresOnDay(gomock.Any(), date, universe, "test_expression").Return(
			&l2_service.ScoresResultsOnDay{
				SymbolScores: map[string]*float64{
					"AAPL": internal.FloatPointer(100),
					"GOOG": internal.FloatPointer(100),
					"NVDA": internal.FloatPointer(100),
					"MSFT": internal.FloatPointer(100),
				},
			}, nil,
		)
	}

	proposedTrades, err := handler.GenerateProposedTrades(ctx, date)
	require.NoError(t, err)

	require.Equal(
		t,
		"",
		cmp.Diff(
			[]domain.ProposedTrade{
				{
					Symbol:        "GOOG",
					ExactQuantity: decimal.NewFromFloat(2.5),
					ExpectedPrice: 100,
				},
				{
					Symbol:        "NVDA",
					ExactQuantity: decimal.NewFromFloat(2.5),
					ExpectedPrice: 100,
				},
				{
					Symbol:        "MSFT",
					ExactQuantity: decimal.NewFromFloat(2.5),
					ExpectedPrice: 100,
				},
				{
					Symbol:        "AAPL",
					ExactQuantity: decimal.NewFromFloat(2.5),
					ExpectedPrice: 100,
				},
			},
			proposedTrades,
			cmpopts.SortSlices(func(i, j domain.ProposedTrade) bool {
				return i.Symbol < j.Symbol
			}),
		),
	)
}
