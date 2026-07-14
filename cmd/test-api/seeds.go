package main

import (
	"database/sql"
	"sort"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/testseed"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var seededUserAccountID *uuid.UUID

var seeds = map[string]func(*sql.DB){
	"home_strategies":        seedHomeStrategies,
	"active_investment":      seedActiveInvestment,
	"liquidating_investment": seedLiquidatingInvestment,
}

func seedHomeStrategies(db *sql.DB) {
	aapl := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
	goog := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "GOOG", Name: "Google"})
	meta := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "META", Name: "Meta"})
	testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "SPY", Name: "SPDR S&P 500 ETF"})

	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{
		Name:        "SPY_TOP_80",
		DisplayName: "SPY_TOP_80",
	})
	for _, id := range []uuid.UUID{aapl.TickerID, goog.TickerID, meta.TickerID} {
		testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, id)
	}

	// Cover the frontend's default backtest range (today - 3y -> today) with
	// a ~100-day buffer for the longest nDaysAgo() lookback in the seeded
	// strategies (90-day momentum).
	end := time.Now().UTC()
	start := end.AddDate(-3, 0, -100)
	testseed.InsertSyntheticPrices(db, testseed.SyntheticPricesOpts{
		Symbols: []string{"AAPL", "GOOG", "META", "SPY"},
		Start:   start,
		End:     end,
	})

	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "test@gmail.com"})

	testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "7_day_momentum_monthly",
		UserAccountID:     user.UserAccountID,
		AssetUniverse:     "SPY_TOP_80",
		NumAssets:         3,
		RebalanceInterval: "MONTHLY",
		FactorExpression:  "pricePercentChange(\n  nDaysAgo(7),\n  currentDate\n)",
		Published:         true,
	})
	testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "30_day_momentum_monthly",
		UserAccountID:     user.UserAccountID,
		AssetUniverse:     "SPY_TOP_80",
		NumAssets:         3,
		RebalanceInterval: "MONTHLY",
		FactorExpression:  "pricePercentChange(\n  nDaysAgo(30),\n  currentDate\n)",
		Published:         true,
	})
	testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "90_day_momentum_monthly",
		UserAccountID:     user.UserAccountID,
		AssetUniverse:     "SPY_TOP_80",
		NumAssets:         3,
		RebalanceInterval: "MONTHLY",
		FactorExpression:  "pricePercentChange(\n  nDaysAgo(90),\n  currentDate\n)",
		Published:         true,
	})
}

func seedActiveInvestment(db *sql.DB) {
	seedInvestment(db, false)
}

func seedLiquidatingInvestment(db *sql.DB) {
	seedInvestment(db, true)
}

func seedInvestment(db *sql.DB, liquidationRequested bool) {
	amd := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AMD", Name: "Advanced Micro Devices"})
	intc := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "INTC", Name: "Intel"})
	amat := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AMAT", Name: "Applied Materials"})
	testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "SPY", Name: "SPDR S&P 500 ETF"})

	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{
		Name:        "SPY_TOP_80",
		DisplayName: "SPY_TOP_80",
	})
	for _, id := range []uuid.UUID{amd.TickerID, intc.TickerID, amat.TickerID} {
		testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, id)
	}

	end := time.Now().UTC()
	start := end.AddDate(-3, 0, -100)
	testseed.InsertSyntheticPrices(db, testseed.SyntheticPricesOpts{
		Symbols: []string{"AMD", "INTC", "AMAT", "SPY"},
		Start:   start,
		End:     end,
	})

	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "test@gmail.com"})
	seededUserAccountID = &user.UserAccountID
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "Seeded Momentum",
		UserAccountID:     user.UserAccountID,
		AssetUniverse:     "SPY_TOP_80",
		NumAssets:         3,
		RebalanceInterval: "MONTHLY",
		FactorExpression:  "pricePercentChange(\n  nDaysAgo(30),\n  currentDate\n)",
		Published:         true,
	})
	investment := testseed.CreateInvestment(db, testseed.InvestmentOpts{
		StrategyID:    strategy.StrategyID,
		UserAccountID: user.UserAccountID,
		AmountDollars: 100,
		StartDate:     time.Now().UTC().AddDate(0, -3, 0),
	})
	if liquidationRequested {
		now := time.Now().UTC()
		_, err := table.Investment.
			UPDATE(table.Investment.LiquidationRequestedAt, table.Investment.ModifiedAt).
			SET(postgres.TimestampzT(now), postgres.TimestampzT(now)).
			WHERE(table.Investment.InvestmentID.EQ(postgres.UUID(investment.InvestmentID))).
			Exec(db)
		if err != nil {
			panic(err)
		}
	}

	version := testseed.CreateInvestmentHoldingsVersion(db, investment.InvestmentID)
	for _, holding := range []struct {
		ticker   model.Ticker
		quantity string
	}{
		{ticker: amd, quantity: "0.15"},
		{ticker: intc, quantity: "1.2"},
		{ticker: amat, quantity: "0.1"},
	} {
		qty, err := decimal.NewFromString(holding.quantity)
		if err != nil {
			panic(err)
		}
		testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{
			VersionID: version.InvestmentHoldingsVersionID,
			TickerID:  holding.ticker.TickerID,
			Quantity:  qty,
		})
	}
}

func sortedSeedNames() []string {
	names := make([]string, 0, len(seeds))
	for name := range seeds {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
