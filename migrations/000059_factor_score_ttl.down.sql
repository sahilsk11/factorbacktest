DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_cron') THEN
        PERFORM cron.unschedule('expire-factor-scores');
    END IF;
END;
$$;
