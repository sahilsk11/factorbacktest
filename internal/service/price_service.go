package service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"fmt"
	"sync"
	"time"

	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
)

/**

behavior - when i ask for a price, it should figure out the price without db lookups
if the price is missing from the cache, it should sync pricing

this should also handle weekends/non-trading days. it should figure out the most recent
trading day, and use that price

*/

type PriceService interface {
	LoadCache(inputs []LoadPriceCacheInput) (*PriceCache, error)
	UpdatePricesIfNeeded(ctx context.Context, symbols []string) error
}

type LoadPriceCacheInput struct {
	Date   time.Time
	Symbol string
}

type priceServiceHandler struct {
	AdjPriceRepository repository.AdjustedPriceRepository
	Db                 *sql.DB
}

type PriceCache struct {
	cache       map[string]map[string]float64
	tradingDays []time.Time
	// Tx                 *sql.Tx
	adjPriceRepository repository.AdjustedPriceRepository
}

func (pr PriceCache) Get(symbol string, date time.Time) (float64, error) {
	// find the relevant trading day for the price
	closestTradingDay := date
	for i := 0; i < len(pr.tradingDays)-1; i++ {
		if pr.tradingDays[i+1].After(date) {
			closestTradingDay = pr.tradingDays[i]
			break
		}
	}
	// if the trading dates given do not include the given date, then just use original date
	if pr.tradingDays == nil || pr.tradingDays[0].After(date) || pr.tradingDays[len(pr.tradingDays)-1].Before(date) {
		closestTradingDay = date
	}
	date = closestTradingDay

	if _, ok := pr.cache[symbol]; ok {
		if _, ok := pr.cache[symbol][date.Format(time.DateOnly)]; ok {
			return pr.cache[symbol][date.Format(time.DateOnly)], nil
		}
	}

	// missed l1 cache - check db

	price, err := pr.adjPriceRepository.Get(symbol, date)
	if err != nil {
		return 0, err
	}
	pr.cache[symbol][date.Format(time.DateOnly)] = price

	// TODO - handle missing here too

	return price, nil
}

func NewPriceService(db *sql.DB, adjPriceRepository repository.AdjustedPriceRepository) PriceService {
	return &priceServiceHandler{
		AdjPriceRepository: adjPriceRepository,
		Db:                 db,
	}
}

func (h priceServiceHandler) LoadCache(inputs []LoadPriceCacheInput) (*PriceCache, error) {
	setInputs := []repository.ListFromSetInput{}
	for _, d := range inputs {
		setInputs = append(setInputs, repository.ListFromSetInput{
			Symbol: d.Symbol,
			Date:   d.Date,
		})
	}

	if len(setInputs) == 0 {
		return &PriceCache{
			cache:              map[string]map[string]float64{},
			tradingDays:        []time.Time{},
			adjPriceRepository: h.AdjPriceRepository,
		}, nil
	}

	prices, err := h.AdjPriceRepository.ListFromSet(setInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}

	cache := make(map[string]map[string]float64)
	for _, p := range prices {
		if _, ok := cache[p.Symbol]; !ok {
			cache[p.Symbol] = make(map[string]float64)
		}
		cache[p.Symbol][p.Date.Format(time.DateOnly)] = p.Price
	}

	return &PriceCache{
		cache:              cache,
		tradingDays:        nil, // let's try to remove this
		adjPriceRepository: h.AdjPriceRepository,
	}, nil
}

// UpdatePricesIfNeeded determines if the prices we currently have stored are up to date
// and fetches + updates prices if they are not
// Currently not used, since we rely on UpdateUniversePrices
// TODO - handle stock splits
func (h priceServiceHandler) UpdatePricesIfNeeded(ctx context.Context, symbols []string) error {
	// need a better way of handling this too
	symbols = append(symbols, "SPY")

	latestPrices, err := h.AdjPriceRepository.LatestPrices(symbols)
	if err != nil {
		return fmt.Errorf("failed to get latest prices: %w", err)
	}

	// somehow we need to figure out the real last trading day
	actualLastTradingDay := time.Now().UTC().AddDate(0, 0, -7)
	assetsToUpdate := []domain.AssetPrice{}
	for _, price := range latestPrices {
		if price.Date.Before(actualLastTradingDay) {
			assetsToUpdate = append(assetsToUpdate, price)
		}
	}

	// update prices
	fmt.Printf("updating %d assets\n", len(assetsToUpdate))

	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// i think this should use UpdateUniversePrices
	// instead
	for _, s := range assetsToUpdate {
		err = IngestPrices(tx, s.Symbol, h.AdjPriceRepository, &s.Date)
		if err != nil {
			return fmt.Errorf("failed to ingest historical prices for %s: %w", s.Symbol, err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit update prices changes: %w", err)
	}

	return nil
}

func IngestPrices(
	tx *sql.Tx,
	symbol string,
	adjPricesRepository repository.AdjustedPriceRepository,
	start *time.Time,
) error {
	s := time.Date(2000, 1, 0, 0, 0, 0, 0, time.UTC)
	if start != nil {
		s = *start
	}
	now := time.Now()
	params := &chart.Params{
		Start:    datetime.New(&s),
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
	tickerRepository repository.TickerRepository,
	adjPricesRepository repository.AdjustedPriceRepository,
) error {
	assets, err := tickerRepository.List()
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		return fmt.Errorf("no assets found in universe")
	}
	assets = append(assets, model.Ticker{
		Symbol: "SPY",
	})

	symbols := []string{}
	for _, a := range assets[:10] {
		symbols = append(symbols, a.Symbol)
	}

	asyncIngestPrices(context.Background(), tx, symbols, adjPricesRepository)

	return nil
}

func asyncIngestPrices(ctx context.Context, tx *sql.Tx, symbols []string, adjPriceRepository repository.AdjustedPriceRepository) error {
	numGoroutines := 10

	inputCh := make(chan string, len(symbols))

	var wg sync.WaitGroup
	for _, f := range symbols {
		wg.Add(1)
		inputCh <- f
	}
	close(inputCh)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case symbol, ok := <-inputCh:
					if !ok {
						return
					}
					err := IngestPrices(tx, symbol, adjPriceRepository, nil)
					if err != nil {
						fmt.Printf("failed to ingest price for %s: %s\n", symbol, err.Error())
					}
					wg.Done()
				}
			}
		}()
	}

	wg.Wait()

	return nil
}
