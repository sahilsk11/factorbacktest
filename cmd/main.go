package main

import (
	"alpha/api"
	"alpha/internal"
	"alpha/internal/app"
	"alpha/internal/domain"
	"alpha/internal/repository"
	"alpha/pkg/datajockey"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
)

func New() (*sql.DB, error) {
	connStr := "postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return dbConn, nil
}

func NewTx() (*sql.Tx, error) {
	dbConn, err := New()
	if err != nil {
		return nil, err
	}

	return dbConn.Begin()
	// return roTx(dbConn)
}

func roTx(dbConn *sql.DB) (*sql.Tx, error) {
	ctx := context.Background()
	return dbConn.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
			ReadOnly:  true,
		},
	)
}

func main() {
	// ingestFundamentals("AAPL")
	ap()
}

func ap() {
	dbConn, err := New()
	if err != nil {
		log.Fatal(err)
	}
	apiHandler := api.ApiHandler{
		BenchmarkHandler: internal.BenchmarkHandler{
			PriceRepository: repository.AdjustedPriceRepositoryHandler{},
		},
		BacktestHandler: app.BacktestHandler{
			PriceRepository: repository.AdjustedPriceRepositoryHandler{},
			FactorMetricsHandler: internal.FactorMetricsHandler{
				AdjustedPriceRepository:     repository.AdjustedPriceRepositoryHandler{},
				AssetFundamentalsRepository: repository.AssetFundamentalsRepositoryHandler{},
			},
			UniverseRepository: repository.UniverseRepositoryHandler{},
		},
		Db: dbConn,
	}
	err = apiHandler.StartApi(3009)
	if err != nil {
		log.Fatal(err)
	}
}

func backtest(tx *sql.Tx) {

	exp := `
	(
		(
			pricePercentChange(
				addDate(currentDate, 0, 0, -7),
				currentDate
			) * 0.5 +
			pricePercentChange(
				addDate(currentDate, 0, -1, 0),
				currentDate
			) * 0.3 +
			pricePercentChange(
				addDate(currentDate, 0, -6, 0),
				currentDate
			) * 0.2
		) / 3
	) / stdev(addDate(currentDate, -5, 0, 0),currentDate)
	
	`
	u := repository.UniverseRepositoryHandler{}
	assets, err := u.List(tx)
	if err != nil {
		log.Fatal(err)
	}
	startPortfolio := domain.Portfolio{
		Positions: map[string]*domain.Position{},
	}
	for _, a := range assets[:20] {
		startPortfolio.Positions[a.Symbol] = &domain.Position{
			Symbol:   a.Symbol,
			Quantity: 100,
		}
	}

	factorMetricsHandler := internal.FactorMetricsHandler{
		AdjustedPriceRepository:     repository.AdjustedPriceRepositoryHandler{},
		AssetFundamentalsRepository: repository.AssetFundamentalsRepositoryHandler{},
	}
	h := app.BacktestHandler{
		PriceRepository:      repository.AdjustedPriceRepositoryHandler{},
		FactorMetricsHandler: factorMetricsHandler,
		UniverseRepository:   repository.UniverseRepositoryHandler{},
	}
	backtestInput := app.BacktestInput{
		RoTx: tx,
		FactorOptions: app.FactorOptions{
			Expression: exp,
			Intensity:  0.9,
			Name:       "momentum",
		},
		BacktestStart:    time.Now().AddDate(-3, 0, 0),
		BacktestEnd:      time.Now(),
		SamplingInterval: time.Hour * 24 * 30,
		StartPortfolio:   startPortfolio,
	}
	out, err := h.Backtest(backtestInput)
	if err != nil {
		log.Fatal(err)
	}

	internal.Pprint(out)
}

func exp(tx *sql.Tx) {
	adjPricesRepo := repository.AdjustedPriceRepositoryHandler{}
	metricsHandler := internal.FactorMetricsHandler{
		AdjustedPriceRepository: adjPricesRepo,
	}

	exp := `
	(
		(
			pricePercentChange(
				addDate(currentDate, 0, 0, -7),
				currentDate
			) * 0.5 +
			pricePercentChange(
				addDate(currentDate, 0, -1, 0),
				currentDate
			) * 0.3 +
			pricePercentChange(
				addDate(currentDate, 0, -6, 0),
				currentDate
			) * 0.2
		) / 3
	) / stdev(addDate(currentDate, -5, 0, 0), currentDate)
	
	`

	aapl, err := internal.EvaluateFactorExpression(
		tx,
		exp,
		"AAPL",
		metricsHandler,
		time.Now(),
	)
	if err != nil {
		log.Fatal(err)
	}

	csco, err := internal.EvaluateFactorExpression(
		tx,
		exp,
		"CSCO",
		metricsHandler,
		time.Now(),
	)
	if err != nil {
		log.Fatal(err)
	}

	nvda, err := internal.EvaluateFactorExpression(
		tx,
		exp,
		"NVDA",
		metricsHandler,
		time.Now(),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(aapl.Value, csco.Value, nvda.Value)

	internal.Pprint(aapl.Reason)
	internal.Pprint(csco.Reason)
	internal.Pprint(nvda.Reason)

}

func Ingest(tx *sql.Tx, symbol string) {
	ingestPrices(symbol)
	ingestFundamentals(symbol)
	err := tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

func updateUniversePrices() {
	tx, err := NewTx()
	if err != nil {
		log.Fatal(err)
	}
	err = internal.UpdateUniversePrices(
		tx,
		repository.UniverseRepositoryHandler{},
		repository.AdjustedPriceRepositoryHandler{},
	)
	if err != nil {
		log.Fatal(err)
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

func ingestPrices(symbol string) {
	dbConn, err := New()
	if err != nil {
		log.Fatal(err)
	}
	tx, err := dbConn.Begin()
	if err != nil {
		log.Fatal(err)
	}

	adjPricesRepository := repository.AdjustedPriceRepositoryHandler{}

	err = internal.IngestPrices(tx, symbol, adjPricesRepository)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

func ingestFundamentals(symbol string) {
	dbConn, err := New()
	if err != nil {
		log.Fatal(err)
	}

	tx, err := dbConn.Begin()
	if err != nil {
		log.Fatal(err)
	}

	secrets, err := internal.LoadSecrets()
	if err != nil {
		log.Fatal(err)
	}

	djClient := datajockey.Client{
		HttpClient: http.DefaultClient,
		ApiKey:     secrets.DataJockeyApiKey,
	}

	afRepository := repository.AssetFundamentalsRepositoryHandler{}

	err = internal.IngestFundamentals(
		tx,
		djClient,
		symbol,
		afRepository,
	)
	if err != nil {
		log.Fatal(err)
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}
