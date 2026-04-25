package cmd

import (
	"database/sql"
	"factorbacktest/api"
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

	_ "github.com/lib/pq"
)

// betterAuthJwksURL returns the URL the API uses to fetch the Better Auth
// JWKS. Defaults to the local sidecar that runs alongside the Go binary in
// the production Fly image.
func betterAuthJwksURL() string {
	if v := os.Getenv("BETTER_AUTH_JWKS_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:3001/api/auth/jwks"
}

// betterAuthExpectedIssuer returns the `iss` claim the Go middleware will
// require on Better Auth JWTs. Better Auth stamps `iss = baseURL`, so we
// pull it from the same env var the auth-service uses (APP_BASE_URL).
// Empty disables the check.
func betterAuthExpectedIssuer() string {
	if v := os.Getenv("BETTER_AUTH_EXPECTED_ISSUER"); v != "" {
		return v
	}
	return os.Getenv("APP_BASE_URL")
}

// this is gross sry

func CloseDependencies(handler *api.ApiHandler) {
	err := handler.Db.Close()
	if err != nil {
		log.Fatalf("failed to close db: %v", err)
	}
}

func InitializeDependencies(secrets util.Secrets, overrides *api.ApiHandler) (*api.ApiHandler, error) {
	var gptRepository repository.GptRepository
	var alpacaRepository repository.AlpacaRepository
	var priceService data.PriceService
	if overrides != nil {
		alpacaRepository = overrides.AlpacaRepository
		priceService = overrides.PriceService
	}
	var err error

	if secrets.ChatGPTApiKey != "" {
		gptRepository, err = repository.NewGptRepository(secrets.ChatGPTApiKey)
		if err != nil {
			return nil, err
		}
	}

	if alpacaRepository == nil && secrets.Alpaca.ApiKey != "" {
		alpacaRepository = repository.NewAlpacaRepository(secrets.Alpaca.ApiKey, secrets.Alpaca.ApiSecret, secrets.Alpaca.Endpoint)
	}

	dbConnStr := secrets.Db.ToConnectionStr()

	dbConn, err := sql.Open("postgres", dbConnStr)
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
	tradeOrderRepository := repository.NewTradeOrderRepository(dbConn)
	rebalancerRunRepository := repository.NewRebalancerRunRepository(dbConn)
	investmentTradeRepository := repository.NewInvestmentTradeRepository(dbConn)
	holdingsVersionRepository := repository.NewInvestmentHoldingsVersionRepository(dbConn)
	investmentRebalanceRepository := repository.NewInvestmentRebalanceRepository(dbConn)
	excessVolumeRepository := repository.NewExcessTradeVolumeRepository(dbConn)
	rebalancePriceRepository := repository.NewRebalancePriceRepository(dbConn)

	quoteProvider := data.NewHybridQuoteProvider(alpacaRepository)
	if priceService == nil {
		priceService = data.NewPriceService(dbConn, priceRepository, nil, quoteProvider)
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
		Port: secrets.Port,
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
		BetterAuthJwksURL:            betterAuthJwksURL(),
		BetterAuthExpectedIssuer:     betterAuthExpectedIssuer(),
	}

	return apiHandler, nil
}
