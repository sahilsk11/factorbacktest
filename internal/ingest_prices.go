package internal

import (
	"alpha/internal/db/models/postgres/public/model"
	"alpha/internal/repository"
	"database/sql"
	"fmt"
	"time"

	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
)

func IngestPrices(
	tx *sql.Tx,
	symbol string,
	adjPricesRepository repository.AdjustedPriceRepository,
) error {
	start := time.Date(2018, 1, 0, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	params := &chart.Params{
		Start:    datetime.New(&start),
		End:      datetime.New(&now),
		Symbol:   symbol,
		Interval: datetime.OneDay,
	}
	iter := chart.Get(params)

	models := []model.AdjustedPrice{}

	for iter.Next() {
		models = append(models, model.AdjustedPrice{
			Symbol:    symbol,
			Date:      time.Unix(int64(iter.Bar().Timestamp), 0),
			Price:     iter.Bar().AdjClose.InexactFloat64(),
			CreatedAt: time.Now().UTC(),
		})
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to get prices for %s: %w", symbol, err)
	}

	err := adjPricesRepository.Add(tx, models)
	if err != nil {
		return err
	}

	return nil
}

func UpdateUniversePrices(
	tx *sql.Tx,
	universeRepository repository.UniverseRepository,
	adjPricesRepository repository.AdjustedPriceRepository,
) error {
	assets, err := universeRepository.List(tx)
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		return fmt.Errorf("no assets found in universe")
	}

	errors := []error{}

	for _, a := range assets {
		err = IngestPrices(tx, a.Symbol, adjPricesRepository)
		if err != nil {
			err = fmt.Errorf("failed to ingest historical prices for %s: %w", a.Symbol, err)
			fmt.Println(err)
			errors = append(errors, err)
		} else {
			fmt.Println("added", a.Symbol)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to update %d/%d universe prices. first err: %w", len(errors), len(assets), errors[0])
	}

	return nil
}
