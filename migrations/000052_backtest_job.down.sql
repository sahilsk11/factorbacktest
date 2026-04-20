DROP TRIGGER IF EXISTS backtest_job_updated_at ON backtest_job;
DROP FUNCTION IF EXISTS update_backtest_job_updated_at();
DROP TABLE IF EXISTS backtest_job;
DROP TYPE IF EXISTS backtest_job_status;
