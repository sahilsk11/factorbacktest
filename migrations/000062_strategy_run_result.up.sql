-- Persist the full published backtest payload (snapshots + metrics) so
-- landing-card clicks can reuse the latest run instead of recomputing.
alter table strategy_run
add column result jsonb;
