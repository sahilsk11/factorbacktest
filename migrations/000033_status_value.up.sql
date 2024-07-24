drop view investment_trade_status;

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
investment_trade.rebalancer_run_id,
investment_id,
trade_order.trade_order_id,
ticker.ticker_id
from investment_trade
left join trade_order on trade_order.trade_order_id = investment_trade.trade_order_id
join ticker on investment_trade.ticker_id = ticker.ticker_id;