create type rebalancer_run_type as enum (
  'SCHEDULED_INVESTMENT_REBALANCE',
  'MANUAL_INVESTMENT_REBALANCE',
  -- i know these aren't really rebalancing action
  -- but they kind of mean, "some action around changes
  -- portfolio positions"
  'DEPOSIT',
  'WITHDRAWAL'
);

alter table rebalancer_run
add column rebalancer_run_type rebalancer_run_type not null;