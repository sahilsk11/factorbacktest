package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type TradeOrderRepository interface {
	Add(tx *sql.Tx, to model.TradeOrder) (*model.TradeOrder, error)
	Update(tx *sql.Tx, tradeOrderID uuid.UUID, to model.TradeOrder, columns postgres.ColumnList) (*model.TradeOrder, error)
	Get(TradeOrderGetFilter) (*model.TradeOrder, error)
	List() ([]model.TradeOrder, error)
}

type tradeOrderRepositoryHandler struct {
	Db *sql.DB
}

func NewTradeOrderRepository(db *sql.DB) TradeOrderRepository {
	return tradeOrderRepositoryHandler{Db: db}
}

func (h tradeOrderRepositoryHandler) Add(tx *sql.Tx, to model.TradeOrder) (*model.TradeOrder, error) {
	to.CreatedAt = time.Now().UTC()
	to.ModifiedAt = time.Now().UTC()
	query := table.TradeOrder.
		INSERT(table.TradeOrder.MutableColumns).
		MODEL(to).
		RETURNING(table.TradeOrder.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.TradeOrder{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert trade order: %w", err)
	}

	return &out, nil
}

func (h tradeOrderRepositoryHandler) Update(tx *sql.Tx, tradeOrderID uuid.UUID, to model.TradeOrder, columns postgres.ColumnList) (*model.TradeOrder, error) {
	to.ModifiedAt = time.Now().UTC()
	columns = append(columns, table.TradeOrder.ModifiedAt)
	query := table.TradeOrder.
		UPDATE(columns).
		WHERE(
			table.TradeOrder.TradeOrderID.EQ(postgres.UUID(tradeOrderID)),
		).
		MODEL(to).
		RETURNING(table.TradeOrder.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.TradeOrder{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to update trade order: %w", err)
	}

	return &out, nil
}

type TradeOrderGetFilter struct {
	TradeOrderID *uuid.UUID
	ProviderID   *uuid.UUID
}

func (h tradeOrderRepositoryHandler) Get(filter TradeOrderGetFilter) (*model.TradeOrder, error) {
	query := table.TradeOrder.
		SELECT(table.TradeOrder.AllColumns)

	if filter.TradeOrderID != nil {
		query = query.WHERE(table.TradeOrder.TradeOrderID.EQ(postgres.UUID(filter.TradeOrderID)))
	} else if filter.ProviderID != nil {
		query = query.WHERE(table.TradeOrder.ProviderID.EQ(postgres.UUID(filter.ProviderID)))

	} else {
		return nil, fmt.Errorf("invalid TradeOrderGetFilter")
	}

	result := model.TradeOrder{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade order: %w", err)
	}

	return &result, nil
}

func (h tradeOrderRepositoryHandler) List() ([]model.TradeOrder, error) {
	query := table.TradeOrder.SELECT(table.TradeOrder.AllColumns)
	result := []model.TradeOrder{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list trade orders: %w", err)
	}

	return result, nil
}
