package internal

import (
	"context"
	"database/sql"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"fmt"
	"time"
)

/**

behavior - when i ask for a price, it should figure out the price without db lookups
if the price is missing from the cache, it should sync pricing

this should also handle weekends/non-trading days. it should figure out the most recent
trading day, and use that price

*/

type PriceService interface {
	LoadCache(tx *sql.Tx, inputs []DataInput) (*PriceCache, error)
	UpdatePricesIfNeeded(ctx context.Context, tx *sql.Tx, symbols []string) error
}

type priceServiceHandler struct {
	AdjPriceRepository repository.AdjustedPriceRepository
}

type PriceCache struct {
	cache       map[string]map[string]float64
	tradingDays []time.Time
	// Tx                 *sql.Tx
	adjPriceRepository repository.AdjustedPriceRepository
}

func (pr PriceCache) Get(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
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

	price, err := pr.adjPriceRepository.Get(tx, symbol, date)
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
	}
}

func (h priceServiceHandler) LoadCache(tx *sql.Tx, inputs []DataInput) (*PriceCache, error) {
	setInputs := []repository.ListFromSetInput{}
	for _, d := range inputs {
		setInputs = append(setInputs, repository.ListFromSetInput{
			Symbol: d.Symbol,
			Date:   d.Date,
		})
	}
	prices, err := h.AdjPriceRepository.ListFromSet(tx, setInputs)
	if err != nil {
		return nil, err
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

func (h priceServiceHandler) UpdatePricesIfNeeded(ctx context.Context, tx *sql.Tx, symbols []string) error {
	// need a better way of handling this too
	symbols = append(symbols, "SPY")

	latestPrices, err := h.AdjPriceRepository.LatestPrices(tx, symbols)
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

	// i think this should use UpdateUniversePrices
	// instead
	for _, s := range assetsToUpdate {
		err = IngestPrices(tx, s.Symbol, h.AdjPriceRepository, &s.Date)
		if err != nil {
			return fmt.Errorf("failed to ingest historical prices for %s: %w", s.Symbol, err)
		}
	}

	return nil
}
