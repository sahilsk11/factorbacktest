# auth-service

Reusable Better Auth service. Mounts at `/api/auth/*`, owns its own
PostgreSQL schema (`auth` by default), and bridges users to the host app via
`public.app_user_profile`.

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
    ├── config.ts            # zod-validated config from env
    ├── plugins.ts           # emailOTP + phoneNumber + jwt
    ├── server.ts            # Hono on 127.0.0.1:AUTH_INTERNAL_PORT
    ├── providers/           # email + sms transports (console / resend / twilio)
    └── sync/
        └── app-user-profile.ts  # databaseHooks bridging auth.user -> public
```

## Local quick start

```bash
cp .env.example .env
# edit BETTER_AUTH_SECRET, GOOGLE_*, etc.

npm install
npm run bootstrap         # CREATE SCHEMA auth + public.app_user_profile
npm run migrate           # better-auth CLI lays out tables in `auth` schema
npm run dev               # serves on http://127.0.0.1:3001/api/auth
```

The default `EMAIL_PROVIDER=console` and `SMS_PROVIDER=console` log OTP codes
to stdout, so you can do an end-to-end OTP flow without any external accounts.

## Production (inside this repo's Fly app)

The Go API reverse-proxies `/api/auth/*` to `127.0.0.1:3001`, so this service
binds only to localhost and never sees public traffic directly. See the root
`Dockerfile.fly` for the multi-stage build that combines Go + Node into a
single image.

## Reusing in another project

1. Copy this folder.
2. Adjust `auth.ts` if you want to add/remove plugins.
3. Adjust `src/sync/app-user-profile.ts` to match the host app's profile table
   (or set `APP_USER_SYNC_ENABLED=false` and consume `auth.user.id` from JWT
   claims directly).
4. Provide env vars per `.env.example`.
5. Reverse-proxy `/api/auth/*` to this service from your main app (or expose
   it directly).
