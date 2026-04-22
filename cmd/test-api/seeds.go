package main

import (
	"database/sql"
	"sort"

	"factorbacktest/internal/testseed"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var seeds = map[string]func(*sql.DB){
	"investment_basic": seedInvestmentBasic,
	"prices_only":      seedPricesOnly,
}

func seedInvestmentBasic(db *sql.DB) {
	aapl := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
	goog := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "GOOG", Name: "Google"})
	meta := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "META", Name: "Meta"})
	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "SPY_TOP_80"})
	for _, id := range []uuid.UUID{aapl.TickerID, goog.TickerID, meta.TickerID} {
		testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, id)
	}
	testseed.InsertPrices2020(db)
	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "test@gmail.com"})
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "test_strategy",
		UserAccountID:     user.UserAccountID,
		AssetUniverse:     "SPY_TOP_80",
		NumAssets:         3,
		RebalanceInterval: "MONTHLY",
		FactorExpression:  "pricePercentChange(\n  nDaysAgo(7),\n  currentDate\n)",
	})
	inv := testseed.CreateInvestment(db, testseed.InvestmentOpts{
		StrategyID:    strategy.StrategyID,
		UserAccountID: user.UserAccountID,
		AmountDollars: 100,
	})
	hv := testseed.CreateInvestmentHoldingsVersion(db, inv.InvestmentID)
	cash := testseed.LookupTickerBySymbol(db, ":CASH")
	testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{
		VersionID: hv.InvestmentHoldingsVersionID,
		TickerID:  cash.TickerID,
		Quantity:  decimal.NewFromInt(100),
	})
}

func seedPricesOnly(db *sql.DB) {
	testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
	testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "GOOG", Name: "Google"})
	testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "META", Name: "Meta"})
	testseed.InsertPrices2020(db)
}

func sortedSeedNames() []string {
	names := make([]string, 0, len(seeds))
	for name := range seeds {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
