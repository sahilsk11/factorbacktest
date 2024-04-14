package internal

import (
	"database/sql"
	"factorbacktest/internal/repository"
	"time"
)

/**

behavior - when i ask for a price, it should figure out the price without db lookups
if the price is missing from the cache, it should sync pricing

this should also handle weekends/non-trading days. it should figure out the most recent
trading day, and use that price

*/

type PriceService interface {
	LoadCache(symbols []string, start time.Time, end time.Time) (*PriceCache, error)
}

type priceServiceHandler struct {
	Db                 *sql.DB
	AdjPriceRepository repository.AdjustedPriceRepository
}

type priceCache map[string]map[time.Time]float64

type PriceCache struct {
	cache              priceCache
	tradingDays        []time.Time
	tx                 *sql.Tx
	adjPriceRepository repository.AdjustedPriceRepository
}

func (pr PriceCache) Get(symbol string, date time.Time) (float64, error) {
	// find the relevant trading day for the price
	closestTradingDay := date
	for i := 0; i < len(pr.tradingDays)-1; i++ {
		if pr.tradingDays[i+1].After(date) {
			closestTradingDay = pr.tradingDays[i]
		}
	}
	// if the trading dates given do not include the given date, then just use original date
	if pr.tradingDays == nil || pr.tradingDays[0].After(date) || pr.tradingDays[len(pr.tradingDays)-1].Before(date) {
		closestTradingDay = date
	}
	date = closestTradingDay

	if _, ok := pr.cache[symbol]; ok {
		if _, ok := pr.cache[symbol][date]; ok {
			return pr.cache[symbol][date], nil
		}
	}

	// missed l1 cache - check db

	price, err := pr.adjPriceRepository.Get(pr.tx, symbol, date)
	if err != nil {
		return 0, err
	}
	pr.cache[symbol][date] = price

	// TODO - handle missing here too

	return price, nil
}

func NewPriceService(db *sql.DB, adjPriceRepository repository.AdjustedPriceRepository) PriceService {
	return &priceServiceHandler{
		Db:                 db,
		AdjPriceRepository: adjPriceRepository,
	}
}

func (h priceServiceHandler) LoadCache(symbols []string, start time.Time, end time.Time) (*PriceCache, error) {
	tx, err := h.Db.Begin()
	if err != nil {
		return nil, err
	}

	tradingDays, err := h.AdjPriceRepository.ListTradingDays(tx, start, end)
	if err != nil {
		return nil, err
	}

	prices, err := h.AdjPriceRepository.List(tx, symbols, start, end)
	if err != nil {
		return nil, err
	}

	cache := make(priceCache)
	for _, p := range prices {
		cache[p.Symbol][p.Date] = p.Price
	}

	return &PriceCache{
		cache:       cache,
		tradingDays: tradingDays,
		tx:          tx,
	}, nil
}
