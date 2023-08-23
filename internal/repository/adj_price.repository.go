package repository

import (
	"alpha/internal/db/models/postgres/public/model"
	. "alpha/internal/db/models/postgres/public/table"
	"alpha/internal/domain"
	"database/sql"
	"fmt"
	"time"

	. "github.com/go-jet/jet/v2/postgres"
)

type AdjustedPriceRepository interface {
	Add(*sql.Tx, []model.AdjustedPrice) error
	Get(*sql.Tx, string, time.Time) (float64, error)
	List(tx *sql.Tx, symbol string, start, end time.Time) ([]domain.AssetPrice, error)
}

type AdjustedPriceRepositoryHandler struct{}

func (h AdjustedPriceRepositoryHandler) Add(tx *sql.Tx, adjPrices []model.AdjustedPrice) error {
	query := AdjustedPrice.
		INSERT(AdjustedPrice.MutableColumns).
		MODELS(adjPrices)

	_, err := query.Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to add adjusted prices to db: %w", err)
	}

	return nil
}

func (h AdjustedPriceRepositoryHandler) Get(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	minDate := DateT(date.AddDate(0, 0, -3))
	maxDate := DateT(date)
	// use range so we can do t-3 for weekends or holidays
	query := AdjustedPrice.
		SELECT(AdjustedPrice.AllColumns).
		WHERE(
			AND(
				AdjustedPrice.Symbol.EQ(String(symbol)),
				AdjustedPrice.Date.BETWEEN(minDate, maxDate),
			),
		).
		ORDER_BY(AdjustedPrice.Date.DESC()).
		LIMIT(1)

	result := model.AdjustedPrice{}
	err := query.Query(tx, &result)
	if err != nil {
		return 0, fmt.Errorf("failed to query price for %s on %v: %w", symbol, date, err)
	}

	return result.Price, nil
}

func (h AdjustedPriceRepositoryHandler) List(tx *sql.Tx, symbol string, start, end time.Time) ([]domain.AssetPrice, error) {
	minDate := DateT(start)
	maxDate := DateT(end)
	// use range so we can do t-3 for weekends or holidays
	query := AdjustedPrice.
		SELECT(AdjustedPrice.AllColumns).
		WHERE(
			AND(
				AdjustedPrice.Symbol.EQ(String(symbol)),
				AdjustedPrice.Date.BETWEEN(minDate, maxDate),
			),
		).
		ORDER_BY(AdjustedPrice.Date.ASC())

	result := []model.AdjustedPrice{}
	err := query.Query(tx, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list prices for %s: %w", symbol, err)
	}

	out := []domain.AssetPrice{}
	for _, p := range result {
		out = append(out, domain.AssetPrice{
			Symbol: p.Symbol,
			Date:   p.Date,
			Price:  p.Price,
		})
	}

	return out, nil
}
