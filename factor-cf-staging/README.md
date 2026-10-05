# Factor on Cloudflare (staging) — Worker + Container + Cron

**Live staging (same Neon as Fly):** https://factor-cf-staging.sahilkapur-a.workers.dev  

Full **Factor Go API** (`Dockerfile.cf-api`) behind a Worker + **Container** (`sleepAfter = 5m`), with **Cron Triggers** replacing the Fly supercronic machine. **Production Fly is unchanged** (`api.factor.trade`). **No prod DNS cutover** in this work.

## Scaffold (upstreamed in PR #170)

| Path | Role |
| ---- | ---- |
| `wrangler.toml` | Worker name `factor-cf-staging`, cron UTC schedule, container image, `[vars]` incl. `FB_DISABLE_FACTOR_SCORE_DB=1` |
| `src/index.ts` | HTTP proxy + `scheduled()` cron POSTs to `/internal/cron/*` |
| `src/env.ts` | Fly `FB_SECRETS_FROM_ENV=1` secret → container env map |
| `src/cron.ts` | Fly `crontab` → UTC cron expressions |
| `../Dockerfile.cf-api` | Go `/app/api` only (no supercronic) |
| `scripts/deploy.sh` | Docker + wrangler deploy |
| `scripts/benchmark.sh` | HTTP TTFB Fly vs CF |
| `scripts/bench-backtests.py` | POST `/backtest` matrix (run separately) |
| `secrets.example.env` | Placeholder secret names for `.dev.vars` |

Architecture detail: [`docs/factor-cloudflare-containers-spike.md`](../docs/factor-cloudflare-containers-spike.md).

## Deploy with `CF_DEPLOY_RESUME_BUILDER`

Cloudflare **Containers** deploy requires a token with Containers scope. Sahil’s vault key **`CF_DEPLOY_RESUME_BUILDER`** works; a generic **`CLOUDFLARE_API_TOKEN` often returns 403** on container image push.

```bash
cd factor-cf-staging
cp secrets.example.env .dev.vars   # fill from Fly secrets — never commit
# Same DATABASE_URL / CRON_SECRET / Alpaca / auth as factorbacktest Fly app
export CF_DEPLOY_RESUME_BUILDER='…'   # mapped to CLOUDFLARE_API_TOKEN in deploy.sh
export CLOUDFLARE_ACCOUNT_ID='…'
npm install
bash scripts/deploy.sh
```

`deploy.sh` exports `CLOUDFLARE_API_TOKEN="${CF_DEPLOY_RESUME_BUILDER:-…}"`, uploads `.dev.vars` via `wrangler secret put`, then `wrangler deploy` (builds `Dockerfile.cf-api` with Docker).

First deploy may take several minutes; container routes can 503 until provisioning finishes ([CF Containers get started](https://developers.cloudflare.com/containers/get-started/)).

## Cron (UTC, EDT)

Mirrors [`crontab`](../crontab) in **America/New_York** during **EDT (UTC−4)**:

| ET (Mon–Fri) | UTC cron | Route |
| ------------ | -------- | ----- |
| 08:30 | `30 12 * * 1-5` | sendSavedStrategySummaryEmails |
| 09:00 | `0 13 * * 1-5` | updatePrices |
| 10:00 | `0 14 * * 1-5` | rebalance |
| */15 10:00–16:59 | `*/15 14-20 * * 1-5` | updateOrders |

Shift UTC **+1h** after US DST ends.

## Factor-score DB cache (benchmark dry runs)

Backtests normally **read/write `factor_score`** in Neon. For CF vs Fly benches, disable that cache:

| Control | Use |
| ------- | --- |
| **`FB_DISABLE_FACTOR_SCORE_DB=1`** | Staging container (set in `wrangler.toml` `[vars]`) |
| **`FB_BENCH_ALLOW_SCORE_CACHE_BYPASS=1`** + header **`X-FB-Disable-Factor-Score-DB: 1`** | Per POST `/backtest` on Fly after deploy |

See `internal/util/factor_score_bench.go` and [`docs/benchmark-backtests-2026-10-05.md`](../docs/benchmark-backtests-2026-10-05.md).

## HTTP benchmark (2026-10-05)

| Path | Fly prod (warm) | CF staging (cold container + Neon) |
| ---- | --------------- | ------------------------------------ |
| `GET /publishedStrategies` | **~0.10–0.15s** TTFB median | **~1.3–1.7s** TTFB median when container slept (Sahil box benches) |
| `GET /` | **~0.07s** warm | Higher on cold wake; use published route for Neon-in-path |

Fly warm remains **much faster** for interactive landing reads. CF tradeoff is scale-to-zero compute cost, not raw warm latency.

```bash
COLD_IDLE_SEC=180 bash scripts/benchmark.sh
```

## Backtest matrix

```bash
python3 scripts/bench-backtests.py   # 7-day momentum + exploding rockets × 1/3/5/10y
```

Run separately from HTTP TTFB; results go to `bench-results-latest.json`.
