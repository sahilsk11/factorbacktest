create table published_strategy (
  published_strategy_id uuid primary key default uuid_generate_v4(),
  -- these are identical to saved strategy, but we copy them
  -- because a saved strategy is mutable but a published strat is not
  -- if we really want, we could do something fancy to consolidate
  strategy_name text not null,
  factor_expression text not null,
  backtest_start date not null,
  backtest_end date not null,
  rebalance_interval text not null,
  num_assets int not null,
  asset_universe text not null references asset_universe(asset_universe_name),
  creator_account_id uuid not null references user_account(user_account_id),
  created_at timestamp with time zone not null default now(),
  modified_at timestamp with time zone not null default now(),
  deleted_at timestamp with time zone
);

create table published_strategy_stats (
  published_strategy_stats_id uuid primary key default uuid_generate_v4(),
  published_strategy_id uuid not null references published_strategy(published_strategy_id),
  one_year_return float,
  two_year_return float,
  five_year_return float,
  diversification float,
  sharpe_ratio float not null,
  created_at timestamp with time zone not null default now()
);

create table published_strategy_holdings (
  published_strategy_holdings_id uuid primary key default uuid_generate_v4(),
  published_strategy_id uuid not null references published_strategy(published_strategy_id),
  created_at timestamp with time zone not null default now(),
  ticker_id uuid references ticker(ticker_id) not null,
  quantity decimal not null
);

