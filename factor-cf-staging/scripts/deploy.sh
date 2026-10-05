#!/usr/bin/env bash
# Deploy Factor API staging to Cloudflare Containers (workers.dev).
# Requires: Docker, Node, CF token with Containers scope (CF_DEPLOY_RESUME_BUILDER).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
STAGING="$ROOT/factor-cf-staging"
cd "$STAGING"

export CLOUDFLARE_API_TOKEN="${CF_DEPLOY_RESUME_BUILDER:-${CLOUDFLARE_API_TOKEN:-}}"
if [[ -z "${CLOUDFLARE_API_TOKEN}" ]]; then
	echo "error: set CF_DEPLOY_RESUME_BUILDER or CLOUDFLARE_API_TOKEN" >&2
	exit 1
fi

if [[ -z "${CLOUDFLARE_ACCOUNT_ID:-}" ]]; then
	echo "error: set CLOUDFLARE_ACCOUNT_ID" >&2
	exit 1
fi

if ! docker info >/dev/null 2>&1; then
	echo "error: Docker must be running (wrangler builds/pushes the container image)" >&2
	exit 1
fi

npm install --silent

if [[ -f ".dev.vars" ]]; then
	echo "==> uploading Worker secrets from .dev.vars"
	while IFS='=' read -r key value; do
		[[ -z "${key}" || "${key}" =~ ^# ]] && continue
		[[ "${key}" == "CLOUDFLARE_ACCOUNT_ID" ]] && continue
		value="${value%$'\r'}"
		if [[ -n "${value}" ]]; then
			printf '%s' "${value}" | npx wrangler secret put "${key}" >/dev/null
			echo "  secret set: ${key}"
		fi
	done < ".dev.vars"
fi

COMMIT="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo cf-staging)"

echo "==> wrangler deploy (commit ${COMMIT})"
npx wrangler deploy --build-arg "commit_hash=${COMMIT}"

echo "Done. Staging URL: https://factor-api-staging.sahilkapur-a.workers.dev (or your account subdomain)"
