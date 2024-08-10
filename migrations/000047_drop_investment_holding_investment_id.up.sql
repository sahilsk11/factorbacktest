drop view latest_investment_holdings;

CREATE VIEW latest_investment_holdings AS  WITH rankedholdings AS (
         SELECT rr.investment_id AS investment_id,
            rr.investment_holdings_version_id,
            rr.created_at,
            row_number() OVER (PARTITION BY rr.investment_id ORDER BY rr.created_at DESC) AS rn
           FROM investment_holdings ih
             JOIN investment_holdings_version rr ON ih.investment_holdings_version_id = rr.investment_holdings_version_id
        )
 SELECT i.investment_holdings_id,
    rankedholdings.investment_id,
    i.ticker_id,
    ticker.symbol,
    i.quantity,
    rankedholdings.created_at,
    i.investment_holdings_version_id
   FROM rankedholdings
     JOIN investment_holdings i ON rankedholdings.investment_holdings_version_id = i.investment_holdings_version_id
     JOIN ticker ON ticker.ticker_id = i.ticker_id
  WHERE rankedholdings.rn = 1;

alter table
investment_holdings
drop column investment_id;
