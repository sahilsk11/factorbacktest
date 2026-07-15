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

type ReconciliationRepository interface {
	Add(model.ReconciliationRun) (*model.ReconciliationRun, error)
	Get(uuid.UUID) (*model.ReconciliationRun, error)
	SetStatus(*sql.Tx, uuid.UUID, string) error
}

type reconciliationRepository struct{ db *sql.DB }

func NewReconciliationRepository(db *sql.DB) ReconciliationRepository {
	return reconciliationRepository{db: db}
}

func (r reconciliationRepository) Add(run model.ReconciliationRun) (*model.ReconciliationRun, error) {
	run.CreatedAt = time.Now().UTC()
	var out model.ReconciliationRun
	err := table.ReconciliationRun.INSERT(table.ReconciliationRun.MutableColumns).
		MODEL(run).RETURNING(table.ReconciliationRun.AllColumns).Query(r.db, &out)
	if err != nil {
		return nil, fmt.Errorf("add reconciliation run: %w", err)
	}
	return &out, nil
}

func (r reconciliationRepository) Get(id uuid.UUID) (*model.ReconciliationRun, error) {
	var out model.ReconciliationRun
	err := table.ReconciliationRun.SELECT(table.ReconciliationRun.AllColumns).
		WHERE(table.ReconciliationRun.ReconciliationRunID.EQ(postgres.UUID(id))).Query(r.db, &out)
	if err != nil {
		return nil, fmt.Errorf("get reconciliation run: %w", err)
	}
	return &out, nil
}

func (r reconciliationRepository) SetStatus(tx *sql.Tx, id uuid.UUID, status string) error {
	columns := postgres.ColumnList{table.ReconciliationRun.Status}
	run := model.ReconciliationRun{ReconciliationRunID: id, Status: status}
	if status == "APPLIED" {
		now := time.Now().UTC()
		run.AppliedAt = &now
		columns = append(columns, table.ReconciliationRun.AppliedAt)
	}
	stmt := table.ReconciliationRun.UPDATE(columns).MODEL(run).
		WHERE(table.ReconciliationRun.ReconciliationRunID.EQ(postgres.UUID(id)))
	var db qrm.Executable = r.db
	if tx != nil {
		db = tx
	}
	_, err := stmt.Exec(db)
	return err
}
