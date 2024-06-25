package service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/montanaflynn/stats"
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
	LoadPriceCache(inputs []LoadPriceCacheInput, stdevs []LoadStdevCacheInput) (*PriceCache, error)
}

type LoadPriceCacheInput struct {
	Date   time.Time
	Symbol string
}

type LoadStdevCacheInput struct {
	Start  time.Time
	End    time.Time
	Symbol string
}

type priceServiceHandler struct {
	AdjPriceRepository repository.AdjustedPriceRepository
	Db                 *sql.DB
}

type stdevCache struct {
	// symbol, start, end
	cache map[string]map[time.Time]map[time.Time]float64
}

func (c stdevCache) get(symbol string, start, end time.Time) (float64, bool) {
	if symbolValue, ok := c.cache[symbol]; ok {
		if startValue, ok := symbolValue[start]; ok {
			if stdev, ok := startValue[end]; ok {
				return stdev, true
			}
		}
	}
	return 0, false
}

type cacheMiss struct {
	Symbol string
	Date   time.Time
}
type PriceCache struct {
	prices             map[string]map[string]float64
	stdevs             *stdevCache
	tradingDays        []time.Time
	adjPriceRepository repository.AdjustedPriceRepository
	ReadMutex          *sync.RWMutex

	priceMisses []cacheMiss
	stdevMisses []cacheMiss
}

func (pr *PriceCache) Get(symbol string, date time.Time) (float64, error) {
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

	pr.ReadMutex.RLock()
	if _, ok := pr.prices[symbol]; ok {
		if price, ok := pr.prices[symbol][date.Format(time.DateOnly)]; ok {
			pr.ReadMutex.RUnlock()
			return price, nil
		}
	}
	pr.ReadMutex.RUnlock()

	// fmt.Printf("price cache miss %s %s\n", symbol, date.Format(time.DateOnly))

	// missed l1 cache - check db

	price, err := pr.adjPriceRepository.Get(symbol, date)
	if err != nil {
		return 0, err
	}
	pr.ReadMutex.Lock()
	pr.prices[symbol][date.Format(time.DateOnly)] = price
	pr.ReadMutex.Unlock()

	// TODO - handle missing here too

	return price, nil
}

func percentChange(end, start float64) float64 {
	return ((end - start) / end) * 100
}

func stdevsFromPriceMap(priceCache map[string]map[string]float64, stdevInputs []LoadStdevCacheInput, tradingDays []time.Time) (*stdevCache, error) {
	c := map[string]map[time.Time]map[time.Time]float64{}

	get := func(symbol string, t time.Time) (float64, bool) {
		if a, ok := priceCache[symbol]; ok {
			if b, ok := a[t.Format(time.DateOnly)]; ok {
				return b, true
			}
		}
		return 0, false
	}

	for _, in := range stdevInputs {
		intradayChanges := []float64{}
		relevantTradingDays := []time.Time{}
		skip := false
		for _, t := range tradingDays {
			if t.Equal(in.Start) || t.After(in.Start) || t.Equal(in.End) || t.After(in.End) {
				relevantTradingDays = append(relevantTradingDays, t)
			}
		}
		for i := 1; i < len(relevantTradingDays); i++ {
			startPrice, ok := get(in.Symbol, relevantTradingDays[i-1])
			if !ok {
				skip = true
				continue
				// return nil, fmt.Errorf("missing price in cache for %s on %v", in.Symbol, relevantTradingDays[i-1])
			}
			endPrice, ok := get(in.Symbol, relevantTradingDays[i])
			if !ok {
				skip = true
				continue
				// return nil, fmt.Errorf("missing price in cache for %s on %v", in.Symbol, relevantTradingDays[i])
			}
			intradayChanges = append(intradayChanges, percentChange(
				endPrice,
				startPrice,
			))
		}
		if skip {
			continue
		}

		stdev, err := stats.StandardDeviationSample(intradayChanges)
		if err != nil {
			return nil, err
		}
		magicNumber := math.Sqrt(252)

		set := func(symbol string, start, end time.Time, v float64) {
			if _, ok := c[symbol]; !ok {
				c[symbol] = map[time.Time]map[time.Time]float64{}
			}
			if _, ok := c[symbol][start]; !ok {
				c[symbol][start] = map[time.Time]float64{}
			}
			c[symbol][start][end] = v
		}

		set(in.Symbol, in.Start, in.End, stdev*magicNumber)
	}

	return &stdevCache{
		cache: c,
	}, nil
}

func (pr *PriceCache) GetStdev(ctx context.Context, symbol string, start, end time.Time) (float64, error) {
	if result, ok := pr.stdevs.get(symbol, start, end); ok {
		return result, nil
	}

	return 0, fmt.Errorf("stdev cache miss %s %s to %s", symbol, start.Format(time.DateOnly), end.Format(time.DateOnly))
}

func NewPriceService(db *sql.DB, adjPriceRepository repository.AdjustedPriceRepository) PriceService {
	return &priceServiceHandler{
		AdjPriceRepository: adjPriceRepository,
		Db:                 db,
	}
}

// LoadPriceCache uses dry-run results to populate prices and stdevs
func (h priceServiceHandler) LoadPriceCache(inputs []LoadPriceCacheInput, stdevInputs []LoadStdevCacheInput) (*PriceCache, error) {
	type minMax struct {
		min *time.Time
		max *time.Time
	}
	var (
		absMin *time.Time
		absMax *time.Time
	)
	minMaxMap := map[string]*minMax{}
	for _, in := range inputs {
		if _, ok := minMaxMap[in.Symbol]; !ok {
			minMaxMap[in.Symbol] = &minMax{}
		}
		mp := minMaxMap[in.Symbol]
		if mp.min == nil || in.Date.Before(*mp.min) {
			mp.min = &in.Date
		}
		if mp.max == nil || in.Date.After(*mp.max) {
			mp.max = &in.Date
		}
		if absMin == nil || in.Date.Before(*absMin) {
			absMin = &in.Date
		}
		if absMax == nil || in.Date.Before(*absMax) {
			absMax = &in.Date
		}
	}
	for _, in := range stdevInputs {
		if _, ok := minMaxMap[in.Symbol]; !ok {
			minMaxMap[in.Symbol] = &minMax{}
		}
		mp := minMaxMap[in.Symbol]
		if mp.min == nil || in.Start.Before(*mp.min) {
			mp.min = &in.Start
		}
		if mp.max == nil || in.End.After(*mp.max) {
			mp.max = &in.End
		}
		if absMin == nil || in.Start.Before(*absMin) {
			absMin = &in.Start
		}
		if absMax == nil || in.End.Before(*absMax) {
			absMax = &in.End
		}
	}
	getInputs := []repository.GetManyInput{}
	for symbol, minMaxValues := range minMaxMap {
		getInputs = append(getInputs, repository.GetManyInput{
			Symbol:  symbol,
			MinDate: *minMaxValues.min,
			MaxDate: *minMaxValues.max,
		})
	}

	if len(getInputs) == 0 {
		return &PriceCache{
			prices: map[string]map[string]float64{},
			stdevs: &stdevCache{
				cache: map[string]map[time.Time]map[time.Time]float64{},
			},
			tradingDays:        []time.Time{},
			adjPriceRepository: h.AdjPriceRepository,
			ReadMutex:          &sync.RWMutex{},
		}, nil
	}

	// super random, no idea what this represents
	tradingDays, err := h.AdjPriceRepository.ListTradingDays(*absMin, *absMax)
	if err != nil {
		return nil, err
	}

	// TODO - we're gonna have lots of stdev values in this
	// if we decide to optimize, we should remove them
	prices, err := h.AdjPriceRepository.GetMany(getInputs)
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

	stdevCache, err := stdevsFromPriceMap(cache, stdevInputs, tradingDays)
	if err != nil {
		return nil, err
	}

	return &PriceCache{
		prices:             cache,
		stdevs:             stdevCache,
		tradingDays:        tradingDays,
		adjPriceRepository: h.AdjPriceRepository,
		ReadMutex:          &sync.RWMutex{},
	}, nil
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
	for _, a := range assets {
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
