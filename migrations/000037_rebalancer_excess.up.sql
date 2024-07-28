drop view latest_excess_trade_volume;

alter table excess_trade_volume
add column rebalancer_run_id uuid not null references rebalancer_run(rebalancer_run_id);

alter table excess_trade_volume
rename column latest_trade_order_id to trade_order_id;

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
rebalancer_run_id,
created_at
from ranked_excess
join ticker on ticker.ticker_id = ranked_excess.ticker_id
where ranked_excess.row_num = 1;