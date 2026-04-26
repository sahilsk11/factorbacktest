-- Schedule a nightly pg_cron job to delete factor_score rows older than 2
-- weeks. factor_score is a computed cache; scores that haven't been needed
-- in two weeks are unlikely to be needed again and can always be recomputed.
--
-- Guarded by a pg_available_extensions check so the migration is a no-op in
-- environments where pg_cron isn't installed (e.g. the CI test database).
-- On prod (RDS) pg_cron is available and the job is created normally.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_cron') THEN
        CREATE EXTENSION IF NOT EXISTS pg_cron;
        PERFORM cron.schedule(
            'expire-factor-scores',
            '0 3 * * *',
            $job$DELETE FROM factor_score WHERE created_at < now() - INTERVAL '2 weeks'$job$
        );
    ELSE
        RAISE NOTICE 'pg_cron not available in this environment — skipping expire-factor-scores schedule';
    END IF;
END;
$$;
