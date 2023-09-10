package main

import (
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
	secrets, err := internal.LoadSecrets()
	if err != nil {
		log.Fatal(err)
	}

	dbConn, err := sql.Open("postgres", secrets.Db.ToConnectionStr())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}

	return dbConn, nil
}

func NewTx() (*sql.Tx, error) {
	dbConn, err := New()
	if err != nil {
		return nil, err
	}

	return dbConn.Begin()
}

func main() {
	// internal.IngestPrices()
	gpt()
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
		AdjustedPriceRepository:     repository.NewAdjustedPriceRepository(),
		AssetFundamentalsRepository: repository.AssetFundamentalsRepositoryHandler{},
	}
	h := app.BacktestHandler{
		PriceRepository:      repository.NewAdjustedPriceRepository(),
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
	out, err := h.Backtest(context.Background(), backtestInput)
	if err != nil {
		log.Fatal(err)
	}

	internal.Pprint(out)
}

func exp(tx *sql.Tx) {
	adjPricesRepo := repository.NewAdjustedPriceRepository()
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
		log.Fatal(fmt.Errorf("failed to create tx: %w", err))
	}

	err = internal.UpdateUniversePrices(
		tx,
		repository.UniverseRepositoryHandler{},
		repository.NewAdjustedPriceRepository(),
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

	adjPricesRepository := repository.NewAdjustedPriceRepository()

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

func gpt() {
	ctx := context.Background()
	secrets, err := internal.LoadSecrets()
	if err != nil {
		log.Fatal(err)
	}
	gptRepository, err := repository.NewGptRepository(secrets.ChatGPTApiKey)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := gptRepository.ConstructFactorEquation(ctx, "undervalued stocks using pb ratio")
	if err != nil {
		log.Fatal(err)
	}
	internal.Pprint(resp)

}

func updateUniverseFundamentals() {
	db, err := New()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to create tx: %w", err))
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

	ur := repository.UniverseRepositoryHandler{}

	err = internal.IngestUniverseFundamentals(
		db,
		djClient,
		afRepository,
		ur,
	)
	if err != nil {
		log.Fatal(err)
	}
}
