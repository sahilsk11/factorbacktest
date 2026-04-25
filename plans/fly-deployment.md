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
(it is, in `fly.toml`). The loader reads the original camelCase JSON leaf
names from `secrets.json` directly, so the secret names match the keys you
already know. Linux env vars are case-sensitive, so these lowercase names
don't collide with any vars Fly or Docker auto-inject.

```sh
flyctl secrets set \
  host='<rds-host>' \
  port='5432' \
  user='<db-user>' \
  password='<db-password>' \
  database='<dbname>' \
  enableSsl='true' \
  jwt='<jwt-secret>' \
  apiKey='<alpaca-key>' \
  apiSecret='<alpaca-secret>' \
  endpoint='https://api.alpaca.markets' \
  dataJockey='<data-jockey-key>' \
  gpt='<chatgpt-key>' \
  region='us-east-1' \
  fromEmail='noreply@factor.trade'
```

Easiest source: `secrets.json` locally (or AWS Secrets Manager `prod/factor`).
The Fly console "import from JSON" UI also works and produces the same flat
secret names. `enableSsl` is optional and defaults to `true` if unset.

### 3. Wire up CI

Get a deploy token and add it to GitHub:

```sh
flyctl auth token
gh secret set FLY_API_TOKEN --body '<paste-token>'
```

After this, `git push origin master` triggers both `deploy-lambda.yml` and
`deploy-fly.yml` in parallel.

### 4. Confirm RDS reachability

Fly machines connect to RDS over the public internet, so the RDS security
group needs to permit that traffic on the Postgres port. In rough order of
preference:

1. **Allowlist Fly's egress IPs only** — Fly publishes per-region egress
   ranges; pin those into the SG. Narrow blast radius, recommended for
   anything past the initial A/B test.
2. **WireGuard / Fly private networking** — peer Fly into the AWS VPC and
   keep RDS private. Most work; strongest posture.
3. **Bastion or RDS Proxy with IAM auth** — middle ground.
4. **`0.0.0.0/0` on the Postgres port** — fastest and how this PR was
   smoke-tested. Acceptable as a *temporary* state for A/B testing because
   the connection is SSL (`enableSsl=true`) and Postgres still requires the
   user/password to authenticate, but **must not be the cutover state.** If
   you choose this, plan to narrow it before sending real traffic.

Confirm reachability before deploying:

```sh
nc -zv <rds-host> 5432   # from your laptop or any external network
```

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

## Embedded Better Auth sidecar

The Fly machine now runs the Go API and a Node Better Auth sidecar in the
same container ([Dockerfile.fly](../Dockerfile.fly), [scripts/start.sh](../scripts/start.sh)).
The Go process binds the public port (3009) and reverse-proxies
`/api/auth/*` to `127.0.0.1:3001`. The sidecar code lives in
[auth-service/](../auth-service/).

### Additional secrets (5–6 values; everything else is in fly.toml)

Non-secret config (`APP_BASE_URL`, `TRUSTED_ORIGINS`, feature flags,
`EMAIL_PROVIDER`, etc.) lives in [fly.toml](../fly.toml)'s `[env]` block —
source-controlled and diffable.  Only the actual sensitive values go through
`flyctl secrets set`. The names match the camelCase pattern already used by
the Go side.

```sh
flyctl secrets set \
  betterAuthSecret="$(openssl rand -hex 32)" \
  googleClientId='<google-client-id>' \
  googleClientSecret='<google-client-secret>' \
  twilioAccountSid='<twilio-verify-account-sid>' \
  twilioAuthToken='<twilio-verify-auth-token>' \
  twilioVerifyServiceSid='<twilio-verify-service-sid>'
```

SMS OTP runs through **Twilio Verify** (Twilio generates, sends, and
validates the code; we never store SMS codes). The auth-service
auto-detects Twilio Verify mode from the presence of
`twilioVerifyServiceSid` and wires `verifyOTP` accordingly.

Email OTP is **disabled** for the initial cutover (`FEATURE_EMAIL_OTP=false`
in `fly.toml`) to preserve parity with the previous Supabase setup. To
enable later: wire an email provider (Resend or SES), set its API key, and
flip the flag.

DB connection info (`host`, `password`, etc.) is reused from the secrets
already set for the Go API — the auth-service builds its own connection
string from them. There is no separate `DATABASE_URL`.

`APP_USER_SYNC_ENABLED` is set to `false` in `fly.toml` because this app
already has `user_account` as the canonical app user table and
`getGoogleAuthMiddleware` upserts into it on every authenticated request.
The generic `public.app_user_profile` bridge table created by the
auth-service bootstrap is for new projects that don't have an existing
user table.

### Google OAuth redirect URI

Add **`https://factorbacktest.fly.dev/api/auth/callback/google`** to the
authorized redirect URIs in the Google Cloud Console (Credentials → OAuth
2.0 Client ID). For local dev, also add `http://localhost:3009/api/auth/callback/google`.

### Database prerequisite

The auth-service runs `CREATE SCHEMA IF NOT EXISTS auth` and
`npx @better-auth/cli migrate` on every container start. The database user
must have permission to create schemas; if you use a least-privilege user,
grant once:

```sql
GRANT CREATE ON DATABASE <dbname> TO <db-user>;
GRANT ALL PRIVILEGES ON SCHEMA auth TO <db-user>;
ALTER DEFAULT PRIVILEGES IN SCHEMA auth GRANT ALL ON TABLES TO <db-user>;
```

### Cutover from Supabase

The Go middleware in [api/api.go](../api/api.go) tries Better Auth JWTs
first and falls back to Supabase, so deploying this image leaves existing
Supabase sessions working. Sequence:

1. Set the new secrets above (does not affect the live Supabase path).
2. Update Google's OAuth redirect URIs.
3. `flyctl deploy --remote-only` (or push to `master`).
4. Smoke test all three flows on the deployed URL:
   - Google: click "Continue with Google", verify a session cookie comes back.
   - Email: enter address, watch `flyctl logs` for the OTP (Resend provider
     sends a real email; the console provider would log to stdout).
   - SMS: enter phone, verify Twilio delivers a code, finish sign-in.
5. Verify `auth.user`, `auth.session`, etc. are populated and `user_account`
   gets `Provider='BETTER_AUTH'` rows for new sign-ins.
6. Once confident, remove Supabase entirely:
   - `flyctl secrets unset jwt` (the Supabase HS256 secret).
   - Drop `parseSupabaseJWT` and the Supabase fallback branch from
     [api/api.go](../api/api.go).
   - Drop the `Jwt` field from `Secrets` in
     [internal/util/util.go](../internal/util/util.go) (and the
     `JwtDecodeToken` plumbing in [cmd/util.go](../cmd/util.go) /
     [api/api.go](../api/api.go)).
   - Remove `@supabase/supabase-js` from
     [frontend/package.json](../frontend/package.json) and any leftover
     references.

### Cost note

The Node sidecar idles at <50MB RSS and shares the same `shared-cpu-2x` /
1GB machine. No size bump needed unless traffic patterns change.