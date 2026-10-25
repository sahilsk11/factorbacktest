CREATE TABLE corporate_action (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  symbol text NOT NULL,
  effective_date date NOT NULL,
  type text NOT NULL CHECK (type IN ('split','dividend','symbol_change','acquired','delisted','other')),
  ratio numeric NULL,
  cash_amount numeric NULL,
  new_symbol text NULL,
  source text NOT NULL,
  notes text NULL,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (symbol, effective_date, type)
);
