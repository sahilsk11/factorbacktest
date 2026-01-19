package cmd

import (
	"database/sql"
	"factorbacktest/api"
	integration_tests "factorbacktest/integration-tests"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/calculator"
	"factorbacktest/internal/data"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"

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

	factorMetricsHandler := calculator.NewFactorMetricsHandler(
		priceRepository,
		repository.AssetFundamentalsRepositoryHandler{},
	)

	tickerRepository := repository.NewTickerRepository(dbConn)
	factorScoreRepository := repository.NewFactorScoreRepository(dbConn)
	userAccountRepository := repository.NewUserAccountRepository(dbConn)
	emailPreferenceRepository := repository.NewEmailPreferenceRepository(dbConn)
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

	priceService := data.NewPriceService(dbConn, priceRepository, nil)

	if strings.EqualFold(os.Getenv("ALPHA_ENV"), "test") || UseMockAlpaca {
		alpacaRepository = integration_tests.NewMockAlpacaRepositoryForTests()
		priceService = integration_tests.NewMockPriceServiceForTests(
			priceService,
		)
	}

	assetUniverseRepository := repository.NewAssetUniverseRepository(dbConn)
	factorExpressionService := calculator.NewFactorExpressionService(dbConn, factorMetricsHandler, priceService, factorScoreRepository, priceRepository)
	backtestHandler := service.BacktestHandler{
		PriceRepository:         priceRepository,
		AssetUniverseRepository: assetUniverseRepository,
		Db:                      dbConn,
		PriceService:            priceService,
		FactorExpressionService: factorExpressionService,
	}
	tradingService := service.NewTradeService(
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
	investmentService := service.NewInvestmentService(
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
		priceService,
	)
	strategyService := service.NewStrategyService(
		strategyRepository,
		assetUniverseRepository,
		priceRepository,
		backtestHandler,
	)

	// Initialize email repository and service
	emailRepository, err := repository.NewEmailRepository(secrets.SES.Region, secrets.SES.FromEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to create email repository: %w", err)
	}
	emailService := service.NewEmailService(emailRepository)

	// Initialize strategy summary app
	strategySummaryApp := app.NewStrategySummaryApp(
		emailService,
		userAccountRepository,
		emailPreferenceRepository,
		strategyRepository,
		assetUniverseRepository,
		priceService,
		factorExpressionService,
		tickerRepository,
		priceRepository,
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
		StrategySummaryApp:           strategySummaryApp,
		JwtDecodeToken:               secrets.Jwt,
	}

	return apiHandler, nil
}
