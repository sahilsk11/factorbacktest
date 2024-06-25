package repository

import (
	"alpha/internal/db/models/postgres/public/model"
	. "alpha/internal/db/models/postgres/public/table"
	"database/sql"
	"fmt"
)

type AssetFundamentalsRepository interface {
	Add(*sql.Tx, []model.AssetFundamental) error
}

type AssetFundamentalsRepositoryHandler struct{}

func (h AssetFundamentalsRepositoryHandler) Add(tx *sql.Tx, af []model.AssetFundamental) error {
	query := AssetFundamental.
		INSERT(AssetFundamental.MutableColumns).
		MODELS(af)

	_, err := query.Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to add asset fundamental to db: %w", err)
	}

	return nil
}
