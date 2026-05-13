package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"factorbacktest/internal/testseed"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

var seeds = map[string]func(*sql.DB){
	"home_strategies": seedHomeStrategies,
}

func main() {
	dsn := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres connection string")
	seedName := flag.String("seed", "home_strategies", "seed name")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("-database-url or DATABASE_URL is required")
	}
	fn, ok := seeds[*seedName]
	if !ok {
		log.Fatalf("unknown seed %q; known: %v", *seedName, sortedSeedNames())
	}

	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	fn(db)
	log.Printf("seed %q applied", *seedName)
}

func seedHomeStrategies(db *sql.DB) {
	if alreadySeeded(db, "SPY_TOP_80") {
		log.Printf("seed already present; skipping")
		return
	}

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

func alreadySeeded(db *sql.DB, universeName string) bool {
	var count int
	err := db.QueryRow(`SELECT count(*) FROM asset_universe WHERE asset_universe_name = $1`, universeName).Scan(&count)
	if err != nil {
		panic(fmt.Errorf("check seed state: %w", err))
	}
	return count > 0
}

func sortedSeedNames() []string {
	names := make([]string, 0, len(seeds))
	for name := range seeds {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
