alter table investment_trade
add column expected_price decimal not null default 0;

alter table investment_trade
alter column expected_price
drop default;

drop view investment_trade_status;
create view investment_trade_status as
select
investment_trade_id,
investment_trade.side,
symbol,
status,
quantity,
investment_trade.expected_price,
quantity::decimal * investment_trade.expected_price::decimal as "expected_amount",
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