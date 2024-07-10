package repository

import (
	"database/sql"
	"errors"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type SavedStrategyRepository interface {
	List(uuid.UUID) ([]model.SavedStrategy, error)
	ListMatchingStrategies(m model.SavedStrategy) ([]model.SavedStrategy, error)
	Add(m model.SavedStrategy) error
	SetBookmarked(savedStrategyID uuid.UUID, bookmarked bool) error
}

type savedStrategyRepositoryHandler struct {
	Db *sql.DB
}

func NewSavedStrategyRepository(db *sql.DB) SavedStrategyRepository {
	return savedStrategyRepositoryHandler{db}
}

func (h savedStrategyRepositoryHandler) Add(m model.SavedStrategy) error {
	m.CreatedAt = time.Now().UTC()
	m.ModifiedAt = time.Now().UTC()

	query := table.SavedStrategy.INSERT(table.SavedStrategy.MutableColumns).MODEL(m)
	_, err := query.Exec(h.Db)
	if err != nil {
		return fmt.Errorf("failed to insert saved strategy: %w", err)
	}

	return nil
}

// ignores bookmarker col
func (h savedStrategyRepositoryHandler) ListMatchingStrategies(m model.SavedStrategy) ([]model.SavedStrategy, error) {
	query := table.SavedStrategy.
		SELECT(table.SavedStrategy.AllColumns).
		WHERE(
			postgres.AND(
				// table.SavedStrategy.StrategyName.EQ(postgres.String(m.StrategyName)),
				table.SavedStrategy.FactorExpression.EQ(postgres.String(m.FactorExpression)),
				// idk how to deal with dates rn
				// table.SavedStrategy.BacktestStart.EQ(postgres.DateT(m.BacktestStart)),
				// table.SavedStrategy.BacktestEnd.EQ(postgres.DateT(m.BacktestEnd)),
				table.SavedStrategy.RebalanceInterval.EQ(postgres.String(m.RebalanceInterval)),
				table.SavedStrategy.NumAssets.EQ(postgres.Int32(m.NumAssets)),
				table.SavedStrategy.AssetUniverse.EQ(postgres.String(m.AssetUniverse)),
				table.SavedStrategy.UserAccountID.EQ(postgres.UUID(m.UserAccountID)),
			),
		).ORDER_BY(
		table.SavedStrategy.CreatedAt.DESC(),
	)

	out := []model.SavedStrategy{}
	err := query.Query(h.Db, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return out, nil
}

func (h savedStrategyRepositoryHandler) List(userAccountID uuid.UUID) ([]model.SavedStrategy, error) {
	query := table.SavedStrategy.
		SELECT(table.SavedStrategy.AllColumns).WHERE(
		table.SavedStrategy.UserAccountID.EQ(postgres.UUID(userAccountID)),
	).ORDER_BY(
		table.SavedStrategy.CreatedAt.DESC(),
	)

	out := []model.SavedStrategy{}
	err := query.Query(h.Db, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return out, nil
}

func (h savedStrategyRepositoryHandler) SetBookmarked(savedStrategyID uuid.UUID, bookmarked bool) error {
	query := table.SavedStrategy.UPDATE(
		table.SavedStrategy.Bookmarked,
		table.SavedStrategy.ModifiedAt,
	).SET(
		postgres.Bool(bookmarked),
		postgres.DateT(time.Now().UTC()),
	).WHERE(
		table.SavedStrategy.SavedStragyID.EQ(postgres.UUID(savedStrategyID)),
	)

	_, err := query.Exec(h.Db)
	if err != nil {
		return err
	}

	return nil
}
