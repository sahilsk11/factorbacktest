-- Idempotent bootstrap for the Better Auth schema.
-- Safe to run on every container start.
--
-- Better Auth then creates/updates its own tables inside this schema via
-- `npx better-auth migrate` (Kysely adapter, search_path-aware).

CREATE SCHEMA IF NOT EXISTS auth;

-- Cross-schema bridge: app-owned profile keyed by Better Auth user id.
-- Lives in `public` so the Go API and existing app code can join against it
-- without ever touching `auth.user` directly.
CREATE TABLE IF NOT EXISTS public.app_user_profile (
    auth_user_id  TEXT PRIMARY KEY,
    email         TEXT,
    display_name  TEXT,
    phone_number  TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS app_user_profile_email_idx
    ON public.app_user_profile (email);
CREATE INDEX IF NOT EXISTS app_user_profile_phone_idx
    ON public.app_user_profile (phone_number);
