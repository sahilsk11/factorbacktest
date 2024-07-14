create table strategy_investment_holdings(
  strategy_investment_holdings_id uuid primary key default uuid_generate_v4(),
  strategy_investment_id uuid references strategy_investment(strategy_investment_id),
  date date not null,
  ticker uuid references ticker(ticker_id) not null,
  quantity decimal not null
);

alter table strategy_investment add column end_date date;

insert into ticker (symbol, name) values (':CASH', 'cash');

CREATE VIEW latest_strategy_investment_holdings AS
WITH ranked_holdings AS (
    SELECT
        sih.strategy_investment_holdings_id,
        sih.strategy_investment_id,
        sih.date,
        sih.ticker,
        sih.quantity,
        t.symbol,
        ROW_NUMBER() OVER (PARTITION BY sih.strategy_investment_id ORDER BY sih.date DESC) AS rnk
    FROM
        strategy_investment_holdings sih
    JOIN
        ticker t ON sih.ticker = t.ticker_id
)
SELECT
    strategy_investment_holdings_id,
    strategy_investment_id,
    date,
    ticker,
    quantity,
    symbol
FROM
    ranked_holdings
WHERE
    rnk = 1;
