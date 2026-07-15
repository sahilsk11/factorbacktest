CREATE TABLE reconciliation_run (
  reconciliation_run_id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  status text NOT NULL CHECK (status IN ('PREVIEW', 'APPLIED', 'STALE')),
  broker_snapshot jsonb NOT NULL,
  ledger_snapshot jsonb NOT NULL,
  proposed_adjustments jsonb NOT NULL,
  applied_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now()
);

ALTER TABLE investment_holdings_version
ADD COLUMN reconciliation_run_id uuid REFERENCES reconciliation_run(reconciliation_run_id);
