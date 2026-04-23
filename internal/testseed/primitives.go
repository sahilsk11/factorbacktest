package testseed

import (
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
	"math"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/gocarina/gocsv"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TickerOpts struct {
	Symbol string
	Name   string
}

func CreateTicker(db *sql.DB, opts TickerOpts) model.Ticker {
	out := model.Ticker{}
	err := table.Ticker.
		INSERT(table.Ticker.MutableColumns).
		MODEL(model.Ticker{
			Symbol: opts.Symbol,
			Name:   opts.Name,
		}).
		RETURNING(table.Ticker.AllColumns).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("CreateTicker: %w", err))
	}
	return out
}

type AssetUniverseOpts struct {
	Name        string
	DisplayName string
}

func CreateAssetUniverse(db *sql.DB, opts AssetUniverseOpts) model.AssetUniverse {
	displayName := opts.DisplayName
	if displayName == "" {
		displayName = opts.Name
	}
	out := model.AssetUniverse{}
	err := table.AssetUniverse.
		INSERT(table.AssetUniverse.MutableColumns).
		MODEL(model.AssetUniverse{
			AssetUniverseName: opts.Name,
			DisplayName:       displayName,
		}).
		RETURNING(table.AssetUniverse.AllColumns).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("CreateAssetUniverse: %w", err))
	}
	return out
}

func CreateAssetUniverseTicker(db *sql.DB, universeID, tickerID uuid.UUID) model.AssetUniverseTicker {
	out := model.AssetUniverseTicker{}
	err := table.AssetUniverseTicker.
		INSERT(table.AssetUniverseTicker.MutableColumns).
		MODEL(model.AssetUniverseTicker{
			AssetUniverseID: universeID,
			TickerID:        tickerID,
		}).
		RETURNING(table.AssetUniverseTicker.AllColumns).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("CreateAssetUniverseTicker: %w", err))
	}
	return out
}

type UserAccountOpts struct {
	Email     string
	FirstName string
	LastName  string
	Provider  model.UserAccountProviderType
}

func CreateUserAccount(db *sql.DB, opts UserAccountOpts) model.UserAccount {
	email := opts.Email
	if email == "" {
		email = "test@gmail.com"
	}
	firstName := opts.FirstName
	if firstName == "" {
		firstName = "Test"
	}
	lastName := opts.LastName
	if lastName == "" {
		lastName = "User"
	}
	provider := opts.Provider
	if provider == "" {
		provider = model.UserAccountProviderType_Manual
	}
	now := time.Now()
	out := model.UserAccount{}
	err := table.UserAccount.
		INSERT(table.UserAccount.MutableColumns).
		MODEL(model.UserAccount{
			Email:     &email,
			FirstName: &firstName,
			LastName:  &lastName,
			Provider:  provider,
			CreatedAt: now,
			UpdatedAt: now,
		}).
		RETURNING(table.UserAccount.AllColumns).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("CreateUserAccount: %w", err))
	}
	return out
}

type StrategyOpts struct {
	Name              string
	FactorExpression  string
	AssetUniverse     string
	RebalanceInterval string
	NumAssets         int32
	UserAccountID     uuid.UUID
	Published         bool
}

func CreateStrategy(db *sql.DB, opts StrategyOpts) model.Strategy {
	userID := opts.UserAccountID
	now := time.Now()
	out := model.Strategy{}
	err := table.Strategy.
		INSERT(table.Strategy.MutableColumns).
		MODEL(model.Strategy{
			StrategyName:      opts.Name,
			FactorExpression:  opts.FactorExpression,
			RebalanceInterval: opts.RebalanceInterval,
			NumAssets:         opts.NumAssets,
			AssetUniverse:     opts.AssetUniverse,
			UserAccountID:     &userID,
			CreatedAt:         now,
			ModifiedAt:        now,
			Published:         opts.Published,
		}).
		RETURNING(table.Strategy.AllColumns).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("CreateStrategy: %w", err))
	}
	return out
}

type InvestmentOpts struct {
	StrategyID    uuid.UUID
	UserAccountID uuid.UUID
	AmountDollars int32
	StartDate     time.Time
}

func CreateInvestment(db *sql.DB, opts InvestmentOpts) model.Investment {
	startDate := opts.StartDate
	if startDate.IsZero() {
		startDate = time.Now()
	}
	now := time.Now()
	out := model.Investment{}
	err := table.Investment.
		INSERT(table.Investment.MutableColumns).
		MODEL(model.Investment{
			StrategyID:    opts.StrategyID,
			UserAccountID: opts.UserAccountID,
			AmountDollars: opts.AmountDollars,
			StartDate:     startDate,
			CreatedAt:     now,
			ModifiedAt:    now,
		}).
		RETURNING(table.Investment.AllColumns).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("CreateInvestment: %w", err))
	}
	return out
}

func CreateInvestmentHoldingsVersion(db *sql.DB, investmentID uuid.UUID) model.InvestmentHoldingsVersion {
	out := model.InvestmentHoldingsVersion{}
	err := table.InvestmentHoldingsVersion.
		INSERT(table.InvestmentHoldingsVersion.MutableColumns).
		MODEL(model.InvestmentHoldingsVersion{
			InvestmentID:    investmentID,
			CreatedAt:       time.Now(),
			RebalancerRunID: nil,
		}).
		RETURNING(table.InvestmentHoldingsVersion.AllColumns).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("CreateInvestmentHoldingsVersion: %w", err))
	}
	return out
}

type InvestmentHoldingOpts struct {
	VersionID uuid.UUID
	TickerID  uuid.UUID
	Quantity  decimal.Decimal
}

func CreateInvestmentHolding(db *sql.DB, opts InvestmentHoldingOpts) model.InvestmentHoldings {
	out := model.InvestmentHoldings{}
	err := table.InvestmentHoldings.
		INSERT(table.InvestmentHoldings.MutableColumns).
		MODEL(model.InvestmentHoldings{
			TickerID:                    opts.TickerID,
			Quantity:                    opts.Quantity,
			CreatedAt:                   time.Now(),
			InvestmentHoldingsVersionID: opts.VersionID,
		}).
		RETURNING(table.InvestmentHoldings.AllColumns).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("CreateInvestmentHolding: %w", err))
	}
	return out
}

func LookupTickerBySymbol(db *sql.DB, symbol string) model.Ticker {
	out := model.Ticker{}
	err := table.Ticker.
		SELECT(table.Ticker.AllColumns).
		WHERE(table.Ticker.Symbol.EQ(postgres.String(symbol))).
		Query(db, &out)
	if err != nil {
		panic(fmt.Errorf("LookupTickerBySymbol(%q): %w", symbol, err))
	}
	return out
}

//go:embed data/prices_2020.csv
var prices2020CSV []byte

func InsertPrices2020(db *sql.DB) {
	type Row struct {
		Date   string          `csv:"date"`
		Symbol string          `csv:"symbol"`
		Price  decimal.Decimal `csv:"price"`
	}
	rows := []Row{}
	if err := gocsv.Unmarshal(bytes.NewReader(prices2020CSV), &rows); err != nil {
		panic(fmt.Errorf("InsertPrices2020: parse csv: %w", err))
	}

	models := make([]model.AdjustedPrice, 0, len(rows))
	for _, row := range rows {
		date, err := time.Parse(time.DateOnly, row.Date)
		if err != nil {
			panic(fmt.Errorf("InsertPrices2020: parse date %q: %w", row.Date, err))
		}
		models = append(models, model.AdjustedPrice{
			Date:   date,
			Symbol: row.Symbol,
			Price:  row.Price,
		})
	}

	if _, err := table.AdjustedPrice.
		INSERT(table.AdjustedPrice.MutableColumns).
		MODELS(models).
		Exec(db); err != nil {
		panic(fmt.Errorf("InsertPrices2020: insert: %w", err))
	}
}

type SyntheticPricesOpts struct {
	Symbols []string
	Start   time.Time
	End     time.Time
}

func InsertSyntheticPrices(db *sql.DB, opts SyntheticPricesOpts) {
	if opts.Start.IsZero() || opts.End.IsZero() {
		panic(fmt.Errorf("InsertSyntheticPrices: Start and End must be set"))
	}
	if opts.End.Before(opts.Start) {
		panic(fmt.Errorf("InsertSyntheticPrices: End before Start"))
	}

	start := time.Date(opts.Start.Year(), opts.Start.Month(), opts.Start.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(opts.End.Year(), opts.End.Month(), opts.End.Day(), 0, 0, 0, 0, time.UTC)

	// Per-symbol deterministic price series: base price + linear drift + sinusoidal oscillation.
	// Different amplitude/period/phase per symbol so relative momentum rankings vary over time.
	bases := []float64{100, 150, 200, 300, 400, 500, 600, 700}
	drifts := []float64{0.03, 0.05, 0.02, 0.04, 0.035, 0.045, 0.025, 0.05}
	amps := []float64{8, 15, 12, 20, 10, 18, 14, 22}
	periods := []float64{45, 60, 30, 90, 50, 75, 40, 55}
	phases := []float64{0, 1.3, 2.7, 0.9, 2.1, 0.4, 1.8, 3.0}

	const chunkSize = 5000

	for sIdx, symbol := range opts.Symbols {
		base := bases[sIdx%len(bases)]
		drift := drifts[sIdx%len(drifts)]
		amp := amps[sIdx%len(amps)]
		period := periods[sIdx%len(periods)]
		phase := phases[sIdx%len(phases)]

		batch := make([]model.AdjustedPrice, 0, chunkSize)
		dayIdx := 0
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			f := float64(dayIdx)
			price := base + drift*f + amp*math.Sin(2*math.Pi*f/period+phase)
			if price < 1 {
				price = 1
			}
			batch = append(batch, model.AdjustedPrice{
				Date:   d,
				Symbol: symbol,
				Price:  decimal.NewFromFloat(price).Round(4),
			})
			if len(batch) >= chunkSize {
				flushAdjustedPrices(db, batch)
				batch = batch[:0]
			}
			dayIdx++
		}
		if len(batch) > 0 {
			flushAdjustedPrices(db, batch)
		}
	}
}

func flushAdjustedPrices(db *sql.DB, models []model.AdjustedPrice) {
	if _, err := table.AdjustedPrice.
		INSERT(table.AdjustedPrice.MutableColumns).
		MODELS(models).
		Exec(db); err != nil {
		panic(fmt.Errorf("InsertSyntheticPrices: insert: %w", err))
	}
}
