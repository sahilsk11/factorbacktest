package seeds

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/gocarina/gocsv"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Hammer is the main seed engine. commit=true commits after each phase,
// commit=false (default) leaves the transaction open for rollback (integration test mode).
type Hammer struct {
	Tx     *sql.Tx
	Commit bool
	// CsvPath is the path to the sample_prices_2020.csv file.
	// Defaults to "sample_prices_2020.csv" for integration tests running
	// from the package directory. Use "tools/seeds/sample_prices_2020.csv"
	// when running from the repo root (e.g., cmd/seed-e2e).
	CsvPath string
}

// NewHammer creates a new Hammer with the given transaction.
// Default Commit=false for rollback in integration tests.
// Default CsvPath="sample_prices_2020.csv".
func NewHammer(tx *sql.Tx) *Hammer {
	return &Hammer{Tx: tx, Commit: false, CsvPath: "sample_prices_2020.csv"}
}

// SeedUniverse inserts AAPL, GOOG, META tickers + :CASH + SPY_TOP_80 universe + ticker links.
func (h *Hammer) SeedUniverse() error {
	cashTickerID := uuid.New()

	modelsToInsert := []model.Ticker{
		{
			Symbol: "AAPL",
			Name:   "Apple",
		},
		{
			Symbol: "GOOG",
			Name:   "Google",
		},
		{
			Symbol: "META",
			Name:   "Meta",
		},
	}
	query := table.Ticker.INSERT(table.Ticker.MutableColumns).MODELS(modelsToInsert).RETURNING(table.Ticker.AllColumns)
	insertedTickers := []model.Ticker{}
	err := query.Query(h.Tx, &insertedTickers)
	if err != nil {
		return fmt.Errorf("failed to insert tickers: %w", err)
	}

	_, err = table.Ticker.INSERT(table.Ticker.AllColumns).MODEL(model.Ticker{
		Symbol:   ":CASH",
		Name:     "cash",
		TickerID: cashTickerID,
	}).Exec(h.Tx)
	if err != nil {
		return err
	}

	query = table.AssetUniverse.INSERT(table.AssetUniverse.MutableColumns).MODEL(model.AssetUniverse{
		AssetUniverseName: "SPY_TOP_80",
	}).RETURNING(table.AssetUniverse.AllColumns)

	universe := model.AssetUniverse{}
	err = query.Query(h.Tx, &universe)
	if err != nil {
		return fmt.Errorf("failed to insert universe: %w", err)
	}

	tickerModels := []model.AssetUniverseTicker{}
	for _, m := range insertedTickers {
		tickerModels = append(tickerModels, model.AssetUniverseTicker{
			TickerID:        m.TickerID,
			AssetUniverseID: universe.AssetUniverseID,
		})
	}

	query = table.AssetUniverseTicker.
		INSERT(table.AssetUniverseTicker.MutableColumns).
		MODELS(tickerModels)

	_, err = query.Exec(h.Tx)
	if err != nil {
		return fmt.Errorf("failed to insert asset universe tickers: %w", err)
	}

	return h.commitIf()
}

// SeedPrices reads sample_prices_2020.csv and bulk-inserts adjusted_price rows.
// Uses h.CsvPath for the CSV file location.
func (h *Hammer) SeedPrices() error {
	f, err := os.Open(h.CsvPath)
	if err != nil {
		return err
	}
	defer f.Close()

	type Row struct {
		Date   string          `csv:"date"`
		Symbol string          `csv:"symbol"`
		Price  decimal.Decimal `csv:"price"`
	}
	rows := []Row{}
	gocsv.UnmarshalFile(f, &rows)

	models := []model.AdjustedPrice{}
	for _, row := range rows {
		date, err := time.Parse(time.DateOnly, row.Date)
		if err != nil {
			return err
		}
		models = append(models, model.AdjustedPrice{
			Date:   date,
			Symbol: row.Symbol,
			Price:  row.Price,
		})
	}

	query := table.AdjustedPrice.INSERT(table.AdjustedPrice.MutableColumns).MODELS(models)
	_, err = query.Exec(h.Tx)
	if err != nil {
		return err
	}

	return h.commitIf()
}

// SeedPublishedStrategy inserts one published strategy + strategy_run with pre-computed stats.
// Returns the strategyID and runID so callers can use them.
func (h *Hammer) SeedPublishedStrategy(userID string) (strategyID, runID uuid.UUID, err error) {
	strategyID = uuid.New()
	runID = uuid.New()
	now := time.Now()

	uidPtr := (*uuid.UUID)(nil)
	if userID != "" {
		uid, parseErr := uuid.Parse(userID)
		if parseErr == nil {
			uidPtr = &uid
		}
	}

	strategy := model.Strategy{
		StrategyID:        strategyID,
		StrategyName:      "7_day_momentum_e2e",
		FactorExpression:  "pricePercentChange(\n  nDaysAgo(7),\n  currentDate\n)",
		RebalanceInterval:  "weekly",
		NumAssets:          3,
		AssetUniverse:     "SPY_TOP_80",
		UserAccountID:     uidPtr,
		CreatedAt:          now,
		ModifiedAt:         now,
		Published:          true,
		Saved:              true,
	}

	query := table.Strategy.INSERT(table.Strategy.MutableColumns).MODEL(strategy)
	_, err = query.Exec(h.Tx)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("failed to insert strategy: %w", err)
	}

	sharpe := 1.27
	annReturn := 0.336989043
	stdev := 0.19

	startDate, _ := time.Parse(time.DateOnly, "2020-01-10")
	endDate, _ := time.Parse(time.DateOnly, "2020-12-31")

	strategyRun := model.StrategyRun{
		StrategyRunID:     runID,
		StrategyID:        strategyID,
		StartDate:         startDate,
		EndDate:           endDate,
		SharpeRatio:       &sharpe,
		AnnualizedReturn:  &annReturn,
		AnnualuzedStdev:    &stdev,
		CreatedAt:          now,
	}

	query = table.StrategyRun.INSERT(table.StrategyRun.MutableColumns).MODEL(strategyRun)
	_, err = query.Exec(h.Tx)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("failed to insert strategy run: %w", err)
	}

	return strategyID, runID, h.commitIf()
}

// SeedAll is a convenience that runs SeedUniverse, SeedPrices, SeedPublishedStrategy in order.
// Commits only if h.Commit=true.
func (h *Hammer) SeedAll(userID string) error {
	if err := h.SeedUniverse(); err != nil {
		return err
	}
	if err := h.SeedPrices(); err != nil {
		return err
	}
	_, _, err := h.SeedPublishedStrategy(userID)
	return err
}

// commitIf commits the transaction if Hammer.Commit=true, otherwise does nothing.
func (h *Hammer) commitIf() error {
	if h.Commit {
		return h.Tx.Commit()
	}
	return nil
}
