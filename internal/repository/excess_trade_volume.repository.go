package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/db/models/postgres/public/view"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ExcessTradeVolumeRepository interface {
	Add(tx *sql.Tx, m model.ExcessTradeVolume) (*model.ExcessTradeVolume, error)
	Update(tx *sql.Tx, m model.ExcessTradeVolume, columns postgres.ColumnList) (*model.ExcessTradeVolume, error)
	ListByTickerID(tx *sql.Tx) (map[uuid.UUID]decimal.Decimal, error)
}

type excessTradeVolumeRepositoryHandler struct {
	Db *sql.DB
}

func NewExcessTradeVolumeRepository(db *sql.DB) ExcessTradeVolumeRepository {
	return excessTradeVolumeRepositoryHandler{
		Db: db,
	}
}

func (h excessTradeVolumeRepositoryHandler) Add(tx *sql.Tx, m model.ExcessTradeVolume) (*model.ExcessTradeVolume, error) {
	if m.Quantity.LessThan(decimal.Zero) {
		return nil, fmt.Errorf("cannot create excess trade volume with quantity %f", m.Quantity.InexactFloat64())
	}

	m.CreatedAt = time.Now().UTC()

	query := table.ExcessTradeVolume.
		INSERT(
			table.ExcessTradeVolume.MutableColumns,
		).
		MODEL(m).
		RETURNING(table.ExcessTradeVolume.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.ExcessTradeVolume{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert excess trade volume: %w", err)
	}

	return &out, nil
}

func (h excessTradeVolumeRepositoryHandler) ListByTickerID(tx *sql.Tx) (map[uuid.UUID]decimal.Decimal, error) {
	table := view.LatestExcessTradeVolume

	query := table.SELECT(table.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	results := []model.LatestExcessTradeVolume{}
	err := query.Query(db, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to list excess trade volume: %w", err)
	}

	out := map[uuid.UUID]decimal.Decimal{}

	for _, r := range results {
		if r.Quantity.GreaterThan(decimal.Zero) {
			out[*r.TickerID] = *r.Quantity
		}
	}

	return out, nil
}

func (h excessTradeVolumeRepositoryHandler) Update(tx *sql.Tx, m model.ExcessTradeVolume, columns postgres.ColumnList) (*model.ExcessTradeVolume, error) {
	query := table.ExcessTradeVolume.
		UPDATE(columns).
		WHERE(table.ExcessTradeVolume.ExcessTradeVolumeID.EQ(
			postgres.UUID(m.ExcessTradeVolumeID),
		)).
		MODEL(m).
		RETURNING(
			table.ExcessTradeVolume.AllColumns,
		)

	out := model.ExcessTradeVolume{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}
