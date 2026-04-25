# auth-service

Reusable Better Auth service. Mounts at `/api/auth/*`, owns its own
PostgreSQL schema (`auth` by default), and bridges users to the host app via
`public.app_user_profile` (when enabled).

## Layout

```
auth-service/
├── auth.ts                 # CLI-discoverable entry: builds betterAuth instance
├── package.json
├── tsconfig.json
├── .env.example
├── scripts/
│   ├── bootstrap-schema.sql # CREATE SCHEMA auth + public.app_user_profile
│   ├── bootstrap.ts         # idempotent runner for bootstrap-schema.sql
│   └── entrypoint.sh        # bootstrap -> migrate -> serve
└── src/
    ├── secrets.ts           # loads secrets from secrets.json or env vars
    ├── config.ts            # zod-validated non-secret config from env
    ├── plugins.ts           # emailOTP + phoneNumber + jwt
    ├── server.ts            # Hono on 127.0.0.1:AUTH_INTERNAL_PORT
    ├── providers/           # email + sms transports (console / resend / twilio)
    └── sync/
        └── app-user-profile.ts  # databaseHooks bridging auth.user -> public
```

## How config + secrets resolve

The auth-service uses the same loading pattern as the Go API:

1. **Secrets** (DB credentials + Better Auth secret + provider API keys):
   - If `FB_SECRETS_FROM_ENV=1`, read camelCase env vars
     (`betterAuthSecret`, `host`, `password`, `googleClientSecret`, ...).
     Same names Fly already uses for the Go side.
   - Otherwise, walk up looking for `secrets.json` and read its `db` and
     `auth` sections. See [secrets-test.json](../secrets-test.json) for the
     shape.
2. **Non-secret config** (URLs, feature flags, provider names): read from
   process env with sensible defaults, so a fresh checkout with a populated
   `secrets.json` "just works" without any extra files. Production
   overrides live in [fly.toml](../fly.toml)'s `[env]` block.

## Local quick start

Add an `auth` section to your `secrets.json` at the repo root (one-time):

```json
{
  ...,
  "auth": {
    "betterAuthSecret": "<openssl rand -hex 32>",
    "googleClientId": "",
    "googleClientSecret": "",
    "resendApiKey": "",
    "twilioAccountSid": "",
    "twilioAuthToken": ""
  }
}
```

Then:

```bash
cd auth-service
npm install
npm run bootstrap   # CREATE SCHEMA auth + public.app_user_profile
npm run migrate     # better-auth CLI lays out tables in `auth` schema
npm run dev         # serves on http://127.0.0.1:3001/api/auth
```

Email and SMS default to the `console` provider, so OTP codes print to
stdout — you can sign in end-to-end without any external accounts. Google
defaults to off; flip `FEATURE_GOOGLE=true` (env or fly.toml) once you've
added real OAuth credentials to `secrets.json`.

## Production (inside this repo's Fly app)

The Go API reverse-proxies `/api/auth/*` to `127.0.0.1:3001`, so this
service binds only to localhost. The multi-stage `Dockerfile.fly` and
`scripts/start.sh` supervisor in the repo root run both processes together
in one Fly machine. See [plans/fly-deployment.md](../plans/fly-deployment.md)
for the deploy / cutover runbook.

## Reusing in another project

1. Copy this folder.
2. Adjust `auth.ts` if you want to add/remove plugins.
3. Adjust `src/sync/app-user-profile.ts` to match the host app's profile
   table (or set `APP_USER_SYNC_ENABLED=false` and consume `auth.user.id`
   from JWT claims directly).
4. Either provide a `secrets.json` with `db` + `auth` sections, or set
   `FB_SECRETS_FROM_ENV=1` and supply the same fields as env vars.
5. Reverse-proxy `/api/auth/*` to this service from your main app, or
   expose port 3001 directly.
