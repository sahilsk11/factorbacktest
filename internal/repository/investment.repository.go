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
}

func (h investmentRepositoryHandler) List(filter StrategyInvestmentListFilter) ([]model.Investment, error) {
	query := table.Investment.
		SELECT(table.Investment.AllColumns).
		ORDER_BY(table.Investment.CreatedAt.DESC())

	whereClauses := []postgres.BoolExpression{
		table.Investment.EndDate.IS_NULL(),
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

	query = query.WHERE(postgres.AND(whereClauses...))

	result := []model.Investment{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investments: %w", err)
	}

	return result, nil
}

func (h investmentRepositoryHandler) RequestLiquidation(investmentID, userAccountID uuid.UUID) (*model.Investment, error) {
	result := model.Investment{}
	err := h.Db.QueryRow(`
		UPDATE investment
		SET liquidation_requested_at = COALESCE(liquidation_requested_at, now()),
		    modified_at = now()
		WHERE investment_id = $1
		  AND user_account_id = $2
		  AND end_date IS NULL
		RETURNING investment_id, amount_dollars, start_date, strategy_id,
		          user_account_id, created_at, modified_at, end_date, paused_at,
		          liquidation_requested_at`, investmentID, userAccountID).Scan(
		&result.InvestmentID,
		&result.AmountDollars,
		&result.StartDate,
		&result.StrategyID,
		&result.UserAccountID,
		&result.CreatedAt,
		&result.ModifiedAt,
		&result.EndDate,
		&result.PausedAt,
		&result.LiquidationRequestedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvestmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to request liquidation for investment %s: %w", investmentID, err)
	}
	return &result, nil
}

func (h investmentRepositoryHandler) CompleteLiquidation(tx *sql.Tx, investmentID uuid.UUID) (bool, error) {
	type execer interface {
		Exec(query string, args ...any) (sql.Result, error)
	}
	var db execer = h.Db
	if tx != nil {
		db = tx
	}
	result, err := db.Exec(`
		UPDATE investment
		SET end_date = CURRENT_DATE, modified_at = now()
		WHERE investment_id = $1
		  AND liquidation_requested_at IS NOT NULL
		  AND end_date IS NULL`, investmentID)
	if err != nil {
		return false, fmt.Errorf("failed to complete liquidation for investment %s: %w", investmentID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
