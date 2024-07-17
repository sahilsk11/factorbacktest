drop view latest_investment_holdings;

alter table
  investment_holdings rename column ticker to ticker_id;

create view latest_investment_holdings as WITH RankedHoldings AS (
  SELECT
    ih.investment_id,
    rr.rebalancer_run_id,
    rr.date as "rebalancer_run_date",
    ROW_NUMBER() OVER (
      PARTITION BY ih.investment_id
      ORDER BY
        rr.date DESC,
        rr.created_at DESC
    ) AS rn
  FROM
    investment_holdings ih
    JOIN rebalancer_run rr ON ih.rebalancer_run_id = rr.rebalancer_run_id
)
SELECT
  investment_holdings_id,
  i.investment_id,
  i.ticker_id,
  symbol,
  quantity,
  RankedHoldings.rebalancer_run_date,
  i.rebalancer_run_id
FROM
  RankedHoldings
  join investment_holdings i on i.investment_id = RankedHoldings.investment_id
  and RankedHoldings.rebalancer_run_id = i.rebalancer_run_id
  join ticker on ticker.ticker_id = i.ticker_id
WHERE
  rn = 1;