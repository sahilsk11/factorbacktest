package main

import (
	"database/sql"
	"sort"
	"time"

	"factorbacktest/internal/testseed"

	"github.com/google/uuid"
)

var seeds = map[string]func(*sql.DB){
	"home_strategies": seedHomeStrategies,
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

	start := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Now().UTC()
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

func sortedSeedNames() []string {
	names := make([]string, 0, len(seeds))
	for name := range seeds {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
