alter table trade_order
add column rebalancer_run_id uuid not null references rebalancer_run(rebalancer_run_id);