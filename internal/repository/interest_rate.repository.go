package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	treasury_client "factorbacktest/pkg/treasury"
	"time"

	"github.com/go-jet/jet/v2/postgres"
)

type InterestRateRepository interface {
	GetRatesOnDate(date time.Time, tx *sql.Tx) (*domain.InterestRateMap, error)
	Add(m domain.InterestRateMap, date time.Time, tx *sql.Tx) error
}

type interestRateRepository struct{}

func NewInterestRateRepository() InterestRateRepository {
	return interestRateRepository{}
}

func (r interestRateRepository) GetRatesOnDate(date time.Time, tx *sql.Tx) (*domain.InterestRateMap, error) {
	query := table.InterestRate.SELECT(table.InterestRate.AllColumns).
		WHERE(
			table.InterestRate.Date.EQ(postgres.DateT(date)),
		)

	out := []model.InterestRate{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, err
	}

	if len(out) == 0 {
		m, err := treasury_client.GetInterestRatesOnDay(date)
		if err != nil {
			return nil, err
		}

		err = r.Add(*m, date, tx)
		if err != nil {
			return nil, err
		}

		return m, nil
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
