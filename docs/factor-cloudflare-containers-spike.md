# Factor → Cloudflare Containers spike (2026-10-05)

**Status:** **Staging is live** at https://factor-cf-staging.sahilkapur-a.workers.dev (Factor Go API + cron on CF Containers, **same Neon as Fly**). Upstream scaffold in PR #170. **No prod DNS cutover.**

**Related code:**

- [`factor-cf-staging/`](../factor-cf-staging/) — Worker + Container + Cron (this stack)
- [`factor-cf-spike/`](../factor-cf-spike/) — Alpine cold-start probe (`factor-cf-spike.sahilkapur-a.workers.dev`)
- [`docs/benchmark-backtests-2026-10-05.md`](benchmark-backtests-2026-10-05.md) — backtest matrix + cache bypass

**Sahil direction (2026-10-05):** migrate **both** cron and core API to CF (shared Neon with Fly), then benchmark CF vs Fly cold/warm — not cron-only Phase A.

---

## 1. Current Factor on Fly (as mapped from this repo)

### Processes (`fly.toml`)

| Process | Command | Role |
| ------- | ------- | ---- |
| `web` | `/app/api` | Public Go API on `:3009` (Gin). Serves backtests, auth, investments, and **cron HTTP handlers**. |
| `cron` | `supercronic /app/crontab` | Always-on sidecar VM that **only schedules HTTP POSTs** to the web API. Does not run business logic locally. |

Fly app name: **`factorbacktest`**, region **`iad`**.

### VM sizing (both process groups)

```53:55:fly.toml
[[vm]]
  size = "shared-cpu-2x"
  memory = "1gb"
```

Each process group typically runs on **its own machine(s)**. Today that implies:

- **Web:** `min_machines_running = 1` with `auto_stop_machines = "stop"` / `auto_start_machines = true` — one warm web VM for interactive traffic.
- **Cron:** no `http_service` entry — the supercronic VM stays up to fire curls on an ET schedule (not scale-to-zero).

### What cron actually runs (`crontab`)

All jobs are **`curl -X POST`** to production API routes with `X-Cron-Secret: $CRON_SECRET`:

| Schedule (ET, weekdays) | Endpoint | Purpose |
| ----------------------- | -------- | ------- |
| 08:30 | `/internal/cron/sendSavedStrategySummaryEmails` | Saved-strategy summary email |
| 09:00 | `/internal/cron/updatePrices` | Ingest adjusted prices before rebalance |
| 10:00 | `/internal/cron/rebalance` | Portfolio rebalance |
| */15 10:00–16:00 | `/internal/cron/updateOrders` | Poll broker for fill updates |

Implementation lives in `api/api.go` (`/internal/cron/*` + `requireCronSecret`).

### Deploy / migrations

- Image: `Dockerfile.fly` → `api` + `migrate` binaries, Debian slim, supercronic (cron image only).
- `release_command = "/app/migrate"` runs before traffic shifts.
- Secrets: `FB_SECRETS_FROM_ENV=1` (Fly-injected env mirrors former `secrets.json`).

### Neon / Postgres dependencies

- Runtime: `DATABASE_URL` (pooled), migrations: `MIGRATE_DATABASE_URL` when set (`internal/util/util.go`).
- Pool tuned for Neon scale-to-zero: `MaxIdleConns=3`, `ConnMaxIdleTime=45s` (`cmd/util.go`) so idle Fly web does not keep Neon awake 24/7.
- **Every authenticated request** hits `app_auth.user_session` in Postgres (no in-process session cache) — safe for cold starts; adds one indexed read per request.

### OAuth / session / “always warm” state

| Concern | Needs always-on process? |
| ------- | ------------------------- |
| Google OAuth + SMS/email OTP | No — state in signed cookies + Postgres rows |
| Session cookies (`__Host-` prefix) | No — validated against Postgres each request |
| In-memory rate limiters (`golang.org/x/time/rate`) | **Soft** — reset on process cold start (acceptable for spike; document for prod migration) |
| Factor score cache | No — persisted in `factor_score` table |
| Cron mutex / overlap | Today: single supercronic VM; on CF use idempotent cron handlers + DO serialization (see CF cron example) |

**Not implemented in repo crontab:** `AuthSessionRepository.DeleteExpired` (auth README recommends external cron) — optional hygiene job if sessions grow.

### Frontend / API split

- **API:** Fly (`https://api.factor.trade`).
- **Frontend:** S3/CloudFront (`factor.trade`) — out of scope for this spike.

### Health / cheap read endpoints

- `GET /` → JSON welcome (used by Fly HTTP check) — minimal DB work.
- `GET /publishedStrategies` → landing-page data; heavier than `/` (see baseline below).

---

## 2. Target architecture (full Factor backend on Cloudflare)

**Goal:** One staging/production stack on CF: Worker edge + **Container running `/app/api`** + **Cron Triggers** posting to `/internal/cron/*` on the same container. **Neon URL shared with Fly** until cutover.

```mermaid
flowchart LR
  subgraph edge [Workers edge]
    W[Worker fetch proxy]
    CR[Cron scheduled POSTs]
  end
  subgraph compute [Containers sleepAfter 5m]
    API[Factor Go API :3009]
  end
  subgraph data [Neon shared]
    PG[(Postgres)]
  end
  Users --> W
  W --> API
  CR --> API
  API --> PG
```

Implemented in [`factor-cf-staging/`](../factor-cf-staging/):

- `@cloudflare/containers` `FactorApiContainer` — `defaultPort = 3009`, `sleepAfter = 5m`, `enableInternet = true` (Neon)
- Worker `fetch` → `getContainer(FACTOR_API, "primary").fetch(request)` (all routes)
- Worker `scheduled` → POST `http://container/internal/cron/...` with `X-Cron-Secret` (mirrors supercronic)
- Container env from Worker secrets via `factorContainerEnv()` — same names as Fly `FB_SECRETS_FROM_ENV=1`
- Image: [`Dockerfile.cf-api`](../Dockerfile.cf-api) (Go API only; no supercronic)

**Staging URL (live):** `https://factor-cf-staging.sahilkapur-a.workers.dev`  
**Prod Fly unchanged:** `https://api.factor.trade`

**Deploy token:** vault key **`CF_DEPLOY_RESUME_BUILDER`** (export as `CLOUDFLARE_API_TOKEN` in `scripts/deploy.sh`). Generic **`CLOUDFLARE_API_TOKEN` often 403s** on Containers image push.

### Cron on Cloudflare (concrete)

- **Cron expression granularity:** 1-minute steps; **wall-clock execution budget ~15 minutes** per trigger ([Cron Triggers](https://developers.cloudflare.com/workers/configuration/cron-triggers/)).
- Factor longest job must stay **well under 15m** (rebalance + updateOrders today run on web process — measure before migration).
- Use **one Durable Object** per cron scheduler (`getByName("factor-cron")`) so overlapping ticks wait (CF example pattern) — jobs must stay **idempotent** (already required for broker/cron safety).

### Risks and mitigations

| Risk | Notes |
| ---- | ----- |
| **Container cold start** | Public CF Containers rebuild ~**648ms median** (Birthday Week 2025 announcement) — Sahil’s bar for interactive Factor. Must measure with **real `Dockerfile.fly` image**, not just Alpine spike. |
| **Neon wake on cold** | Scale-to-zero compute adds **~0.5–3s** depending on idle time; pool settings help only when warm. |
| **OAuth / cookies** | Cross-domain FE/API unchanged until DNS moves; `__Host-` cookies require HTTPS and correct host. |
| **Jobs > 15m** | Not supported on Cron Triggers; would need Queues, Workflow, or stay on Fly for that job. |
| **Docker image size / pull time** | Local build: **~25MB** Go `api` binary alone; full runtime image likely **~100–150MB+** (Debian + migrate + templates). Affects cold pull. |
| **DO / Container SDK migration** | Cloudflare deadline **2026-12-31** for legacy Container class patterns — use current `wrangler.toml` `[[containers]]` + `exports` layout (see spike). |
| **In-memory rate limits** | Cold start resets buckets — acceptable for auth abuse protection if combined with Twilio/Resend limits. |
| **Money path** | `rebalance`, `updateOrders`, Alpaca — **no prod DNS cutover** until cron + interactive cold paths are proven in staging. |

---

## 3. HTTP benchmarks (Fly warm vs CF cold Neon path)

### Method

```bash
curl -sS -o /dev/null -w 'ttfb=%{time_starttransfer}s\n' URL
# or: cd factor-cf-staging && COLD_IDLE_SEC=180 bash scripts/benchmark.sh
```

- **Fly:** `https://api.factor.trade` — warm web VM (`min_machines_running = 1`).
- **CF staging:** `https://factor-cf-staging.sahilkapur-a.workers.dev` — container `sleepAfter = 5m`, **same Neon** as Fly.
- **Neon-touching route:** `GET /publishedStrategies` (strategy list + latest run stats from Postgres).

### Summary (2026-10-05)

| Platform | Route | Typical TTFB | Notes |
| -------- | ----- | ------------ | ----- |
| **Fly prod** | `GET /publishedStrategies` | **~0.10–0.15s** warm median | Always-on web; **much faster** for interactive reads |
| **Fly prod** | `GET /` | **~0.07s** warm median | Minimal handler |
| **CF staging** | `GET /publishedStrategies` (cold) | **~1.3–1.7s** median band | Container wake + Go boot + **Neon** query (Sahil box benches after idle) |
| **CF staging** | same (warm container) | **~0.4–0.9s** | Still slower than Fly warm; no always-on VM |
| **CF spike** (Alpine) | `GET /health` | ~0.1s warm; ~1–2s+ cold | Not Neon; probe only |

**Takeaway:** CF scale-to-zero is viable for cron / off-peak if **~1.5s cold landing reads** are acceptable; Fly warm remains the latency baseline for `factor.trade` marketing until DNS cutover is explicitly approved.

Fly **machine** cold start on prod not measured (no non-prod stop).

### Deploy / re-bench

```bash
cd factor-cf-staging
cp secrets.example.env .dev.vars
export CF_DEPLOY_RESUME_BUILDER='…'
export CLOUDFLARE_ACCOUNT_ID='…'
bash scripts/deploy.sh
COLD_IDLE_SEC=180 bash scripts/benchmark.sh
```

### Factor-score cache bypass (backtest benches)

Not used for HTTP TTFB above. For POST `/backtest` dry runs:

- **`FB_DISABLE_FACTOR_SCORE_DB=1`** on staging (in `wrangler.toml`)
- Optional Fly: **`FB_BENCH_ALLOW_SCORE_CACHE_BYPASS=1`** + **`X-FB-Disable-Factor-Score-DB: 1`**

Details: [`docs/benchmark-backtests-2026-10-05.md`](benchmark-backtests-2026-10-05.md).

---

## 4. Cost estimate (rough, before/after)

Prices vary by contract; treat as **order-of-magnitude** for go/no-go discussion.

### Today — Fly always-on (this repo config)

| Component | Assumption | Approx monthly |
| --------- | ---------- | -------------- |
| Web VM | 1× `shared-cpu-2x` 1GB always on | ~\$10–15 |
| Cron VM | 1× same size always on (supercronic) | ~\$10–15 |
| **Subtotal Fly compute** | 2 machines | **~\$20–30** |

Neon, Alpaca, Twilio, Resend, AWS frontend, etc. unchanged.

Draft PR **#169** (1×/512 shrink) reduces web cost but is **orthogonal** to this spike.

### Target — CF scale-to-zero (steady state)

| Component | Assumption | Approx monthly |
| --------- | ---------- | -------------- |
| Workers Paid | Already subscribed | \$5 (existing) |
| Containers | Included allotment + usage when awake | Often **\$0–10** for cron-heavy / low QPS if sleep-to-zero |
| Fly web + cron (both always-on) | **~\$20–30** | Removed after CF cutover |
| CF Workers Paid + Containers sleep | **~\$5–15** typical low QPS | Shared Neon unchanged |

**Win condition:** Drop **both** Fly VMs when CF cold+warm TTFB meets Sahil’s bar and cron proves reliable for 1–2 weeks on staging.


---

## 5. Go / no-go gates (explicit)

| Gate | Threshold | Measured? |
| ---- | --------- | --------- |
| Interactive cold (Go + Neon) | `GET /publishedStrategies` cold **~1.3–1.7s** vs Fly warm **~0.1s** | **Measured** on staging (HTTP) |
| Interactive warm CF | Still **> Fly warm** (~0.4–0.9s observed when container hot) | Partial |
| Cron | All four jobs fire on staging; idempotent; **< 15m** | Verify via logs |
| Backtest wall time | 7-day + exploding × 1/3/5/10y, cache off | **Separate run** — see benchmark-backtests doc |
| Cost | Both Fly VMs eliminated at steady state | Estimate only |

**Next:** Finish backtest matrix with **`FB_DISABLE_FACTOR_SCORE_DB`** on both sides → compare wall times. DNS cutover (`api.factor.trade`) only after explicit Sahil approval.

---

## 6. Non-prod projects

| Directory | Purpose |
| --------- | ------- |
| [`factor-cf-staging/`](../factor-cf-staging/) | Full Go API + cron crons on Worker |
| [`factor-cf-spike/`](../factor-cf-spike/) | Alpine `/health` latency probe |

---

## References

- [Cloudflare Containers cron example](https://developers.cloudflare.com/containers/examples/cron/)
- [Workers Cron Triggers](https://developers.cloudflare.com/workers/configuration/cron-triggers/)
- Factor Fly config: [`fly.toml`](../fly.toml), [`crontab`](../crontab), [`Dockerfile.fly`](../Dockerfile.fly)
- Latency runbook (Fly vs Lambda): [`docs/latency.md`](latency.md) — note `min_machines_running` in runbook text may predate current `fly.toml` value of **1**.
