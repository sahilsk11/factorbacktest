package repository

import (
	"database/sql"
	"errors"
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
	RequestLiquidation(investmentID, userAccountID uuid.UUID) (*model.Investment, error)
	CompleteLiquidation(tx *sql.Tx, investmentID uuid.UUID) (bool, error)
}

var ErrInvestmentNotFound = errors.New("investment not found")

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
	IncludePaused  bool
	IncludeEnded   bool
}

func (h investmentRepositoryHandler) List(filter StrategyInvestmentListFilter) ([]model.Investment, error) {
	query := table.Investment.
		SELECT(table.Investment.AllColumns).
		ORDER_BY(table.Investment.CreatedAt.DESC())

	whereClauses := []postgres.BoolExpression{}
	if !filter.IncludeEnded {
		whereClauses = append(whereClauses, table.Investment.EndDate.IS_NULL())
	}
	if !filter.IncludePaused {
		whereClauses = append(whereClauses, postgres.OR(
			table.Investment.PausedAt.IS_NULL(),
			table.Investment.LiquidationRequestedAt.IS_NOT_NULL(),
		))
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

	if len(whereClauses) > 0 {
		query = query.WHERE(postgres.AND(whereClauses...))
	}

	result := []model.Investment{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investments: %w", err)
	}

	return result, nil
}

func (h investmentRepositoryHandler) RequestLiquidation(investmentID, userAccountID uuid.UUID) (*model.Investment, error) {
	t := table.Investment
	now := time.Now().UTC()
	query := t.UPDATE(
		t.LiquidationRequestedAt,
		t.ModifiedAt,
	).SET(
		postgres.TimestampzExp(postgres.COALESCE(
			t.LiquidationRequestedAt,
			postgres.TimestampzT(now),
		)),
		postgres.TimestampzT(now),
	).WHERE(
		t.InvestmentID.EQ(postgres.UUID(investmentID)).
			AND(t.UserAccountID.EQ(postgres.UUID(userAccountID))).
			AND(t.EndDate.IS_NULL()),
	).RETURNING(t.AllColumns)

	result := model.Investment{}
	err := query.Query(h.Db, &result)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, ErrInvestmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to request liquidation for investment %s: %w", investmentID, err)
	}
	return &result, nil
}

func (h investmentRepositoryHandler) CompleteLiquidation(tx *sql.Tx, investmentID uuid.UUID) (bool, error) {
	t := table.Investment
	query := t.UPDATE(
		t.EndDate,
		t.ModifiedAt,
	).SET(
		postgres.CURRENT_DATE(),
		postgres.NOW(),
	).WHERE(
		t.InvestmentID.EQ(postgres.UUID(investmentID)).
			AND(t.LiquidationRequestedAt.IS_NOT_NULL()).
			AND(t.EndDate.IS_NULL()),
	)

	var db qrm.Executable = h.Db
	if tx != nil {
		db = tx
	}
	result, err := query.Exec(db)
	if err != nil {
		return false, fmt.Errorf("failed to complete liquidation for investment %s: %w", investmentID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
