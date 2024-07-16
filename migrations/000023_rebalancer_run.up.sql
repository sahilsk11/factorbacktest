create type rebalancer_run_state as enum (
  'COMPLETED',
  'PENDING',
  'ERROR'
);

create table rebalancer_run(
  rebalancer_run_id uuid primary key default uuid_generate_v4(),
  date date not null,
  created_at timestamp with time zone not null default now()
);

create table investment_rebalance(
  investment_rebalance_id uuid primary key default uuid_generate_v4(),
  rebalancer_run_id uuid references rebalancer_run(rebalancer_run_id) not null,
  strategy_investment_id uuid references strategy_investment(strategy_investment_id) not null,
  state rebalancer_run_state not null,
  created_at timestamp with time zone not null default now(),
  modified_at timestamp with time zone not null default now()
);

create table investment_rebalance_trade (
  investment_rebalance_trade_id uuid primary key default uuid_generate_v4(),
  investment_rebalance_id uuid not null references investment_rebalance(investment_rebalance_id),
  ticker_id uuid not null references ticker(ticker_id),
  amount_in_dollars decimal not null,
  side trade_order_side not null,
  created_at timestamp with time zone not null default now()
);