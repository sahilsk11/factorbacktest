create table factor_score(
  factor_score_id uuid default uuid_generate_v4() primary key,
  ticker_id uuid not null references ticker(ticker_id),
  factor_expression_hash text not null,
  date date not null,
  score float not null,
  created_at timestamp with time zone not null default now(),
  updated_at timestamp with time zone not null default now(),
  unique(factor_expression_hash, ticker_id, date)
);
