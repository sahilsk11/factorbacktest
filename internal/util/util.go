package util

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"factorbacktest/internal/db/models/postgres/public/model"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
	DataJockeyApiKey string        `json:"dataJockey"`
	ChatGPTApiKey    string        `json:"gpt"`
	Db               DbSecrets     `json:"db"`
	Alpaca           AlpacaSecrets `json:"alpaca"`
	Jwt              string        `json:"jwt"`
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

func (t DbSecrets) ToConnectionStr() string {
	x := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s",
		t.Host, t.Port, t.User, t.Password, t.Database)
	if !t.EnableSsl {
		x += " sslmode=disable"
	}
	return x
}

func NewTestDb() (*sql.DB, error) {
	connStr := "postgresql://postgres:postgres@localhost:5440/postgres_test?sslmode=disable"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return dbConn, nil
}

func LoadSecrets() (*Secrets, error) {
	secretsFile := "/go/src/app/secrets.json"
	if os.Getenv("ALPHA_ENV") == "dev" {
		secretsFile = "secrets-dev.json"
	} else if os.Getenv("ALPHA_ENV") == "test" {
		secretsFile = "secrets-test.json"
	} else if os.Getenv("ALPHA_ENV") == "prod" {
		secretsFile = "secrets.json"
	}
	f, err := os.ReadFile(secretsFile)
	if err != nil {
		return nil, fmt.Errorf("could not open secrets.json: %w", err)
	}

	secrets := Secrets{}
	err = json.Unmarshal(f, &secrets)
	if err != nil {
		return nil, err
	}

	return &secrets, nil
}

func HashFactorExpression(in string) string {
	regex := regexp.MustCompile(`\s+`)
	cleanedExpression := regex.ReplaceAllString(in, "")
	expressionHasher := sha256.New()
	expressionHasher.Write([]byte(cleanedExpression))
	expressionHash := hex.EncodeToString(expressionHasher.Sum(nil))

	return expressionHash
}
