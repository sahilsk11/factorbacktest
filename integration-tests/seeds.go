package integration_tests

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/gocarina/gocsv"
	"github.com/shopspring/decimal"
)

// seedPrices seeds price data from sample_prices_2020.csv into the database.
func seedPrices(tx *sql.Tx) error {
	f, err := os.Open("sample_prices_2020.csv")
	if err != nil {
		return err
	}

	defer f.Close()

	type Row struct {
		Date   string          `csv:"date"`
		Symbol string          `csv:"symbol"`
		Price  decimal.Decimal `csv:"price"`
	}
	rows := []Row{}
	gocsv.UnmarshalFile(f, &rows)

	models := []model.AdjustedPrice{}
	for _, row := range rows {
		date, err := time.Parse(time.DateOnly, row.Date)
		if err != nil {
			return err
		}
		models = append(models, model.AdjustedPrice{
			Date:   date,
			Symbol: row.Symbol,
			Price:  row.Price,
		})
	}

	query := table.AdjustedPrice.INSERT(table.AdjustedPrice.MutableColumns).MODELS(models)
	_, err = query.Exec(tx)
	return err
}

// seedUniverse seeds ticker and asset universe data into the database.
func seedUniverse(tx *sql.Tx) error {
	modelsToInsert := []model.Ticker{
		{
			Symbol: "AAPL",
			Name:   "Apple",
		},
		{
			Symbol: "GOOG",
			Name:   "Google",
		},
		{
			Symbol: "META",
			Name:   "Meta",
		},
	}
	query := table.Ticker.INSERT(table.Ticker.MutableColumns).MODELS(modelsToInsert).RETURNING(table.Ticker.AllColumns)
	insertedTickers := []model.Ticker{}
	err := query.Query(tx, &insertedTickers)
	if err != nil {
		return fmt.Errorf("failed to insert tickers: %w", err)
	}

	_, err = table.Ticker.INSERT(table.Ticker.AllColumns).MODEL(model.Ticker{
		Symbol:   ":CASH",
		Name:     "cash",
		TickerID: cashTicker,
	}).Exec(tx)
	if err != nil {
		return err
	}

	query = table.AssetUniverse.INSERT(table.AssetUniverse.MutableColumns).MODEL(model.AssetUniverse{
		AssetUniverseName: "SPY_TOP_80",
	}).RETURNING(table.AssetUniverse.AllColumns)

	universe := model.AssetUniverse{}
	err = query.Query(tx, &universe)
	if err != nil {
		return fmt.Errorf("failed to insert universe: %w", err)
	}

	tickerModels := []model.AssetUniverseTicker{}
	for _, m := range insertedTickers {
		tickerModels = append(tickerModels, model.AssetUniverseTicker{
			TickerID:        m.TickerID,
			AssetUniverseID: universe.AssetUniverseID,
		})
	}

	query = table.AssetUniverseTicker.
		INSERT(table.AssetUniverseTicker.MutableColumns).
		MODELS(tickerModels)

	_, err = query.Exec(tx)
	return err
}
