alter table strategy_investment_holdings
add column created_at timestamp with time zone not null default now();