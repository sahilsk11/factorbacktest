create table strategy_investment_holdings(
  strategy_investment_holdings_id uuid primary key default uuid_generate_v4(),
  strategy_investment_id uuid references strategy_investment(strategy_investment_id),
  date date not null,
  ticker uuid references ticker(ticker_id) not null,
  quantity decimal not null
);

alter table strategy_investment add column end_date date;

insert into ticker (symbol, name) values (':CASH', 'cash');