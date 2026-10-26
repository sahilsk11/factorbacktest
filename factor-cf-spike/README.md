# factor-cf-spike

Non-production scaffold to measure **Cloudflare Containers cold start** and validate **Cron Triggers → Durable Object → Container** (`scheduling_policy = "durable_object"`).

This is **not** the Factor Go API image. Production remains on Fly (`api.factor.trade`). Do not point DNS here.

## Prerequisites

- Cloudflare account with **Workers Paid** (Containers allotment).
- Docker engine available locally for `wrangler deploy` image build (or remote build if enabled).
- Wrangler **≥ 4.136.0** (Containers + cron example requirements).

## Secrets / env (blockers in Cloud Agent)

This environment did **not** expose:

| Variable | Purpose |
| -------- | ------- |
| `CLOUDFLARE_API_TOKEN` | `wrangler deploy` |
| `CLOUDFLARE_ACCOUNT_ID` | Account routing |

Optional: set `WRANGLER_SEND_METRICS=false` in CI.

## Deploy

```bash
cd factor-cf-spike
npm install
export CLOUDFLARE_API_TOKEN=...
export CLOUDFLARE_ACCOUNT_ID=...
npx wrangler deploy
```

Worker URL will be `https://factor-cf-spike.<subdomain>.workers.dev` (unless renamed).

## Measure cold TTFB (container + tiny httpd)

After the worker has been idle long enough for the container to scale to zero (CF docs: on the order of minutes; confirm in dashboard):

```bash
WORKER=https://factor-cf-spike.<your-subdomain>.workers.dev
curl -sS -o /dev/null -w 'cold: ttfb=%{time_starttransfer}s total=%{time_total}s code=%{http_code}\n' "$WORKER/health"
curl -sS "$WORKER/health" | jq .
```

Run 3–5 cold sequences (wait between runs). Record:

- `time_starttransfer` (Worker TTFB — includes container start + pull + httpd bind)
- Response JSON `boot_iso` (container process start time)

**Neon is not in this spike.** Factor prod cold path must add Neon wake (~ hundreds of ms to a few s) on top of any container start.

## Test cron locally

```bash
npx wrangler dev --test-scheduled
# other terminal:
curl 'http://localhost:8787/__scheduled?cron=0+14+*+*+*'
```

Deployed cron is `0 14 * * *` UTC (daily). Use dashboard logs or `wrangler tail` after deploy.

## Next spike iteration (not in this PR)

- Swap `Dockerfile` for a slim **`Dockerfile.fly`-derived** image ( ~25MB Go binary + Debian runtime ) and repeat cold `/` measurements.
- Cron Worker POST to **staging** Factor cron routes with `CRON_SECRET` (instead of supercronic on Fly).
- Compare against Fly baseline in `docs/factor-cloudflare-containers-spike.md`.
