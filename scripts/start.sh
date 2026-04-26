#!/usr/bin/env bash
# Supervisor for the Fly.io image. Runs the Node auth-service and the Go API
# in the same container. tini (PID 1) reaps zombies and forwards signals to
# this script; we forward to both children on shutdown.
set -euo pipefail

cd /app

# ---------------------------------------------------------------------------
# Bootstrap auth schema + apply Better Auth migrations.
# Both are idempotent so it's safe to do this on every container start.
# ---------------------------------------------------------------------------
echo "[start.sh] running auth-service bootstrap"
( cd /app/auth-service && node dist/scripts/bootstrap.js )

# Better Auth migrations are deliberately NOT run at container boot. The CLI
# is shipped as a separate `@better-auth/cli` package whose version line
# doesn't track the runtime library, and `npx --yes @better-auth/cli@latest`
# pulls ~150 packages from npm on every cold start (~2.5 min). The auth
# schema itself is created by the bootstrap step above (idempotent), and any
# additive migrations should be run from a developer machine via
# `npm run migrate` in `auth-service/` before the corresponding deploy.
echo "[start.sh] skipping runtime better-auth migrate (run \`npm run migrate\` locally before deploys that change auth schema)"

# ---------------------------------------------------------------------------
# Start the Node auth-service. Binds to 127.0.0.1:3001 (set in env).
# ---------------------------------------------------------------------------
echo "[start.sh] launching auth-service"
( cd /app/auth-service && node dist/src/server.js ) &
NODE_PID=$!

# Wait for /api/auth/ok before starting Go so the reverse proxy sees a healthy
# upstream from the first request. Time-bounded so a broken sidecar still
# fails the container quickly instead of hanging forever.
echo "[start.sh] waiting for auth-service health"
HEALTHY=0
for i in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:3001/api/auth/ok >/dev/null 2>&1; then
    echo "[start.sh] auth-service is up"
    HEALTHY=1
    break
  fi
  if ! kill -0 "$NODE_PID" 2>/dev/null; then
    echo "[start.sh] auth-service exited before becoming healthy"
    exit 1
  fi
  sleep 1
done

if [ "$HEALTHY" != "1" ]; then
  # Don't start Go if the sidecar isn't responding — the proxy would
  # immediately 502 on every /api/auth/* call and Fly's health check
  # would still pass, masking the failure indefinitely.
  echo "[start.sh] auth-service did not become healthy in 30s; aborting"
  kill -TERM "$NODE_PID" 2>/dev/null || true
  exit 1
fi

# ---------------------------------------------------------------------------
# Start the Go API in the foreground (so its exit triggers container exit).
# ---------------------------------------------------------------------------
echo "[start.sh] launching Go API"
/app/api &
GO_PID=$!

terminate() {
  echo "[start.sh] received signal, shutting down children"
  kill -TERM "$GO_PID" 2>/dev/null || true
  kill -TERM "$NODE_PID" 2>/dev/null || true
  wait "$GO_PID" 2>/dev/null || true
  wait "$NODE_PID" 2>/dev/null || true
  exit 0
}
trap terminate TERM INT

# If either process dies, exit so Fly restarts the machine.
wait -n "$GO_PID" "$NODE_PID"
EXIT_CODE=$?
echo "[start.sh] a child process exited (code=$EXIT_CODE), shutting down"
kill -TERM "$GO_PID" 2>/dev/null || true
kill -TERM "$NODE_PID" 2>/dev/null || true
wait "$GO_PID" 2>/dev/null || true
wait "$NODE_PID" 2>/dev/null || true
exit "$EXIT_CODE"
