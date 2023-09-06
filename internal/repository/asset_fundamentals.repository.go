package repository

import (
	"alpha/internal/db/models/postgres/public/model"
	. "alpha/internal/db/models/postgres/public/table"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
)

type AssetFundamentalsRepository interface {
	Add(*sql.Tx, []model.AssetFundamental) error
	Get(tx *sql.Tx, symbol string, date time.Time) (*model.AssetFundamental, error)
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

func (h AssetFundamentalsRepositoryHandler) Get(tx *sql.Tx, symbol string, date time.Time) (*model.AssetFundamental, error) {
	query := AssetFundamental.
		SELECT(AssetFundamental.AllColumns).
		WHERE(AssetFundamental.Symbol.EQ(postgres.String(symbol)))

	out := &model.AssetFundamental{}
	err := query.Query(tx, out)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset fundamental with symbol %s: %w", symbol, err)
	}

	return out, nil
}
