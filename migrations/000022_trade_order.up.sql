create type trade_order_side as enum (
  'BUY',
  'SELL'
);
create type trade_order_status as enum (
  'PENDING',
  'COMPLETED',
  'ERROR',
  'CANCELED'
);

create table trade_order(
  trade_order_id uuid primary key default uuid_generate_v4(),
  provider_id uuid not null unique,
  ticker_id uuid not null references ticker(ticker_id),
  side trade_order_side not null,
  requested_quantity decimal not null,
  status trade_order_status not null,
  filled_quantity decimal,
  filled_price decimal,
  created_at timestamp with time zone,
  modified_at timestamp with time zone,
  notes text
)