package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
	"time"

	"factorbacktest/internal/domain"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

type BacktestJobRepository interface {
	Create(job domain.BacktestJob) (*domain.BacktestJob, error)
	Get(id uuid.UUID) (*domain.BacktestJob, error)
	Update(job domain.BacktestJob) error
	UpdateStage(id uuid.UUID, stage string, progressPct int) error
	MarkCompleted(id uuid.UUID, result interface{}) error
	MarkFailed(id uuid.UUID, errMsg string) error
}

type backtestJobRepositoryHandler struct {
	Db *sql.DB
}

func NewBacktestJobRepository(db *sql.DB) BacktestJobRepository {
	return backtestJobRepositoryHandler{db}
}

func (h backtestJobRepositoryHandler) Create(job domain.BacktestJob) (*domain.BacktestJob, error) {
	if job.BacktestJobID == uuid.Nil {
		job.BacktestJobID = uuid.New()
	}
	job.CreatedAt = time.Now().UTC()
	job.UpdatedAt = time.Now().UTC()
	job.Status = domain.BacktestJobStatusPending
	job.ProgressPct = 0

	query := table.BacktestJob.INSERT(
		table.BacktestJob.MutableColumns,
	).MODEL(job).RETURNING(table.BacktestJob.AllColumns)

	out := domain.BacktestJob{}
	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to create backtest job: %w", err)
	}
	return &out, nil
}

func (h backtestJobRepositoryHandler) Get(id uuid.UUID) (*domain.BacktestJob, error) {
	query := table.BacktestJob.SELECT(
		table.BacktestJob.AllColumns,
	).WHERE(table.BacktestJob.BacktestJobID.EQ(postgres.UUID(id)))

	out := domain.BacktestJob{}
	err := query.Query(h.Db, &out)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get backtest job: %w", err)
	}
	return &out, nil
}

func (h backtestJobRepositoryHandler) Update(job domain.BacktestJob) error {
	job.UpdatedAt = time.Now().UTC()

	query := table.BacktestJob.UPDATE(
		table.BacktestJob.MutableColumns,
	).MODEL(job).WHERE(
		table.BacktestJob.BacktestJobID.EQ(postgres.UUID(job.BacktestJobID)),
	)

	_, err := query.Exec(h.Db)
	if err != nil {
		return fmt.Errorf("failed to update backtest job: %w", err)
	}
	return nil
}

func (h backtestJobRepositoryHandler) UpdateStage(id uuid.UUID, stage string, progressPct int) error {
	query := table.BacktestJob.UPDATE(
		table.BacktestJob.MutableColumns,
	).MODEL(&domain.BacktestJob{
		BacktestJobID: id,
		CurrentStage:  stage,
		ProgressPct:  progressPct,
		Status:       domain.BacktestJobStatusRunning,
		UpdatedAt:    time.Now().UTC(),
	}).WHERE(
		table.BacktestJob.BacktestJobID.EQ(postgres.UUID(id)),
	)

	_, err := query.Exec(h.Db)
	if err != nil {
		return fmt.Errorf("failed to update backtest job stage: %w", err)
	}
	return nil
}

func (h backtestJobRepositoryHandler) MarkCompleted(id uuid.UUID, result interface{}) error {
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	query := table.BacktestJob.UPDATE(
		table.BacktestJob.MutableColumns,
	).MODEL(&domain.BacktestJob{
		BacktestJobID: id,
		Status:       domain.BacktestJobStatusCompleted,
		ProgressPct:  100,
		CurrentStage: domain.StageDone,
		Result:       string(resultBytes),
		UpdatedAt:    time.Now().UTC(),
	}).WHERE(
		table.BacktestJob.BacktestJobID.EQ(postgres.UUID(id)),
	)

	_, err = query.Exec(h.Db)
	if err != nil {
		return fmt.Errorf("failed to mark backtest job completed: %w", err)
	}
	return nil
}

func (h backtestJobRepositoryHandler) MarkFailed(id uuid.UUID, errMsg string) error {
	query := table.BacktestJob.UPDATE(
		table.BacktestJob.MutableColumns,
	).MODEL(&domain.BacktestJob{
		BacktestJobID: id,
		Status:        domain.BacktestJobStatusFailed,
		ErrorMessage:  &errMsg,
		UpdatedAt:     time.Now().UTC(),
	}).WHERE(
		table.BacktestJob.BacktestJobID.EQ(postgres.UUID(id)),
	)

	_, err := query.Exec(h.Db)
	if err != nil {
		return fmt.Errorf("failed to mark backtest job failed: %w", err)
	}
	return nil
}
