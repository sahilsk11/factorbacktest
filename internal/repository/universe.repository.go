package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	. "factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
)

type UniverseRepository interface {
	List() ([]model.Universe, error)
}

type universeRepositoryHandler struct {
	Db *sql.DB
}

func NewUniverseRepository(db *sql.DB) UniverseRepository {
	return universeRepositoryHandler{Db: db}
}

func (h universeRepositoryHandler) List() ([]model.Universe, error) {
	query := Universe.SELECT(Universe.AllColumns)
	result := []model.Universe{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get universe symbols: %w", err)
	}

	return result, nil
}
