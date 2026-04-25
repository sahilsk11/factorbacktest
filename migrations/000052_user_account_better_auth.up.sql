-- Add BETTER_AUTH to the user_account_provider_type enum so the Go API
-- can persist users created via the embedded Better Auth service.
ALTER TYPE user_account_provider_type ADD VALUE IF NOT EXISTS 'BETTER_AUTH';
