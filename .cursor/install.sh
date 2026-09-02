#!/usr/bin/env bash
# Cloud Agent install phase: idempotent, durable setup that is baked into the
# environment build snapshot. It must terminate and must NOT leave any
# long-running process alive (Postgres and the app servers are started in
# .cursor/start.sh instead).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> factorbacktest install: system packages"
export DEBIAN_FRONTEND=noninteractive
sudo apt-get update -qq
# postgresql        - local Postgres server for dev/test (matches docker-compose)
# libpq-dev         - headers for building psycopg2 from tools/requirements.txt
# python3-dev/venv  - build + isolate the tools/ Python environment
# build-essential   - C toolchain for the psycopg2 native extension
sudo apt-get install -y -qq \
  postgresql postgresql-contrib \
  libpq-dev python3-dev python3-venv build-essential

echo "==> factorbacktest install: Go build + tooling"
# Build every package so the module graph and build cache are warm.
go build ./...
# go-jet CLI is used by `make db-models` to regenerate DB models.
go install github.com/go-jet/jet/v2/cmd/jet@v2.10.1

echo "==> factorbacktest install: Python tooling (tools/env)"
if [ ! -x tools/env/bin/python ]; then
  python3 -m venv tools/env
fi
tools/env/bin/pip install --upgrade pip -q
tools/env/bin/pip install -q -r tools/requirements.txt

echo "==> factorbacktest install: frontend-v2 dependencies"
( cd frontend-v2 && npm ci )

echo "==> factorbacktest install: initialize Postgres data directory"
# Only initialize the cluster here (durable state captured by the snapshot).
# Starting the server and applying migrations happens per-boot in start.sh.
PG_BINDIR="$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)"
PGDATA="$HOME/.factorbacktest/pgdata"
mkdir -p "$HOME/.factorbacktest/sock"
if [ ! -s "$PGDATA/PG_VERSION" ]; then
  # -A trust: local connections need no password. The app/tools still pass
  # user=postgres/password=postgres (from secrets-test.json); trust ignores it.
  "$PG_BINDIR/initdb" -U postgres -A trust -D "$PGDATA"
  # Bind to 5440 to match docker-compose.yml and secrets-test.json, and put the
  # unix socket somewhere writable ($HOME) since /var/run/postgresql is not.
  sed -i "s/^#\?port = .*/port = 5440/" "$PGDATA/postgresql.conf"
  sed -i "s/^#\?listen_addresses = .*/listen_addresses = 'localhost'/" "$PGDATA/postgresql.conf"
  echo "unix_socket_directories = '$HOME/.factorbacktest/sock'" >> "$PGDATA/postgresql.conf"
fi

echo "==> factorbacktest install: complete"
