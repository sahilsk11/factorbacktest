package service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
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
	LoadPriceCache(ctx context.Context, inputs []LoadPriceCacheInput, stdevs []LoadStdevCacheInput) (*PriceCache, error)
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

	priceMisses []cacheMiss
	stdevMisses []cacheMiss
}

// Get retrieves the price for an asset on the given day
// it uses the preloaded price cache, or retrieves from db
func (pr *PriceCache) Get(symbol string, date time.Time) (float64, error) {
	if _, ok := pr.prices[symbol]; ok {
		if price, ok := pr.prices[symbol][date.Format(time.DateOnly)]; ok {
			return price, nil
		}
	}

	// todo - restore the l2 get here, once we have a way of marking
	// something as known missing

	return 0, fmt.Errorf("price cache miss %s %s\n", symbol, date.Format(time.DateOnly))
}

func percentChange(end, start float64) float64 {
	return ((end - start) / end) * 100
}

func stdevsFromPriceMap(priceCache map[string]map[string]float64, stdevInputs []LoadStdevCacheInput, tradingDays []time.Time) (*stdevCache, error) {
	// profile, endProfile := domain.GetProfile(ctx)
	// defer endProfile()

	c := map[string]map[time.Time]map[time.Time]float64{}

	get := func(symbol string, t time.Time) (float64, bool) {
		if a, ok := priceCache[symbol]; ok {
			if b, ok := a[t.Format(time.DateOnly)]; ok {
				return b, true
			}
		}
		return 0, false
	}

	fmt.Printf("%d stdev inputs\n", len(stdevInputs))
	for _, in := range stdevInputs {
		intradayChanges := []float64{}
		relevantTradingDays := []time.Time{}
		skip := false
		for _, t := range tradingDays {
			if (t.Equal(in.Start) || t.After(in.Start)) && (t.Equal(in.End) || t.Before(in.End)) {
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

type minMax struct {
	min *time.Time
	max *time.Time
}

// LoadPriceCache uses dry-run results to populate prices and stdevs
// it's expected to populate results for all days in the inputs, even
// if they are non-trading days
func (h priceServiceHandler) LoadPriceCache(ctx context.Context, inputs []LoadPriceCacheInput, stdevInputs []LoadStdevCacheInput) (*PriceCache, error) {
	profile, endProfile := domain.GetProfile(ctx)
	defer endProfile()
	absMin, absMax, minMaxMap := constructMinMaxMap(inputs, stdevInputs)

	// uhh so the getInput technically tells us which date the equation will
	// want to fetch on, but if it's not on a trading day, we're kinda fucked?
	// i think we should just do like 7 days before absMin so we have the price
	// on the requested day...
	// i.e. on day 1 of backtest, do (priceChange(n-1, n)) on a Monday and suddenly we
	// need to fetch price from the like prev Friday
	getInputs := []repository.GetManyInput{}
	for symbol, minMaxValues := range minMaxMap {
		getInputs = append(getInputs, repository.GetManyInput{
			Symbol:  symbol,
			MinDate: (*minMaxValues.min).AddDate(0, 0, -7),
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
		}, nil
	}

	_, endSpan := profile.StartNewSpan("get many query")
	// TODO - we're gonna have lots of stdev values in this
	// if we decide to optimize, we should remove them
	prices, err := h.AdjPriceRepository.GetMany(getInputs)
	if err != nil {
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}
	endSpan()

	// super random, no idea what this represents
	tradingDays, err := h.AdjPriceRepository.ListTradingDays(*absMin, *absMax)
	if err != nil {
		return nil, err
	}

	// when constructing the cache, let's load values into non-trading days?
	// TODO - when doing n-anything ago, we need to be certain about using
	// real days or trading days
	// either way, i think we should populate non-trading day values
	// in the cache with the most recent value

	// this is fine - just load everything we definitely know into the cache
	span, endSpan := profile.StartNewSpan("filling price cache")
	newProfile, endNewProfile := span.NewSubProfile()
	_, endNewSpan := newProfile.StartNewSpan("loading values from query result")
	cache := make(map[string]map[string]float64)
	for _, p := range prices {
		if _, ok := cache[p.Symbol]; !ok {
			cache[p.Symbol] = make(map[string]float64)
		}
		cache[p.Symbol][p.Date.Format(time.DateOnly)] = p.Price
	}
	endNewSpan()

	// can we also fill anything that was asked for in the cache
	_, endNewSpan = newProfile.StartNewSpan("filling price cache gaps")
	fillPriceCacheGaps(inputs, cache)
	endNewSpan()

	_, endNewSpan = newProfile.StartNewSpan("populating stdev cache")
	stdevCache, err := stdevsFromPriceMap(cache, stdevInputs, tradingDays)
	if err != nil {
		return nil, err
	}
	endNewSpan()
	endNewProfile()
	endSpan()

	return &PriceCache{
		prices:             cache,
		stdevs:             stdevCache,
		tradingDays:        tradingDays,
		adjPriceRepository: h.AdjPriceRepository,
	}, nil
}

func constructMinMaxMap(inputs []LoadPriceCacheInput, stdevInputs []LoadStdevCacheInput) (*time.Time, *time.Time, map[string]*minMax) {
	var (
		absMin *time.Time
		absMax *time.Time
	)

	minMaxMap := map[string]*minMax{}
	for _, in := range inputs {
		date := in.Date

		if _, ok := minMaxMap[in.Symbol]; !ok {
			minMaxMap[in.Symbol] = &minMax{}
		}

		mp := minMaxMap[in.Symbol]
		if mp.min == nil || in.Date.Before(*mp.min) {
			mp.min = &date
		}
		if mp.max == nil || in.Date.After(*mp.max) {
			mp.max = &date
		}
		if absMin == nil || in.Date.Before(*absMin) {
			absMin = &date
		}
		if absMax == nil || in.Date.After(*absMax) {
			absMax = &date
		}
	}
	for _, in := range stdevInputs {
		if _, ok := minMaxMap[in.Symbol]; !ok {
			minMaxMap[in.Symbol] = &minMax{}
		}
		mp := minMaxMap[in.Symbol]
		start := in.Start
		end := in.End
		if mp.min == nil || in.Start.Before(*mp.min) {
			mp.min = &start
		}
		if mp.max == nil || in.End.After(*mp.max) {
			mp.max = &end
		}
		if absMin == nil || in.Start.Before(*absMin) {
			absMin = &start
		}
		if absMax == nil || in.End.After(*absMax) {
			absMax = &end
		}
	}

	return absMin, absMax, minMaxMap
}

func fillPriceCacheGaps(inputs []LoadPriceCacheInput, cache map[string]map[string]float64) {
	for _, in := range inputs {
		// if we have no data on the symbol, skip
		// but this should be super rare and we should
		// mark as missing
		if symbolCache, ok := cache[in.Symbol]; ok {
			mostRecentDate := in.Date

			newPrice, found := symbolCache[in.Date.Format(time.DateOnly)]
			numTries := 0

			// instead of doing this linear scan, we could binary search
			// for the most recent date
			for !found && numTries < 7 {

				mostRecentDate = mostRecentDate.AddDate(0, 0, -1)
				newPrice, found = symbolCache[mostRecentDate.Format(time.DateOnly)]
				numTries++
			}

			if found && numTries > 0 {
				symbolCache[in.Date.Format(time.DateOnly)] = newPrice
			}
		}
	}
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
