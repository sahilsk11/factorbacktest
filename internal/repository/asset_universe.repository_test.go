package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
	"testing"

	"github.com/go-jet/jet/v2/postgres"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func newTestDb() (*sql.DB, error) {
	connStr := "postgresql://postgres:postgres@localhost:5440/postgres_test?sslmode=disable"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return dbConn, nil
}

func cleanupUniverse(db *sql.DB) error {
	if _, err := table.AdjustedPrice.DELETE().WHERE(postgres.Bool(true)).Exec(db); err != nil {
		return err
	}
	if _, err := table.AssetUniverseTicker.DELETE().WHERE(postgres.Bool(true)).Exec(db); err != nil {
		return err
	}
	if _, err := table.AssetUniverse.DELETE().WHERE(postgres.Bool(true)).Exec(db); err != nil {
		return err
	}
	if _, err := table.Ticker.DELETE().WHERE(postgres.Bool(true)).Exec(db); err != nil {
		return err
	}
	return nil
}

func seedOneUniverse(tx *sql.Tx) error {
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
	if err != nil {
		return fmt.Errorf("failed to insert asset universe tickers: %w", err)
	}

	return nil
}

func seedTwoUniverses(tx *sql.Tx) error {
	err := seedOneUniverse(tx)
	if err != nil {
		return fmt.Errorf("failed to seed first universe: %w", err)
	}

	query := table.Ticker.SELECT(table.Ticker.AllColumns).
		WHERE(table.Ticker.Symbol.EQ(postgres.String("AAPL")))
	appleTicker := model.Ticker{}
	err = query.Query(tx, &appleTicker)
	if err != nil {
		return fmt.Errorf("failed to query Apple ticker: %w", err)
	}
	query = nil

	// kind of awkward but i don't have a second universe yet lol
	query1 := table.AssetUniverse.INSERT(table.AssetUniverse.MutableColumns).MODEL(model.AssetUniverse{
		AssetUniverseName: "ALL",
	}).RETURNING(table.AssetUniverse.AllColumns)

	universe := model.AssetUniverse{}
	err = query1.Query(tx, &universe)
	if err != nil {
		return fmt.Errorf("failed to insert universe: %w", err)
	}

	query1 = table.AssetUniverseTicker.
		INSERT(table.AssetUniverseTicker.MutableColumns).
		MODEL(model.AssetUniverseTicker{
			TickerID:        appleTicker.TickerID,
			AssetUniverseID: universe.AssetUniverseID,
		})

	_, err = query1.Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to insert asset universe tickers: %w", err)
	}

	return nil
}

func Test_assetUniverseRepositoryHandler_GetAssets(t *testing.T) {
	db, err := newTestDb()
	require.NoError(t, err)
	t.Run("get assets from one universe", func(t *testing.T) {
		cleanupUniverse(db)
		tx, err := db.Begin()
		require.NoError(t, err)
		defer func() {
			err := tx.Rollback()
			require.NoError(t, err)
		}()
		err = seedOneUniverse(tx)
		require.NoError(t, err)

		handler := assetUniverseRepositoryHandler{tx}

		tickers, err := handler.GetAssets("SPY_TOP_80")
		require.NoError(t, err)
		require.Equal(t, 3, len(tickers))
	})
	t.Run("get assets from all universes", func(t *testing.T) {
		cleanupUniverse(db)
		tx, err := db.Begin()
		require.NoError(t, err)
		defer func() {
			err := tx.Rollback()
			require.NoError(t, err)
		}()
		err = seedTwoUniverses(tx)
		require.NoError(t, err)

		handler := assetUniverseRepositoryHandler{tx}

		tickerSet := map[string]struct{}{}

		tickers, err := handler.GetAssets("ALL")
		require.NoError(t, err)

		for _, ticker := range tickers {
			if _, ok := tickerSet[ticker.Symbol]; ok {
				require.Fail(t, fmt.Sprintf("duplicate entry for ticker %s", ticker.Symbol))
			}
			tickerSet[ticker.Symbol] = struct{}{}
		}

		require.Equal(t, 3, len(tickers))
	})
}
