CREATE TABLE admin_mutation_audit (
  admin_mutation_audit_id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  created_at timestamptz NOT NULL DEFAULT now(),
  actor text NOT NULL,
  kind text NOT NULL,
  investment_id uuid REFERENCES investment(investment_id),
  note text NOT NULL,
  dry_run boolean NOT NULL DEFAULT false,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX admin_mutation_audit_created_at_idx
  ON admin_mutation_audit (created_at DESC);

CREATE INDEX admin_mutation_audit_investment_id_idx
  ON admin_mutation_audit (investment_id)
  WHERE investment_id IS NOT NULL;
