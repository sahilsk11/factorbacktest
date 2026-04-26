-- Install pg_cron if not already present and schedule a nightly job to
-- delete factor_score rows older than 2 weeks. factor_score is a computed
-- cache; scores that haven't been needed in two weeks are unlikely to be
-- needed again, and they can always be recomputed on demand.

CREATE EXTENSION IF NOT EXISTS pg_cron;

SELECT cron.schedule(
    'expire-factor-scores',
    '0 3 * * *',
    $$DELETE FROM factor_score WHERE created_at < now() - INTERVAL '2 weeks'$$
);
