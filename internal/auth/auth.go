// Package auth provides session-based authentication for the factor.trade
// API. Identity is established via Google OIDC or Twilio Verify SMS; once
// proven, the package issues a signed session cookie and persists the
// session row in app_auth.user_session. The Go API reads the cookie via
// auth.Service.Middleware() and resolves it to a UUID userAccountID.
//
// Security-critical primitives are delegated to vetted libraries:
//
//   - golang.org/x/oauth2 + coreos/go-oidc  -> OAuth code/PKCE flow + ID token verification
//   - Twilio Verify REST API                -> OTP generation, validation, fraud protection
//   - crypto/hmac, crypto/subtle            -> cookie signature
//   - crypto/rand                           -> session ids, state, nonce, PKCE verifier
//
// What this package owns: routing, cookie attribute hygiene, session
// lifecycle (sliding TTL + absolute cap), state-cookie CSRF on the OAuth
// callback, Origin-header allowlist on state-changing POSTs, per-IP +
// per-phone rate limiting on /auth/sms/send, and the user-creation glue
// between an identity claim and public.user_account.
//
// See README.md in this directory for the full threat model and the
// invariants each test in *_test.go is asserting.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// Config bundles every value the auth package needs at construction time.
// Loaded from util.Secrets + a couple of process env vars in cmd/api/main.go.
type Config struct {
	// PublicBaseURL is the externally-visible API origin, e.g.
	// "https://api.factor.trade". The OAuth redirect URI registered with
	// Google is computed as PublicBaseURL + "/auth/google/callback".
	PublicBaseURL string

	// FrontendBaseURL is where the OAuth callback redirects on success,
	// e.g. "https://factor.trade". The package never accepts a redirect
	// target from query params; this is the only success destination.
	FrontendBaseURL string

	// AllowedOrigins is the allowlist enforced on state-changing POSTs
	// (sign-out, sms/send, sms/verify) via the Origin header. Should
	// match the existing CORS allowlist exactly.
	AllowedOrigins []string

	// SessionSecret is the HMAC-SHA256 key used to sign session cookies.
	// Must be at least 32 bytes (typically 32 hex-decoded bytes from
	// `openssl rand -hex 32`). Rotating it logs every user out.
	SessionSecret []byte

	// SessionTTL is the sliding window: each authenticated request sets
	// expires_at = now + SessionTTL. Default: 30 days.
	SessionTTL time.Duration

	// SessionAbsoluteMaxAge is the hard cap from session creation. Even
	// if a user is active every day, after this duration the session is
	// rejected and they must re-authenticate. Default: 90 days.
	SessionAbsoluteMaxAge time.Duration

	Google GoogleConfig
	Twilio TwilioConfig

	// Now is overridable for tests. Defaults to time.Now in production.
	Now func() time.Time
}

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL defaults to PublicBaseURL + "/auth/google/callback"
	// if left empty. Whatever value ends up in Service must also be added
	// to Google Cloud Console's "Authorized redirect URIs" for the OAuth
	// client; mismatch makes Google refuse the auth request.
	RedirectURL string
}

type TwilioConfig struct {
	AccountSID       string
	AuthToken        string
	VerifyServiceSID string
	// HTTPClient is optional. When nil the package uses an http.Client
	// with a 10s total timeout. Tests override this to inject a fake.
	HTTPClient *http.Client
}

// UserStore is how this package finds or creates an application user given
// an identity claim. Implemented in cmd/api/main.go as a thin wrapper over
// repository.UserAccountRepository.GetOrCreateByProviderIdentity.
type UserStore interface {
	// GetOrCreateByGoogle is called after a verified Google ID token.
	// googleSub is the stable Google user identifier (the "sub" claim).
	// Email/name come from the same token; both may be empty if the
	// granted scope didn't include them. Returns the application user id.
	GetOrCreateByGoogle(ctx context.Context, googleSub, email, firstName, lastName string) (uuid.UUID, error)

	// GetOrCreateByPhone is called after Twilio Verify confirms a phone
	// number. Email is unknown at this point.
	GetOrCreateByPhone(ctx context.Context, phoneNumber string) (uuid.UUID, error)
}

// SessionStore mirrors repository.AuthSessionRepository. Re-declared here
// so the auth package doesn't depend on the repository package directly
// (cleaner for tests; lets us swap in an in-memory fake without DB).
type SessionStore interface {
	Create(ctx context.Context, s SessionRow) error
	Get(ctx context.Context, id string) (*SessionRow, error)
	Touch(ctx context.Context, id string, newExpiresAt time.Time) error
	Delete(ctx context.Context, id string) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// SessionRow is the in-memory representation of an app_auth.user_session
// row. Identical to repository.AuthSession; duplicated to keep auth
// independent of the repository package.
type SessionRow struct {
	ID            string
	UserAccountID uuid.UUID
	CreatedAt     time.Time
	ExpiresAt     time.Time
	LastSeenAt    time.Time
	IP            string
	UserAgent     string
}

// ErrSessionNotFound is what SessionStore.Get returns when no row matches.
// Re-exported so callers can treat "no session" as a sentinel without
// importing the repository package.
var ErrSessionNotFound = errors.New("auth session not found")

// Service is the public surface of the package. Construct once at boot
// with New, then call RegisterRoutes and Use(Middleware()).
type Service struct {
	cfg       Config
	users     UserStore
	sessions  SessionStore
	verifier  *oidc.IDTokenVerifier
	oauth2cfg *oauth2.Config
	twilio    *twilioClient
	smsLimit  *rateLimiter
	now       func() time.Time
}

// Default values applied when Config leaves a field zero.
const (
	defaultSessionTTL            = 30 * 24 * time.Hour
	defaultSessionAbsoluteMaxAge = 90 * 24 * time.Hour
	minSessionSecretBytes        = 32
)

// New constructs a Service. Eager validation (secret length, required
// strings) so misconfiguration fails at boot instead of on the first auth
// request. The ctx is used to fetch Google's OIDC discovery document; pass
// a context with a sensible timeout (e.g. 10s).
func New(ctx context.Context, cfg Config, users UserStore, sessions SessionStore) (*Service, error) {
	if users == nil {
		return nil, errors.New("auth.New: UserStore is required")
	}
	if sessions == nil {
		return nil, errors.New("auth.New: SessionStore is required")
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
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.Google.ClientID})

	oauth2cfg := &oauth2.Config{
		ClientID:     cfg.Google.ClientID,
		ClientSecret: cfg.Google.ClientSecret,
		RedirectURL:  cfg.Google.RedirectURL,
		Endpoint:     googleoauth.Endpoint,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}

	twilio := newTwilioClient(cfg.Twilio)

	// Per-process token bucket. Documented gap (see README): when the API
	// scales beyond one machine, attackers can scale attempts by parallel
	// hitting different machines. Compensating controls: Twilio Verify's
	// own per-phone limits + fraud-detection settings, and cost monitoring.
	smsLimit := newRateLimiter()

	return &Service{
		cfg:       cfg,
		users:     users,
		sessions:  sessions,
		verifier:  verifier,
		oauth2cfg: oauth2cfg,
		twilio:    twilio,
		smsLimit:  smsLimit,
		now:       cfg.Now,
	}, nil
}

// RegisterRoutes mounts the auth endpoints onto the given gin router under
// /auth/*. Idempotent. Mount BEFORE the existing JWT middleware in
// api/api.go so unauthenticated requests to these routes (sign-in, OAuth
// callback) aren't rejected.
func (s *Service) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/auth")

	g.GET("/google/start", s.handleGoogleStart)
	g.GET("/google/callback", s.handleGoogleCallback)

	g.POST("/sms/send", s.requireOrigin(), s.handleSmsSend)
	g.POST("/sms/verify", s.requireOrigin(), s.handleSmsVerify)

	g.POST("/sign-out", s.requireOrigin(), s.handleSignOut)
	g.GET("/session", s.handleGetSession)
}

// SessionStoreCleanup runs DeleteExpired in a background goroutine on the
// given interval until ctx is cancelled. Caller is responsible for kicking
// it off in main; the auth package itself doesn't spawn goroutines on its
// own. Logs failures via the standard Go log package (callers can wrap if
// they want structured logging).
func (s *Service) RunSessionCleanup(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			n, err := s.sessions.DeleteExpired(deleteCtx, s.now())
			cancel()
			if err != nil {
				logf("session cleanup failed: %v", err)
				continue
			}
			if n > 0 {
				logf("session cleanup: deleted %d expired rows", n)
			}
		}
	}
}
