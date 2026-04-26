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
	"os"

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
	//   blow past the RDS connection cap and start failing requests with
	//   "too many connections for role". 25 is comfortably under the
	//   db.t-class default and matches the steady-state we already observe
	//   in pg_stat_activity.
	// - MaxIdleConns=2 is too low for our 10-goroutine fan-outs (see
	//   factor_score.repository.go). On a burst the pool returns 10 conns
	//   but only keeps 2 idle, closing the rest. The next batch then pays
	//   a fresh TLS handshake (~3 RTT) per new conn.
	// - ConnMaxLifetime=infinite means a connection killed by RDS-side
	//   maintenance/failover sits in the pool until we try to use it and
	//   get a half-open socket error. 30m forces a periodic refresh.
	// - ConnMaxIdleTime=5m frees conns that were opened during a burst and
	//   are no longer needed, so we don't hold idle conns indefinitely.
	dbConn.SetMaxOpenConns(25)
	dbConn.SetMaxIdleConns(10)
	dbConn.SetConnMaxLifetime(30 * time.Minute)
	dbConn.SetConnMaxIdleTime(5 * time.Minute)

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

	// Pick the email backend. Default is Resend (post-SES migration);
	// EMAIL_PROVIDER=ses is the one-release fallback that stays available
	// until we delete the SES code path. The two implementations satisfy
	// the same repository.EmailRepository interface so the service layer
	// is unaware of which is wired.
	emailRepository, err := buildEmailRepository(secrets)
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

// buildEmailRepository selects the email backend based on the
// EMAIL_PROVIDER env var. Default is Resend (post-SES migration); set
// EMAIL_PROVIDER=ses to revert to AWS SES without redeploying code.
// Removing the SES path is a follow-up PR once Resend has soaked.
func buildEmailRepository(secrets util.Secrets) (repository.EmailRepository, error) {
	provider := os.Getenv("EMAIL_PROVIDER")
	if provider == "" {
		provider = "resend"
	}
	switch provider {
	case "ses":
		return repository.NewSESEmailRepository(secrets.SES.Region, secrets.SES.FromEmail)
	case "resend":
		return repository.NewResendEmailRepository(
			secrets.Resend.APIKey,
			secrets.Resend.FromEmail,
			secrets.Resend.FromName,
		)
	default:
		return nil, fmt.Errorf("unknown EMAIL_PROVIDER=%q (expected resend or ses)", provider)
	}
}
