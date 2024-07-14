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

type StrategyInvestmentHoldingsRepository interface {
	Add(tx *sql.Tx, sih model.StrategyInvestmentHoldings) (*model.StrategyInvestmentHoldings, error)
	Get(id uuid.UUID) (*model.StrategyInvestmentHoldings, error)
	List() ([]model.StrategyInvestmentHoldings, error)
}

type strategyInvestmentHoldingsRepositoryHandler struct {
	Db *sql.DB
}

func NewStrategyInvestmentHoldingsRepository(db *sql.DB) StrategyInvestmentHoldingsRepository {
	return strategyInvestmentHoldingsRepositoryHandler{Db: db}
}

func (h strategyInvestmentHoldingsRepositoryHandler) Add(tx *sql.Tx, sih model.StrategyInvestmentHoldings) (*model.StrategyInvestmentHoldings, error) {
	sih.CreatedAt = time.Now().UTC()
	query := table.StrategyInvestmentHoldings.
		INSERT(
			table.StrategyInvestmentHoldings.StrategyInvestmentID,
			table.StrategyInvestmentHoldings.Date,
			table.StrategyInvestmentHoldings.Ticker,
			table.StrategyInvestmentHoldings.Quantity,
		).
		MODEL(sih).
		RETURNING(table.StrategyInvestmentHoldings.AllColumns)

	out := model.StrategyInvestmentHoldings{}
	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert strategy investment holding: %w", err)
	}

	return &out, nil
}

func (h strategyInvestmentHoldingsRepositoryHandler) Get(id uuid.UUID) (*model.StrategyInvestmentHoldings, error) {
	query := table.StrategyInvestmentHoldings.
		SELECT(table.StrategyInvestmentHoldings.AllColumns).
		WHERE(table.StrategyInvestmentHoldings.StrategyInvestmentHoldingsID.EQ(postgres.UUID(id)))

	result := model.StrategyInvestmentHoldings{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy investment holding: %w", err)
	}

	return &result, nil
}

func (h strategyInvestmentHoldingsRepositoryHandler) List() ([]model.StrategyInvestmentHoldings, error) {
	query := table.StrategyInvestmentHoldings.SELECT(table.StrategyInvestmentHoldings.AllColumns)
	result := []model.StrategyInvestmentHoldings{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investment holdings: %w", err)
	}

	return result, nil
}
