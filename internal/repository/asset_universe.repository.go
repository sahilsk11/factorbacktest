package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
)

type AssetUniverseRepository interface {
	GetAssets(string) ([]model.Ticker, error)
	AddAssets(tx *sql.Tx, universe model.AssetUniverse, tickers []model.Ticker) error
}

type assetUniverseRepositoryHandler struct {
	Db qrm.Queryable
}

func NewAssetUniverseRepository(db *sql.DB) AssetUniverseRepository {
	return assetUniverseRepositoryHandler{
		Db: db,
	}
}

func (h assetUniverseRepositoryHandler) GetAssets(name string) ([]model.Ticker, error) {
	query := postgres.SELECT(table.Ticker.AllColumns).FROM(
		table.Ticker.
			INNER_JOIN(
				table.AssetUniverseTicker,
				table.AssetUniverseTicker.TickerID.EQ(table.Ticker.TickerID),
			).
			INNER_JOIN(
				table.AssetUniverse,
				table.AssetUniverse.AssetUniverseID.EQ(table.AssetUniverseTicker.AssetUniverseID),
			),
	)

	if name != "ALL" {
		query = query.WHERE(table.AssetUniverse.AssetUniverseName.EQ(postgres.String(name)))
	}

	// i don't understand where duplicates are
	// being filtered

	tickers := []model.Ticker{}
	err := query.Query(h.Db, &tickers)
	if err != nil {
		return nil, fmt.Errorf("failed to query assets from %s: %w", name, err)
	}

	return tickers, nil
}

func (h assetUniverseRepositoryHandler) AddAssets(tx *sql.Tx, universe model.AssetUniverse, tickers []model.Ticker) error {
	models := []model.AssetUniverseTicker{}
	for _, t := range tickers {
		models = append(models, model.AssetUniverseTicker{
			TickerID:        t.TickerID,
			AssetUniverseID: universe.AssetUniverseID,
		})
	}
	query := table.AssetUniverseTicker.INSERT(table.AssetUniverseTicker.MutableColumns).MODELS(models)

	_, err := query.Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to add assets to universe %s: %w", universe.AssetUniverseName, err)
	}

	return nil
}
