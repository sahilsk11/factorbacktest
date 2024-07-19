-- create type rebalancer_run_state as enum (
--   'COMPLETED',
--   'PENDING',
--   'ERROR'
-- );

-- alter type rebalancer_run_state add value 'PARTIAL_ERROR';

alter table rebalancer_run
add column rebalancer_run_state rebalancer_run_state not null default 'PENDING';

alter table rebalancer_run add column
modified_at timestamp with time zone not null default now();

alter table rebalancer_run add column
num_investments_attempted int not null;

create table investment_rebalance_error (
  investment_rebalance_error_id uuid primary key default uuid_generate_v4(),
  rebalancer_run_id uuid not null references rebalancer_run(rebalancer_run_id),
  investment_id uuid not null references investment(investment_id),
  error_message text not null
);

alter table investment_trade
add column modified_at timestamp with time zone not null default now();

-- maybe use num failed / num attempted?