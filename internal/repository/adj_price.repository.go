package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"fmt"
	"sync"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
)

type PriceCache map[string]map[time.Time]float64

type TradingDatesCache struct {
	Start time.Time
	End   time.Time
	Days  map[string]struct{}
}

func (h adjustedPriceRepositoryHandler) GetFromPriceCache(symbol string, date time.Time) *float64 {
	pc := h.PriceCache
	h.ReadMutex.RLock()
	if _, ok := pc[symbol]; ok {
		if price, ok := pc[symbol][date]; ok {
			h.ReadMutex.RUnlock()
			return &price
		}
	}
	h.ReadMutex.RUnlock()
	return nil
}

func (h adjustedPriceRepositoryHandler) AddToPriceCache(symbol string, date time.Time, price float64) {
	pc := h.PriceCache
	h.ReadMutex.Lock()
	if _, ok := pc[symbol]; !ok {
		pc[symbol] = map[time.Time]float64{}
	}
	pc[symbol][date] = price
	h.ReadMutex.Unlock()
}

type AdjustedPriceRepository interface {
	Add(*sql.Tx, []model.AdjustedPrice) error
	Get(string, time.Time) (float64, error)
	GetMany([]string, time.Time) (map[string]float64, error)
	List(symbols []string, start, end time.Time) ([]domain.AssetPrice, error)
	ListTradingDays(tx *sql.DB, start, end time.Time) ([]time.Time, error)
	LatestPrices(tx qrm.Queryable, symbols []string) ([]domain.AssetPrice, error)

	// this is weird
	ListFromSet(tx qrm.Queryable, set []ListFromSetInput) ([]domain.AssetPrice, error)
}

type ListFromSetInput struct {
	Symbol string
	Date   time.Time
}

func NewAdjustedPriceRepository(db *sql.DB) AdjustedPriceRepository {
	return &adjustedPriceRepositoryHandler{
		Db:         db,
		PriceCache: make(PriceCache),
		ReadMutex:  &sync.RWMutex{},
	}
}

type adjustedPriceRepositoryHandler struct {
	Db         *sql.DB
	PriceCache PriceCache
	ReadMutex  *sync.RWMutex
	days       []time.Time
}

func (h adjustedPriceRepositoryHandler) Add(tx *sql.Tx, adjPrices []model.AdjustedPrice) error {
	query := table.AdjustedPrice.
		INSERT(table.AdjustedPrice.MutableColumns).
		MODELS(adjPrices).
		ON_CONFLICT(
			table.AdjustedPrice.Symbol, table.AdjustedPrice.Date,
		).DO_UPDATE(
		postgres.SET(
			table.AdjustedPrice.Price.SET(table.AdjustedPrice.EXCLUDED.Price),
		),
	)

	_, err := query.Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to add adjusted prices to db: %w", err)
	}

	return nil
}

func (h adjustedPriceRepositoryHandler) Get(symbol string, date time.Time) (float64, error) {
	if pc := h.GetFromPriceCache(symbol, date); pc != nil {
		return *pc, nil
	}

	// fmt.Println("cache miss", symbol, date)

	minDate := postgres.DateT(date.AddDate(0, 0, -3))
	maxDate := postgres.DateT(date)
	// use range so we can do t-3 for weekends or holidays
	query := table.AdjustedPrice.
		SELECT(table.AdjustedPrice.AllColumns).
		WHERE(
			postgres.AND(
				table.AdjustedPrice.Symbol.EQ(postgres.String(symbol)),
				table.AdjustedPrice.Date.BETWEEN(minDate, maxDate),
			),
		).
		ORDER_BY(table.AdjustedPrice.Date.DESC()).
		LIMIT(1)

	results := []model.AdjustedPrice{}
	err := query.Query(h.Db, &results)
	if err != nil {
		return 0, fmt.Errorf("failed to query price for %s on %v: %w", symbol, date, err)
	}
	if len(results) == 0 {
		return 0, fmt.Errorf("no results for %s on %v", symbol, date)
	}
	result := results[0]

	h.AddToPriceCache(symbol, date, result.Price)
	return result.Price, nil
}

// assumes input date is a trading day
func (h adjustedPriceRepositoryHandler) GetMany(symbols []string, date time.Time) (map[string]float64, error) {
	cachedResults := map[string]float64{}
	symbolSet := map[string]bool{}
	postgresStr := []postgres.Expression{}

	for _, s := range symbols {
		if _, ok := symbolSet[s]; !ok {
			cachedPrice := h.GetFromPriceCache(s, date)
			if cachedPrice == nil {
				postgresStr = append(postgresStr, postgres.String(s))
			} else {
				cachedResults[s] = *cachedPrice
			}
		}
		symbolSet[s] = false

	}

	res := []model.AdjustedPrice{}
	if len(postgresStr) > 0 {
		query := table.AdjustedPrice.
			SELECT(table.AdjustedPrice.AllColumns).
			WHERE(
				postgres.AND(
					table.AdjustedPrice.Symbol.IN(postgresStr...),
					table.AdjustedPrice.Date.EQ(postgres.DateT(date)),
				),
			).
			ORDER_BY(table.AdjustedPrice.Date.DESC())

		err := query.Query(h.Db, &res)
		if err != nil {
			return nil, fmt.Errorf("failed to query prices for %d symbols on date %v: %w", len(postgresStr), date, err)
		}
	}

	out := map[string]float64{}
	for _, r := range res {
		out[r.Symbol] = r.Price
	}

	for symbol, cachedPrice := range cachedResults {
		out[symbol] = cachedPrice
	}

	return out, nil
}

func (h adjustedPriceRepositoryHandler) List(symbols []string, start, end time.Time) ([]domain.AssetPrice, error) {
	minDate := postgres.DateT(start)
	maxDate := postgres.DateT(end)
	symbolsFilter := []postgres.Expression{}
	for _, s := range symbols {
		symbolsFilter = append(symbolsFilter, postgres.String(s))
	}
	// use range so we can do t-3 for weekends or holidays
	query := table.AdjustedPrice.
		SELECT(table.AdjustedPrice.AllColumns).
		WHERE(
			postgres.AND(
				table.AdjustedPrice.Symbol.IN(symbolsFilter...),
				table.AdjustedPrice.Date.BETWEEN(minDate, maxDate),
			),
		).
		ORDER_BY(table.AdjustedPrice.Date.ASC())

	result := []model.AdjustedPrice{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list prices for %v: %w", symbols, err)
	}

	out := []domain.AssetPrice{}
	for _, p := range result {
		out = append(out, domain.AssetPrice{
			Symbol: p.Symbol,
			Date:   p.Date,
			Price:  p.Price,
		})
		h.AddToPriceCache(p.Symbol, p.Date, p.Price)
	}

	return out, nil
}

func (h *adjustedPriceRepositoryHandler) ListTradingDays(tx *sql.DB, start, end time.Time) ([]time.Time, error) {
	minDate := postgres.DateT(start)
	maxDate := postgres.DateT(end)
	// use range so we can do t-3 for weekends or holidays
	query := table.AdjustedPrice.
		SELECT(table.AdjustedPrice.Date).
		WHERE(
			postgres.AND(
				table.AdjustedPrice.Date.BETWEEN(minDate, maxDate),
			),
		).
		GROUP_BY(table.AdjustedPrice.Date).
		HAVING(postgres.COUNT(postgres.String("*")).GT(postgres.Int(3))). // TODO - make this better
		ORDER_BY(table.AdjustedPrice.Date.ASC())

	q, args := query.Sql()

	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list trading days: %w", err)
	}

	out := []time.Time{}
	for rows.Next() {
		var d time.Time
		err := rows.Scan(&d)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		out = append(out, d)
	}

	h.days = out
	return out, nil
}

func (h adjustedPriceRepositoryHandler) LatestPrices(tx qrm.Queryable, symbols []string) ([]domain.AssetPrice, error) {
	out := []domain.AssetPrice{}
	for _, s := range symbols {
		query := table.AdjustedPrice.SELECT(table.AdjustedPrice.AllColumns).
			WHERE(table.AdjustedPrice.Symbol.EQ(postgres.String(s))).
			ORDER_BY(table.AdjustedPrice.Date.DESC()).
			LIMIT(1)
		model := model.AdjustedPrice{}
		err := query.Query(tx, &model)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest price for %s: %w", s, err)
		}
		out = append(out, domain.AssetPrice{
			Symbol: model.Symbol,
			Date:   model.Date,
			Price:  model.Price,
		})
	}

	return out, nil
}

func (h adjustedPriceRepositoryHandler) ListFromSet(tx qrm.Queryable, set []ListFromSetInput) ([]domain.AssetPrice, error) {
	expressions := []postgres.BoolExpression{}
	symbolRanges := map[string]*struct {
		min time.Time
		max time.Time
	}{}
	for _, s := range set {
		if _, ok := symbolRanges[s.Symbol]; !ok {
			symbolRanges[s.Symbol] = &struct {
				min time.Time
				max time.Time
			}{
				min: s.Date,
				max: s.Date,
			}
		}
		if s.Date.After(symbolRanges[s.Symbol].max) {
			symbolRanges[s.Symbol].max = s.Date
		}
		if s.Date.Before(symbolRanges[s.Symbol].min) {
			symbolRanges[s.Symbol].min = s.Date
		}
	}

	for symbol, rng := range symbolRanges {
		expressions = append(
			expressions,
			postgres.AND(
				table.AdjustedPrice.Symbol.EQ(postgres.String(symbol)),
				table.AdjustedPrice.Date.LT_EQ(postgres.DateT(rng.max)),
				table.AdjustedPrice.Date.GT_EQ(postgres.DateT(rng.min)),
			),
		)
	}

	if len(expressions) == 0 {
		return nil, fmt.Errorf("no prices to include")
	}

	query := table.AdjustedPrice.SELECT(table.AdjustedPrice.AllColumns).
		WHERE(postgres.OR(
			expressions...,
		))

	results := []model.AdjustedPrice{}
	err := query.Query(tx, &results)
	if err != nil {
		return nil, err
	}

	out := []domain.AssetPrice{}
	for _, price := range results {
		out = append(out, domain.AssetPrice{
			Symbol: price.Symbol,
			Price:  price.Price,
			Date:   price.Date,
		})
	}

	return out, nil
}
