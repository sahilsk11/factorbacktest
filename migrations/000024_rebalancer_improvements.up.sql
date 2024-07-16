-- okay the tables are like
-- investment_rebalance_trade
-- strategy_investment_holdings
-- rebalancer_run
-- investment_rebalance
-- strategy_investment
-- trade_order
-- rename strategy_investment -> investment
alter table
  strategy_investment rename to investment;

alter table
  investment rename column strategy_investment_id to investment_id;

-- drop investment_rebalance
alter table
  investment_rebalance_trade drop column investment_rebalance_id;

alter table
  investment_rebalance_trade
add
  column investment_id uuid not null references investment(investment_id);

alter table
  investment_rebalance_trade
add
  column rebalancer_run_id uuid not null references rebalancer_run(rebalancer_run_id);

drop table investment_rebalance;

-- rename investment_rebalance_trade -> investment_trade
alter table
  investment_rebalance_trade rename to investment_trade;

alter table
  investment_trade rename column investment_rebalance_trade_id to investment_trade_id;

-- rename strategy_investment_holdings -> holdings
alter table
  strategy_investment_holdings rename to investment_holdings;

alter table
  investment_holdings rename column strategy_investment_holdings_id to investment_holdings_id;

alter table
  investment_holdings rename column strategy_investment_id to investment_id;

-- make holdings reference last rebalance run
alter table
  investment_holdings
add
  column rebalancer_run_id uuid not null references rebalancer_run(rebalancer_run_id);

drop view latest_strategy_investment_holdings;

alter table
  investment_holdings drop column date;

CREATE VIEW latest_investment_holdings AS
WITH latest_runs AS (
    SELECT
        ih.investment_id,
        MAX(rr.date) AS latest_date
    FROM
        investment_holdings ih
    JOIN
        rebalancer_run rr ON ih.rebalancer_run_id = rr.rebalancer_run_id
    GROUP BY
        ih.investment_id
),
latest_holdings AS (
    SELECT
        ih.*
    FROM
        investment_holdings ih
    JOIN
        rebalancer_run rr ON ih.rebalancer_run_id = rr.rebalancer_run_id
    JOIN
        latest_runs lr ON ih.investment_id = lr.investment_id AND rr.date = lr.latest_date
)
SELECT
    lh.investment_holdings_id,
    lh.investment_id,
    lh.ticker,
    t.symbol,
    lh.quantity,
    lh.created_at,
    lh.rebalancer_run_id
FROM
    latest_holdings lh
JOIN
    ticker t ON lh.ticker = t.ticker_id
JOIN (
    SELECT
        investment_id,
        ticker,
        MAX(created_at) AS max_created_at
    FROM
        latest_holdings
    GROUP BY
        investment_id, ticker
) max_dates ON lh.investment_id = max_dates.investment_id 
           AND lh.ticker = max_dates.ticker 
           AND lh.created_at = max_dates.max_created_at;
