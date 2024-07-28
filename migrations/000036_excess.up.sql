create table excess_trade_volume (
  excess_trade_volume_id uuid primary key default uuid_generate_v4(),
  ticker_id uuid not null references ticker(ticker_id),
  quantity decimal not null,
  last_trade_order_id uuid not null references trade_order(trade_order_id),
  created_at timestamp with time zone not null default now()
);

create view latest_excess_trade_volume as
with ranked_excess as (
  select
    *,
    row_number() over (
      partition by ticker_id
      order by created_at desc
    ) as row_num
  from excess_trade_volume
)
select
excess_trade_volume_id,
ticker.ticker_id,
symbol,
quantity,
last_trade_order_id,
created_at
from ranked_excess
join ticker on ticker.ticker_id = ranked_excess.ticker_id
where ranked_excess.row_num = 1;