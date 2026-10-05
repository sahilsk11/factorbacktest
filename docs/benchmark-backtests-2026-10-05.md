# CF vs Fly backtest benchmark (2026-10-05)

Wall-clock time for **POST `/backtest`** on published strategies (same Neon). End date **2026-10-05**; horizons **1 / 3 / 5 / 10** calendar years.

## Factor-score DB cache bypass (required for dry runs)

Persistence lives in `internal/calculator/factor_expression.service.go` (`getPrecomputedScores` + `FactorScoreRepository.AddMany`).

| Mechanism | When |
| --------- | ---- |
| **`FB_DISABLE_FACTOR_SCORE_DB=1`** | Process env — always skip `factor_score` read/write (set on Fly + CF staging for bench windows). |
| **`FB_BENCH_ALLOW_SCORE_CACHE_BYPASS=1`** + header **`X-FB-Disable-Factor-Score-DB: 1`** | Per-request bypass on POST `/backtest` (wired in `api/backtest.resolver.go` → `domain.WithFactorScoreDBDisabled`). |

Implementation: `util.FactorScoreDBDisabled(ctx)` in `internal/util/factor_score_bench.go`.

CF staging `wrangler.toml` sets **`FB_DISABLE_FACTOR_SCORE_DB = "1"`** on the container via `[vars]`.

**Bench script:** `factor-cf-staging/scripts/bench-backtests.py` (sends the header on every run).

## Strategies (from prod `/publishedStrategies`)

| Key | Name | Universe | # symbols |
| --- | ---- | -------- | --------- |
| `seven_day_momentum` | 7-day momentum | SPY_TOP_300 | 10 |
| `exploding_rockets` | exploding rockets | SPY_TOP_300 | 3 |

Expressions match Neon published rows (fetched 2026-10-05).

## Results — Fly prod (`https://api.factor.trade`)

**Caveat:** Prod Fly did **not** yet run this PR’s bypass code at measurement time. The header was sent but **ignored**; **`factor_score` cache warmed** (7-day 1y **7.2s** on first run, **2.8s** on second; exploding 1y **8.5s** then **1.5s**). Treat the table below as **cached / warm-score** until Fly deploys this branch and sets:

```bash
fly secrets set FB_BENCH_ALLOW_SCORE_CACHE_BYPASS=1 FB_DISABLE_FACTOR_SCORE_DB=1 -a factorbacktest
# or deploy PR and use header-only bypass
```

| Strategy | 1y wall (s) | 3y | 5y | 10y |
| -------- | ----------- | -- | -- | --- |
| 7-day momentum | 2.8 | 6.9 | 13.0 | 26.9 |
| exploding rockets | 1.5 | 9.9 | 14.4 | 27.9 |

Raw JSON: [`factor-cf-staging/bench-results-latest.json`](../factor-cf-staging/bench-results-latest.json)  
Log: `/opt/cursor/artifacts/fly-backtest-bench.log`

## Results — CF staging (`https://factor-cf-staging.sahilkapur-a.workers.dev`)

**Live** — same Neon as Fly; staging sets **`FB_DISABLE_FACTOR_SCORE_DB=1`** in `wrangler.toml`.

Backtest wall-time matrix (7-day momentum + exploding rockets × 1/3/5/10y) is tracked **separately** from HTTP TTFB. Run:

```bash
cd factor-cf-staging
python3 scripts/bench-backtests.py
```

CF HTTP cold Neon path (Sahil box, 2026-10-05): **`GET /publishedStrategies` ~1.3–1.7s** TTFB median after container sleep vs Fly warm **~0.10–0.15s** — see [`factor-cf-staging/README.md`](../factor-cf-staging/README.md).

## HTTP TTFB (supplementary)

See [`docs/factor-cloudflare-containers-spike.md`](factor-cloudflare-containers-spike.md) and `scripts/benchmark.sh` for `GET /` and `/publishedStrategies` TTFB.

## Go / no-go

Re-run this table with **cache disabled on both platforms**, then compare wall times at **10y exploding rockets** (heaviest). Prod DNS cutover remains out of scope until Sahil approves after dry-run parity.
