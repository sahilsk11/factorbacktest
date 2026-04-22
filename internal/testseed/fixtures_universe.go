package testseed

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/gocarina/gocsv"
	"github.com/shopspring/decimal"
)

// FixtureBaseUniverse is a minimal asset universe used by every flow that
// needs a ticker + universe. It matches the shape the integration tests
// relied on before this package existed: three tickers (AAPL, GOOG, META) in
// a universe named SPY_TOP_80.
const FixtureBaseUniverse = "base_universe"

// Keys exposed by base_universe.
const (
	KeyUniverseID    = "universe_id"
	KeyAAPLTickerID  = "aapl_ticker_id"
	KeyGOOGTickerID  = "goog_ticker_id"
	KeyMETATickerID  = "meta_ticker_id"
	KeyTickerIDBySym = "ticker_id_by_symbol" // map[string]uuid.UUID
)

var baseUniverseFixture = Fixture{
	Name: FixtureBaseUniverse,
	Apply: func(ctx context.Context, db *sql.DB, _ map[string]Result) (Result, error) {
		tickers := []model.Ticker{
			{Symbol: "AAPL", Name: "Apple"},
			{Symbol: "GOOG", Name: "Google"},
			{Symbol: "META", Name: "Meta"},
		}
		var inserted []model.Ticker
		if err := table.Ticker.
			INSERT(table.Ticker.MutableColumns).
			MODELS(tickers).
			RETURNING(table.Ticker.AllColumns).
			Query(db, &inserted); err != nil {
			return nil, fmt.Errorf("insert tickers: %w", err)
		}

		var universe model.AssetUniverse
		if err := table.AssetUniverse.
			INSERT(table.AssetUniverse.MutableColumns).
			MODEL(model.AssetUniverse{AssetUniverseName: "SPY_TOP_80"}).
			RETURNING(table.AssetUniverse.AllColumns).
			Query(db, &universe); err != nil {
			return nil, fmt.Errorf("insert universe: %w", err)
		}

		links := make([]model.AssetUniverseTicker, 0, len(inserted))
		bySymbol := map[string]any{}
		for _, t := range inserted {
			links = append(links, model.AssetUniverseTicker{
				TickerID:        t.TickerID,
				AssetUniverseID: universe.AssetUniverseID,
			})
			bySymbol[t.Symbol] = t.TickerID
		}
		if _, err := table.AssetUniverseTicker.
			INSERT(table.AssetUniverseTicker.MutableColumns).
			MODELS(links).
			Exec(db); err != nil {
			return nil, fmt.Errorf("insert asset_universe_ticker: %w", err)
		}

		return Result{
			KeyUniverseID:    universe.AssetUniverseID,
			KeyAAPLTickerID:  bySymbol["AAPL"],
			KeyGOOGTickerID:  bySymbol["GOOG"],
			KeyMETATickerID:  bySymbol["META"],
			KeyTickerIDBySym: bySymbol,
		}, nil
	},
}

//go:embed data/prices_2020.csv
var prices2020CSV []byte

// FixturePrices2020 inserts the full 2020 price series for the base
// universe tickers. Depends on base_universe so the tickers exist.
const FixturePrices2020 = "prices_2020"

var prices2020Fixture = Fixture{
	Name:         FixturePrices2020,
	Dependencies: []string{FixtureBaseUniverse},
	Apply: func(ctx context.Context, db *sql.DB, _ map[string]Result) (Result, error) {
		type row struct {
			Date   string          `csv:"date"`
			Symbol string          `csv:"symbol"`
			Price  decimal.Decimal `csv:"price"`
		}
		var rows []row
		if err := gocsv.Unmarshal(bytes.NewReader(prices2020CSV), &rows); err != nil {
			return nil, fmt.Errorf("parse embedded prices csv: %w", err)
		}

		models := make([]model.AdjustedPrice, 0, len(rows))
		for _, r := range rows {
			d, err := time.Parse(time.DateOnly, r.Date)
			if err != nil {
				return nil, fmt.Errorf("parse date %q: %w", r.Date, err)
			}
			models = append(models, model.AdjustedPrice{
				Date:   d,
				Symbol: r.Symbol,
				Price:  r.Price,
			})
		}

		if _, err := table.AdjustedPrice.
			INSERT(table.AdjustedPrice.MutableColumns).
			MODELS(models).
			Exec(db); err != nil {
			return nil, fmt.Errorf("insert adjusted_price: %w", err)
		}
		return Result{"row_count": len(models)}, nil
	},
}
