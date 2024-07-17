alter table trade_order
drop column requested_amount_in_dollars;

alter table trade_order
add column requested_quantity decimal not null;

alter table trade_order
add column expected_price decimal not null;


alter table investment_trade
drop column amount_in_dollars;

alter table investment_trade
add column quantity decimal not null;

alter table investment_trade
add column trade_order_id uuid references trade_order(trade_order_id);