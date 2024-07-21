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

type RebalancerRunRepository interface {
	Add(tx *sql.Tx, rr model.RebalancerRun) (*model.RebalancerRun, error)
	Get(id uuid.UUID) (*model.RebalancerRun, error)
	List() ([]model.RebalancerRun, error)
	Update(tx *sql.Tx, rr *model.RebalancerRun, columns postgres.ColumnList) (*model.RebalancerRun, error)
}

type rebalancerRunRepositoryHandler struct {
	Db *sql.DB
}

func NewRebalancerRunRepository(db *sql.DB) RebalancerRunRepository {
	return rebalancerRunRepositoryHandler{Db: db}
}

func (h rebalancerRunRepositoryHandler) Add(tx *sql.Tx, rr model.RebalancerRun) (*model.RebalancerRun, error) {
	rr.CreatedAt = time.Now().UTC()
	rr.ModifiedAt = time.Now().UTC()

	query := table.RebalancerRun.
		INSERT(
			table.RebalancerRun.MutableColumns,
		).
		MODEL(rr).
		RETURNING(table.RebalancerRun.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.RebalancerRun{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert rebalancer run: %w", err)
	}

	return &out, nil
}

func (h rebalancerRunRepositoryHandler) Update(tx *sql.Tx, rr *model.RebalancerRun, columns postgres.ColumnList) (*model.RebalancerRun, error) {
	rr.ModifiedAt = time.Now().UTC()
	if rr.RebalancerRunID == uuid.Nil {
		return nil, fmt.Errorf("failed to update rebalancer run - id not provided in inputted model")
	}
	query := table.RebalancerRun.
		UPDATE(columns).
		MODEL(rr).
		RETURNING(table.RebalancerRun.AllColumns).
		WHERE(table.RebalancerRun.RebalancerRunID.EQ(
			postgres.UUID(rr.RebalancerRunID),
		))

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.RebalancerRun{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to update rebalancer run %s: %w", rr.RebalancerRunID.String(), err)
	}

	return &out, nil
}

func (h rebalancerRunRepositoryHandler) Get(id uuid.UUID) (*model.RebalancerRun, error) {
	query := table.RebalancerRun.
		SELECT(table.RebalancerRun.AllColumns).
		WHERE(table.RebalancerRun.RebalancerRunID.EQ(postgres.UUID(id)))

	result := model.RebalancerRun{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get rebalancer run: %w", err)
	}

	return &result, nil
}

func (h rebalancerRunRepositoryHandler) List() ([]model.RebalancerRun, error) {
	query := table.RebalancerRun.SELECT(table.RebalancerRun.AllColumns)
	result := []model.RebalancerRun{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list rebalancer runs: %w", err)
	}

	return result, nil
}
