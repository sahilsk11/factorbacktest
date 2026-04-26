package util

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/logger"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func NewTestDb() (*sql.DB, error) {
	connStr := "postgresql://postgres:postgres@localhost:5440/postgres_test?sslmode=disable"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return dbConn, nil
}

func Pprint(i interface{}) {
	bytes, err := json.MarshalIndent(i, "", "    ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}

func StringPointer(s string) *string {
	return &s
}

func FloatPointer(f float64) *float64 {
	return &f
}

func TimePointer(t time.Time) *time.Time {
	return &t
}

func BoolPointer(b bool) *bool {
	return &b
}

func DecimalPointer(d decimal.Decimal) *decimal.Decimal {
	return &d
}

func TradeOrderSidePointer(m model.TradeOrderSide) *model.TradeOrderSide {
	return &m
}

func UUIDPointer(u uuid.UUID) *uuid.UUID {
	return &u
}

type Secrets struct {
	Port             int           `json:"port"`
	DataJockeyApiKey string        `json:"dataJockey"`
	ChatGPTApiKey    string        `json:"gpt"`
	Db               DbSecrets     `json:"db"`
	Alpaca           AlpacaSecrets `json:"alpaca"`
	SES              SESSecrets    `json:"ses"`
	Resend           ResendSecrets `json:"resend"`
	Auth             AuthSecrets   `json:"auth"`
}

// AuthSecrets backs the custom Go auth package in internal/auth. All values
// are required when the API process is configured to mount /auth/* routes
// (i.e. has both a Google client and a Twilio Verify service to call).
type AuthSecrets struct {
	// SessionSecret is the HMAC-SHA256 key used to sign session cookies.
	// Generate with `openssl rand -hex 32`. Rotating it logs every user
	// out (existing cookies fail HMAC verification).
	SessionSecret string `json:"sessionSecret"`
	// GoogleClientID / GoogleClientSecret come from Google Cloud Console.
	GoogleClientID     string `json:"googleClientId"`
	GoogleClientSecret string `json:"googleClientSecret"`
	// TwilioAccountSID / TwilioAuthToken authenticate REST calls to Twilio.
	// TwilioVerifyServiceSID identifies the Verify service that owns the
	// SMS template, fraud-protection settings, and rate limits.
	TwilioAccountSID       string `json:"twilioAccountSid"`
	TwilioAuthToken        string `json:"twilioAuthToken"`
	TwilioVerifyServiceSID string `json:"twilioVerifyServiceSid"`
}

type AlpacaSecrets struct {
	ApiKey    string `json:"apiKey"`
	ApiSecret string `json:"apiSecret"`
	Endpoint  string `json:"endpoint"`
}

type DbSecrets struct {
	Host      string `json:"host"`
	User      string `json:"user"`
	Port      string `json:"port"`
	Password  string `json:"password"`
	Database  string `json:"database"`
	EnableSsl bool   `json:"enableSsl"`
}

type SESSecrets struct {
	Region    string `json:"region"`    // e.g., "us-east-1"
	FromEmail string `json:"fromEmail"` // e.g., "noreply@factor.trade"
}

// ResendSecrets backs the Resend transactional-email + email-OTP path
// (internal/repository/resend_email.repository.go and the email-OTP
// handlers in internal/auth/email_otp.go). FromEmail must be on a
// domain that's been verified in the Resend dashboard.
type ResendSecrets struct {
	APIKey    string `json:"apiKey"`    // re_xxx from https://resend.com/api-keys
	FromEmail string `json:"fromEmail"` // e.g., "noreply@factor.trade"
	FromName  string `json:"fromName"`  // e.g., "Factor"; optional display name
}

func (t DbSecrets) ToConnectionStr() string {
	x := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s",
		t.Host, t.Port, t.User, t.Password, t.Database)
	if !t.EnableSsl {
		x += " sslmode=disable"
	}
	return x
}

func LoadSecrets() (*Secrets, error) {
	// Opt-in path for environments that inject secrets as env vars (e.g. Fly.io).
	// FB_SECRETS_FROM_ENV=1 is an explicit signal, so a failure here is terminal:
	// falling through to the file chain would obscure the real cause (a
	// missing or typo'd Fly secret).
	if os.Getenv("FB_SECRETS_FROM_ENV") == "1" {
		secrets, err := loadSecretsFromEnv()
		if err != nil {
			return nil, fmt.Errorf("FB_SECRETS_FROM_ENV=1 but loading from env failed: %w", err)
		}
		logger.New().Infof("loaded secrets from env vars")
		return secrets, nil
	}

	var fileErr error
	for _, path := range secretsFileCandidates() {
		secrets, err := loadSecretsFromFile(path)
		if err == nil {
			return secrets, nil
		}
		fileErr = err
	}

	if fileErr == nil {
		fileErr = errors.New("no secrets file candidates configured")
	}
	return nil, fmt.Errorf("failed to load secrets from local files: %v", fileErr)
}

func secretsFileCandidates() []string {
	// Keep compatibility with both local dev and container paths.
	switch os.Getenv("ALPHA_ENV") {
	case "dev":
		return []string{"secrets-dev.json", "/go/src/app/secrets-dev.json"}
	case "test":
		return []string{"secrets-test.json", "/go/src/app/secrets-test.json"}
	case "prod":
		return []string{"secrets.json", "/go/src/app/secrets.json"}
	default:
		return []string{"/go/src/app/secrets.json", "secrets.json"}
	}
}

func loadSecretsFromFile(path string) (*Secrets, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("secrets file %q does not exist", path)
		}
		return nil, fmt.Errorf("could not read secrets file %q: %w", path, err)
	}

	secrets := Secrets{}
	if err := json.Unmarshal(f, &secrets); err != nil {
		return nil, fmt.Errorf("could not parse secrets file %q: %w", path, err)
	}

	return &secrets, nil
}

// loadSecretsFromEnv reads secrets directly from process env vars. The Fly
// console flattened our nested secrets.json so each JSON leaf field became
// its own env-var secret with the original camelCase name (e.g. dataJockey,
// host, apiKey). Linux env vars are case-sensitive, so these lowercase names
// don't collide with anything Fly or Docker auto-inject (PORT, HOSTNAME, etc.).
func loadSecretsFromEnv() (*Secrets, error) {
	get := func(name string) string { return os.Getenv(name) }

	required := map[string]string{
		"dataJockey": get("dataJockey"),
		"gpt":        get("gpt"),
		"host":       get("host"),
		"port":       get("port"),
		"user":       get("user"),
		"password":   get("password"),
		"database":   get("database"),
		"apiKey":     get("apiKey"),
		"apiSecret":  get("apiSecret"),
		"endpoint":   get("endpoint"),
	}
	var missing []string
	for k, v := range required {
		if v == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %v", missing)
	}

	enableSsl := true
	if v := get("enableSsl"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid enableSsl=%q: %w", v, err)
		}
		enableSsl = parsed
	}

	// Auth secrets are optional in env-loading: the Go API can boot without
	// them (e.g. local dev that doesn't exercise /auth/*). The auth package
	// itself fails fast at New() if it's wired up with empty values, so a
	// misconfigured prod deploy still surfaces loudly.
	auth := AuthSecrets{
		SessionSecret:          get("sessionSecret"),
		GoogleClientID:         get("googleClientId"),
		GoogleClientSecret:     get("googleClientSecret"),
		TwilioAccountSID:       get("twilioAccountSid"),
		TwilioAuthToken:        get("twilioAuthToken"),
		TwilioVerifyServiceSID: get("twilioVerifyServiceSid"),
	}

	return &Secrets{
		DataJockeyApiKey: required["dataJockey"],
		ChatGPTApiKey:    required["gpt"],
		Db: DbSecrets{
			Host:      required["host"],
			Port:      required["port"],
			User:      required["user"],
			Password:  required["password"],
			Database:  required["database"],
			EnableSsl: enableSsl,
		},
		Alpaca: AlpacaSecrets{
			ApiKey:    required["apiKey"],
			ApiSecret: required["apiSecret"],
			Endpoint:  required["endpoint"],
		},
		SES: SESSecrets{
			// Optional after the Resend migration. Only consumed when
			// EMAIL_PROVIDER=ses in cmd/util.go.
			Region:    get("region"),
			FromEmail: get("fromEmail"),
		},
		Resend: ResendSecrets{
			// Resend is the default email provider after the SES
			// migration. Empty values are tolerated here so a binary
			// that doesn't need email (e.g. local dev) still boots;
			// cmd/util.go fails loudly if the active provider needs
			// values that aren't set.
			APIKey:    get("resend_apiKey"),
			FromEmail: get("resend_fromEmail"),
			FromName:  get("resend_fromName"),
		},
		Auth: auth,
	}, nil
}

func HashFactorExpression(in string) string {
	regex := regexp.MustCompile(`\s+`)
	cleanedExpression := regex.ReplaceAllString(in, "")
	expressionHasher := sha256.New()
	expressionHasher.Write([]byte(cleanedExpression))
	expressionHash := hex.EncodeToString(expressionHasher.Sum(nil))

	return expressionHash
}
