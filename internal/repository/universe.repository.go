package repository

import (
	"alpha/internal/db/models/postgres/public/model"
	. "alpha/internal/db/models/postgres/public/table"
	"fmt"

	"github.com/go-jet/jet/v2/qrm"
)

type UniverseRepository interface {
	List(db qrm.Queryable) ([]model.Universe, error)
}

type UniverseRepositoryHandler struct{}

func (h UniverseRepositoryHandler) List(tx qrm.Queryable) ([]model.Universe, error) {
	query := Universe.SELECT(Universe.AllColumns)
	result := []model.Universe{}
	err := query.Query(tx, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get universe symbols: %w", err)
	}

	return result, nil
}
