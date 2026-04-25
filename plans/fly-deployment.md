# Fly.io deployment runbook

This is the parallel deployment of the Go API on Fly.io. The Lambda + API
Gateway deploy still runs unchanged; Fly is a separate target you can hit at
`https://factorbacktest.fly.dev` for A/B testing and (eventually) cut over to.

## Files

- [fly.toml](../fly.toml) — Fly app config (shared-cpu-2x, 1GB, auto-stop)
- [Dockerfile.fly](../Dockerfile.fly) — image for Fly (no `secrets.json` baked in)
- [.github/workflows/deploy-fly.yml](../.github/workflows/deploy-fly.yml) — auto-deploys on push to `master`
- `Makefile` `deploy-fly` target for manual deploys

## One-time setup

### 1. Create the app

```sh
flyctl apps create factorbacktest --org personal
```

(Pick a different org if you don't want it on the personal one.)

### 2. Set the 13 runtime secrets

Fly machines load `Secrets` via env vars when `FB_SECRETS_FROM_ENV=1` is set
(it is, in `fly.toml`). Set the values once with `fly secrets set`. They're
encrypted at rest and only decrypted into the machine's environment.

```sh
flyctl secrets set \
  FB_DB_HOST='<rds-host>' \
  FB_DB_PORT='5432' \
  FB_DB_USER='<user>' \
  FB_DB_PASSWORD='<password>' \
  FB_DB_NAME='<dbname>' \
  FB_DB_ENABLE_SSL='true' \
  FB_JWT='<jwt-secret>' \
  FB_ALPACA_API_KEY='<key>' \
  FB_ALPACA_API_SECRET='<secret>' \
  FB_ALPACA_ENDPOINT='https://api.alpaca.markets' \
  FB_DATA_JOCKEY_API_KEY='<key>' \
  FB_CHATGPT_API_KEY='<key>' \
  FB_SES_REGION='us-east-1' \
  FB_SES_FROM_EMAIL='noreply@factor.trade'
```

You can pull the current values from AWS Secrets Manager (`prod/factor`) to
seed these. `FB_DB_ENABLE_SSL` is optional and defaults to `true` if unset.

### 3. Wire up CI

Get a deploy token and add it to GitHub:

```sh
flyctl auth token
gh secret set FLY_API_TOKEN --body '<paste-token>'
```

After this, `git push origin master` triggers both `deploy-lambda.yml` and
`deploy-fly.yml` in parallel.

### 4. Confirm RDS allows the world

The RDS security group needs `0.0.0.0/0` on the Postgres port (or at minimum
Fly's egress range). Without this, the machine boots but can't connect to
Postgres and the healthcheck on `/` will fail.

## Day-to-day commands

```sh
flyctl deploy --remote-only      # manual deploy from current branch
flyctl logs                      # tail logs
flyctl status                    # machine state
flyctl ssh console               # shell into the running machine
flyctl scale vm shared-cpu-4x    # bump CPU class
flyctl scale memory 2048         # bump memory (MB)
flyctl secrets list              # which env-var secrets are set (names only)
flyctl releases                  # deploy history
flyctl releases revert <version> # rollback to a prior release
```

## Smoke test

1. `curl https://factorbacktest.fly.dev/` — expect a 200 from the root handler
   in [api/api.go](../api/api.go) line 105.
2. In the browser dev tools on `factor.trade`, override the API URL for one
   tab (e.g. via DevTools "Local overrides" or a small `sessionStorage` shim
   in `frontend/src/App.tsx`'s `endpoint` const) and run a backtest.
3. Run a deliberately heavy backtest (20yr, monthly, SPY_TOP_80) that would
   504 on Lambda's 29-second API Gateway cliff. Confirm it completes on Fly.
4. `flyctl logs` — verify no errors, and no "failed to load secrets from
   AWS" warning (that path should be skipped because `FB_SECRETS_FROM_ENV=1`).

## Cutover (later, when ready)

When you want to send real traffic to Fly:

1. Add a custom domain: `flyctl certs add api.factor.trade` and update DNS.
2. Update `endpoint` in [frontend/src/App.tsx](../frontend/src/App.tsx) to
   point at the new domain.
3. Deploy frontend (`make deploy-fe`).
4. Watch `flyctl logs` and CloudWatch for errors for a day.
5. Once stable, retire Lambda: stop the GitHub Actions workflow, delete the
   Lambda + API Gateway via console / `terraform destroy`, drop
   `cmd/lambda/`, `Dockerfile.lambda`, the `aws-lambda-go` deps, and the
   AWS Secrets Manager loader path in `internal/util/util.go`.

## Cost notes

`shared-cpu-2x` with `auto_stop_machines = "stop"` and
`min_machines_running = 0` means the machine sleeps when idle and wakes on
the next request (a few-hundred-ms cold start). Expect a few dollars a
month. Bump to `performance-2x` if backtests feel CPU-bound.
