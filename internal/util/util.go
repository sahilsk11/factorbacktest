package util

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

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
	Jwt              string        `json:"jwt"`
	SES              SESSecrets    `json:"ses"`
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
	// falling through to the AWS / file chain would obscure the real cause (a
	// missing or typo'd Fly secret) and risks loading the wrong secrets in any
	// future env that has both this flag and AWS creds present.
	if os.Getenv("FB_SECRETS_FROM_ENV") == "1" {
		secrets, envErr := loadSecretsFromEnv()
		if envErr != nil {
			return nil, fmt.Errorf("FB_SECRETS_FROM_ENV=1 but loading from env failed: %w", envErr)
		}
		logger.New().Infof("loaded secrets from env vars")
		return secrets, nil
	}

	// Default behavior: prefer AWS Secrets Manager, fall back to a local secrets file.
	secrets, awsErr := loadSecretsFromAWS()
	if awsErr == nil {
		return secrets, nil
	}

	logger.New().Errorf("failed to load secrets from AWS; falling back to local file: %s", awsErr.Error())

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
	return nil, fmt.Errorf("failed to load secrets from AWS (%v) and from local files (%v)", awsErr, fileErr)
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

func loadSecretsFromAWS() (*Secrets, error) {
	secretName := "prod/factor"
	region := "us-east-1"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	svc := secretsmanager.NewFromConfig(config)

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := svc.GetSecretValue(ctx, input)
	if err != nil {
		// For a list of exceptions thrown, see
		// https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
		return nil, fmt.Errorf("failed to get secret %q from Secrets Manager: %w", secretName, err)
	}

	var secretBytes []byte
	if result.SecretString != nil {
		secretBytes = []byte(*result.SecretString)
	} else if len(result.SecretBinary) > 0 {
		// SDK usually returns raw bytes; some setups store base64-encoded JSON.
		secretBytes = result.SecretBinary
	} else {
		return nil, fmt.Errorf("secret %q from Secrets Manager had no SecretString or SecretBinary", secretName)
	}

	secrets := Secrets{}
	if err := json.Unmarshal(secretBytes, &secrets); err != nil {
		// If SecretBinary was base64-encoded JSON, try decoding then unmarshalling.
		decoded, decErr := base64.StdEncoding.DecodeString(string(secretBytes))
		if decErr == nil {
			if err2 := json.Unmarshal(decoded, &secrets); err2 == nil {
				return &secrets, nil
			}
		}
		return nil, fmt.Errorf("failed to unmarshal secret from Secrets Manager: %w", err)
	}

	logger.New().Infof("loaded secrets from Secrets Manager")

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
		"jwt":        get("jwt"),
		"host":       get("host"),
		"port":       get("port"),
		"user":       get("user"),
		"password":   get("password"),
		"database":   get("database"),
		"apiKey":     get("apiKey"),
		"apiSecret":  get("apiSecret"),
		"endpoint":   get("endpoint"),
		"region":     get("region"),
		"fromEmail":  get("fromEmail"),
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

	return &Secrets{
		DataJockeyApiKey: required["dataJockey"],
		ChatGPTApiKey:    required["gpt"],
		Jwt:              required["jwt"],
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
			Region:    required["region"],
			FromEmail: required["fromEmail"],
		},
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
