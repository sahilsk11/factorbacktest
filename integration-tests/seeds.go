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

func seedPrices(db *sql.DB) error {
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
	err = gocsv.UnmarshalFile(f, &rows)
	if err != nil {
		return fmt.Errorf("failed to read csv: %w", err)
	}

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
	_, err = query.Exec(db)
	return err
}

func seedUniverse(db *sql.DB) error {
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
	err := query.Query(db, &insertedTickers)
	if err != nil {
		return fmt.Errorf("failed to insert tickers: %w", err)
	}

	query = table.AssetUniverse.INSERT(table.AssetUniverse.MutableColumns).MODEL(model.AssetUniverse{
		AssetUniverseName: "SPY_TOP_80",
	}).RETURNING(table.AssetUniverse.AllColumns)

	universe := model.AssetUniverse{}
	err = query.Query(db, &universe)
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

	_, err = query.Exec(db)
	return err
}
