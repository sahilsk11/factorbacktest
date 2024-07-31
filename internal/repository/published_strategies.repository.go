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

type PublishedStrategyRepository interface {
	Add(tx *sql.Tx, ps model.PublishedStrategy) (*model.PublishedStrategy, error)
	Get(tx *sql.Tx, id uuid.UUID) (*model.PublishedStrategy, error)
	List() ([]model.PublishedStrategy, error)
	GetLatestStats(publishedStrategyID uuid.UUID) (*model.PublishedStrategyStats, error)
}

type publishedStrategyRepositoryHandler struct {
	Db *sql.DB
}

func NewPublishedStrategyRepository(db *sql.DB) PublishedStrategyRepository {
	return publishedStrategyRepositoryHandler{Db: db}
}

func (h publishedStrategyRepositoryHandler) Add(tx *sql.Tx, ps model.PublishedStrategy) (*model.PublishedStrategy, error) {
	ps.CreatedAt = time.Now().UTC()
	ps.ModifiedAt = time.Now().UTC()
	query := table.PublishedStrategy.
		INSERT(
			table.PublishedStrategy.MutableColumns,
		).
		MODEL(ps).
		RETURNING(table.PublishedStrategy.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.PublishedStrategy{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert published strategy: %w", err)
	}

	return &out, nil
}

func (h publishedStrategyRepositoryHandler) Get(tx *sql.Tx, id uuid.UUID) (*model.PublishedStrategy, error) {
	query := table.PublishedStrategy.
		SELECT(table.PublishedStrategy.AllColumns).
		WHERE(table.PublishedStrategy.PublishedStrategyID.EQ(postgres.UUID(id)))

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	result := model.PublishedStrategy{}
	err := query.Query(db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get published strategy: %w", err)
	}

	return &result, nil
}

func (h publishedStrategyRepositoryHandler) List() ([]model.PublishedStrategy, error) {
	query := table.PublishedStrategy.SELECT(table.PublishedStrategy.AllColumns)

	result := []model.PublishedStrategy{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list published strategies: %w", err)
	}

	return result, nil
}

func (h publishedStrategyRepositoryHandler) GetLatestStats(publishedStrategyID uuid.UUID) (*model.PublishedStrategyStats, error) {
	t := table.PublishedStrategyStats
	// todo - consider using versions/join
	query := t.SELECT(t.AllColumns).
		WHERE(
			t.PublishedStrategyID.EQ(postgres.UUID(publishedStrategyID)),
		).
		ORDER_BY(
			t.CreatedAt.DESC(),
		).LIMIT(1)

	out := model.PublishedStrategyStats{}
	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to get published strategy stats: %w", err)
	}

	return &out, nil
}
