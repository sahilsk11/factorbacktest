package cmd

import (
	"context"
	"database/sql"
	"encoding/hex"
	"factorbacktest/api"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/auth"
	"factorbacktest/internal/calculator"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"factorbacktest/internal/util"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
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

	authService, err := buildAuthService(context.Background(), secrets, userAccountRepository, dbConn)
	if err != nil {
		// Don't fail boot on auth misconfig in dev — the API can still
		// serve unauthenticated routes. In prod where secrets are wired,
		// we log the error so it's visible. If this becomes routine we
		// should promote it to a hard failure.
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
		JwtDecodeToken:               secrets.Jwt,
		BetterAuthJwksURL:            betterAuthJwksURL(),
		BetterAuthExpectedIssuer:     betterAuthExpectedIssuer(),
		AuthService:                  authService,
	}

	return apiHandler, nil
}

// buildAuthService constructs the custom Go auth.Service from secrets.
// Returns (nil, err) when any required field is missing — the API treats
// that as "auth disabled" rather than a fatal error so local-dev runs
// without auth secrets continue to work.
func buildAuthService(
	ctx context.Context,
	secrets util.Secrets,
	users repository.UserAccountRepository,
	dbConn *sql.DB,
) (*auth.Service, error) {
	if secrets.Auth.SessionSecret == "" {
		return nil, fmt.Errorf("auth.sessionSecret is empty")
	}
	secretBytes, err := hex.DecodeString(secrets.Auth.SessionSecret)
	if err != nil || len(secretBytes) < 32 {
		return nil, fmt.Errorf("auth.sessionSecret must be 32+ hex-encoded random bytes (try `openssl rand -hex 32`): %w", err)
	}

	frontend := os.Getenv("FACTOR_AUTH_FRONTEND_BASE_URL")
	if frontend == "" {
		frontend = "http://localhost:3000"
	}
	publicBase := os.Getenv("APP_BASE_URL")
	if publicBase == "" {
		publicBase = "http://localhost:3009"
	}

	allowed := []string{
		"http://localhost:3000",
		"https://factorbacktest.net",
		"https://www.factorbacktest.net",
		"https://factor.trade",
		"https://www.factor.trade",
		publicBase,
	}
	if extra := os.Getenv("EXTRA_ALLOWED_ORIGINS"); extra != "" {
		// Same behavior as api/api.go's CORS allowlist; let test harnesses
		// add their own origin without baking it into the binary.
		for _, o := range splitCSV(extra) {
			allowed = append(allowed, o)
		}
	}

	cfg := auth.Config{
		PublicBaseURL:   publicBase,
		FrontendBaseURL: frontend,
		AllowedOrigins:  allowed,
		SessionSecret:   secretBytes,
		Google: auth.GoogleConfig{
			ClientID:     secrets.Auth.GoogleClientID,
			ClientSecret: secrets.Auth.GoogleClientSecret,
		},
		Twilio: auth.TwilioConfig{
			AccountSID:       secrets.Auth.TwilioAccountSID,
			AuthToken:        secrets.Auth.TwilioAuthToken,
			VerifyServiceSID: secrets.Auth.TwilioVerifyServiceSID,
		},
	}

	users2 := userStoreAdapter{repo: users}
	sessions := sessionStoreAdapter{repo: repository.NewAuthSessionRepository(dbConn)}

	bootCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	svc, err := auth.New(bootCtx, cfg, users2, sessions)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// userStoreAdapter implements auth.UserStore over the existing
// UserAccountRepository. Lives in cmd/ rather than internal/auth so the
// auth package stays free of repository imports (cleaner for tests).
type userStoreAdapter struct {
	repo repository.UserAccountRepository
}

func (a userStoreAdapter) GetOrCreateByGoogle(_ context.Context, googleSub, email, firstName, lastName string) (uuid.UUID, error) {
	in := &model.UserAccount{
		Provider:   model.UserAccountProviderType_LocalGoogle,
		ProviderID: util.StringPointer(googleSub),
	}
	if email != "" {
		in.Email = util.StringPointer(email)
	}
	if firstName != "" {
		in.FirstName = util.StringPointer(firstName)
	}
	if lastName != "" {
		in.LastName = util.StringPointer(lastName)
	}
	row, err := a.repo.GetOrCreateByProviderIdentity(in)
	if err != nil {
		return uuid.Nil, err
	}
	return row.UserAccountID, nil
}

func (a userStoreAdapter) GetOrCreateByPhone(_ context.Context, phoneNumber string) (uuid.UUID, error) {
	in := &model.UserAccount{
		Provider:    model.UserAccountProviderType_LocalSms,
		ProviderID:  util.StringPointer(phoneNumber),
		PhoneNumber: util.StringPointer(phoneNumber),
	}
	row, err := a.repo.GetOrCreateByProviderIdentity(in)
	if err != nil {
		return uuid.Nil, err
	}
	return row.UserAccountID, nil
}

// sessionStoreAdapter bridges the repository's AuthSession type to the
// auth package's SessionRow. The two are structurally identical; we
// re-declare to keep the auth package independent of repository.
type sessionStoreAdapter struct {
	repo repository.AuthSessionRepository
}

func (a sessionStoreAdapter) Create(ctx context.Context, s auth.SessionRow) error {
	return a.repo.Create(ctx, repository.AuthSession{
		ID:            s.ID,
		UserAccountID: s.UserAccountID,
		CreatedAt:     s.CreatedAt,
		ExpiresAt:     s.ExpiresAt,
		LastSeenAt:    s.LastSeenAt,
		IP:            s.IP,
		UserAgent:     s.UserAgent,
	})
}

func (a sessionStoreAdapter) Get(ctx context.Context, id string) (*auth.SessionRow, error) {
	row, err := a.repo.Get(ctx, id)
	if err != nil {
		if err == repository.ErrSessionNotFound {
			return nil, auth.ErrSessionNotFound
		}
		return nil, err
	}
	return &auth.SessionRow{
		ID:            row.ID,
		UserAccountID: row.UserAccountID,
		CreatedAt:     row.CreatedAt,
		ExpiresAt:     row.ExpiresAt,
		LastSeenAt:    row.LastSeenAt,
		IP:            row.IP,
		UserAgent:     row.UserAgent,
	}, nil
}

func (a sessionStoreAdapter) Touch(ctx context.Context, id string, newExpiresAt time.Time) error {
	return a.repo.Touch(ctx, id, newExpiresAt)
}

func (a sessionStoreAdapter) Delete(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

func (a sessionStoreAdapter) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	return a.repo.DeleteExpired(ctx, before)
}
