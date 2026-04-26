-- Reverse of 000054_drop_better_auth.up.sql. Recreates only the structures
-- the Go side ever cared about (the public bridge table). The `auth` schema
-- itself is recreated empty; its tables were CLI-owned and would be rebuilt
-- by `npx better-auth migrate` if the sidecar were ever resurrected.

CREATE SCHEMA IF NOT EXISTS auth;

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
