package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

type TradeOrderRepository interface {
	Add(tx *sql.Tx, to model.TradeOrder) (*model.TradeOrder, error)
	Update(tx *sql.Tx, to model.TradeOrder, columns postgres.ColumnList) (*model.TradeOrder, error)
	Get(id uuid.UUID) (*model.TradeOrder, error)
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

	out := model.TradeOrder{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert trade order: %w", err)
	}

	return &out, nil
}

func (h tradeOrderRepositoryHandler) Update(tx *sql.Tx, to model.TradeOrder, columns postgres.ColumnList) (*model.TradeOrder, error) {
	to.CreatedAt = time.Now().UTC()
	to.ModifiedAt = time.Now().UTC()
	query := table.TradeOrder.
		UPDATE(columns).
		MODEL(to).
		RETURNING(table.TradeOrder.AllColumns)

	out := model.TradeOrder{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to update trade order: %w", err)
	}

	return &out, nil
}

func (h tradeOrderRepositoryHandler) Get(id uuid.UUID) (*model.TradeOrder, error) {
	query := table.TradeOrder.
		SELECT(table.TradeOrder.AllColumns).
		WHERE(table.TradeOrder.TradeOrderID.EQ(postgres.UUID(id)))

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
