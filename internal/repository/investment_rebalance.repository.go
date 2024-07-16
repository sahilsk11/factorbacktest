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
	Get(id uuid.UUID) (*model.InvestmentRebalance, error)
	List() ([]model.InvestmentRebalance, error)
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
			table.InvestmentRebalance.RebalancerRunID,
			table.InvestmentRebalance.StrategyInvestmentID,
			table.InvestmentRebalance.State,
			table.InvestmentRebalance.CreatedAt,
			table.InvestmentRebalance.ModifiedAt,
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

func (h investmentRebalanceRepositoryHandler) Get(id uuid.UUID) (*model.InvestmentRebalance, error) {
	query := table.InvestmentRebalance.
		SELECT(table.InvestmentRebalance.AllColumns).
		WHERE(table.InvestmentRebalance.InvestmentRebalanceID.EQ(postgres.UUID(id)))

	result := model.InvestmentRebalance{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get investment rebalance: %w", err)
	}

	return &result, nil
}

func (h investmentRebalanceRepositoryHandler) List() ([]model.InvestmentRebalance, error) {
	query := table.InvestmentRebalance.SELECT(table.InvestmentRebalance.AllColumns)
	result := []model.InvestmentRebalance{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list investment rebalances: %w", err)
	}

	return result, nil
}
