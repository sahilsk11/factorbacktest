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

type InvestmentRebalanceTradeRepository interface {
	Add(tx *sql.Tx, irt model.InvestmentRebalanceTrade) (*model.InvestmentRebalanceTrade, error)
	Get(id uuid.UUID) (*model.InvestmentRebalanceTrade, error)
	List() ([]model.InvestmentRebalanceTrade, error)
}

type investmentRebalanceTradeRepositoryHandler struct {
	Db *sql.DB
}

func NewInvestmentRebalanceTradeRepository(db *sql.DB) InvestmentRebalanceTradeRepository {
	return investmentRebalanceTradeRepositoryHandler{Db: db}
}

func (h investmentRebalanceTradeRepositoryHandler) Add(tx *sql.Tx, irt model.InvestmentRebalanceTrade) (*model.InvestmentRebalanceTrade, error) {
	irt.CreatedAt = time.Now().UTC()
	query := table.InvestmentRebalanceTrade.
		INSERT(
			table.InvestmentRebalanceTrade.InvestmentRebalanceID,
			table.InvestmentRebalanceTrade.TickerID,
			table.InvestmentRebalanceTrade.AmountInDollars,
			table.InvestmentRebalanceTrade.Side,
			table.InvestmentRebalanceTrade.CreatedAt,
		).
		MODEL(irt).
		RETURNING(table.InvestmentRebalanceTrade.AllColumns)

	out := model.InvestmentRebalanceTrade{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert investment rebalance trade: %w", err)
	}

	return &out, nil
}

func (h investmentRebalanceTradeRepositoryHandler) Get(id uuid.UUID) (*model.InvestmentRebalanceTrade, error) {
	query := table.InvestmentRebalanceTrade.
		SELECT(table.InvestmentRebalanceTrade.AllColumns).
		WHERE(table.InvestmentRebalanceTrade.InvestmentRebalanceTradeID.EQ(postgres.UUID(id)))

	result := model.InvestmentRebalanceTrade{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get investment rebalance trade: %w", err)
	}

	return &result, nil
}

func (h investmentRebalanceTradeRepositoryHandler) List() ([]model.InvestmentRebalanceTrade, error) {
	query := table.InvestmentRebalanceTrade.SELECT(table.InvestmentRebalanceTrade.AllColumns)
	result := []model.InvestmentRebalanceTrade{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list investment rebalance trades: %w", err)
	}

	return result, nil
}
