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
  provider_id uuid,
  ticker_id uuid not null references ticker(ticker_id),
  side trade_order_side not null,
  requested_amount_in_dollars decimal not null,
  status trade_order_status not null,
  filled_quantity decimal not null,
  filled_price decimal,
  filled_at timestamp with time zone,
  created_at timestamp with time zone not null default now(),
  modified_at timestamp with time zone not null default now(),
  notes text
)