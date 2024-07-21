package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/db/models/postgres/public/view"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type InvestmentTradeRepository interface {
	Add(tx *sql.Tx, irt model.InvestmentTrade) (*model.InvestmentTrade, error)
	AddMany(tx *sql.Tx, m []*model.InvestmentTrade) ([]model.InvestmentTrade, error)
	Get(id uuid.UUID) (*model.InvestmentTrade, error)
	List(tx *sql.Tx, filter InvestmentTradeListFilter) ([]model.InvestmentTradeStatus, error)
	Update(tx *sql.Tx, m model.InvestmentTrade, columns postgres.ColumnList) (*model.InvestmentTrade, error)
}

type investmentTradeRepositoryHandler struct {
	Db *sql.DB
}

func NewInvestmentTradeRepository(db *sql.DB) InvestmentTradeRepository {
	return investmentTradeRepositoryHandler{Db: db}
}

func (h investmentTradeRepositoryHandler) Add(tx *sql.Tx, irt model.InvestmentTrade) (*model.InvestmentTrade, error) {
	irt.CreatedAt = time.Now().UTC()
	irt.ModifiedAt = time.Now().UTC()

	query := table.InvestmentTrade.
		INSERT(
			table.InvestmentTrade.MutableColumns,
		).
		MODEL(irt).
		RETURNING(table.InvestmentTrade.AllColumns)

	out := model.InvestmentTrade{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert investment trade: %w", err)
	}

	return &out, nil
}

func (h investmentTradeRepositoryHandler) AddMany(tx *sql.Tx, models []*model.InvestmentTrade) ([]model.InvestmentTrade, error) {
	if len(models) == 0 {
		return []model.InvestmentTrade{}, nil
	}

	for _, m := range models {
		m.CreatedAt = time.Now().UTC()
		m.ModifiedAt = time.Now().UTC()
	}

	query := table.InvestmentTrade.
		INSERT(
			table.InvestmentTrade.MutableColumns,
		).
		MODELS(models).
		RETURNING(table.InvestmentTrade.AllColumns)

	out := []model.InvestmentTrade{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert investment trade models: %w", err)
	}

	return out, nil
}

func (h investmentTradeRepositoryHandler) Get(id uuid.UUID) (*model.InvestmentTrade, error) {
	query := table.InvestmentTrade.
		SELECT(table.InvestmentTrade.AllColumns).
		WHERE(table.InvestmentTrade.InvestmentTradeID.EQ(postgres.UUID(id)))

	result := model.InvestmentTrade{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get investment rebalance trade: %w", err)
	}

	return &result, nil
}

type InvestmentTradeListFilter struct {
	TradeOrderID *uuid.UUID
}

func (h investmentTradeRepositoryHandler) List(tx *sql.Tx, listFilter InvestmentTradeListFilter) ([]model.InvestmentTradeStatus, error) {
	query := view.InvestmentTradeStatus.SELECT(view.InvestmentTradeStatus.AllColumns)
	if listFilter.TradeOrderID != nil {
		query = query.WHERE(
			view.InvestmentTradeStatus.TradeOrderID.EQ(
				postgres.UUID(listFilter.TradeOrderID),
			),
		)
	}

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	result := []model.InvestmentTradeStatus{}
	err := query.Query(db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list investment rebalance trades: %w", err)
	}

	return result, nil
}

func (h investmentTradeRepositoryHandler) Update(tx *sql.Tx, m model.InvestmentTrade, columns postgres.ColumnList) (*model.InvestmentTrade, error) {
	m.ModifiedAt = time.Now().UTC()

	query := table.InvestmentTrade.UPDATE(columns).
		MODEL(m).
		WHERE(
			table.InvestmentTrade.InvestmentTradeID.EQ(
				postgres.UUID(m.InvestmentTradeID),
			),
		).RETURNING(
		table.InvestmentTrade.AllColumns,
	)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.InvestmentTrade{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to update investment trade %s: %w", m.InvestmentTradeID.String(), err)
	}

	return &out, nil
}
