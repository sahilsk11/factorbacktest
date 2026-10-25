CREATE TABLE raw_price (
  symbol text NOT NULL,
  date date NOT NULL,
  close numeric NOT NULL,
  volume numeric NULL,
  source text NOT NULL,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (symbol, date)
);
