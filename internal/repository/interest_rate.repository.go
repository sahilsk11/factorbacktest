package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	treasury_client "factorbacktest/pkg/treasury"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
)

type InterestRateRepository interface {
	GetRatesOnDate(date time.Time, tx *sql.Tx) (*domain.InterestRateMap, error)
	GetRatesOnDates(dates []time.Time, tx *sql.Tx) (map[string]domain.InterestRateMap, error)
	Add(m domain.InterestRateMap, date time.Time, tx *sql.Tx) error
}

type interestRateRepository struct {
	DB *sql.DB
}

func NewInterestRateRepository(db *sql.DB) InterestRateRepository {
	return interestRateRepository{
		DB: db,
	}
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

		err = r.Add(m, date, tx)
		if err != nil {
			return nil, err
		}

		return &m, nil
	}

	m := domain.InterestRateMap{
		Rates: map[int]float64{},
	}
	for _, row := range out {
		m.Rates[int(row.DurationMonths)] = row.InterestRate
	}

	return &m, nil
}

func (r interestRateRepository) GetRatesOnDates(dates []time.Time, tx *sql.Tx) (map[string]domain.InterestRateMap, error) {
	datesSet := map[string]struct{}{}
	postgresStr := []postgres.Expression{}
	for _, d := range dates {
		dateStr := d.Format(time.DateOnly)
		datesSet[dateStr] = struct{}{}
		postgresStr = append(postgresStr, postgres.DateT(d))
	}

	query := table.InterestRate.SELECT(table.InterestRate.AllColumns).
		WHERE(table.InterestRate.Date.IN(postgresStr...))

	rows := []model.InterestRate{}
	err := query.Query(tx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to query rates: %w", err)
	}

	out := map[string]domain.InterestRateMap{}
	for _, row := range rows {
		dateStr := row.Date.Format(time.DateOnly)
		if _, ok := datesSet[dateStr]; ok {
			delete(datesSet, dateStr)
		}
		if _, ok := out[dateStr]; !ok {
			out[dateStr] = domain.InterestRateMap{
				Rates: map[int]float64{},
			}
		}
		out[dateStr].Rates[int(row.DurationMonths)] = row.InterestRate
	}

	fmt.Printf("missing %d rates\n", len(datesSet))

	maps := []domain.InterestRateMap{}
	mapTimes := []time.Time{}

	for dateStr := range datesSet {
		date, err := time.Parse(time.DateOnly, dateStr)
		if err != nil {
			return nil, fmt.Errorf("failed to get date from set: %v", err)
		}

		m, err := treasury_client.GetInterestRatesOnDay(date)
		if err != nil {
			return nil, err
		}

		maps = append(maps, m)
		mapTimes = append(mapTimes, date)

		out[dateStr] = m
	}

	if len(maps) > 0 {
		err = r.AddMany(maps, mapTimes)
		if err != nil {
			return nil, fmt.Errorf("failed to add missing values back: %w", err)
		}

	}

	return out, nil
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

func (r interestRateRepository) AddMany(maps []domain.InterestRateMap, dates []time.Time) error {
	models := []model.InterestRate{}
	for i, m := range maps {
		for duration, rate := range m.Rates {
			models = append(models, model.InterestRate{
				Date:           dates[i],
				DurationMonths: int32(duration),
				InterestRate:   rate,
			})
		}
	}

	if len(models) == 0 {
		return nil
	}

	query := table.InterestRate.INSERT(table.InterestRate.MutableColumns).MODELS(models)

	_, err := query.Exec(r.DB)
	if err != nil {
		return err
	}

	return nil
}
