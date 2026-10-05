# Factor API on Cloudflare (staging)

Full **Factor Go API** (`Dockerfile.cf-api`) behind a Worker + **Container** (`sleepAfter = 5m`), with **Cron Triggers** replacing the Fly supercronic machine. Uses the **same Neon** credentials as Fly via Worker secrets (`FB_SECRETS_FROM_ENV=1` parity).

**Production Fly is unchanged** (`api.factor.trade`). Staging URL (expected):

`https://factor-api-staging.sahilkapur-a.workers.dev`

## Deploy prerequisites

| Requirement | Notes |
| ----------- | ----- |
| `CF_DEPLOY_RESUME_BUILDER` | Sahil’s token name with **Containers** scope (generic `CLOUDFLARE_API_TOKEN` 403s on Containers). |
| `CLOUDFLARE_ACCOUNT_ID` | Account id |
| Docker | Local engine for `wrangler deploy` image build |
| `.dev.vars` | Copy from `secrets.example.env`; fill from Fly secrets (never commit) |

```bash
cd factor-cf-staging
cp secrets.example.env .dev.vars
# edit .dev.vars — DATABASE_URL must match Fly/Neon prod branch used by Factor
export CF_DEPLOY_RESUME_BUILDER='...'
export CLOUDFLARE_ACCOUNT_ID='...'
npm install
bash scripts/deploy.sh
```

First deploy can take several minutes; container routes may error until provisioning completes ([CF docs](https://developers.cloudflare.com/containers/get-started/)).

## Cron (UTC, EDT)

Worker cron expressions mirror [`crontab`](../crontab) in **America/New_York** during **EDT (UTC−4)**:

| ET (Mon–Fri) | UTC cron | Route |
| ------------ | -------- | ----- |
| 08:30 | `30 12 * * 1-5` | sendSavedStrategySummaryEmails |
| 09:00 | `0 13 * * 1-5` | updatePrices |
| 10:00 | `0 14 * * 1-5` | rebalance |
| */15 10:00–16:59 | `*/15 14-20 * * 1-5` | updateOrders |

After US DST ends, shift UTC crons **+1 hour** or update `wrangler.toml`.

## Benchmark

```bash
# Warm Fly vs CF staging (+ spike); optional cold wait for scale-to-zero
COLD_IDLE_SEC=180 bash scripts/benchmark.sh
```

## Architecture

- Worker `fetch` → `getContainer(FACTOR_API, "primary").fetch(request)` → Go API `:3009`
- Worker `scheduled` → POST `http://container/internal/cron/...` with `X-Cron-Secret`
- Container env built from Worker secrets in `src/env.ts` (Fly flatten names)

See [`docs/factor-cloudflare-containers-spike.md`](../docs/factor-cloudflare-containers-spike.md) for full migration notes and go/no-go gates.
