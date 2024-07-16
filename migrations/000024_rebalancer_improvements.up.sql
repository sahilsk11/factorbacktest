-- okay the tables are like
-- investment_rebalance_trade
-- strategy_investment_holdings
-- rebalancer_run
-- investment_rebalance
-- strategy_investment
-- trade_order


-- rename strategy_investment -> investment
alter table strategy_investment
rename to investment;
alter table investment
rename column strategy_investment_id to investment_id;

-- drop investment_rebalance
alter table investment_rebalance_trade
drop column investment_rebalance_id;

alter table investment_rebalance_trade
add column investment_id uuid not null references investment(investment_id);

alter table investment_rebalance_trade
add column rebalancer_run_id uuid not null references rebalancer_run(rebalancer_run_id);

drop table investment_rebalance;

-- rename investment_rebalance_trade -> investment_trade
alter table investment_rebalance_trade
rename to investment_trade;
alter table investment_trade
rename column investment_rebalance_trade_id to investment_trade_id;

-- rename strategy_investment_holdings -> holdings
alter table strategy_investment_holdings
rename to investment_holdings;
alter table investment_holdings
rename column strategy_investment_holdings_id to holdings_id;