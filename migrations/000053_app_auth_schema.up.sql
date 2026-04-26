-- Custom Go auth schema. Coexists with Better Auth's `auth` schema during
-- the cutover. Tables here are owned by `internal/auth`.
--
-- Identity is keyed off public.user_account.user_account_id; no user data
-- is duplicated. Sessions are short-lived tuples that the auth middleware
-- looks up on every authenticated request.
CREATE SCHEMA IF NOT EXISTS app_auth;

-- One row per active session. The cookie value is `<id>.<HMAC>`; we look
-- up by id and verify HMAC in constant time before trusting the row.
--
-- created_at + expires_at give us two enforcement layers:
--   * expires_at (sliding) is bumped on every authenticated request, so
--     active users stay logged in.
--   * created_at + 90 days is an absolute cap enforced in the auth package
--     (never bumped). Keeps a hijacked cookie from being valid forever.
CREATE TABLE app_auth.user_session (
    id              TEXT PRIMARY KEY,
    user_account_id UUID NOT NULL
        REFERENCES public.user_account(user_account_id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip              INET,
    user_agent      TEXT
);

CREATE INDEX user_session_user_account_id_idx
    ON app_auth.user_session (user_account_id);
CREATE INDEX user_session_expires_at_idx
    ON app_auth.user_session (expires_at);

-- New provider values for the custom auth flow. Distinct from the legacy
-- GOOGLE / BETTER_AUTH values so we can tell new vs legacy users apart
-- during the cutover and roll back cleanly if needed.
ALTER TYPE user_account_provider_type ADD VALUE IF NOT EXISTS 'LOCAL_GOOGLE';
ALTER TYPE user_account_provider_type ADD VALUE IF NOT EXISTS 'LOCAL_SMS';
