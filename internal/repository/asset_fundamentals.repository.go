package repository

import (
	"alpha/internal/db/models/postgres/public/model"
	. "alpha/internal/db/models/postgres/public/table"
	"database/sql"
	"fmt"
	"time"

	. "github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
)

type AssetFundamentalsRepository interface {
	Add(qrm.Executable, []model.AssetFundamental) error
	Get(tx *sql.Tx, symbol string, date time.Time) (*model.AssetFundamental, error)
}

type AssetFundamentalsRepositoryHandler struct{}

func (h AssetFundamentalsRepositoryHandler) Add(tx qrm.Executable, af []model.AssetFundamental) error {
	if len(af) == 0 {
		return fmt.Errorf("no models were provided to insert into asset_fundamental")
	}
	query := AssetFundamental.
		INSERT(AssetFundamental.MutableColumns).
		MODELS(af).
		ON_CONFLICT(
			AssetFundamental.Symbol, AssetFundamental.StartDate, AssetFundamental.EndDate,
		).DO_NOTHING()

	_, err := query.Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to add asset fundamental to db: %w", err)
	}

	return nil
}

func (h AssetFundamentalsRepositoryHandler) Get(tx *sql.Tx, symbol string, date time.Time) (*model.AssetFundamental, error) {
	d := DateT(date)
	query := AssetFundamental.
		SELECT(AssetFundamental.AllColumns).
		WHERE(
			AND(
				AssetFundamental.Symbol.EQ(String(symbol)),
				AssetFundamental.StartDate.LT_EQ(d),
				AssetFundamental.EndDate.GT_EQ(d),
			),
		)

	out := &model.AssetFundamental{}
	err := query.Query(tx, out)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset fundamental with symbol %s: %w", symbol, err)
	}

	return out, nil
}
