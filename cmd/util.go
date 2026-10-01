package cmd

import (
	"context"
	"database/sql"
	"factorbacktest/api"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/auth"
	"factorbacktest/internal/calculator"
	"factorbacktest/internal/data"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"time"

	"factorbacktest/internal/util"
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

	// Pool sizing. Without these calls we get Go's defaults: MaxIdleConns=2,
	// MaxOpenConns=unlimited, both lifetimes infinite. That's wrong for a
	// long-lived API process talking to managed Postgres:
	//
	// - Unlimited MaxOpenConns lets a traffic burst (or a runaway query)
	//   blow past the managed Postgres connection cap and start failing requests with
	//   "too many connections for role". 25 stays comfortably below typical
	//   managed Postgres limits and matches the steady-state we already observe
	//   in pg_stat_activity.
	// - MaxIdleConns balances burst fan-outs (see factor_score.repository.go,
	//   up to ~10 concurrent queries) against Neon scale-to-zero: idle conns
	//   must drop before the ~5m autosuspend window or compute never sleeps.
	//   3 keeps a little headroom for back-to-back requests without holding
	//   a large warm pool overnight.
	// - ConnMaxLifetime=infinite means a connection killed by database-side
	//   maintenance/failover sits in the pool until we try to use it and
	//   get a half-open socket error. 30m forces a periodic refresh.
	// - ConnMaxIdleTime must stay well under Neon's autosuspend interval (~5m).
	//   A 5m idle timeout matched that window and kept endpoints active 24/7
	//   whenever the Fly web VM held any idle pool connection.
	dbConn.SetMaxOpenConns(25)
	dbConn.SetMaxIdleConns(3)
	dbConn.SetConnMaxLifetime(30 * time.Minute)
	dbConn.SetConnMaxIdleTime(45 * time.Second)

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
		investmentRebalanceRepository,
		holdingsRepository,
		holdingsVersionRepository,
		rebalancerRunRepository,
		excessVolumeRepository,
		strategyInvestmentRepository,
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

	// Email transport is optional. When secrets.Resend.APIKey is empty
	// (e.g. cmd/test-api booting without real credentials) we leave the
	// repo nil; downstream consumers (EmailService, auth.Service) all
	// nil-check and return loud errors rather than silently no-op'ing.
	// The SES implementation in internal/repository/ses_email.repository.go
	// is intentionally retained but unwired; it can be revived if Resend
	// ever proves unsuitable.
	var emailRepository repository.EmailRepository
	if secrets.Resend.APIKey != "" {
		emailRepository, err = repository.NewResendEmailRepository(
			secrets.Resend.APIKey,
			secrets.Resend.FromEmail,
			secrets.Resend.FromName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create resend email repository: %w", err)
		}
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

	// Auth is opt-in: NewFromSecrets returns an error when required secrets
	// aren't set, and we treat that as "auth disabled" rather than a fatal
	// boot error so local-dev binaries without auth secrets still work.
	// emailRepository is passed in so the auth package can deliver email
	// OTPs through whichever provider cmd/util.go selected above.
	authService, err := auth.NewFromSecrets(context.Background(), secrets, dbConn, emailRepository)
	if err != nil {
		log.Printf("[auth] not enabled: %v", err)
	}

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
		AuthService:                  authService,
	}

	return apiHandler, nil
}
