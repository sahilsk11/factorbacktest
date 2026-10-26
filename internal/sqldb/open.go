package sqldb

import (
	"context"
	"database/sql"
	"factorbacktest/internal/util"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Default pool tuning for the long-lived API process against managed Postgres
// (Neon scale-to-zero). See cmd/util.go comments for rationale.
const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 3
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 45 * time.Second
	defaultConnectTimeout  = 10 // seconds (lib/pq connect_timeout)
	defaultPingTimeout     = 15 * time.Second
)

// Open connects to Postgres with driver timeouts and pool limits suited to Neon.
// PingContext verifies the pool can reach the database at process start (migrate
// release_command normally wakes compute before the web VM boots).
func Open(connStr string) (*sql.DB, error) {
	connStr = util.EnsurePostgresConnectTimeout(connStr, defaultConnectTimeout)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	ConfigurePool(db)

	ctx, cancel := context.WithTimeout(context.Background(), defaultPingTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	return db, nil
}

// ConfigurePool applies shared pool limits. Tests that open sql.DB directly can
// call this for parity with production.
func ConfigurePool(db *sql.DB) {
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)
	db.SetConnMaxIdleTime(defaultConnMaxIdleTime)
}
