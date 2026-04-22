package testseed

import (
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
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
