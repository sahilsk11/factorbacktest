package cmd

import (
	"database/sql"
	"factorbacktest/api"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/repository"
	l1_service "factorbacktest/internal/service/l1"
	l2_service "factorbacktest/internal/service/l2"
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

	priceRepository := repository.NewAdjustedPriceRepository(dbConn)

	factorMetricsHandler := l2_service.NewFactorMetricsHandler(
		priceRepository,
		repository.AssetFundamentalsRepositoryHandler{},
	)

	tickerRepository := repository.NewTickerRepository(dbConn)
	factorScoreRepository := repository.NewFactorScoreRepository(dbConn)
	userAccountRepository := repository.NewUserAccountRepository(dbConn)
	savedStrategyRepository := repository.NewSavedStrategyRepository(dbConn)
	strategyInvestmentRepository := repository.NewStrategyInvestmentRepository(dbConn)

	priceService := l1_service.NewPriceService(dbConn, priceRepository)
	assetUniverseRepository := repository.NewAssetUniverseRepository(dbConn)
	factorExpressionService := l2_service.NewFactorExpressionService(dbConn, factorMetricsHandler, priceService, factorScoreRepository)

	apiHandler := &api.ApiHandler{
		BenchmarkHandler: internal.BenchmarkHandler{
			PriceRepository: priceRepository,
		},
		BacktestHandler: app.BacktestHandler{
			PriceRepository:         priceRepository,
			AssetUniverseRepository: assetUniverseRepository,
			Db:                      dbConn,
			PriceService:            priceService,
			FactorExpressionService: factorExpressionService,
		},
		UserStrategyRepository:       repository.UserStrategyRepositoryHandler{},
		ContactRepository:            repository.ContactRepositoryHandler{},
		Db:                           dbConn,
		GptRepository:                gptRepository,
		ApiRequestRepository:         repository.ApiRequestRepositoryHandler{},
		LatencencyTrackingRepository: repository.NewLatencyTrackingRepository(dbConn),
		TickerRepository:             tickerRepository,
		PriceService:                 priceService,
		PriceRepository:              priceRepository,
		AssetUniverseRepository:      assetUniverseRepository,
		UserAccountRepository:        userAccountRepository,
		SavedStrategyRepository:      savedStrategyRepository,
		StrategyInvestmentRepository: strategyInvestmentRepository,
	}

	return apiHandler, nil
}
