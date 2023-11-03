package main

import (
	"database/sql"
	"factorbacktest/api"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/repository"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	secrets, err := internal.LoadSecrets()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to load secrets: %w", err))
	}

	dbConn, err := sql.Open("postgres", secrets.Db.ToConnectionStr())
	if err != nil {
		log.Fatal("failed to connect to db: %w", err)
	}

	defer dbConn.Close()

	gptRepository, err := repository.NewGptRepository(secrets.ChatGPTApiKey)
	if err != nil {
		log.Fatal(err)
	}

	apiHandler := api.ApiHandler{
		BenchmarkHandler: internal.BenchmarkHandler{
			PriceRepository: repository.NewAdjustedPriceRepository(),
		},
		BacktestHandler: app.BacktestHandler{
			PriceRepository: repository.NewAdjustedPriceRepository(),
			FactorMetricsHandler: internal.FactorMetricsHandler{
				AdjustedPriceRepository:     repository.NewAdjustedPriceRepository(),
				AssetFundamentalsRepository: repository.AssetFundamentalsRepositoryHandler{},
			},
			UniverseRepository: repository.UniverseRepositoryHandler{},
			Db:                 dbConn,
		},
		UserStrategyRepository: repository.UniverseRepositoryHandler{},
		ContactRepository:      repository.ContactRepositoryHandler{},
		Db:                     dbConn,
		GptRepository:          gptRepository,
		ApiRequestRepository:   repository.ApiRequestRepositoryHandler{},
	}

	err = apiHandler.StartApi(3009)
	if err != nil {
		log.Fatal(err)
	}
}
