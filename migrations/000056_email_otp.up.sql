-- Email-OTP storage. Mirrors the SMS-OTP shape (Twilio Verify owns SMS
-- code state for us, but for email we hash + persist locally because we
-- own the transport via Resend/SES).
--
-- code_hash is bcrypt over the 6-digit code so DB-read does not yield
-- the code. attempts_left starts at 5; handler decrements on each
-- mismatch and refuses verification once it hits 0. consumed_at flips
-- non-null exactly once per successful verify, preventing replay.
CREATE TABLE app_auth.email_otp (
    email_otp_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,                    -- normalized lowercase
    code_hash TEXT NOT NULL,                -- bcrypt hash of the 6-digit code
    expires_at TIMESTAMPTZ NOT NULL,
    attempts_left INT NOT NULL DEFAULT 5,
    consumed_at TIMESTAMPTZ,
    ip_created_from TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX email_otp_lookup_idx
    ON app_auth.email_otp (email, consumed_at, expires_at DESC);
