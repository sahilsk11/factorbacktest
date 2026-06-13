package util

import (
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("dataJockey", "data-jockey")
	t.Setenv("gpt", "gpt")
	t.Setenv("apiKey", "alpaca-key")
	t.Setenv("apiSecret", "alpaca-secret")
	t.Setenv("endpoint", "alpaca-endpoint")
}

func TestLoadSecretsFromEnvPrefersDatabaseURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DATABASE_URL", "postgresql://user:pass@ep-test.us-east-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require")

	secrets, err := loadSecretsFromEnv()
	if err != nil {
		t.Fatalf("loadSecretsFromEnv() error = %v", err)
	}

	connStr := secrets.Db.ToConnectionStr()
	if !strings.Contains(connStr, "ep-test.us-east-1.aws.neon.tech") {
		t.Fatalf("connection string did not use DATABASE_URL: %q", connStr)
	}
	if strings.Contains(connStr, "channel_binding") {
		t.Fatalf("connection string still contains channel_binding: %q", connStr)
	}
	if !strings.Contains(connStr, "sslmode=require") {
		t.Fatalf("connection string should preserve sslmode=require: %q", connStr)
	}
}

func TestLoadSecretsFromEnvFallsBackToSplitDBFields(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("host", "db.example.com")
	t.Setenv("port", "5432")
	t.Setenv("user", "db-user")
	t.Setenv("password", "db-pass")
	t.Setenv("database", "app-db")
	t.Setenv("enableSsl", "false")

	secrets, err := loadSecretsFromEnv()
	if err != nil {
		t.Fatalf("loadSecretsFromEnv() error = %v", err)
	}

	want := "host=db.example.com port=5432 user=db-user password=db-pass dbname=app-db sslmode=disable"
	if got := secrets.Db.ToConnectionStr(); got != want {
		t.Fatalf("ToConnectionStr() = %q, want %q", got, want)
	}
}

func TestMigrationConnectionStrPrefersMigrateDatabaseURL(t *testing.T) {
	t.Setenv("MIGRATE_DATABASE_URL", "postgresql://migrator:pass@ep-migrate.us-east-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require")

	secrets := Secrets{
		Db: DbSecrets{
			URL: "postgresql://app:pass@ep-app.us-east-1.aws.neon.tech/neondb?sslmode=require",
		},
	}

	connStr := MigrationConnectionStr(secrets)
	if !strings.Contains(connStr, "ep-migrate.us-east-1.aws.neon.tech") {
		t.Fatalf("MigrationConnectionStr() did not use MIGRATE_DATABASE_URL: %q", connStr)
	}
	if strings.Contains(connStr, "channel_binding") {
		t.Fatalf("migration connection string still contains channel_binding: %q", connStr)
	}
}
