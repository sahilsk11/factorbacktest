CREATE TYPE backtest_job_status AS ENUM ('pending', 'running', 'completed', 'failed');

CREATE TABLE backtest_job (
    backtest_job_id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    strategy_id uuid NOT NULL REFERENCES strategy(strategy_id),
    status backtest_job_status NOT NULL DEFAULT 'pending',
    current_stage text,
    progress_pct int NOT NULL DEFAULT 0,
    result jsonb,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backtest_job_strategy_id ON backtest_job(strategy_id);
CREATE INDEX idx_backtest_job_status ON backtest_job(status);

-- Helper function to update updated_at
CREATE OR REPLACE FUNCTION update_backtest_job_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER backtest_job_updated_at
    BEFORE UPDATE ON backtest_job
    FOR EACH ROW
    EXECUTE FUNCTION update_backtest_job_updated_at();
