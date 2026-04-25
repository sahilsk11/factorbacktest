#!/usr/bin/env sh
set -e

echo "[auth-service] running bootstrap"
node dist/scripts/bootstrap.js

echo "[auth-service] running better-auth migrations"
# Use the compiled JS so this works in dist-only runtime images. Pinned
# version mirrors what scripts/start.sh uses in the production Fly image.
npx --yes @better-auth/cli@1.6.9 migrate --yes --config ./dist/auth.js

echo "[auth-service] starting server"
exec node dist/src/server.js
