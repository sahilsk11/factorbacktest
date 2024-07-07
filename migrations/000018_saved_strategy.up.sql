create table saved_strategy(
  saved_stragy_id uuid default uuid_generate_v4() primary key,
  strategy_name text not null,
  factor_expression text not null,
  backtest_start date not null,
  backtest_end date not null,
  rebalance_interval text not null,
  num_assets int not null,
  asset_universe text not null references asset_universe(asset_universe_name),
  bookmarked boolean not null default false,
  user_account_id uuid not null references user_account(user_account_id),
  created_at timestamp with time zone not null default now(),
  modified_at timestamp with time zone not null default now()
)