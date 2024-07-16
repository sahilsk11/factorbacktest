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

type InvestmentTradeRepository interface {
	Add(tx *sql.Tx, irt model.InvestmentTrade) (*model.InvestmentTrade, error)
	Get(id uuid.UUID) (*model.InvestmentTrade, error)
	List() ([]model.InvestmentTrade, error)
}

type investmentTradeRepositoryHandler struct {
	Db *sql.DB
}

func NewInvestmentTradeRepository(db *sql.DB) InvestmentTradeRepository {
	return investmentTradeRepositoryHandler{Db: db}
}

func (h investmentTradeRepositoryHandler) Add(tx *sql.Tx, irt model.InvestmentTrade) (*model.InvestmentTrade, error) {
	irt.CreatedAt = time.Now().UTC()
	query := table.InvestmentTrade.
		INSERT(
			table.InvestmentTrade.MutableColumns,
		).
		MODEL(irt).
		RETURNING(table.InvestmentTrade.AllColumns)

	out := model.InvestmentTrade{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert investment rebalance trade: %w", err)
	}

	return &out, nil
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

func (h investmentTradeRepositoryHandler) List() ([]model.InvestmentTrade, error) {
	query := table.InvestmentTrade.SELECT(table.InvestmentTrade.AllColumns)
	result := []model.InvestmentTrade{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list investment rebalance trades: %w", err)
	}

	return result, nil
}
