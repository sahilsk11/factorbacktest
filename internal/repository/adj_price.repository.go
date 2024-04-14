package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	. "factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"fmt"
	"sync"
	"time"

	. "github.com/go-jet/jet/v2/postgres"
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
	Get(*sql.Tx, string, time.Time) (float64, error)
	GetMany(*sql.Tx, []string, time.Time) (map[string]float64, error)
	List(db qrm.Queryable, symbols []string, start, end time.Time) ([]domain.AssetPrice, error)
	ListTradingDays(tx *sql.Tx, start, end time.Time) ([]time.Time, error)
}

func NewAdjustedPriceRepository() AdjustedPriceRepository {
	return &adjustedPriceRepositoryHandler{
		PriceCache: make(PriceCache),
		ReadMutex:  &sync.RWMutex{},
	}
}

type adjustedPriceRepositoryHandler struct {
	PriceCache PriceCache
	ReadMutex  *sync.RWMutex
	days       []time.Time
}

func (h adjustedPriceRepositoryHandler) Add(tx *sql.Tx, adjPrices []model.AdjustedPrice) error {
	query := AdjustedPrice.
		INSERT(AdjustedPrice.MutableColumns).
		MODELS(adjPrices).
		ON_CONFLICT(
			AdjustedPrice.Symbol, AdjustedPrice.Date,
		).DO_UPDATE(
		SET(
			AdjustedPrice.Price.SET(AdjustedPrice.EXCLUDED.Price),
		),
	)

	_, err := query.Exec(tx)
	if err != nil {
		return fmt.Errorf("failed to add adjusted prices to db: %w", err)
	}

	return nil
}

func (h adjustedPriceRepositoryHandler) Get(tx *sql.Tx, symbol string, date time.Time) (float64, error) {

	if pc := h.GetFromPriceCache(symbol, date); pc != nil {
		return *pc, nil
	}

	fmt.Println("cache miss", symbol, date)

	minDate := DateT(date.AddDate(0, 0, -3))
	maxDate := DateT(date)
	// use range so we can do t-3 for weekends or holidays
	query := AdjustedPrice.
		SELECT(AdjustedPrice.AllColumns).
		WHERE(
			AND(
				AdjustedPrice.Symbol.EQ(String(symbol)),
				AdjustedPrice.Date.BETWEEN(minDate, maxDate),
			),
		).
		ORDER_BY(AdjustedPrice.Date.DESC()).
		LIMIT(1)

	result := model.AdjustedPrice{}
	err := query.Query(tx, &result)
	if err != nil {
		return 0, fmt.Errorf("failed to query price for %s on %v: %w", symbol, date, err)
	}

	h.AddToPriceCache(symbol, date, result.Price)
	return result.Price, nil
}

// assumes input date is a trading day
func (h adjustedPriceRepositoryHandler) GetMany(tx *sql.Tx, symbols []string, date time.Time) (map[string]float64, error) {
	cachedResults := map[string]float64{}
	symbolSet := map[string]bool{}
	postgresStr := []Expression{}

	for _, s := range symbols {
		if _, ok := symbolSet[s]; !ok {
			cachedPrice := h.GetFromPriceCache(s, date)
			if cachedPrice == nil {
				postgresStr = append(postgresStr, String(s))
			} else {
				cachedResults[s] = *cachedPrice
			}
		}
		symbolSet[s] = false

	}

	res := []model.AdjustedPrice{}
	if len(postgresStr) > 0 {
		query := AdjustedPrice.
			SELECT(AdjustedPrice.AllColumns).
			WHERE(
				AND(
					AdjustedPrice.Symbol.IN(postgresStr...),
					AdjustedPrice.Date.EQ(DateT(date)),
				),
			).
			ORDER_BY(AdjustedPrice.Date.DESC())

		err := query.Query(tx, &res)
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

func (h adjustedPriceRepositoryHandler) List(db qrm.Queryable, symbols []string, start, end time.Time) ([]domain.AssetPrice, error) {
	minDate := DateT(start)
	maxDate := DateT(end)
	symbolsFilter := []Expression{}
	for _, s := range symbols {
		symbolsFilter = append(symbolsFilter, String(s))
	}
	// use range so we can do t-3 for weekends or holidays
	query := AdjustedPrice.
		SELECT(AdjustedPrice.AllColumns).
		WHERE(
			AND(
				AdjustedPrice.Symbol.IN(symbolsFilter...),
				AdjustedPrice.Date.BETWEEN(minDate, maxDate),
			),
		).
		ORDER_BY(AdjustedPrice.Date.ASC())

	result := []model.AdjustedPrice{}
	err := query.Query(db, &result)
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

func (h *adjustedPriceRepositoryHandler) ListTradingDays(tx *sql.Tx, start, end time.Time) ([]time.Time, error) {
	minDate := DateT(start)
	maxDate := DateT(end)
	// use range so we can do t-3 for weekends or holidays
	query := AdjustedPrice.
		SELECT(AdjustedPrice.Date).
		WHERE(
			AND(
				AdjustedPrice.Date.BETWEEN(minDate, maxDate),
			),
		).
		GROUP_BY(AdjustedPrice.Date).
		HAVING(COUNT(String("*")).GT(Int(10))).
		ORDER_BY(AdjustedPrice.Date.ASC())

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
