package repository

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"fmt"
	"sync"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/shopspring/decimal"
)

type priceCache map[string]map[time.Time]decimal.Decimal

type TradingDatesCache struct {
	Start time.Time
	End   time.Time
	Days  map[string]struct{}
}

func (h adjustedPriceRepositoryHandler) GetFromPriceCache(symbol string, date time.Time) *decimal.Decimal {
	pc := h.priceCache
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

func (h adjustedPriceRepositoryHandler) AddToPriceCache(symbol string, date time.Time, price decimal.Decimal) {
	pc := h.priceCache
	h.ReadMutex.Lock()
	if _, ok := pc[symbol]; !ok {
		pc[symbol] = map[time.Time]decimal.Decimal{}
	}
	pc[symbol][date] = price
	h.ReadMutex.Unlock()
}

type AdjustedPriceRepository interface {
	Add(*sql.Tx, []model.AdjustedPrice) error
	Get(string, time.Time) (decimal.Decimal, error)
	GetManyOnDay([]string, time.Time) (map[string]decimal.Decimal, error)
	List(symbols []string, start, end time.Time) ([]domain.AssetPrice, error)
	ListTradingDays(start, end time.Time) ([]time.Time, error)
	LatestTradingDay() (*time.Time, error)
	LatestPrices(symbols []string) ([]domain.AssetPrice, error)

	// this is weird
	GetMany([]GetManyInput) ([]domain.AssetPrice, error)
}

func NewAdjustedPriceRepository(db *sql.DB) AdjustedPriceRepository {
	return &adjustedPriceRepositoryHandler{
		Db:         db,
		priceCache: make(priceCache),
		ReadMutex:  &sync.RWMutex{},
	}
}

type adjustedPriceRepositoryHandler struct {
	Db         *sql.DB
	priceCache priceCache
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

	var db qrm.Executable = h.Db
	if tx != nil {
		db = tx
	}

	_, err := query.Exec(db)
	if err != nil {
		return fmt.Errorf("failed to add adjusted prices to db: %w", err)
	}

	return nil
}

func (h adjustedPriceRepositoryHandler) Get(symbol string, date time.Time) (decimal.Decimal, error) {
	if pc := h.GetFromPriceCache(symbol, date); pc != nil {
		return *pc, nil
	}

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
		return decimal.Zero, fmt.Errorf("failed to query price for %s on %v: %w", symbol, date, err)
	}
	if len(results) == 0 {
		return decimal.Zero, fmt.Errorf("no results for %s on %v", symbol, date)
	}
	result := results[0]

	h.AddToPriceCache(symbol, date, result.Price)
	return result.Price, nil
}

// assumes input date is a trading day
func (h adjustedPriceRepositoryHandler) GetManyOnDay(symbols []string, date time.Time) (map[string]decimal.Decimal, error) {
	cachedResults := map[string]decimal.Decimal{}
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

	out := map[string]decimal.Decimal{}
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

func (h adjustedPriceRepositoryHandler) LatestTradingDay() (*time.Time, error) {
	query := table.AdjustedPrice.
		SELECT(table.AdjustedPrice.Date).
		GROUP_BY(table.AdjustedPrice.Date).
		HAVING(postgres.COUNT(postgres.String("*")).GT(postgres.Int(3))). // TODO - make this better
		ORDER_BY(table.AdjustedPrice.Date.DESC()).
		LIMIT(1)

	q, args := query.Sql()

	rows, err := h.Db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list trading days: %w", err)
	}

	for rows.Next() {
		var d time.Time
		err := rows.Scan(&d)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		return &d, nil
	}

	return nil, fmt.Errorf("failed to get trading day")
}

func (h *adjustedPriceRepositoryHandler) ListTradingDays(start, end time.Time) ([]time.Time, error) {
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

	rows, err := h.Db.Query(q, args...)
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

func (h adjustedPriceRepositoryHandler) LatestPrices(symbols []string) ([]domain.AssetPrice, error) {
	out := []domain.AssetPrice{}
	for _, s := range symbols {
		query := table.AdjustedPrice.SELECT(table.AdjustedPrice.AllColumns).
			WHERE(table.AdjustedPrice.Symbol.EQ(postgres.String(s))).
			ORDER_BY(table.AdjustedPrice.Date.DESC()).
			LIMIT(1)
		model := model.AdjustedPrice{}
		err := query.Query(h.Db, &model)
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

type GetManyInput struct {
	Symbol  string
	MinDate time.Time
	MaxDate time.Time
}

func (h adjustedPriceRepositoryHandler) GetMany(inputs []GetManyInput) ([]domain.AssetPrice, error) {
	ctx := context.Background()

	// TODO:
	// is it really better to do this instead
	// of passing individual dates?
	//
	// also this function looks like it's async,
	// but it's not! turns out making async/batch
	// made the latency 5x. clearly this function
	// needs additional instrumentation

	expressions := []postgres.BoolExpression{}

	type workResult struct {
		models []model.AdjustedPrice
		err    error
	}

	for _, in := range inputs {
		expressions = append(
			expressions,
			postgres.AND(
				table.AdjustedPrice.Symbol.EQ(postgres.String(in.Symbol)),
				table.AdjustedPrice.Date.LT_EQ(postgres.DateT(in.MaxDate)),
				table.AdjustedPrice.Date.GT_EQ(postgres.DateT(in.MinDate)),
			),
		)
	}
	if len(expressions) == 0 {
		return nil, fmt.Errorf("no prices to include")
	}

	batchSize := 10000
	inputCh := make(chan []postgres.BoolExpression, len(inputs))
	resultCh := make(chan workResult, len(inputs))

	numGoroutines := 1
	var wg sync.WaitGroup
	for start := 0; start < len(expressions); start += batchSize {
		end := start + batchSize
		if end > len(expressions) {
			end = len(expressions)
		}
		expr := expressions[start:end]

		wg.Add(1)
		inputCh <- expr
	}
	close(inputCh)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case input, ok := <-inputCh:
					if !ok {
						return
					}

					query := table.AdjustedPrice.SELECT(table.AdjustedPrice.AllColumns).
						WHERE(postgres.OR(input...))

					results := []model.AdjustedPrice{}
					err := query.Query(h.Db, &results)
					resultCh <- workResult{
						models: results,
						err:    err,
					}

					wg.Done()
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	out := []domain.AssetPrice{}
	for result := range resultCh {
		if result.err != nil {
			return nil, result.err
		}
		for _, m := range result.models {
			out = append(out, domain.AssetPrice{
				Symbol: m.Symbol,
				Price:  m.Price,
				Date:   m.Date,
			})
		}
	}

	return out, nil
}
