-- pg_cron was already installed on prod by migration 000058_factor_score_ttl
-- (which ran as a duplicate 000058 before the naming conflict was caught).
-- This migration just ensures the cron job is scheduled on environments where
-- pg_cron is installed, and is a no-op everywhere else (e.g. CI).
--
-- CREATE EXTENSION is intentionally absent: pg_cron requires
-- shared_preload_libraries to be configured at the server level before the
-- library can be loaded. Extension installation is an infrastructure concern,
-- not a migration concern.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.schedule(
            'expire-factor-scores',
            '0 3 * * *',
            $job$DELETE FROM factor_score WHERE created_at < now() - INTERVAL '2 weeks'$job$
        );
    ELSE
        RAISE NOTICE 'pg_cron not installed — skipping expire-factor-scores schedule';
    END IF;
END;
$$;
