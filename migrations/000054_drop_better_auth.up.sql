-- Drop the Better Auth Node sidecar's database state. The Go API never read
-- from these objects; they were owned entirely by `auth-service/` (now
-- deleted) and by the Better Auth CLI's migrations.
--
-- We deliberately do NOT touch `public.user_account_provider_type`'s
-- 'SUPABASE' / 'BETTER_AUTH' enum values. Existing user_account rows still
-- carry those values; removing the enum members would require recreating
-- the type and migrating data. Those users simply can't authenticate
-- anymore — they re-sign in via Google or SMS and get LOCAL_GOOGLE / LOCAL_SMS
-- on next login.

-- Public bridge table that auth-service kept in sync with Better Auth's
-- `auth.user`. Nothing in the Go codebase references it.
DROP INDEX IF EXISTS public.app_user_profile_email_idx;
DROP INDEX IF EXISTS public.app_user_profile_phone_idx;
DROP TABLE IF EXISTS public.app_user_profile;

-- Better Auth's own schema. CLI-managed tables: auth.user, auth.session,
-- auth.account, auth.verification, auth.jwks (and any others Better Auth
-- added). CASCADE because we don't track the exact set in this repo.
DROP SCHEMA IF EXISTS auth CASCADE;
