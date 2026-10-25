package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/go-jet/jet/v2/postgres"
)

type RawPriceRepository interface {
	UpsertMany(rows []model.RawPrice) error
	ListBySymbolRange(symbol string, start, end time.Time) ([]model.RawPrice, error)
}

type rawPriceRepositoryHandler struct {
	Db *sql.DB
}

func NewRawPriceRepository(db *sql.DB) RawPriceRepository {
	return rawPriceRepositoryHandler{Db: db}
}

func (h rawPriceRepositoryHandler) UpsertMany(rows []model.RawPrice) error {
	if len(rows) == 0 {
		return nil
	}

	t := table.RawPrice
	query := t.INSERT(
		t.Symbol, t.Date, t.Close, t.Volume, t.Source, t.IngestedAt,
	).MODELS(rows).ON_CONFLICT(t.Symbol, t.Date).DO_UPDATE(postgres.SET(
		t.Close.SET(t.EXCLUDED.Close),
		t.Volume.SET(t.EXCLUDED.Volume),
		t.Source.SET(t.EXCLUDED.Source),
		t.IngestedAt.SET(t.EXCLUDED.IngestedAt),
	))

	if _, err := query.Exec(h.Db); err != nil {
		return fmt.Errorf("failed to upsert raw prices: %w", err)
	}
	return nil
}

func (h rawPriceRepositoryHandler) ListBySymbolRange(symbol string, start, end time.Time) ([]model.RawPrice, error) {
	t := table.RawPrice
	query := t.SELECT(t.AllColumns).WHERE(postgres.AND(
		t.Symbol.EQ(postgres.String(symbol)),
		t.Date.BETWEEN(postgres.DateT(start), postgres.DateT(end)),
	)).ORDER_BY(t.Date.ASC())

	out := []model.RawPrice{}
	if err := query.Query(h.Db, &out); err != nil {
		return nil, fmt.Errorf("failed to list raw prices for %s: %w", symbol, err)
	}
	return out, nil
}
