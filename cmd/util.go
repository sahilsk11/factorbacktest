package cmd

import (
	"database/sql"
	"factorbacktest/api"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// this is gross sry

func CloseDependencies(handler *api.ApiHandler) {
	err := handler.Db.Close()
	if err != nil {
		log.Fatalf("failed to close db: %v", err)
	}
}

func InitializeDependencies() (*api.ApiHandler, error) {
	secrets, err := internal.LoadSecrets()
	if err != nil {
		return nil, fmt.Errorf("failed to load secrets: %w", err)
	}

	gptRepository, err := repository.NewGptRepository(secrets.ChatGPTApiKey)
	if err != nil {
		return nil, err
	}

	dbConn, err := sql.Open("postgres", secrets.Db.ToConnectionStr())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}
	// TODO - possible db leak, since I don't have the defer here

	priceRepository := repository.NewAdjustedPriceRepository()

	priceService := service.NewPriceService(dbConn, priceRepository)

	apiHandler := &api.ApiHandler{
		BenchmarkHandler: internal.BenchmarkHandler{
			PriceRepository: priceRepository,
		},
		BacktestHandler: app.BacktestHandler{
			PriceRepository: repository.NewAdjustedPriceRepository(),
			FactorMetricsHandler: internal.NewFactorMetricsHandler(
				priceRepository,
				repository.AssetFundamentalsRepositoryHandler{},
			),
			UniverseRepository: repository.UniverseRepositoryHandler{},
			Db:                 dbConn,
			PriceService:       priceService,
		},
		UserStrategyRepository:       repository.UniverseRepositoryHandler{},
		ContactRepository:            repository.ContactRepositoryHandler{},
		Db:                           dbConn,
		GptRepository:                gptRepository,
		ApiRequestRepository:         repository.ApiRequestRepositoryHandler{},
		LatencencyTrackingRepository: repository.NewLatencyTrackingRepository(dbConn),
		UniverseRepository:           repository.UniverseRepositoryHandler{},
		PriceService:                 priceService,
		PriceRepository:              repository.NewAdjustedPriceRepository(),
	}

	return apiHandler, nil
}
