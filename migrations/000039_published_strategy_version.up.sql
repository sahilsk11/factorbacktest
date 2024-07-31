create table published_strategy_holdings_version (
  published_strategy_holdings_version_id uuid primary key default uuid_generate_v4(),
  version_id uuid not null default uuid_generate_v4(),
  created_at timestamp with time zone not null
);

alter table
  published_strategy_holdings
add
  column published_strategy_holdings_version_id uuid not null references published_strategy_holdings_version(published_strategy_holdings_version_id);

create view latest_published_strategy_holdings as WITH rankedholdings AS (
  SELECT
    ih.published_strategy_id,
    rr.published_strategy_holdings_version_id,
    rr.created_at,
    row_number() OVER (
      PARTITION BY ih.published_strategy_id
      ORDER BY
        rr.created_at DESC
    ) AS rn
  FROM
    published_strategy_holdings ih
    JOIN published_strategy_holdings_version rr ON ih.published_strategy_holdings_version_id = rr.published_strategy_holdings_version_id
)
SELECT
  i.published_strategy_holdings_id,
  i.published_strategy_id,
  i.ticker_id,
  ticker.symbol,
  i.quantity,
  rankedholdings.created_at,
  i.published_strategy_holdings_version_id
FROM
  rankedholdings
  JOIN published_strategy_holdings i ON i.published_strategy_id = rankedholdings.published_strategy_id
  AND rankedholdings.published_strategy_holdings_version_id = i.published_strategy_holdings_version_id
  JOIN ticker ON ticker.ticker_id = i.ticker_id
WHERE
  rankedholdings.rn = 1;