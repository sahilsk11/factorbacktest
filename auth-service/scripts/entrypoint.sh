#!/usr/bin/env sh
set -e

echo "[auth-service] running bootstrap"
node dist/scripts/bootstrap.js

echo "[auth-service] running better-auth migrations"
npx --yes @better-auth/cli@latest migrate --yes --config ./auth.ts

echo "[auth-service] starting server"
exec node dist/src/server.js
