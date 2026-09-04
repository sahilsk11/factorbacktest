package service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"
	"factorbacktest/internal/testseed"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	_ "github.com/lib/pq"
)

func newHoldingsOverrideTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "postgresql://postgres:postgres@localhost:5440/postgres_test?sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	return db
}

func newHoldingsOverrideTestUser(db *sql.DB) model.UserAccount {
	return testseed.CreateUserAccount(db, testseed.UserAccountOpts{
		Email: "holdings-override-" + uuid.NewString() + "@example.com",
	})
}

func lookupOrCreateTicker(db *sql.DB, symbol, name string) model.Ticker {
	tickerRepo := repository.NewTickerRepository(db)
	ticker, err := tickerRepo.GetBySymbol(symbol)
	if err == nil {
		return *ticker
	}
	return testseed.CreateTicker(db, testseed.TickerOpts{Symbol: symbol, Name: name})
}

func TestAdminReplaceHoldingsCreatesNotedVersion(t *testing.T) {
	db := newHoldingsOverrideTestDB(t)
	defer db.Close()

	user := newHoldingsOverrideTestUser(db)
	universeName := "override-test-" + uuid.NewString()
	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: universeName})
	intc := lookupOrCreateTicker(db, "INTC", "Intel")
	testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, intc.TickerID)
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "override-strategy-" + uuid.NewString(),
		FactorExpression:  "close",
		AssetUniverse:     universe.AssetUniverseName,
		RebalanceInterval: "monthly",
		NumAssets:         1,
		UserAccountID:     user.UserAccountID,
	})
	investment := testseed.CreateInvestment(db, testseed.InvestmentOpts{
		StrategyID:    strategy.StrategyID,
		UserAccountID: user.UserAccountID,
		AmountDollars: 1000,
	})
	initialVersion := testseed.CreateInvestmentHoldingsVersion(db, investment.InvestmentID)
	cashTicker := testseed.LookupTickerBySymbol(db, ":CASH")
	testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{
		VersionID: initialVersion.InvestmentHoldingsVersionID,
		TickerID:  cashTicker.TickerID,
		Quantity:  decimal.NewFromInt(100),
	})

	handler := NewInvestmentService(
		db,
		repository.NewInvestmentRepository(db),
		repository.NewInvestmentHoldingsRepository(db),
		repository.NewAssetUniverseRepository(db),
		repository.NewStrategyRepository(db),
		nil,
		repository.NewTickerRepository(db),
		repository.NewRebalancerRunRepository(db),
		repository.NewInvestmentHoldingsVersionRepository(db),
		repository.NewInvestmentTradeRepository(db),
		BacktestHandler{},
		nil,
		nil,
		repository.NewInvestmentRebalanceRepository(db),
		repository.NewAdjustedPriceRepository(db),
		repository.NewRebalancePriceRepository(db),
		nil,
	)

	cash := 1.22
	result, err := handler.AdminReplaceHoldings(context.Background(), investment.InvestmentID, AdminReplaceHoldingsRequest{
		Note: "Broker ledger correction for INTC",
		Cash: &cash,
		Positions: []AdminReplaceHoldingsPosition{
			{Symbol: "INTC", Quantity: 0.764168},
		},
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, result.VersionID)
	require.Equal(t, cash, result.Holdings.Cash)
	require.Len(t, result.Holdings.Positions, 1)
	require.Equal(t, "INTC", result.Holdings.Positions[0].Symbol)
	require.InDelta(t, 0.764168, result.Holdings.Positions[0].Quantity, 0.000001)

	versionRepo := repository.NewInvestmentHoldingsVersionRepository(db)
	latestVersionID, err := versionRepo.GetLatestVersionID(investment.InvestmentID)
	require.NoError(t, err)
	require.Equal(t, result.VersionID, *latestVersionID)

	version, err := versionRepo.Get(result.VersionID)
	require.NoError(t, err)
	require.NotNil(t, version.Note)
	require.Equal(t, "Broker ledger correction for INTC", *version.Note)
	require.Nil(t, version.RebalancerRunID)

	holdingsRepo := repository.NewInvestmentHoldingsRepository(db)
	latestHoldings, err := holdingsRepo.GetLatestHoldings(nil, investment.InvestmentID)
	require.NoError(t, err)
	require.InDelta(t, cash, latestHoldings.Cash.InexactFloat64(), 0.000001)
	require.Len(t, latestHoldings.Positions, 1)
	require.InDelta(t, 0.764168, latestHoldings.Positions["INTC"].ExactQuantity.InexactFloat64(), 0.000001)
}

func TestReconcileTradesUsesEarliestVersionWithoutNote(t *testing.T) {
	ctrl := gomock.NewController(t)

	holdingsVersionRepository := mock_repository.NewMockInvestmentHoldingsVersionRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)

	investmentID := uuid.New()
	earliestVersionID := uuid.New()
	cash := decimal.NewFromInt(100)
	initialPortfolio := &domain.Portfolio{
		Cash:      &cash,
		Positions: map[string]*domain.Position{},
	}

	handler := investmentServiceHandler{
		HoldingsVersionRepository: holdingsVersionRepository,
		HoldingsRepository:        holdingsRepository,
		InvestmentTradeRepository: investmentTradeRepository,
	}

	holdingsVersionRepository.EXPECT().
		GetLatestNotedVersion(investmentID).
		Return(nil, nil)
	holdingsVersionRepository.EXPECT().
		GetEarliestVersionID(investmentID).
		Return(&earliestVersionID, nil)
	holdingsRepository.EXPECT().
		Get(earliestVersionID).
		Return(initialPortfolio, nil)
	investmentTradeRepository.EXPECT().
		List(nil, gomock.AssignableToTypeOf(repository.InvestmentTradeListFilter{})).
		DoAndReturn(func(_ *sql.Tx, filter repository.InvestmentTradeListFilter) ([]*model.InvestmentTradeStatus, error) {
			require.Equal(t, investmentID, *filter.InvestmentID)
			require.Nil(t, filter.CreatedAfter)
			return []*model.InvestmentTradeStatus{}, nil
		})
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, investmentID).
		Return(initialPortfolio, nil)

	reconErr, err := handler.reconcileTrades(investmentID)
	require.NoError(t, err)
	require.Nil(t, reconErr)
}

func TestReconcileTradesUsesLatestNotedVersionBaseline(t *testing.T) {
	ctrl := gomock.NewController(t)

	holdingsVersionRepository := mock_repository.NewMockInvestmentHoldingsVersionRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)

	investmentID := uuid.New()
	notedVersionID := uuid.New()
	baselineCreatedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	cash := decimal.RequireFromString("1.22")
	intcQty := decimal.RequireFromString("0.764168")
	baselinePortfolio := &domain.Portfolio{
		Cash: &cash,
		Positions: map[string]*domain.Position{
			"INTC": {
				Symbol:        "INTC",
				ExactQuantity: intcQty,
			},
		},
	}

	handler := investmentServiceHandler{
		HoldingsVersionRepository: holdingsVersionRepository,
		HoldingsRepository:        holdingsRepository,
		InvestmentTradeRepository: investmentTradeRepository,
	}

	holdingsVersionRepository.EXPECT().
		GetLatestNotedVersion(investmentID).
		Return(&model.InvestmentHoldingsVersion{
			InvestmentHoldingsVersionID: notedVersionID,
			InvestmentID:                investmentID,
			CreatedAt:                   baselineCreatedAt,
			Note:                        strPtr("manual override"),
		}, nil)
	holdingsRepository.EXPECT().
		Get(notedVersionID).
		Return(baselinePortfolio, nil)
	investmentTradeRepository.EXPECT().
		List(nil, gomock.AssignableToTypeOf(repository.InvestmentTradeListFilter{})).
		DoAndReturn(func(_ *sql.Tx, filter repository.InvestmentTradeListFilter) ([]*model.InvestmentTradeStatus, error) {
			require.Equal(t, investmentID, *filter.InvestmentID)
			require.NotNil(t, filter.CreatedAfter)
			require.Equal(t, baselineCreatedAt, *filter.CreatedAfter)
			return []*model.InvestmentTradeStatus{}, nil
		})
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, investmentID).
		Return(baselinePortfolio, nil)

	reconErr, err := handler.reconcileTrades(investmentID)
	require.NoError(t, err)
	require.Nil(t, reconErr)
}

func TestReconcileTradesPassesAfterOverrideWithLaterTradeReplay(t *testing.T) {
	ctrl := gomock.NewController(t)

	holdingsVersionRepository := mock_repository.NewMockInvestmentHoldingsVersionRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)

	investmentID := uuid.New()
	notedVersionID := uuid.New()
	baselineCreatedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	cash := decimal.RequireFromString("10")
	intcQty := decimal.RequireFromString("1")
	tickerID := uuid.New()
	baselinePortfolio := &domain.Portfolio{
		Cash: &cash,
		Positions: map[string]*domain.Position{
			"INTC": {
				Symbol:        "INTC",
				TickerID:      tickerID,
				ExactQuantity: intcQty,
			},
		},
	}
	buyQty := decimal.RequireFromString("0.5")
	buyPrice := decimal.RequireFromString("20")
	side := model.TradeOrderSide_Buy
	symbol := "INTC"
	tradeCreatedAt := baselineCreatedAt.Add(time.Hour)
	expectedPortfolio := AddTradesToPortfolio([]*model.InvestmentTradeStatus{
		{
			Symbol:      &symbol,
			Side:        &side,
			Quantity:    &buyQty,
			FilledPrice: &buyPrice,
			TickerID:    &tickerID,
			CreatedAt:   &tradeCreatedAt,
		},
	}, baselinePortfolio)

	handler := investmentServiceHandler{
		HoldingsVersionRepository: holdingsVersionRepository,
		HoldingsRepository:        holdingsRepository,
		InvestmentTradeRepository: investmentTradeRepository,
	}

	holdingsVersionRepository.EXPECT().
		GetLatestNotedVersion(investmentID).
		Return(&model.InvestmentHoldingsVersion{
			InvestmentHoldingsVersionID: notedVersionID,
			InvestmentID:                investmentID,
			CreatedAt:                   baselineCreatedAt,
			Note:                        strPtr("manual override"),
		}, nil)
	holdingsRepository.EXPECT().
		Get(notedVersionID).
		Return(baselinePortfolio, nil)
	investmentTradeRepository.EXPECT().
		List(nil, gomock.Any()).
		Return([]*model.InvestmentTradeStatus{
			{
				Symbol:      &symbol,
				Side:        &side,
				Quantity:    &buyQty,
				FilledPrice: &buyPrice,
				TickerID:    &tickerID,
				CreatedAt:   &tradeCreatedAt,
			},
		}, nil)
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, investmentID).
		Return(expectedPortfolio, nil)

	reconErr, err := handler.reconcileTrades(investmentID)
	require.NoError(t, err)
	require.Nil(t, reconErr)
}

func strPtr(s string) *string {
	return &s
}

func TestGetLatestNotedVersionRepository(t *testing.T) {
	db := newHoldingsOverrideTestDB(t)
	defer db.Close()

	user := newHoldingsOverrideTestUser(db)
	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "noted-version-" + uuid.NewString()})
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "noted-version-strategy-" + uuid.NewString(),
		FactorExpression:  "close",
		AssetUniverse:     universe.AssetUniverseName,
		RebalanceInterval: "monthly",
		NumAssets:         1,
		UserAccountID:     user.UserAccountID,
	})
	investment := testseed.CreateInvestment(db, testseed.InvestmentOpts{
		StrategyID:    strategy.StrategyID,
		UserAccountID: user.UserAccountID,
		AmountDollars: 1000,
	})

	note := "first override"
	_, err := table.InvestmentHoldingsVersion.
		INSERT(table.InvestmentHoldingsVersion.MutableColumns).
		MODEL(model.InvestmentHoldingsVersion{
			InvestmentID: investment.InvestmentID,
			CreatedAt:    time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			Note:         &note,
		}).
		Exec(db)
	require.NoError(t, err)

	latestNote := "latest override"
	_, err = table.InvestmentHoldingsVersion.
		INSERT(table.InvestmentHoldingsVersion.MutableColumns).
		MODEL(model.InvestmentHoldingsVersion{
			InvestmentID: investment.InvestmentID,
			CreatedAt:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			Note:         &latestNote,
		}).
		Exec(db)
	require.NoError(t, err)

	repo := repository.NewInvestmentHoldingsVersionRepository(db)
	version, err := repo.GetLatestNotedVersion(investment.InvestmentID)
	require.NoError(t, err)
	require.NotNil(t, version)
	require.Equal(t, latestNote, *version.Note)
}
