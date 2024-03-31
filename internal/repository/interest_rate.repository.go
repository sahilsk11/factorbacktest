package repository

import (
	"database/sql"
	"errors"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	treasury_client "factorbacktest/pkg/treasury"
	"time"

	"github.com/go-jet/jet/v2/postgres"
)

type InterestRateRepository interface {
	GetRatesOnDay(time.Time) (*domain.InterestRateMap, error)
	GetInterestRatesOnDates([]time.Time) ([]domain.InterestRateMap, error)

	Add(m model.InterestRate, tx *sql.Tx) error
}

type interestRateRepository struct{}

func (r interestRateRepository) GetRatesOnDate(date time.Time, tx *sql.Tx) (*domain.InterestRateMap, error) {
	query := table.InterestRate.SELECT(table.InterestRate.AllColumns).
		WHERE(
			table.InterestRate.Date.EQ(postgres.DateT(date)),
		)

	out := []model.InterestRate{}
	err := query.Query(tx, &out)
	if errors.Is(err, sql.ErrNoRows) {
		m, err := treasury_client.GetInterestRatesOnDay(date)
		if err != nil {
			return nil, err
		}

		err = r.Add(*m, date, tx)
		if err != nil {
			return nil, err
		}

		return m, nil
	} else if err != nil {
		return nil, err
	}

	m := domain.InterestRateMap{
		Rates: map[int]float64{},
	}
	for _, row := range out {
		m.Rates[int(row.DurationMonths)] = row.InterestRate
	}

	return &m, nil
}

func (r interestRateRepository) Add(m domain.InterestRateMap, date time.Time, tx *sql.Tx) error {
	models := []model.InterestRate{}
	for duration, rate := range m.Rates {
		models = append(models, model.InterestRate{
			Date:           date,
			DurationMonths: int32(duration),
			InterestRate:   rate,
		})
	}
	query := table.InterestRate.INSERT(table.InterestRate.MutableColumns).MODELS(models)

	_, err := query.Exec(tx)
	if err != nil {
		return err
	}

	return nil
}
