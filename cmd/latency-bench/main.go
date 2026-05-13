package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type target struct {
	name string
	url  string
}

type sample struct {
	target string
	status int
	rows   int
	err    error
	dur    time.Duration
}

func main() {
	mode := flag.String("mode", "api", "benchmark mode: api or db")
	runs := flag.Int("runs", 5, "measured runs per target")
	warmups := flag.Int("warmups", 1, "unmeasured warm-up runs per target")
	timeout := flag.Duration("timeout", 5*time.Minute, "per-operation timeout")

	nameA := flag.String("name-a", "rds", "first target label")
	nameB := flag.String("name-b", "neon", "second target label")

	apiA := flag.String("api-a", os.Getenv("BENCH_API_A"), "first /backtest URL")
	apiB := flag.String("api-b", os.Getenv("BENCH_API_B"), "second /backtest URL")
	payloadPath := flag.String("payload", "", "JSON payload for api mode")
	cold := flag.Bool("cold", false, "perturb factor expression on every measured API run")

	dbA := flag.String("db-a", os.Getenv("BENCH_DB_A"), "first Postgres DSN")
	dbB := flag.String("db-b", os.Getenv("BENCH_DB_B"), "second Postgres DSN")
	query := flag.String("query", "select 1", "SQL query for db mode")
	queryFile := flag.String("query-file", "", "file containing SQL query for db mode")
	printRows := flag.Bool("print-rows", false, "print DB result rows for debugging")
	maxRows := flag.Int("max-rows", 20, "maximum DB result rows to print when -print-rows is set")
	flag.Parse()

	ctx := context.Background()
	switch *mode {
	case "api":
		if err := runAPI(ctx, []target{{*nameA, *apiA}, {*nameB, *apiB}}, *payloadPath, *runs, *warmups, *timeout, *cold); err != nil {
			log.Fatal(err)
		}
	case "db":
		sqlText := *query
		if *queryFile != "" {
			b, err := os.ReadFile(*queryFile)
			if err != nil {
				log.Fatal(err)
			}
			sqlText = string(b)
		}
		if err := runDB(ctx, []target{{*nameA, *dbA}, {*nameB, *dbB}}, sqlText, *runs, *warmups, *timeout, *printRows, *maxRows); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}

func runAPI(ctx context.Context, targets []target, payloadPath string, runs, warmups int, timeout time.Duration, cold bool) error {
	if payloadPath == "" {
		return fmt.Errorf("-payload is required in api mode")
	}
	base, err := os.ReadFile(payloadPath)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if t.url == "" {
			return fmt.Errorf("missing URL for target %s", t.name)
		}
	}

	client := &http.Client{Timeout: timeout}
	prefix := fmt.Sprintf("bench-%d", time.Now().UnixNano())

	for _, t := range targets {
		for i := 0; i < warmups; i++ {
			body, err := taggedPayload(base, fmt.Sprintf("%s-%s-warmup-%d", prefix, t.name, i), false, i)
			if err != nil {
				return err
			}
			_ = postJSON(ctx, client, t, body)
		}
	}

	var samples []sample
	for i := 0; i < runs; i++ {
		for _, t := range targets {
			body, err := taggedPayload(base, fmt.Sprintf("%s-%s-run-%d", prefix, t.name, i), cold, i)
			if err != nil {
				return err
			}
			s := postJSON(ctx, client, t, body)
			samples = append(samples, s)
			printSample(s)
		}
	}
	printSummary(samples)
	return nil
}

func taggedPayload(base []byte, nonce string, cold bool, run int) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(base, &payload); err != nil {
		return nil, err
	}
	factorOptions, ok := payload["factorOptions"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("payload.factorOptions must be an object")
	}
	factorOptions["name"] = nonce
	if cold {
		expr, ok := factorOptions["expression"].(string)
		if !ok {
			return nil, fmt.Errorf("payload.factorOptions.expression must be a string")
		}
		factorOptions["expression"] = fmt.Sprintf("%s + 0.%09d", expr, run+1)
	}
	return json.Marshal(payload)
}

func postJSON(ctx context.Context, client *http.Client, t target, body []byte) sample {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(body))
	if err != nil {
		return sample{target: t.name, err: err}
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		return sample{target: t.name, err: err, dur: dur}
	}
	defer resp.Body.Close()
	_, readErr := io.Copy(io.Discard, resp.Body)
	if readErr != nil {
		err = readErr
	}
	return sample{target: t.name, status: resp.StatusCode, err: err, dur: dur}
}

func runDB(ctx context.Context, targets []target, sqlText string, runs, warmups int, timeout time.Duration, printRows bool, maxRows int) error {
	for _, t := range targets {
		if t.url == "" {
			return fmt.Errorf("missing DSN for target %s", t.name)
		}
	}

	dbs := make(map[string]*sql.DB)
	for _, t := range targets {
		db, err := sql.Open("postgres", t.url)
		if err != nil {
			return err
		}
		defer db.Close()
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		dbs[t.name] = db
	}

	for _, t := range targets {
		for i := 0; i < warmups; i++ {
			_ = queryOnce(ctx, dbs[t.name], t.name, sqlText, timeout, false, maxRows)
		}
	}

	var samples []sample
	for i := 0; i < runs; i++ {
		for _, t := range targets {
			s := queryOnce(ctx, dbs[t.name], t.name, sqlText, timeout, printRows, maxRows)
			samples = append(samples, s)
			printSample(s)
		}
	}
	printSummary(samples)
	return nil
}

func queryOnce(ctx context.Context, db *sql.DB, name, sqlText string, timeout time.Duration, printRows bool, maxRows int) sample {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	rows, err := db.QueryContext(runCtx, sqlText)
	if err != nil {
		return sample{target: name, err: err, dur: time.Since(start)}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return sample{target: name, err: err, dur: time.Since(start)}
	}
	values := make([]any, len(cols))
	for i := range values {
		var raw sql.RawBytes
		values[i] = &raw
	}

	count := 0
	for rows.Next() {
		if err := rows.Scan(values...); err != nil {
			return sample{target: name, err: err, dur: time.Since(start), rows: count}
		}
		if printRows && count < maxRows {
			printDBRow(name, cols, values)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return sample{target: name, err: err, dur: time.Since(start), rows: count}
	}
	return sample{target: name, rows: count, dur: time.Since(start)}
}

func printDBRow(target string, cols []string, values []any) {
	fields := make([]string, 0, len(cols)+1)
	fields = append(fields, fmt.Sprintf("target=%s", target))
	for i, col := range cols {
		raw := *(values[i].(*sql.RawBytes))
		if raw == nil {
			fields = append(fields, fmt.Sprintf("%s=NULL", col))
			continue
		}
		fields = append(fields, fmt.Sprintf("%s=%q", col, string(raw)))
	}
	fmt.Printf("row %s\n", strings.Join(fields, " "))
}

func printSample(s sample) {
	status := ""
	if s.status != 0 {
		status = fmt.Sprintf(" http=%d", s.status)
	}
	rows := ""
	if s.rows != 0 {
		rows = fmt.Sprintf(" rows=%d", s.rows)
	}
	errText := ""
	if s.err != nil {
		errText = fmt.Sprintf(" err=%q", s.err)
	}
	fmt.Printf("sample target=%s%s%s total_ms=%.1f%s\n", s.target, status, rows, float64(s.dur.Microseconds())/1000, errText)
}

func printSummary(samples []sample) {
	byTarget := map[string][]time.Duration{}
	errorsByTarget := map[string]int{}
	for _, s := range samples {
		if s.err != nil || (s.status != 0 && (s.status < 200 || s.status >= 300)) {
			errorsByTarget[s.target]++
			continue
		}
		byTarget[s.target] = append(byTarget[s.target], s.dur)
	}

	var names []string
	for name := range byTarget {
		names = append(names, name)
	}
	for name := range errorsByTarget {
		if _, ok := byTarget[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	fmt.Println()
	fmt.Println("summary target n errors min_ms p50_ms p95_ms max_ms")
	for _, name := range names {
		durations := byTarget[name]
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		fmt.Printf(
			"summary target=%s n=%d errors=%d min_ms=%.1f p50_ms=%.1f p95_ms=%.1f max_ms=%.1f\n",
			name,
			len(durations),
			errorsByTarget[name],
			ms(minDuration(durations)),
			ms(quantile(durations, 0.50)),
			ms(quantile(durations, 0.95)),
			ms(maxDuration(durations)),
		)
	}
}

func quantile(values []time.Duration, q float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	idx := int(math.Ceil(q*float64(len(values)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func minDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func maxDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func init() {
	log.SetFlags(0)
	log.SetPrefix(strings.TrimSuffix(os.Args[0], "/") + ": ")
}
