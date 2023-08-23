CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE adjusted_price(
  id serial primary key,
  date date not null,
  symbol text not null,
  price decimal not null,
  created_at timestamp with time zone not null,
  unique(date, symbol)
);