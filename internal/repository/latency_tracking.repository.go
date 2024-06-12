package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"fmt"

	"github.com/google/uuid"
)

type latencyTrackingRepositoryHandler struct {
	Db *sql.DB
}

type LatencyTrackingRepository interface {
	Add(lt domain.PerformanceProfile, requestID *uuid.UUID) error
}

func NewLatencyTrackingRepository(db *sql.DB) LatencyTrackingRepository {
	return latencyTrackingRepositoryHandler{db}
}

func (h latencyTrackingRepositoryHandler) Add(lt domain.PerformanceProfile, requestID *uuid.UUID) error {
	bytes, err := lt.ToJsonBytes()
	if err != nil {
		return err
	}

	m := model.LatencyTracking{
		ProcessingTimes: string(bytes),
		RequestID:       requestID,
	}
	query := table.LatencyTracking.INSERT(table.LatencyTracking.MutableColumns).MODEL(m)

	_, err = query.Exec(h.Db)
	if err != nil {
		return fmt.Errorf("failed to insert latency tracking: %w", err)
	}

	return nil
}
