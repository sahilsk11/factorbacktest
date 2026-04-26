-- Provider value for the email-OTP login flow. Distinct from the legacy
-- GOOGLE / BETTER_AUTH values (and from LOCAL_GOOGLE / LOCAL_SMS for the
-- new Go auth) so we can tell new email-OTP users apart during cutover.
-- Enum lives in the public schema; see migration 000050.
ALTER TYPE user_account_provider_type ADD VALUE IF NOT EXISTS 'LOCAL_EMAIL';
