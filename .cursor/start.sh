#!/usr/bin/env bash
# Cloud Agent start phase: per-boot reconciliation. Runs every time the VM
# boots. Must be idempotent, avoid duplicate processes, reach a clear success
# state, and then return. Long-running app servers live in `terminals` (see
# .cursor/environment.json), not here.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

PG_BINDIR="$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)"
PGDATA="$HOME/.factorbacktest/pgdata"
export PGHOST=localhost PGPORT=5440 PGUSER=postgres PGPASSWORD=postgres

echo "==> factorbacktest start: ensuring Postgres is running on :5440"
if ! "$PG_BINDIR/pg_isready" -q -h localhost -p 5440; then
  # -w waits for readiness; safe to call when already stopped.
  "$PG_BINDIR/pg_ctl" -D "$PGDATA" -l "$HOME/.factorbacktest/pg.log" -w start
fi
"$PG_BINDIR/pg_isready" -h localhost -p 5440

echo "==> factorbacktest start: ensuring databases exist"
# secrets-test.json points ALPHA_ENV=test at 'postgres_test'; CI also uses the
# default 'postgres' DB (db-models, integration tests). Create both if missing.
for db in postgres_test postgres; do
  if ! "$PG_BINDIR/psql" -tAc "SELECT 1 FROM pg_database WHERE datname='$db'" | grep -q 1; then
    "$PG_BINDIR/psql" -c "CREATE DATABASE $db"
  fi
done

echo "==> factorbacktest start: applying migrations (idempotent)"
# migrations.py tracks schema_version, so re-running 'up' is a no-op once the
# schema is current.
tools/env/bin/python tools/migrations.py up postgres_test
tools/env/bin/python tools/migrations.py up postgres

echo "==> factorbacktest start: ready (Postgres up, migrations applied)"
