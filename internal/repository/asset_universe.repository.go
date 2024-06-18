package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
)

type AssetUniverseRepository interface {
	GetAssets(model.AssetUniverseName) ([]model.Ticker, error)
}

type assetUniverseRepositoryHandler struct {
	Db *sql.DB
}

func NewAssetUniverseRepository(db *sql.DB) AssetUniverseRepository {
	return assetUniverseRepositoryHandler{
		Db: db,
	}
}

func (h assetUniverseRepositoryHandler) GetAssets(name model.AssetUniverseName) ([]model.Ticker, error) {
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
	).
		WHERE(table.AssetUniverse.AssetUniverseName.EQ(postgres.NewEnumValue(name.String())))

	tickers := []model.Ticker{}
	err := query.Query(h.Db, &tickers)
	if err != nil {
		return nil, fmt.Errorf("failed to query assets from %s: %w", name.String(), err)
	}

	return tickers, nil
}
