create view latest_rebalance_holdings as with ranked_versions as (
  select
    investment_holdings_version_id,
    rebalancer_run_id,
    created_at,
    investment_id,
    row_number() over (
      PARTITION BY ihv.rebalancer_run_id,
      ihv.investment_id
      ORDER BY
        ihv.created_at DESC
    ) AS rn
  from
    investment_holdings_version ihv
)
select
  investment_holdings_id,
  investment_id,
  symbol,
  quantity,
  coalesce(
    case
      when ticker.symbol = ':CASH' then 1
      else price
    end,
    null
  ) as "price_at_rebalance",
  quantity * coalesce(
    case
      when ticker.symbol = ':CASH' then 1
      else price
    end,
    null
  ) as "amount_at_rebalance",
  ih.created_at,
  ih.ticker_id,
  ih.investment_holdings_version_id,
  ranked_versions.rebalancer_run_id
from
  investment_holdings ih
  join ranked_versions on ih.investment_holdings_version_id = ranked_versions.investment_holdings_version_id
  join ticker on ih.ticker_id = ticker.ticker_id
  left join rebalance_price rp on rp.rebalancer_run_id = ranked_versions.rebalancer_run_id
  AND rp.ticker_id = ih.ticker_id
where
  rn = 1;