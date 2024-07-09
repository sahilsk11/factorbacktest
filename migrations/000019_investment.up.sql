create table strategy_investment(
  strategy_investment_id uuid primary key default uuid_generate_v4(),
  amount_dollars int not null,
  start_date date not null,
  saved_stragy_id uuid not null references saved_strategy(saved_stragy_id),
  user_account_id uuid not null references user_account(user_account_id),
  created_at timestamp with time zone not null default now(),
  modified_at timestamp with time zone not null default now()
)