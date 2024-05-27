package main

import (
	"context"
	"database/sql"
	"factorbacktest/internal"
	"factorbacktest/internal/repository"
	"factorbacktest/pkg/datajockey"
	"fmt"
	"log"
	"net/http"

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

func main() {
	// internal.IngestPrices()
	// gpt()
	// ingestUniverseFundamentals()
	// ingestFundamentals("AAPL")
	// updateUniversePrices()
}

func Ingest(tx *sql.Tx, symbol string) {
	ingestFundamentals(symbol)
	err := tx.Commit()
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
	defer tx.Rollback()

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

func ingestUniverseFundamentals() {
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
