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

type StrategyInvestmentRepository interface {
	Add(si model.StrategyInvestment) (*model.StrategyInvestment, error)
	Get(id uuid.UUID) (*model.StrategyInvestment, error)
	List(StrategyInvestmentListFilter) ([]model.StrategyInvestment, error)
}

type strategyInvestmentRepositoryHandler struct {
	Db *sql.DB
}

func NewStrategyInvestmentRepository(db *sql.DB) StrategyInvestmentRepository {
	return strategyInvestmentRepositoryHandler{Db: db}
}

func (h strategyInvestmentRepositoryHandler) Add(si model.StrategyInvestment) (*model.StrategyInvestment, error) {
	si.CreatedAt = time.Now().UTC()
	si.ModifiedAt = time.Now().UTC()
	query := table.StrategyInvestment.
		INSERT(
			table.StrategyInvestment.AmountDollars,
			table.StrategyInvestment.StartDate,
			table.StrategyInvestment.SavedStragyID,
			table.StrategyInvestment.UserAccountID,
			table.StrategyInvestment.CreatedAt,
			table.StrategyInvestment.ModifiedAt,
		).
		MODEL(si).
		RETURNING(table.StrategyInvestment.AllColumns)

	out := model.StrategyInvestment{}
	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert strategy investment: %w", err)
	}

	return &out, nil
}

func (h strategyInvestmentRepositoryHandler) Get(id uuid.UUID) (*model.StrategyInvestment, error) {
	query := table.StrategyInvestment.
		SELECT(table.StrategyInvestment.AllColumns).
		WHERE(table.StrategyInvestment.StrategyInvestmentID.EQ(postgres.UUID(id)))

	result := model.StrategyInvestment{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy investment: %w", err)
	}

	return &result, nil
}

type StrategyInvestmentListFilter struct {
	UserAccountIDs []uuid.UUID
}

func (h strategyInvestmentRepositoryHandler) List(filter StrategyInvestmentListFilter) ([]model.StrategyInvestment, error) {
	query := table.StrategyInvestment.
		SELECT(table.StrategyInvestment.AllColumns).
		ORDER_BY(table.StrategyInvestment.CreatedAt.DESC())

	if len(filter.UserAccountIDs) > 0 {
		ids := []postgres.Expression{}
		for _, id := range filter.UserAccountIDs {
			ids = append(ids, postgres.UUID(id))
		}
		query = query.WHERE(
			table.StrategyInvestment.UserAccountID.IN(ids...),
		)
	}

	result := []model.StrategyInvestment{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investments: %w", err)
	}

	return result, nil
}
