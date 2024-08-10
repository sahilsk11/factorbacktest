alter table rebalance_price
add constraint unique_rebalance_price unique (ticker_id, rebalancer_run_id);