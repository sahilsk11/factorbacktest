// Package auth provides session-based authentication for the factor.trade
// API. Identity is established via Google OIDC or Twilio Verify SMS; once
// proven, the package issues a signed session cookie and persists the
// session row in app_auth.user_session. The Go API reads the cookie via
// auth.Service.Middleware() and resolves it to a uuid.UUID userAccountID.
//
// Security-critical primitives are delegated to vetted libraries; what
// this package owns is the glue. See README.md in this directory for the
// full threat model and the invariants tests will assert.
package auth

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// Config bundles every value the auth package needs at construction time.
// Loaded from util.Secrets + a couple of env vars by NewFromSecrets.
type Config struct {
	PublicBaseURL         string        // e.g. https://api.factor.trade; used to compute the OAuth redirect URI
	FrontendBaseURL       string        // e.g. https://factor.trade; the only redirect target after OAuth callback
	AllowedOrigins        []string      // Origin allowlist for state-changing POSTs; should match CORS allowlist
	SessionSecret         []byte        // HMAC-SHA256 key for signing session cookies; must be >=32 bytes
	SessionTTL            time.Duration // sliding window; default 30 days
	SessionAbsoluteMaxAge time.Duration // hard cap from creation; default 90 days
	Google                GoogleConfig
	Twilio                TwilioConfig
	// EmailSender is the transport used by the email-OTP flow to deliver
	// codes. Optional: when nil, /auth/email/* is not registered and the
	// email-OTP rate limiter is never instantiated. Wired from
	// cmd/util.go using whichever provider EMAIL_PROVIDER selected.
	EmailSender repository.EmailRepository
	Now         func() time.Time // overridable for tests
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // defaults to PublicBaseURL + "/auth/google/callback"
}

type TwilioConfig struct {
	AccountSID       string
	AuthToken        string
	VerifyServiceSID string
	HTTPClient       *http.Client // optional; defaults to 10s-timeout client
}

// Service is the public surface. Construct once at boot, then call
// RegisterRoutes and engine.Use(Middleware()).
type Service struct {
	cfg         Config
	users       repository.UserAccountRepository
	sessions    repository.AuthSessionRepository
	verifier    *oidc.IDTokenVerifier
	oauth2cfg   *oauth2.Config
	twilio      *twilioClient
	smsLimit    *rateLimiter
	emailSender repository.EmailRepository    // nil ⇒ email OTP disabled
	emailOTPs   repository.EmailOTPRepository // nil ⇒ email OTP disabled
	emailLimit  *rateLimiter
	now         func() time.Time
	log         *zap.SugaredLogger
}

// ErrSessionNotFound is re-exported so callers can sentinel-check without
// importing the repository package. Kept identical (==) to repository.ErrSessionNotFound.
var ErrSessionNotFound = repository.ErrSessionNotFound

const (
	defaultSessionTTL            = 30 * 24 * time.Hour
	defaultSessionAbsoluteMaxAge = 90 * 24 * time.Hour
	minSessionSecretBytes        = 32
)

// New constructs a Service. Validates eagerly so misconfiguration fails at
// boot rather than on the first auth request. The ctx is used to fetch
// Google's OIDC discovery document; pass a context with a sensible timeout.
//
// emailOTPs is the persistence layer for email-OTP codes. It is paired
// with cfg.EmailSender — when both are non-nil, /auth/email/{send,verify}
// route registration becomes active. When either is nil, email-OTP is
// silently disabled (mirrors how Google/SMS already gracefully no-op on
// missing config in tests + dev).
func New(ctx context.Context, cfg Config, users repository.UserAccountRepository, sessions repository.AuthSessionRepository, emailOTPs repository.EmailOTPRepository) (*Service, error) {
	if users == nil {
		return nil, errors.New("auth.New: users repository is required")
	}
	if sessions == nil {
		return nil, errors.New("auth.New: sessions repository is required")
	}
	if cfg.PublicBaseURL == "" {
		return nil, errors.New("auth.New: Config.PublicBaseURL is required")
	}
	if cfg.FrontendBaseURL == "" {
		return nil, errors.New("auth.New: Config.FrontendBaseURL is required")
	}
	if len(cfg.SessionSecret) < minSessionSecretBytes {
		return nil, fmt.Errorf("auth.New: SessionSecret must be >= %d bytes (got %d)", minSessionSecretBytes, len(cfg.SessionSecret))
	}
	if cfg.Google.ClientID == "" || cfg.Google.ClientSecret == "" {
		return nil, errors.New("auth.New: Google.ClientID and Google.ClientSecret are required")
	}
	if cfg.Twilio.AccountSID == "" || cfg.Twilio.AuthToken == "" || cfg.Twilio.VerifyServiceSID == "" {
		return nil, errors.New("auth.New: Twilio.AccountSID/AuthToken/VerifyServiceSID are required")
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = defaultSessionTTL
	}
	if cfg.SessionAbsoluteMaxAge <= 0 {
		cfg.SessionAbsoluteMaxAge = defaultSessionAbsoluteMaxAge
	}
	if cfg.SessionAbsoluteMaxAge < cfg.SessionTTL {
		return nil, errors.New("auth.New: SessionAbsoluteMaxAge must be >= SessionTTL")
	}
	if cfg.Google.RedirectURL == "" {
		cfg.Google.RedirectURL = cfg.PublicBaseURL + "/auth/google/callback"
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("auth.New: discover Google OIDC provider: %w", err)
	}

	svc := &Service{
		cfg:      cfg,
		users:    users,
		sessions: sessions,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.Google.ClientID}),
		oauth2cfg: &oauth2.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			RedirectURL:  cfg.Google.RedirectURL,
			Endpoint:     googleoauth.Endpoint,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		twilio:   newTwilioClient(cfg.Twilio),
		smsLimit: newRateLimiter(),
		now:      cfg.Now,
		log:      logger.New().With("component", "auth"),
	}
	// Email-OTP wiring is gated on BOTH a sender and a persistence
	// repo being supplied — either alone would be a misconfiguration
	// (no point hashing codes you can't deliver, or sending codes you
	// can't validate). When wiring is incomplete we leave the fields
	// nil and RegisterRoutes will skip /auth/email/*.
	if cfg.EmailSender != nil && emailOTPs != nil {
		svc.emailSender = cfg.EmailSender
		svc.emailOTPs = emailOTPs
		svc.emailLimit = newRateLimiter()
	}
	return svc, nil
}

// NewFromSecrets is the production wiring: reads util.Secrets + the
// FACTOR_AUTH_FRONTEND_BASE_URL / APP_BASE_URL / EXTRA_ALLOWED_ORIGINS env
// vars, builds repository handles off the *sql.DB, and constructs the
// Service. Returns (nil, err) when any required field is missing — the
// caller should treat that as "auth disabled" so a local-dev binary
// without auth secrets still boots.
//
// emailSender is the transport selected by cmd/util.go (Resend by default,
// SES under EMAIL_PROVIDER=ses). When non-nil the email-OTP flow becomes
// available; when nil the rest of auth still works and /auth/email/*
// silently isn't registered.
func NewFromSecrets(ctx context.Context, secrets util.Secrets, db *sql.DB, emailSender repository.EmailRepository) (*Service, error) {
	if secrets.Auth.SessionSecret == "" {
		return nil, fmt.Errorf("auth.sessionSecret is empty")
	}
	secretBytes, err := hex.DecodeString(secrets.Auth.SessionSecret)
	if err != nil {
		return nil, fmt.Errorf("auth.sessionSecret must be hex-encoded (try `openssl rand -hex 32`): %w", err)
	}
	if len(secretBytes) < minSessionSecretBytes {
		return nil, fmt.Errorf("auth.sessionSecret too short: got %d bytes, need at least %d", len(secretBytes), minSessionSecretBytes)
	}

	publicBase := envOr("APP_BASE_URL", "http://localhost:3009")
	frontend := envOr("FACTOR_AUTH_FRONTEND_BASE_URL", "http://localhost:3000")

	cfg := Config{
		PublicBaseURL:   publicBase,
		FrontendBaseURL: frontend,
		AllowedOrigins:  AppOrigins(),
		SessionSecret:   secretBytes,
		Google: GoogleConfig{
			ClientID:     secrets.Auth.GoogleClientID,
			ClientSecret: secrets.Auth.GoogleClientSecret,
		},
		Twilio: TwilioConfig{
			AccountSID:       secrets.Auth.TwilioAccountSID,
			AuthToken:        secrets.Auth.TwilioAuthToken,
			VerifyServiceSID: secrets.Auth.TwilioVerifyServiceSID,
		},
		EmailSender: emailSender,
	}

	bootCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return New(bootCtx, cfg,
		repository.NewUserAccountRepository(db),
		repository.NewAuthSessionRepository(db),
		repository.NewEmailOTPRepository(db),
	)
}

// RegisterRoutes mounts the auth endpoints onto r under /auth/*.
// Mount BEFORE the legacy JWT middleware in api/api.go so unauthenticated
// requests to these routes (sign-in, OAuth callback) aren't rejected.
func (s *Service) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/auth")
	g.GET("/google/start", s.handleGoogleStart)
	g.GET("/google/callback", s.handleGoogleCallback)
	g.POST("/sms/send", s.requireOrigin(), s.handleSmsSend)
	g.POST("/sms/verify", s.requireOrigin(), s.handleSmsVerify)
	if s.emailSender != nil && s.emailOTPs != nil {
		g.POST("/email/send", s.requireOrigin(), s.handleEmailSend)
		g.POST("/email/verify", s.requireOrigin(), s.handleEmailVerify)
	}
	g.POST("/sign-out", s.requireOrigin(), s.handleSignOut)
	g.GET("/session", s.handleGetSession)
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
