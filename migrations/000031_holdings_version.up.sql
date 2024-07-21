-- i think we will need two new tables,
-- and i don't want either

-- one table that stores


create table investment_holdings_version(
  investment_holdings_version_id uuid primary key default uuid_generate_v4(),
  investment_id uuid not null references investment(investment_id),
  created_at timestamp with time zone not null default now()
);

drop view latest_investment_holdings;

alter table investment_holdings
drop column rebalancer_run_id;


alter table investment_holdings
add column investment_holdings_version_id uuid not null references investment_holdings_version(investment_holdings_version_id);

create view latest_investment_holdings as
WITH RankedHoldings AS (
  SELECT
    ih.investment_id,
    rr.investment_holdings_version_id,
    rr.created_at,
    ROW_NUMBER() OVER (
      PARTITION BY ih.investment_id
      ORDER BY
        rr.created_at DESC
    ) AS rn
  FROM
    investment_holdings ih
    JOIN investment_holdings_version rr ON ih.investment_holdings_version_id = rr.investment_holdings_version_id
)
SELECT
  investment_holdings_id,
  i.investment_id,
  i.ticker_id,
  symbol,
  quantity,
  RankedHoldings.created_at,
  i.investment_holdings_version_id
FROM
  RankedHoldings
  join investment_holdings i on i.investment_id = RankedHoldings.investment_id
  and RankedHoldings.investment_holdings_version_id = i.investment_holdings_version_id
  join ticker on ticker.ticker_id = i.ticker_id
WHERE
  rn = 1;