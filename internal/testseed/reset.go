package testseed

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// resetTables is the list of user-data tables truncated by Reset. Order
// does not matter because Reset uses TRUNCATE ... CASCADE; the only reason
// to enumerate them explicitly is to deliberately exclude schema_version
// and to make it obvious that adding a new table should update this list
// (CI covers the integration tests that would otherwise silently drift).
//
// Keep this list in sync with migrations/. Use the list produced by:
//   rg -i "^create table" migrations/*.up.sql
var resetTables = []string{
	"adjusted_price",
	"api_request",
	"api_request_latency",
	"asset_fundamental",
	"asset_universe",
	"asset_universe_ticker",
	"contact_message",
	"email_frequency",
	"email_preference",
	"email_type",
	"excess_trade_volume",
	"factor_score",
	"interest_rate",
	"investment",
	"investment_holdings",
	"investment_holdings_version",
	"investment_rebalance",
	"investment_rebalance_error",
	"investment_trade",
	"latency_tracking",
	"published_strategy",
	"published_strategy_holdings",
	"published_strategy_holdings_version",
	"published_strategy_stats",
	"rebalance_price",
	"rebalancer_run",
	"strategy",
	"strategy_run",
	"ticker",
	"trade_order",
	"user_account",
	"user_strategy",
}

// Reset truncates every user-data table and re-seeds the rows that
// migrations are expected to have inserted (currently just the :CASH
// ticker). It leaves schema_version alone so migrations are not re-applied.
func Reset(ctx context.Context, db *sql.DB) error {
	quoted := make([]string, len(resetTables))
	for i, t := range resetTables {
		quoted[i] = `"` + t + `"`
	}
	stmt := fmt.Sprintf(
		"TRUNCATE TABLE %s RESTART IDENTITY CASCADE",
		strings.Join(quoted, ", "),
	)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	// Migrations/000020 inserts this row; restore it so fixtures that
	// reference :CASH (e.g. investment_basic) continue to work after reset.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO ticker (symbol, name) VALUES (':CASH', 'cash')`,
	); err != nil {
		return fmt.Errorf("restore :CASH ticker: %w", err)
	}
	return nil
}
