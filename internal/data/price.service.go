package data

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/shopspring/decimal"
)

/**

behavior - when i ask for a price, it should figure out the price without db lookups
if the price is missing from the cache, it should sync pricing

this should also handle weekends/non-trading days. it should figure out the most recent
trading day, and use that price

*/

type PriceService interface {
	LoadPriceCache(ctx context.Context, inputs []LoadPriceCacheInput, stdevs []LoadStdevCacheInput) (*PriceCache, error)
	GetLatestPrices(ctx context.Context, symbols []string) (map[string]decimal.Decimal, error)
	IngestPrices(ctx context.Context, tx *sql.Tx, symbol string, adjPricesRepository repository.AdjustedPriceRepository, start *time.Time) error
	UpdateUniversePrices(ctx context.Context, tx *sql.Tx, tickerRepository repository.TickerRepository, adjPricesRepository repository.AdjustedPriceRepository) (int, error)
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
	AlpacaRepository   repository.AlpacaRepository
	QuoteProvider      QuoteProvider
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

func stdevsFromPriceMap(minMaxMap map[string]*minMax, priceCache map[string]map[string]float64, stdevInputs []LoadStdevCacheInput, tradingDays []time.Time) (*stdevCache, error) {
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

	set := func(symbol string, start, end time.Time, v float64) {
		if _, ok := c[symbol]; !ok {
			c[symbol] = map[time.Time]map[time.Time]float64{}
		}
		if _, ok := c[symbol][start]; !ok {
			c[symbol][start] = map[time.Time]float64{}
		}
		c[symbol][start][end] = v
	}

	type returnOnDay struct {
		date time.Time
		ret  float64
	}

	returnsBySymbol := map[string][]returnOnDay{}

	for symbol, minMax := range minMaxMap {
		if _, ok := returnsBySymbol[symbol]; !ok {
			returnsBySymbol[symbol] = []returnOnDay{}
		}
		for i := 1; i < len(tradingDays); i++ {
			t := tradingDays[i]
			if (t.Equal(*minMax.min) || t.After(*minMax.min)) && (t.Equal(*minMax.max) || t.Before(*minMax.max)) {
				newPrice, ok := get(symbol, t)
				if !ok {
					continue
				}
				oldPrice, ok := get(symbol, tradingDays[i-1])
				if !ok {
					continue
				}
				returnsBySymbol[symbol] = append(returnsBySymbol[symbol], returnOnDay{
					date: t,
					ret:  percentChange(newPrice, oldPrice),
				})
			}
		}
	}

	for _, in := range stdevInputs {
		returns, ok := returnsBySymbol[in.Symbol]
		// todo - figure out what case would cause no returns
		if !ok || len(returns) == 0 {
			continue
		}
		bufferedStart := in.Start.AddDate(0, 0, 7)
		bufferedEnd := in.End.AddDate(0, 0, -7)
		if returns[0].date.After(bufferedStart) || returns[len(returns)-1].date.Before(bufferedEnd) {
			continue
		}
		data := []float64{}
		for _, ret := range returns {
			t := ret.date
			if (t.Equal(in.Start) || t.After(in.Start)) && (t.Equal(in.End) || t.Before(in.End)) {
				data = append(data, ret.ret)
			}
		}
		stdev, err := stats.StandardDeviationSample(data)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate stdev for %s between %s and %s: %w", in.Symbol, in.Start.Format(time.DateOnly), in.End.Format(time.DateOnly), err)
		}
		magicNumber := math.Sqrt(252)
		set(in.Symbol, in.Start, in.End, stdev*magicNumber)
	}

	return &stdevCache{
		cache: c,
	}, nil
}

// GetManyOnDay returns prices for the given symbols on the given date, mirroring
// AdjustedPriceRepository.GetManyOnDay's signature so callers in the backtest
// hot path can avoid an extra db round-trip per rebalance day. For symbols
// that aren't present in the cache, it falls back to the underlying repository
// in a single batched query and warns so we can confirm fallback is rare.
func (pr *PriceCache) GetManyOnDay(ctx context.Context, symbols []string, date time.Time) (map[string]decimal.Decimal, error) {
	out := map[string]decimal.Decimal{}
	missing := []string{}
	dateKey := date.Format(time.DateOnly)

	for _, s := range symbols {
		if symbolPrices, ok := pr.prices[s]; ok {
			if price, ok := symbolPrices[dateKey]; ok {
				out[s] = decimal.NewFromFloat(price)
				continue
			}
		}
		missing = append(missing, s)
	}

	if len(missing) > 0 {
		log := logger.FromContext(ctx)
		log.Warnf("PriceCache.GetManyOnDay fallback to repository for %d/%d symbols on %s", len(missing), len(symbols), dateKey)
		if pr.adjPriceRepository == nil {
			return nil, fmt.Errorf("price cache miss for %d symbols on %s and no fallback repository configured", len(missing), dateKey)
		}
		fallback, err := pr.adjPriceRepository.GetManyOnDay(missing, date)
		if err != nil {
			return nil, fmt.Errorf("failed cache-fallback GetManyOnDay: %w", err)
		}
		for k, v := range fallback {
			out[k] = v
		}
	}

	return out, nil
}

func (pr *PriceCache) GetStdev(ctx context.Context, symbol string, start, end time.Time) (float64, error) {
	if result, ok := pr.stdevs.get(symbol, start, end); ok {
		return result, nil
	}

	return 0, fmt.Errorf("stdev cache miss %s %s to %s", symbol, start.Format(time.DateOnly), end.Format(time.DateOnly))
}

func NewPriceService(
	db *sql.DB,
	adjPriceRepository repository.AdjustedPriceRepository,
	alpacaRepository repository.AlpacaRepository,
	quoteProvider QuoteProvider,
) PriceService {
	return &priceServiceHandler{
		AdjPriceRepository: adjPriceRepository,
		Db:                 db,
		AlpacaRepository:   alpacaRepository,
		QuoteProvider:      quoteProvider,
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

	symbols := []string{}
	// uhh so the getInput technically tells us which date the equation will
	// want to fetch on, but if it's not on a trading day, we're kinda fucked?
	// i think we should just do like 7 days before absMin so we have the price
	// on the requested day...
	// i.e. on day 1 of backtest, do (priceChange(n-1, n)) on a Monday and suddenly we
	// need to fetch price from the like prev Friday
	getInputs := []repository.GetManyInput{}
	for symbol, minMaxValues := range minMaxMap {
		symbols = append(symbols, symbol)
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
		cache[p.Symbol][p.Date.Format(time.DateOnly)] = p.Price.InexactFloat64()
	}
	endNewSpan()

	// can we also fill anything that was asked for in the cache
	_, endNewSpan = newProfile.StartNewSpan("filling price cache gaps")
	fillPriceCacheGaps(inputs, cache)
	endNewSpan()

	if h.AlpacaRepository != nil {
		latestPrices, err := h.AlpacaRepository.GetLatestPricesWithTs(symbols)
		if err != nil {
			return nil, err
		}
		for symbol, price := range latestPrices {
			if len(tradingDays) == 0 || price.Date.Format(time.DateOnly) > tradingDays[len(tradingDays)-1].Format(time.DateOnly) {
				// might wanna set time to 0
				zeroedDate := time.Date(
					price.Date.Year(),
					price.Date.Month(),
					price.Date.Day(),
					0,
					0,
					0,
					0,
					time.UTC,
				)
				tradingDays = append(tradingDays, zeroedDate)
			}
			cache[symbol][price.Date.Format(time.DateOnly)] = price.Price.InexactFloat64()
		}
	}

	_, endNewSpan = newProfile.StartNewSpan("populating stdev cache")
	stdevCache, err := stdevsFromPriceMap(minMaxMap, cache, stdevInputs, tradingDays)
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

func (h priceServiceHandler) IngestPrices(
	ctx context.Context,
	tx *sql.Tx,
	symbol string,
	adjPricesRepository repository.AdjustedPriceRepository,
	start *time.Time,
) error {
	s := time.Date(2000, 1, 0, 0, 0, 0, 0, time.UTC)
	if start != nil {
		s = *start
	}
	now := time.Now().UTC()

	points, err := h.QuoteProvider.GetDailyAdjCloses(ctx, symbol, s, now)
	if err != nil {
		return err
	}

	models := []model.AdjustedPrice{}
	createdAt := time.Now().UTC()
	for _, pt := range points {
		models = append(models, model.AdjustedPrice{
			Symbol:    symbol,
			Date:      pt.Date,
			Price:     pt.Price,
			CreatedAt: createdAt,
		})
	}

	if err := adjPricesRepository.Add(tx, models); err != nil {
		return err
	}

	return nil
}

func (h priceServiceHandler) UpdateUniversePrices(
	ctx context.Context,
	tx *sql.Tx,
	tickerRepository repository.TickerRepository,
	adjPricesRepository repository.AdjustedPriceRepository,
) (int, error) {
	log := logger.FromContext(ctx)

	assets, err := tickerRepository.List()
	if err != nil {
		return 0, err
	}
	if len(assets) == 0 {
		return 0, fmt.Errorf("no assets found in universe")
	}

	assets = append(assets, model.Ticker{
		Symbol: "SPY",
	})

	symbols := []string{}
	for _, a := range assets {
		symbols = append(symbols, a.Symbol)
	}

	log.Infof("found %d assets to update", len(assets))

	h.asyncIngestPrices(ctx, tx, symbols, adjPricesRepository)

	log.Infof("updated %d prices", len(symbols))

	return len(symbols), nil
}

func (h priceServiceHandler) asyncIngestPrices(ctx context.Context, tx *sql.Tx, symbols []string, adjPriceRepository repository.AdjustedPriceRepository) error {
	log := logger.FromContext(ctx)
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
					err := h.IngestPrices(ctx, tx, symbol, adjPriceRepository, nil)
					if err != nil {
						log.Warnf("failed to ingest price for %s: %s\n", symbol, err.Error())
					}
					wg.Done()
				}
			}
		}()
	}

	wg.Wait()

	return nil
}

// i absolutely hate this function but it's the only way to get the latest price using yahoo finance
// i don't even think it gets the latest price - it's just last close
//
// TODO - find a better data provider
func (h priceServiceHandler) GetLatestPrices(ctx context.Context, symbols []string) (map[string]decimal.Decimal, error) {
	log := logger.FromContext(ctx)

	if h.QuoteProvider == nil {
		return nil, fmt.Errorf("no quote provider configured")
	}

	resp, err := h.QuoteProvider.GetLatestQuotes(ctx, symbols)
	out := map[string]decimal.Decimal{}
	if resp != nil {
		for symbol, q := range resp.Quotes {
			out[symbol] = q.Price
		}
		if len(resp.Missing) > 0 {
			log.Warnf("missing %d/%d quotes from %s: %v", len(resp.Missing), len(symbols), resp.Provider, resp.Missing)
		}
	}

	// Preserve the old behavior: partial results are ok, but if we couldn't
	// produce any prices for a non-empty input, return an error.
	if len(out) == 0 && len(symbols) > 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get any prices for %d symbols", len(symbols))
	}

	// If provider returned an error but we have some prices, treat as non-fatal
	// (matches prior per-symbol best-effort behavior).
	return out, nil
}
