create table rebalance_price(
  rebalance_price_id uuid primary key default uuid_generate_v4(),
  ticker_id uuid not null references ticker(ticker_id),
  price decimal not null,
  rebalancer_run_id uuid not null references rebalancer_run(rebalancer_run_id),
  created_at timestamp with time zone not null default now()
);