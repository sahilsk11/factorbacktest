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

type InvestmentRepository interface {
	Add(tx *sql.Tx, si model.Investment) (*model.Investment, error)
	Get(id uuid.UUID) (*model.Investment, error)
	List(StrategyInvestmentListFilter) ([]model.Investment, error)
}

type investmentRepositoryHandler struct {
	Db *sql.DB
}

func NewInvestmentRepository(db *sql.DB) InvestmentRepository {
	return investmentRepositoryHandler{Db: db}
}

func (h investmentRepositoryHandler) Add(tx *sql.Tx, si model.Investment) (*model.Investment, error) {
	si.CreatedAt = time.Now().UTC()
	si.ModifiedAt = time.Now().UTC()
	query := table.Investment.
		INSERT(
			table.Investment.MutableColumns,
		).
		MODEL(si).
		RETURNING(table.Investment.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}
	out := model.Investment{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert strategy investment: %w", err)
	}

	return &out, nil
}

func (h investmentRepositoryHandler) Get(id uuid.UUID) (*model.Investment, error) {
	query := table.Investment.
		SELECT(table.Investment.AllColumns).
		WHERE(table.Investment.InvestmentID.EQ(postgres.UUID(id)))

	result := model.Investment{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy investment: %w", err)
	}

	return &result, nil
}

type StrategyInvestmentListFilter struct {
	UserAccountIDs []uuid.UUID
}

func (h investmentRepositoryHandler) List(filter StrategyInvestmentListFilter) ([]model.Investment, error) {
	query := table.Investment.
		SELECT(table.Investment.AllColumns).
		ORDER_BY(table.Investment.CreatedAt.DESC())

	whereClauses := []postgres.BoolExpression{
		table.Investment.PausedAt.IS_NOT_NULL(),
	}
	if len(filter.UserAccountIDs) > 0 {
		ids := []postgres.Expression{}
		for _, id := range filter.UserAccountIDs {
			ids = append(ids, postgres.UUID(id))
		}
		whereClauses = append(whereClauses,
			table.Investment.UserAccountID.IN(ids...),
		)
	}

	query = query.WHERE(postgres.AND(whereClauses...))

	result := []model.Investment{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investments: %w", err)
	}

	return result, nil
}
