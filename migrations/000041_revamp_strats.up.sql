alter table saved_strategy rename to strategy;

alter table strategy
rename column saved_stragy_id to strategy_id;

alter table strategy
rename column bookmarked to saved;

alter table strategy
add column published boolean not null default false;

alter table strategy
drop column backtest_start;

alter table strategy
drop column backtest_end;

create table strategy_run(
  strategy_run_id uuid primary key default uuid_generate_v4(),
  strategy_id uuid not null references strategy(strategy_id),
  start_date date not null,
  end_date date not null,
  sharpe_ratio float,
  annualized_return float,
  annualuzed_stdev float,
  created_at timestamp with time zone not null default now()
);

drop view latest_published_strategy_holdings;
drop table published_strategy_holdings;
drop table published_strategy_holdings_version;
drop table published_strategy_stats;
drop table published_strategy;

alter table investment
rename column saved_stragy_id to strategy_id;
