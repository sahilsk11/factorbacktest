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

type StrategyRepository interface {
	List(StrategyListFilter) ([]model.Strategy, error)
	Add(m model.Strategy) (*model.Strategy, error)
	Get(uuid.UUID) (*model.Strategy, error)
}

type strategyRepositoryHandler struct {
	Db *sql.DB
}

func NewStrategyRepository(db *sql.DB) StrategyRepository {
	return strategyRepositoryHandler{db}
}

func (h strategyRepositoryHandler) Get(id uuid.UUID) (*model.Strategy, error) {
	query := table.Strategy.SELECT(table.Strategy.AllColumns).
		WHERE(table.Strategy.StrategyID.EQ(postgres.UUID(id)))
	out := model.Strategy{}

	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved strategy: %w", err)
	}

	return &out, nil
}

func (h strategyRepositoryHandler) UpdateName(id uuid.UUID, name string) error {
	query := table.Strategy.UPDATE(
		table.Strategy.StrategyName,
	).SET(
		postgres.String(name),
	).WHERE(
		table.Strategy.StrategyID.EQ(postgres.UUID(id)),
	)
	_, err := query.Exec(h.Db)
	if err != nil {
		return fmt.Errorf("failed to update strategy name: %w", err)
	}

	return nil
}

func (h strategyRepositoryHandler) Add(m model.Strategy) (*model.Strategy, error) {
	m.CreatedAt = time.Now().UTC()
	m.ModifiedAt = time.Now().UTC()

	query := table.Strategy.
		INSERT(table.Strategy.MutableColumns).
		MODEL(m).
		RETURNING(table.Strategy.AllColumns)

	out := model.Strategy{}
	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert saved strategy: %w", err)
	}

	return &out, nil
}

// ignores bookmarker col
func (h strategyRepositoryHandler) ListMatchingStrategies(m model.Strategy) ([]model.Strategy, error) {
	query := table.Strategy.
		SELECT(table.Strategy.AllColumns).
		WHERE(
			postgres.AND(
				// table.Strategy.StrategyName.EQ(postgres.String(m.StrategyName)),
				table.Strategy.FactorExpression.EQ(postgres.String(m.FactorExpression)),
				// idk how to deal with dates rn
				// table.Strategy.BacktestStart.EQ(postgres.DateT(m.BacktestStart)),
				// table.Strategy.BacktestEnd.EQ(postgres.DateT(m.BacktestEnd)),
				table.Strategy.RebalanceInterval.EQ(postgres.String(m.RebalanceInterval)),
				table.Strategy.NumAssets.EQ(postgres.Int32(m.NumAssets)),
				table.Strategy.AssetUniverse.EQ(postgres.String(m.AssetUniverse)),
				table.Strategy.UserAccountID.EQ(postgres.UUID(m.UserAccountID)),
			),
		).ORDER_BY(
		table.Strategy.CreatedAt.DESC(),
	)

	out := []model.Strategy{}
	err := query.Query(h.Db, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return out, nil
}

type StrategyListFilter struct {
	SavedByUser *uuid.UUID
	Published   *bool
}

func (h strategyRepositoryHandler) List(filter StrategyListFilter) ([]model.Strategy, error) {
	query := table.Strategy.
		SELECT(table.Strategy.AllColumns).
		ORDER_BY(
			table.Strategy.CreatedAt.DESC(),
		)

	if filter.SavedByUser != nil {
		query = query.WHERE(
			postgres.AND(
				table.Strategy.Saved.IS_TRUE(),
				table.Strategy.UserAccountID.EQ(postgres.UUID(*filter.SavedByUser)),
			),
		)
	}
	if filter.Published != nil && *filter.Published {
		query = query.WHERE(
			table.Strategy.Published.IS_TRUE(),
		)
	}

	out := []model.Strategy{}
	err := query.Query(h.Db, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return out, nil
}
