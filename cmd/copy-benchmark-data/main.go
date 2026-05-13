package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"factorbacktest/internal/util"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

func main() {
	sourceURL := flag.String("source-url", os.Getenv("SOURCE_DATABASE_URL"), "source Postgres connection string")
	targetURL := flag.String("target-url", os.Getenv("TARGET_DATABASE_URL"), "target Postgres connection string")
	universe := flag.String("universe", "SPY_TOP_300", "asset universe name to copy")
	start := flag.String("start", "2021-10-01", "first adjusted_price date to copy")
	end := flag.String("end", "2026-05-10", "last adjusted_price date to copy")
	skipRefData := flag.Bool("skip-ref-data", false, "skip asset_universe, ticker, and membership copy")
	skipPrices := flag.Bool("skip-prices", false, "skip adjusted_price copy")
	replacePrices := flag.Bool("replace-prices", false, "delete target adjusted_price rows for the copied symbols/date window before copying")
	chunkDays := flag.Int("chunk-days", 31, "date-window size for adjusted_price and factor_score copies")
	factorExpression := flag.String("factor-expression", "", "optional factor expression whose cached factor_score rows should be copied")
	replaceFactorScores := flag.Bool("replace-factor-scores", false, "delete target factor_score rows for the copied expression/date window before copying")
	flag.Parse()

	if *sourceURL == "" || *targetURL == "" {
		log.Fatal("-source-url and -target-url are required")
	}
	startDate, err := time.Parse(time.DateOnly, *start)
	if err != nil {
		log.Fatal(err)
	}
	endDate, err := time.Parse(time.DateOnly, *end)
	if err != nil {
		log.Fatal(err)
	}

	source, err := sql.Open("postgres", *sourceURL)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	target, err := sql.Open("postgres", *targetURL)
	if err != nil {
		log.Fatal(err)
	}
	defer target.Close()
	source.SetMaxOpenConns(2)
	target.SetMaxOpenConns(2)

	log.Printf("connecting to source and target")
	ctx := context.Background()
	if err := source.PingContext(ctx); err != nil {
		log.Fatalf("source ping: %v", err)
	}
	if err := target.PingContext(ctx); err != nil {
		log.Fatalf("target ping: %v", err)
	}
	log.Printf("connections ready")

	var symbols []string
	if !*skipRefData {
		log.Printf("copying universe metadata")
		if err := copyUniverse(ctx, source, target, *universe); err != nil {
			log.Fatal(err)
		}
		log.Printf("copying tickers")
		symbols, err = copyTickers(ctx, source, target, *universe)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("copying universe memberships")
		if err := copyUniverseMemberships(ctx, source, target, *universe); err != nil {
			log.Fatal(err)
		}
	} else {
		symbols, err = readTargetUniverseSymbols(ctx, target, *universe)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("using %d existing target symbols for %s", len(symbols), *universe)
	}
	copied := 0
	if !*skipPrices {
		if *replacePrices {
			deleted, err := deleteTargetAdjustedPrices(ctx, target, symbols, startDate, endDate)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("deleted %d target adjusted_price rows for %s..%s", deleted, startDate.Format(time.DateOnly), endDate.Format(time.DateOnly))
		}
		log.Printf("copying adjusted prices")
		copied, err = copyAdjustedPrices(ctx, source, target, symbols, startDate, endDate, *chunkDays)
		if err != nil {
			log.Fatal(err)
		}
	}
	copiedScores := 0
	if *factorExpression != "" {
		hash := util.HashFactorExpression(*factorExpression)
		if *replaceFactorScores {
			deleted, err := deleteTargetFactorScores(ctx, target, symbols, startDate, endDate, hash)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("deleted %d target factor_score rows for hash %s", deleted, hash)
		}
		log.Printf("copying factor_score cache for hash %s", hash)
		copiedScores, err = copyFactorScores(ctx, source, target, symbols, startDate, endDate, hash, *chunkDays)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("copied benchmark data: universe=%s tickers=%d adjusted_prices=%d factor_scores=%d", *universe, len(symbols), copied, copiedScores)
}

func readTargetUniverseSymbols(ctx context.Context, target *sql.DB, universe string) ([]string, error) {
	rows, err := target.QueryContext(ctx, `
		SELECT t.symbol
		FROM ticker t
		JOIN asset_universe_ticker aut ON aut.ticker_id = t.ticker_id
		JOIN asset_universe au ON au.asset_universe_id = aut.asset_universe_id
		WHERE au.asset_universe_name = $1
		ORDER BY t.symbol
	`, universe)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return symbols, nil
}

func copyUniverse(ctx context.Context, source, target *sql.DB, universe string) error {
	row := source.QueryRowContext(ctx, `
		SELECT asset_universe_id::text, asset_universe_name, display_name
		FROM asset_universe
		WHERE asset_universe_name = $1
	`, universe)
	var id, name, displayName string
	if err := row.Scan(&id, &name, &displayName); err != nil {
		return fmt.Errorf("read universe %q: %w", universe, err)
	}
	_, err := target.ExecContext(ctx, `
		INSERT INTO asset_universe (asset_universe_id, asset_universe_name, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (asset_universe_name) DO UPDATE
		SET display_name = EXCLUDED.display_name
	`, id, name, displayName)
	return err
}

func copyTickers(ctx context.Context, source, target *sql.DB, universe string) ([]string, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT t.ticker_id::text, t.symbol, t.name
		FROM ticker t
		JOIN asset_universe_ticker aut ON aut.ticker_id = t.ticker_id
		JOIN asset_universe au ON au.asset_universe_id = aut.asset_universe_id
		WHERE au.asset_universe_name = $1
		ORDER BY t.symbol
	`, universe)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var symbols []string
	for rows.Next() {
		var id, symbol, name string
		if err := rows.Scan(&id, &symbol, &name); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ticker (ticker_id, symbol, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (symbol) DO UPDATE
			SET name = EXCLUDED.name
		`, id, symbol, name); err != nil {
			return nil, err
		}
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return symbols, nil
}

func copyUniverseMemberships(ctx context.Context, source, target *sql.DB, universe string) error {
	rows, err := source.QueryContext(ctx, `
		SELECT t.symbol
		FROM asset_universe_ticker aut
		JOIN ticker t ON t.ticker_id = aut.ticker_id
		JOIN asset_universe au ON au.asset_universe_id = aut.asset_universe_id
		WHERE au.asset_universe_name = $1
	`, universe)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO asset_universe_ticker (ticker_id, asset_universe_id)
			SELECT t.ticker_id, au.asset_universe_id
			FROM ticker t
			CROSS JOIN asset_universe au
			WHERE t.symbol = $1
			  AND au.asset_universe_name = $2
			ON CONFLICT (ticker_id, asset_universe_id) DO NOTHING
		`, symbol, universe); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

func deleteTargetAdjustedPrices(ctx context.Context, target *sql.DB, symbols []string, start, end time.Time) (int, error) {
	result, err := target.ExecContext(ctx, `
		DELETE FROM adjusted_price
		WHERE symbol = ANY($1)
		  AND date BETWEEN $2 AND $3
	`, pq.Array(symbols), start, end)
	if err != nil {
		return 0, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

func copyAdjustedPrices(ctx context.Context, source, target *sql.DB, symbols []string, start, end time.Time, chunkDays int) (int, error) {
	total := 0
	for chunkStart := start; !chunkStart.After(end); chunkStart = chunkStart.AddDate(0, 0, chunkDays) {
		chunkEnd := chunkStart.AddDate(0, 0, chunkDays-1)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		n, err := copyAdjustedPriceChunk(ctx, source, target, symbols, chunkStart, chunkEnd)
		if err != nil {
			return total, err
		}
		total += n
		log.Printf("copied adjusted_price %s..%s rows=%d total=%d", chunkStart.Format(time.DateOnly), chunkEnd.Format(time.DateOnly), n, total)
	}
	return total, nil
}

func copyAdjustedPriceChunk(ctx context.Context, source, target *sql.DB, symbols []string, start, end time.Time) (int, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT date, symbol, price::text, created_at
		FROM adjusted_price
		WHERE symbol = ANY($1)
		  AND date BETWEEN $2 AND $3
		ORDER BY symbol, date
	`, pq.Array(symbols), start, end)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	const batchSize = 1000
	var batch []adjustedPrice
	total := 0
	for rows.Next() {
		var p adjustedPrice
		if err := rows.Scan(&p.date, &p.symbol, &p.price, &p.createdAt); err != nil {
			return total, err
		}
		batch = append(batch, p)
		if len(batch) >= batchSize {
			n, err := insertAdjustedPriceBatch(ctx, target, batch)
			if err != nil {
				return total, err
			}
			total += n
			batch = batch[:0]
		}
	}
	if err := rows.Err(); err != nil {
		return total, err
	}
	if len(batch) > 0 {
		n, err := insertAdjustedPriceBatch(ctx, target, batch)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

type adjustedPrice struct {
	date      time.Time
	symbol    string
	price     string
	createdAt time.Time
}

func insertAdjustedPriceBatch(ctx context.Context, db *sql.DB, batch []adjustedPrice) (int, error) {
	var b strings.Builder
	args := make([]any, 0, len(batch)*4)
	b.WriteString(`INSERT INTO adjusted_price (date, symbol, price, created_at) VALUES `)
	for i, p := range batch {
		if i > 0 {
			b.WriteString(",")
		}
		base := i*4 + 1
		fmt.Fprintf(&b, "($%d,$%d,$%d,$%d)", base, base+1, base+2, base+3)
		args = append(args, p.date, p.symbol, p.price, p.createdAt)
	}
	b.WriteString(` ON CONFLICT (date, symbol) DO UPDATE SET price = EXCLUDED.price, created_at = EXCLUDED.created_at`)
	if _, err := db.ExecContext(ctx, b.String(), args...); err != nil {
		return 0, err
	}
	return len(batch), nil
}

func deleteTargetFactorScores(ctx context.Context, target *sql.DB, symbols []string, start, end time.Time, hash string) (int, error) {
	result, err := target.ExecContext(ctx, `
		DELETE FROM factor_score fs
		USING ticker t
		WHERE t.ticker_id = fs.ticker_id
		  AND t.symbol = ANY($1)
		  AND fs.factor_expression_hash = $2
		  AND fs.date BETWEEN $3 AND $4
	`, pq.Array(symbols), hash, start, end)
	if err != nil {
		return 0, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

func copyFactorScores(ctx context.Context, source, target *sql.DB, symbols []string, start, end time.Time, hash string, chunkDays int) (int, error) {
	total := 0
	for chunkStart := start; !chunkStart.After(end); chunkStart = chunkStart.AddDate(0, 0, chunkDays) {
		chunkEnd := chunkStart.AddDate(0, 0, chunkDays-1)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		n, err := copyFactorScoreChunk(ctx, source, target, symbols, chunkStart, chunkEnd, hash)
		if err != nil {
			return total, err
		}
		total += n
		log.Printf("copied factor_score %s..%s rows=%d total=%d", chunkStart.Format(time.DateOnly), chunkEnd.Format(time.DateOnly), n, total)
	}
	return total, nil
}

func copyFactorScoreChunk(ctx context.Context, source, target *sql.DB, symbols []string, start, end time.Time, hash string) (int, error) {
	log.Printf("querying source factor_score rows")
	rows, err := source.QueryContext(ctx, `
		SELECT t.symbol, fs.factor_expression_hash, fs.date, fs.score, fs.error, fs.created_at, fs.updated_at
		FROM factor_score fs
		JOIN ticker t ON t.ticker_id = fs.ticker_id
		WHERE fs.factor_expression_hash = $1
		  AND fs.date BETWEEN $2 AND $3
		  AND t.symbol = ANY($4)
	`, hash, start, end, pq.Array(symbols))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	log.Printf("streaming source factor_score rows")
	const batchSize = 2000
	count := 0
	batch := []factorScore{}
	for rows.Next() {
		var fs factorScore
		if err := rows.Scan(&fs.symbol, &fs.expressionHash, &fs.date, &fs.score, &fs.scoreErr, &fs.createdAt, &fs.updatedAt); err != nil {
			return count, err
		}
		batch = append(batch, fs)
		if len(batch) >= batchSize {
			n, err := insertFactorScoreBatch(ctx, target, batch)
			if err != nil {
				return count, err
			}
			count += n
			log.Printf("copied factor_score rows: %d", count)
			batch = batch[:0]
		}
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	if len(batch) > 0 {
		n, err := insertFactorScoreBatch(ctx, target, batch)
		if err != nil {
			return count, err
		}
		count += n
	}
	return count, nil
}

type factorScore struct {
	symbol         string
	expressionHash string
	date           time.Time
	score          sql.NullFloat64
	scoreErr       sql.NullString
	createdAt      time.Time
	updatedAt      time.Time
}

func insertFactorScoreBatch(ctx context.Context, db *sql.DB, batch []factorScore) (int, error) {
	var b strings.Builder
	args := make([]any, 0, len(batch)*7)
	b.WriteString(`
		INSERT INTO factor_score (ticker_id, factor_expression_hash, date, score, error, created_at, updated_at)
		SELECT t.ticker_id, v.factor_expression_hash, v.date, v.score, v.error, v.created_at, v.updated_at
		FROM (VALUES `)
	for i, fs := range batch {
		if i > 0 {
			b.WriteString(",")
		}
		base := i*7 + 1
		fmt.Fprintf(&b, "($%d::text,$%d::text,$%d::date,$%d::double precision,$%d::text,$%d::timestamptz,$%d::timestamptz)", base, base+1, base+2, base+3, base+4, base+5, base+6)
		args = append(args, fs.symbol, fs.expressionHash, fs.date, nullableFloat(fs.score), nullableString(fs.scoreErr), fs.createdAt, fs.updatedAt)
	}
	b.WriteString(`) AS v(symbol, factor_expression_hash, date, score, error, created_at, updated_at)
		JOIN ticker t ON t.symbol = v.symbol
		ON CONFLICT (factor_expression_hash, ticker_id, date) DO UPDATE
		SET score = EXCLUDED.score,
		    error = EXCLUDED.error,
		    created_at = EXCLUDED.created_at,
		    updated_at = EXCLUDED.updated_at`)
	if _, err := db.ExecContext(ctx, b.String(), args...); err != nil {
		return 0, err
	}
	return len(batch), nil
}

func nullableFloat(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func nullableString(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}
