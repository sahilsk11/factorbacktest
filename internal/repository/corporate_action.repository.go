package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/go-jet/jet/v2/postgres"
)

type CorporateActionRepository interface {
	UpsertMany(rows []model.CorporateAction) error
	ListBySymbolRange(symbol string, start, end time.Time) ([]model.CorporateAction, error)
	// SymbolsNeedingRecompute returns symbols whose actions were ingested on/after
	// `from`, whose effective date falls in [from, to], or that are acquired/delisted.
	SymbolsNeedingRecompute(from, to time.Time) ([]string, error)
}

type corporateActionRepositoryHandler struct {
	Db *sql.DB
}

func NewCorporateActionRepository(db *sql.DB) CorporateActionRepository {
	return corporateActionRepositoryHandler{Db: db}
}

func (h corporateActionRepositoryHandler) UpsertMany(rows []model.CorporateAction) error {
	if len(rows) == 0 {
		return nil
	}

	t := table.CorporateAction
	query := t.INSERT(t.MutableColumns).MODELS(rows).ON_CONFLICT(
		t.Symbol, t.EffectiveDate, t.Type,
	).DO_UPDATE(postgres.SET(
		t.Ratio.SET(t.EXCLUDED.Ratio),
		t.CashAmount.SET(t.EXCLUDED.CashAmount),
		t.NewSymbol.SET(t.EXCLUDED.NewSymbol),
		t.Source.SET(t.EXCLUDED.Source),
		t.Notes.SET(t.EXCLUDED.Notes),
		t.IngestedAt.SET(t.EXCLUDED.IngestedAt),
	))

	if _, err := query.Exec(h.Db); err != nil {
		return fmt.Errorf("failed to upsert corporate actions: %w", err)
	}
	return nil
}

func (h corporateActionRepositoryHandler) ListBySymbolRange(symbol string, start, end time.Time) ([]model.CorporateAction, error) {
	t := table.CorporateAction
	query := t.SELECT(t.AllColumns).WHERE(postgres.AND(
		t.Symbol.EQ(postgres.String(symbol)),
		t.EffectiveDate.BETWEEN(postgres.DateT(start), postgres.DateT(end)),
	)).ORDER_BY(t.EffectiveDate.ASC(), t.Type.ASC())

	out := []model.CorporateAction{}
	if err := query.Query(h.Db, &out); err != nil {
		return nil, fmt.Errorf("failed to list corporate actions for %s: %w", symbol, err)
	}
	return out, nil
}

func (h corporateActionRepositoryHandler) SymbolsNeedingRecompute(from, to time.Time) ([]string, error) {
	t := table.CorporateAction
	query := t.SELECT(t.Symbol).DISTINCT().WHERE(postgres.OR(
		t.IngestedAt.GT_EQ(postgres.TimestampzT(from.UTC())),
		t.EffectiveDate.BETWEEN(postgres.DateT(from), postgres.DateT(to)),
		t.Type.EQ(postgres.String("acquired")),
		t.Type.EQ(postgres.String("delisted")),
	)).ORDER_BY(t.Symbol.ASC())

	q, args := query.Sql()
	rows, err := h.Db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list symbols needing adjusted-price recompute: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan symbol needing recompute: %w", err)
		}
		out = append(out, symbol)
	}
	return out, rows.Err()
}
