package cmd

import (
	"database/sql"
	"factorbacktest/api"
	integration_tests "factorbacktest/integration-tests"
	"factorbacktest/internal"
	"factorbacktest/internal/repository"
	l1_service "factorbacktest/internal/service/l1"
	l2_service "factorbacktest/internal/service/l2"
	l3_service "factorbacktest/internal/service/l3"
	"factorbacktest/internal/util"
	"fmt"
	"log"
	"os"
	"strings"

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
	secrets, err := util.LoadSecrets()
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
	strategyRepository := repository.NewStrategyRepository(dbConn)
	strategyInvestmentRepository := repository.NewInvestmentRepository(dbConn)
	holdingsRepository := repository.NewInvestmentHoldingsRepository(dbConn)
	alpacaRepository := repository.NewAlpacaRepository(secrets.Alpaca.ApiKey, secrets.Alpaca.ApiSecret, secrets.Alpaca.Endpoint)
	tradeOrderRepository := repository.NewTradeOrderRepository(dbConn)
	rebalancerRunRepository := repository.NewRebalancerRunRepository(dbConn)
	investmentTradeRepository := repository.NewInvestmentTradeRepository(dbConn)
	holdingsVersionRepository := repository.NewInvestmentHoldingsVersionRepository(dbConn)
	investmentRebalanceRepository := repository.NewInvestmentRebalanceRepository(dbConn)
	excessVolumeRepository := repository.NewExcessTradeVolumeRepository(dbConn)
	rebalancePriceRepository := repository.NewRebalancePriceRepository(dbConn)

	if strings.EqualFold(os.Getenv("ALPHA_ENV"), "test") || UseMockAlpaca {
		alpacaRepository = integration_tests.NewMockAlpacaRepositoryForTests()
	}

	var priceServiceAlpacaRepository repository.AlpacaRepository = nil
	if strings.EqualFold(os.Getenv("ALPHA_ENV"), "dev") {
		priceServiceAlpacaRepository = alpacaRepository
	}

	priceService := l1_service.NewPriceService(dbConn, priceRepository, priceServiceAlpacaRepository)
	assetUniverseRepository := repository.NewAssetUniverseRepository(dbConn)
	factorExpressionService := l2_service.NewFactorExpressionService(dbConn, factorMetricsHandler, priceService, factorScoreRepository)
	backtestHandler := l3_service.BacktestHandler{
		PriceRepository:         priceRepository,
		AssetUniverseRepository: assetUniverseRepository,
		Db:                      dbConn,
		PriceService:            priceService,
		FactorExpressionService: factorExpressionService,
	}
	tradingService := l1_service.NewTradeService(
		dbConn,
		alpacaRepository,
		tradeOrderRepository,
		tickerRepository,
		investmentTradeRepository,
		holdingsRepository,
		holdingsVersionRepository,
		rebalancerRunRepository,
		excessVolumeRepository,
	)
	investmentService := l3_service.NewInvestmentService(
		dbConn,
		strategyInvestmentRepository,
		holdingsRepository,
		assetUniverseRepository,
		strategyRepository,
		factorExpressionService,
		tickerRepository,
		rebalancerRunRepository,
		holdingsVersionRepository,
		investmentTradeRepository,
		backtestHandler,
		alpacaRepository,
		tradingService,
		investmentRebalanceRepository,
		priceRepository,
		rebalancePriceRepository,
	)
	strategyService := l3_service.NewStrategyService(
		strategyRepository,
		assetUniverseRepository,
		priceRepository,
		backtestHandler,
	)

	apiHandler := &api.ApiHandler{
		BenchmarkHandler: internal.BenchmarkHandler{
			PriceRepository: priceRepository,
		},
		BacktestHandler:              backtestHandler,
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
		StrategyRepository:           strategyRepository,
		InvestmentRepository:         strategyInvestmentRepository,
		InvestmentService:            investmentService,
		TradingService:               tradingService,
		StrategyService:              strategyService,
		JwtDecodeToken:               secrets.Jwt,
	}

	return apiHandler, nil
}
