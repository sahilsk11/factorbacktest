-- create type rebalancer_run_state as enum (
--   'COMPLETED',
--   'PENDING',
--   'ERROR'
-- );

alter type rebalancer_run_state add value 'PARTIAL_ERROR';

alter table rebalancer_run
add column rebalancer_run_state rebalancer_run_state not null default 'PENDING';

alter table rebalancer_run add column
modified_at timestamp with time zone not null default now();
