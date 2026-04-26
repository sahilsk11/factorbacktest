# Latency investigation runbook

This doc is the playbook for diagnosing where time goes inside a `/backtest`
request on the live Fly deployment. It exists so an agent (or a human) can
land here, follow the steps, and produce a defensible "the bottleneck is
span X at p95 = Y ms" report — without re-deriving the methodology each
time.

The goal of this exercise is **always observability first, optimization
second**. End every investigation with a hypothesis and a span name. Do not
end one with a 30-line code change unless explicitly asked.

## What you have to work with

| Surface                              | What it gives you                                                   |
| ------------------------------------ | ------------------------------------------------------------------- |
| `https://api.factor.trade`           | The Fly deployment under test.                                      |
| `https://tgwmxgtk07.execute-api.us-east-1.amazonaws.com/prod` | The legacy AWS Lambda deployment, kept alive as a baseline. |
| `secrets.json` / `secrets-dev.json`  | Postgres creds for prod RDS (`alpha.cuutadkicrvi.us-east-2.rds.amazonaws.com`). |
| `api_request` table                  | Every request: `request_id`, `route`, `start_ts`, `duration_ms`, `version` (deploy SHA), `request_body`, `ip_address`. |
| `latency_tracking` table             | One row per request, jsonb tree of `(name, elapsed, subSpans)` spans. |
| `latency_span_stats` view            | Flat `(request_id, route, start_ts, version, span_path, depth, elapsed_ms)`. **Use this; it's why we did the work.** |
| `flyctl logs` / `flyctl status`      | Real-time process logs and machine state.                           |

The Fly egress-IP-to-RDS path measures ~22ms TCP RTT. The Lambda path
measures ~12ms. So the same code paying the same query count will be
~10ms-per-round-trip slower on Fly than on Lambda by physics alone — keep
that floor in mind when interpreting numbers.

## Why we kept the Lambda URL as a baseline

The Lambda backend (PR #117 removed the deploy pipeline but the function
itself is still hot) ran the same Go binary against the same RDS for years.
Its measured wall-clock numbers are the **closest thing we have to a
ground-truth lower bound** for any /backtest workload, because:

- Same source tree (commit `26911f2` is what's deployed on Lambda).
- Same RDS database. Same network neighborhood (us-east-1 → us-east-2 inside AWS).
- Different runtime (Lambda Graviton ARM, no scale-to-zero penalty after the
  first invocation).

So the rule is: **a Fly investigation is a success when Fly approaches or
beats Lambda's wall-clock for the same payload**. If Fly is much slower than
Lambda, the gap is either network (us-east-2 RDS over the public internet
from Fly Ashburn) or a real regression, and the spans will tell you which.

## The loop

```
              ┌────────────────────────────────────────────┐
              │  1. Stamp a unique nonce in the payload    │
              └────────────────────────────────────────────┘
                                   │
              ┌────────────────────────────────────────────┐
              │  2. curl Fly + curl Lambda, capture wall   │
              │     clock for both                          │
              └────────────────────────────────────────────┘
                                   │
              ┌────────────────────────────────────────────┐
              │  3. Find the request_id in api_request     │
              │     by matching the nonce                   │
              └────────────────────────────────────────────┘
                                   │
              ┌────────────────────────────────────────────┐
              │  4. SELECT * FROM latency_span_stats       │
              │     WHERE request_id = ...                  │
              └────────────────────────────────────────────┘
                                   │
              ┌────────────────────────────────────────────┐
              │  5. Repeat 1-4 for N runs, aggregate p50   │
              │     and p95 by span_path                    │
              └────────────────────────────────────────────┘
                                   │
              ┌────────────────────────────────────────────┐
              │  6. Pick the slowest leaf span. State a    │
              │     one-line hypothesis. Stop here.         │
              └────────────────────────────────────────────┘
```

## Setup (do this once per session)

```bash
# Postgres client. Skip if psql is already on PATH.
brew install libpq
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"

# DB env. Pulled from secrets.json at the repo root.
export PGHOST=alpha.cuutadkicrvi.us-east-2.rds.amazonaws.com
export PGUSER=postgres
export PGDATABASE=postgres
export PGPORT=5432
export PGPASSWORD="$(jq -r .db.password secrets.json)"

# Sanity: should print server time.
psql -c "select now();"

# Endpoints.
export FLY_URL='https://api.factor.trade/backtest'
export LAMBDA_URL='https://tgwmxgtk07.execute-api.us-east-1.amazonaws.com/prod/backtest'
```

## The sample payload

Drop this at `/tmp/payload.json`. It's a real factor expression
(monthly-rebalanced momentum + volatility, 3-symbol picks from `SPY_TOP_300`
across 3 years). It exercises the heavy paths: factor-score lookups,
adjusted-price scans, and the latest-holdings query.

```json
{
  "factorOptions": {
    "expression": "((pricePercentChange(addDate(currentDate, 0, -6, 0), currentDate) + pricePercentChange(addDate(currentDate, 0, -12, 0), currentDate) + pricePercentChange(addDate(currentDate, 0, -18, 0), currentDate) - 5 * pricePercentChange(addDate(currentDate, 0, 0, -7), currentDate))) * (2 * stdev(addDate(currentDate, -1, 0, 0), currentDate))",
    "name": "PLACEHOLDER_NONCE"
  },
  "backtestStart": "2023-04-26",
  "backtestEnd": "2026-04-26",
  "samplingIntervalUnit": "monthly",
  "startCash": 10000,
  "numSymbols": 3,
  "userID": "e689eff4-6a55-4e86-b4bf-90134a8a8a02",
  "assetUniverse": "SPY_TOP_300"
}
```

Replace `PLACEHOLDER_NONCE` per request — that's how we find the row later.

## Warm vs cold runs (you must report both)

The first time a given factor expression is run, the resolver computes per
`(ticker, date)` factor scores from scratch and writes them into the
persistent `factor_score` table. Every subsequent request with the **same
expression** loads those rows back instead of recomputing — so identical
back-to-back requests measure the warm path almost exclusively.

This matters because the two paths have different bottlenecks:

- **Warm path** — what users feel on a re-run of an existing strategy.
  Dominated by `factor_score` lookups and price scans (the 321-batch
  read pattern noted in `internal/repository/factor_score.repository.go`).
- **Cold path** — what users feel on a brand-new expression, after a
  schema change that invalidates hashes, or when onboarding a new asset
  universe. Dominated by score *computation* and the row insert into
  `factor_score`.

A useful investigation reports **both** numbers. Optimizing one without
measuring the other can silently regress the other.

The expression hash is `sha256(strip_whitespace(expression))`
([internal/util/util.go](../internal/util/util.go) `HashFactorExpression`).
So to force a cold run, just append a no-op term to the expression so the
hash differs while the math doesn't:

```bash
# Cold run: append "+ 0.000000001" (and bump the suffix each iteration so
# every cold run lands on its own hash). Mathematically equivalent to the
# original expression within float precision; semantically a brand-new
# strategy as far as the cache is concerned.
COLD_TAG="cold-$(date +%s%N)"
jq --arg n "$NONCE" \
   --arg expr "$(jq -r '.factorOptions.expression' /tmp/payload.json) + 0.0000000$(printf %d $RANDOM)" \
   '.factorOptions.name = $n | .factorOptions.expression = $expr' \
   /tmp/payload.json > /tmp/tagged.json
```

For warm runs, leave the expression alone and only vary the nonce in
`factorOptions.name`. The hash collapses to the same key so the second
request reads back the rows the first request wrote.

### Two practical patterns

- **Cold then warm.** Fire one cold request to populate `factor_score`,
  then fire N warm requests with the same expression and different
  nonces. Discard the cold result for the warm aggregate; report both.
- **All cold.** Generate a fresh perturbation per request. This is what
  to do when measuring the actual recompute path (e.g. before/after a
  PR that touches `factor_expression.service.go`).

Note: cold runs leak rows into `factor_score`. A few hundred rows per
investigation is fine; do not run thousands of cold benchmarks in tight
loops without an occasional `DELETE FROM factor_score WHERE
factor_expression_hash NOT IN (...)` cleanup.

## Step 1: Stamp a nonce

The `factorOptions.name` field is stored verbatim in
`api_request.request_body`, doesn't affect computation, and is therefore the
ideal handle.

```bash
NONCE="bench-$(date +%s)-$(openssl rand -hex 4)"
echo "nonce: $NONCE"
jq --arg n "$NONCE" '.factorOptions.name = $n' /tmp/payload.json > /tmp/tagged.json
```

## Step 2: Fire and capture wall-clock

```bash
# Fly
curl -s -o /dev/null \
  -w 'Fly:    http=%{http_code} ttfb=%{time_starttransfer}s total=%{time_total}s\n' \
  -X POST "$FLY_URL" \
  -H 'Content-Type: application/json' \
  --data-binary @/tmp/tagged.json

# Lambda baseline (same payload, different nonce so the rows are distinct)
NONCE_LAMBDA="${NONCE}-lambda"
jq --arg n "$NONCE_LAMBDA" '.factorOptions.name = $n' /tmp/payload.json > /tmp/tagged-lambda.json
curl -s -o /dev/null \
  -w 'Lambda: http=%{http_code} ttfb=%{time_starttransfer}s total=%{time_total}s\n' \
  -X POST "$LAMBDA_URL" \
  -H 'Content-Type: application/json' \
  --data-binary @/tmp/tagged-lambda.json
```

`ttfb` ≈ `total` for both — the response is sent in one shot at the end, so
TTFB is effectively the server-side processing time.

## Step 3: Find the request rows

```sql
SELECT request_id, version, duration_ms, start_ts
FROM api_request
WHERE route = '/backtest'
  AND request_body LIKE '%' || :'NONCE' || '%'
ORDER BY start_ts DESC
LIMIT 5;
```

`version` is the short git SHA of the running deploy (post-#fix-latency-observability).
If it's empty/null, you hit a Fly machine that pre-dates the commit-hash
plumbing — re-deploy before drawing any conclusions.

If you don't see your row, check `flyctl logs | grep latency_insert_failed`.
The latency repository never returns errors to the user; failures are
logged with that exact event key.

## Step 4: Drill into spans for one request

```sql
SELECT depth, elapsed_ms, span_path
FROM latency_span_stats
WHERE request_id = :'request_id'
ORDER BY depth, elapsed_ms DESC NULLS LAST;
```

What to look at:

- The deepest leaf with the largest `elapsed_ms` is your prime suspect.
- A span with `elapsed_ms IS NULL` was opened but never `End()`-ed. That's
  an instrumentation bug, not a perf bug — flag it and move on.
- If `request_duration_ms - elapsed_ms_of_root_span > 200ms`, there's
  meaningful work happening **outside** any span (auth middleware, response
  serialization, write of api_request itself). Note this separately — the
  fix is "add a span there", not "optimize the existing spans."

## Step 5: Aggregate across multiple runs

One request is noisy on shared CPU. Fire 5–10 with different nonces, then:

```sql
SELECT span_path,
       count(*) AS n,
       round(percentile_cont(0.50) WITHIN GROUP (ORDER BY elapsed_ms)::numeric) AS p50_ms,
       round(percentile_cont(0.95) WITHIN GROUP (ORDER BY elapsed_ms)::numeric) AS p95_ms,
       max(elapsed_ms) AS max_ms
FROM latency_span_stats
WHERE start_ts > now() - interval '15 minutes'
  AND route = '/backtest'
  AND ip_address_filter_or_version_filter_here -- narrow to your runs
  AND elapsed_ms IS NOT NULL
GROUP BY span_path
HAVING count(*) >= 3
ORDER BY p95_ms DESC NULLS LAST
LIMIT 15;
```

Replace the placeholder `WHERE` clause with whatever isolates your run set
(matching the nonce prefix, your client IP, the deploy SHA, etc.).

This is the single most useful query in the runbook. Run it before and
after any change to see the delta on each span independently.

## Step 6: Compare Fly to the Lambda baseline

Both backends write to the same RDS. Lambda rows are tagged
`version = '26911f2'` and have `ip_address IS NULL` (API Gateway strips it
through the Lambda proxy by default). Fly rows have `version` =
deploy-SHA and `ip_address` = your client's IP.

```sql
SELECT
  CASE WHEN version = '26911f2' THEN 'lambda' ELSE 'fly' END AS backend,
  span_path,
  count(*) AS n,
  round(percentile_cont(0.50) WITHIN GROUP (ORDER BY elapsed_ms)::numeric) AS p50_ms,
  round(percentile_cont(0.95) WITHIN GROUP (ORDER BY elapsed_ms)::numeric) AS p95_ms
FROM latency_span_stats
WHERE start_ts > now() - interval '1 hour'
  AND route = '/backtest'
  AND elapsed_ms IS NOT NULL
GROUP BY backend, span_path
ORDER BY span_path, backend;
```

You'll see two rows per span (one per backend). The delta is your gap.
Anywhere the gap is small (<50ms p95) the network/runtime is fine; anywhere
it's large is a candidate for either a real perf fix or a region-level
infra change.

## Step 7: Compare across deploys (Fly-to-Fly A/B)

When you ship a candidate optimization and want to know if it actually
moved the needle:

```sql
SELECT version, span_path,
       count(*) AS n,
       round(percentile_cont(0.95) WITHIN GROUP (ORDER BY elapsed_ms)::numeric) AS p95_ms
FROM latency_span_stats
WHERE start_ts > now() - interval '24 hours'
  AND route = '/backtest'
  AND version IN (:'before_sha', :'after_sha')
  AND elapsed_ms IS NOT NULL
GROUP BY version, span_path
HAVING count(*) >= 5
ORDER BY span_path, version;
```

Same query shape works for any pair of deploys.

## What to deliver at the end of an investigation

A report with six things, in this order. Anything more is scope creep.

1. **Wall-clock summary.** Median + p95 + max across N runs, **separately for warm and cold**, for both Fly and Lambda. Four numbers per backend. Plain table.
2. **Server-side breakdown.** The top-15 spans table from step 5, **with warm and cold split into separate columns** so reviewers can see whether a regression is in the recompute path or the read path.
3. **Time not in any span.** Compute `wall_clock_ms - root_span_elapsed_ms` (warm runs only — cold runs are dominated by compute, so this signal is noisier there). Report as ms and as % of wall-clock. If >20%, recommend adding a span before recommending any optimization.
4. **Comparison to Lambda.** The step-6 table, narrowed to the top 5 spans by Fly p95. State the largest absolute gap, separately for warm and cold.
5. **One hypothesis.** A single sentence: "Span X is the dominant cost on Fly **on the {warm,cold} path** because Y." That's the seed for the next PR.
6. **Caveats.** Note any cold-start hits you discarded, any nonce-tagging issues (request rows missing from the table), and any unusual Fly machine state (`flyctl status` output if non-default).

Do not write code in the report. Do not propose multiple changes. The
"one-hypothesis" rule keeps subsequent PRs reviewable.

## Sharp edges to know about

- **Cold start.** `fly.toml` has `min_machines_running = 0`. The first
  request after the machine has been idle eats container start + Go boot +
  initial Postgres TLS handshake — expect 5–15s extra. Always warm with a
  throwaway `GET /` (or just a single discard request) before measuring.
  This "cold machine" is a separate concept from "cold cache" above; an
  investigation can have a cold machine *and* a warm cache, etc.
- **The expression hash, not the request, decides cache temperature.**
  Two agents running with the same expression at the same time will
  share warmth — the second to fire reads back what the first wrote.
  Coordinate your runs.
- **Don't fan out concurrent requests.** Each backtest fans out 10
  goroutines into the DB pool (`internal/repository/factor_score.repository.go`).
  Two concurrent requests fight for the 25-conn pool and your numbers
  become un-interpretable. Run sequentially.
- **`latency_tracking` is best-effort.** The `Add()` call is wrapped in a
  swallow-and-log. If a row is missing, check `flyctl logs | grep
  latency_insert_failed`. Don't assume "no row" means "no request" — verify
  via `api_request` first.
- **`pg_stat_statements_reset()` is global.** Useful for "what queries did
  this one request fire" experiments, but it nukes counts for every active
  session. Only run it when you're the only investigator.
- **`api_request.version` is the deploy SHA, not the source SHA.** They
  match for prod deploys but can drift if you `flyctl deploy` from a dirty
  tree. Always confirm `version` matches `git rev-parse --short HEAD` of
  the commit you think you're testing.
- **Fly machines auto-stop.** If you're mid-experiment and walk away for
  >5 minutes, the machine stops and the next request cold-starts. Fire a
  warm-up request before resuming.
- **The Lambda baseline drifts too.** Lambda is currently on commit
  `26911f2`. If you push a fix that touches the application code, Lambda
  won't see it (we don't deploy there anymore). When comparing across
  versions where the application logic itself changed, compare Fly-to-Fly
  using step 7 instead.

## Quick reference: the four queries you'll actually use

```sql
-- 1) Locate your request_id by nonce.
SELECT request_id, version, duration_ms, start_ts
FROM api_request
WHERE route = '/backtest' AND request_body LIKE '%bench-XXXX%'
ORDER BY start_ts DESC LIMIT 5;

-- 2) Span breakdown for one request.
SELECT depth, elapsed_ms, span_path
FROM latency_span_stats
WHERE request_id = '...'::uuid
ORDER BY depth, elapsed_ms DESC NULLS LAST;

-- 3) Aggregate over a recent window.
SELECT span_path,
       count(*) AS n,
       round(percentile_cont(0.50) WITHIN GROUP (ORDER BY elapsed_ms)::numeric) AS p50_ms,
       round(percentile_cont(0.95) WITHIN GROUP (ORDER BY elapsed_ms)::numeric) AS p95_ms
FROM latency_span_stats
WHERE start_ts > now() - interval '15 minutes'
  AND route = '/backtest' AND elapsed_ms IS NOT NULL
GROUP BY span_path HAVING count(*) >= 3
ORDER BY p95_ms DESC NULLS LAST LIMIT 15;

-- 4) Fly vs Lambda baseline.
SELECT
  CASE WHEN version = '26911f2' THEN 'lambda' ELSE 'fly' END AS backend,
  span_path,
  count(*) AS n,
  round(percentile_cont(0.95) WITHIN GROUP (ORDER BY elapsed_ms)::numeric) AS p95_ms
FROM latency_span_stats
WHERE start_ts > now() - interval '1 hour'
  AND route = '/backtest' AND elapsed_ms IS NOT NULL
GROUP BY backend, span_path
ORDER BY span_path, backend;

-- 5) Cleanup: drop factor_score rows from cold-run perturbations. Targets
-- only hashes whose backing user_strategy row was created during this
-- investigation window. The `strategy` table stores raw expression text
-- (not the hash), so we narrow via user_strategy.factor_expression_hash
-- — which is written on every /backtest call by saveUserStrategy() — and
-- only sweep rows we know we created (by start_ts of the request).
DELETE FROM factor_score
WHERE factor_expression_hash IN (
  SELECT DISTINCT us.factor_expression_hash
  FROM user_strategy us
  JOIN api_request ar ON us.request_id = ar.request_id
  WHERE ar.start_ts > now() - interval '2 hours'
    AND ar.request_body LIKE '%bench-%'  -- nonce prefix from this runbook
);
```
