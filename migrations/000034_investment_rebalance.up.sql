create table investment_rebalance(
  investment_rebalance_id uuid primary key default uuid_generate_v4(),
  rebalancer_run_id uuid references rebalancer_run(rebalancer_run_id) not null,
  investment_id uuid references investment(investment_id) not null,
  state rebalancer_run_state not null,
  created_at timestamp with time zone not null default now(),
  modified_at timestamp with time zone not null default now(),
  starting_holdings_version_id uuid not null references investment_holdings_version(investment_holdings_version_id),
  starting_portfolio json not null,
  target_portfolio json not null
);

alter table investment_trade
add column investment_rebalance_id uuid not null references investment_rebalance(investment_rebalance_id);

drop view investment_trade_status;

alter table investment_trade
drop column rebalancer_run_id;

alter table investment_trade
drop column investment_id;


create view investment_trade_status as
select
investment_trade_id,
investment_trade.side,
symbol,
status,
quantity,
filled_price,
quantity::decimal * filled_price::decimal as "filled_amount",
filled_at,
investment_rebalance.rebalancer_run_id,
investment_id,
trade_order.trade_order_id,
ticker.ticker_id
from investment_trade
left join trade_order on trade_order.trade_order_id = investment_trade.trade_order_id
join ticker on investment_trade.ticker_id = ticker.ticker_id
join investment_rebalance on investment_rebalance.investment_rebalance_id = investment_trade.investment_rebalance_id;