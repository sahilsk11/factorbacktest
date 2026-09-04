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

type InvestmentRebalanceRepository interface {
	Add(tx *sql.Tx, ir model.InvestmentRebalance) (*model.InvestmentRebalance, error)
	Get(tx *sql.Tx, id uuid.UUID) (*model.InvestmentRebalance, error)
	List(tx *sql.Tx) ([]model.InvestmentRebalance, error)
	ListByRebalancerRunID(tx *sql.Tx, rebalancerRunID uuid.UUID) ([]model.InvestmentRebalance, error)
	HasPendingForInvestment(investmentID uuid.UUID) (bool, error)
	Update(tx *sql.Tx, ir model.InvestmentRebalance, columns postgres.ColumnList) (*model.InvestmentRebalance, error)
}

type investmentRebalanceRepositoryHandler struct {
	Db *sql.DB
}

func NewInvestmentRebalanceRepository(db *sql.DB) InvestmentRebalanceRepository {
	return investmentRebalanceRepositoryHandler{Db: db}
}

func (h investmentRebalanceRepositoryHandler) Add(tx *sql.Tx, ir model.InvestmentRebalance) (*model.InvestmentRebalance, error) {
	ir.CreatedAt = time.Now().UTC()
	ir.ModifiedAt = time.Now().UTC()
	query := table.InvestmentRebalance.
		INSERT(
			table.InvestmentRebalance.MutableColumns,
		).
		MODEL(ir).
		RETURNING(table.InvestmentRebalance.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.InvestmentRebalance{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert investment rebalance: %w", err)
	}

	return &out, nil
}

func (h investmentRebalanceRepositoryHandler) Get(tx *sql.Tx, id uuid.UUID) (*model.InvestmentRebalance, error) {
	query := table.InvestmentRebalance.
		SELECT(table.InvestmentRebalance.AllColumns).
		WHERE(table.InvestmentRebalance.InvestmentRebalanceID.EQ(postgres.UUID(id)))

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	result := model.InvestmentRebalance{}
	err := query.Query(db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get investment rebalance: %w", err)
	}

	return &result, nil
}

func (h investmentRebalanceRepositoryHandler) List(tx *sql.Tx) ([]model.InvestmentRebalance, error) {
	query := table.InvestmentRebalance.SELECT(table.InvestmentRebalance.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	result := []model.InvestmentRebalance{}
	err := query.Query(db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list investment rebalances: %w", err)
	}

	return result, nil
}

func (h investmentRebalanceRepositoryHandler) ListByRebalancerRunID(
	tx *sql.Tx,
	rebalancerRunID uuid.UUID,
) ([]model.InvestmentRebalance, error) {
	query := table.InvestmentRebalance.
		SELECT(table.InvestmentRebalance.AllColumns).
		WHERE(table.InvestmentRebalance.RebalancerRunID.EQ(postgres.UUID(rebalancerRunID)))

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	result := []model.InvestmentRebalance{}
	if err := query.Query(db, &result); err != nil {
		return nil, fmt.Errorf("failed to list investment rebalances for run: %w", err)
	}
	return result, nil
}

func (h investmentRebalanceRepositoryHandler) HasPendingForInvestment(investmentID uuid.UUID) (bool, error) {
	query := postgres.SELECT(postgres.COUNT(table.InvestmentRebalance.InvestmentRebalanceID).AS("count")).
		FROM(table.InvestmentRebalance).
		WHERE(
			table.InvestmentRebalance.InvestmentID.EQ(postgres.UUID(investmentID)).
				AND(table.InvestmentRebalance.State.EQ(postgres.NewEnumValue("PENDING"))),
		)

	var out struct {
		Count int64
	}
	if err := query.Query(h.Db, &out); err != nil {
		return false, fmt.Errorf("failed to check pending investment rebalances: %w", err)
	}
	return out.Count > 0, nil
}

func (h investmentRebalanceRepositoryHandler) Update(
	tx *sql.Tx,
	ir model.InvestmentRebalance,
	columns postgres.ColumnList,
) (*model.InvestmentRebalance, error) {
	ir.ModifiedAt = time.Now().UTC()
	columns = append(columns, table.InvestmentRebalance.ModifiedAt)
	query := table.InvestmentRebalance.UPDATE(columns).MODEL(ir).
		WHERE(table.InvestmentRebalance.InvestmentRebalanceID.EQ(postgres.UUID(ir.InvestmentRebalanceID))).
		RETURNING(table.InvestmentRebalance.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.InvestmentRebalance{}
	if err := query.Query(db, &out); err != nil {
		return nil, fmt.Errorf("failed to update investment rebalance: %w", err)
	}
	return &out, nil
}
