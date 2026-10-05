#!/usr/bin/env bash
# Side-by-side TTFB benchmark: Fly prod vs CF staging vs optional spike.
set -euo pipefail

FLY="${FLY_URL:-https://api.factor.trade}"
CF_STAGING="${CF_STAGING_URL:-https://factor-api-staging.sahilkapur-a.workers.dev}"
CF_SPIKE="${CF_SPIKE_URL:-https://factor-cf-spike.sahilkapur-a.workers.dev}"
SAMPLES="${SAMPLES:-5}"
COLD_IDLE_SEC="${COLD_IDLE_SEC:-0}"

measure() {
	local label="$1"
	local url="$2"
	local i ttfb
	printf '\n%s\n' "$label"
	for i in $(seq 1 "$SAMPLES"); do
		ttfb="$(curl -sS -o /dev/null -w '%{time_starttransfer}' "$url")"
		printf '  sample %s: ttfb=%ss\n' "$i" "$ttfb"
	done
}

cold_measure() {
	local label="$1"
	local url="$2"
	if [[ "$COLD_IDLE_SEC" -gt 0 ]]; then
		echo "Waiting ${COLD_IDLE_SEC}s for scale-to-zero..."
		sleep "$COLD_IDLE_SEC"
	fi
	local saved=$SAMPLES
	SAMPLES=1
	measure "$label" "$url"
	SAMPLES=$saved
}

echo "Benchmark $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Fly prod: $FLY"
echo "CF staging: $CF_STAGING"
echo "CF spike: $CF_SPIKE"

measure "Fly GET / (warm, n=${SAMPLES})" "${FLY}/"
measure "Fly GET /publishedStrategies (Neon, warm, n=${SAMPLES})" "${FLY}/publishedStrategies"

staging_ok() {
	local url="$1"
	local code
	code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 20 "$url" 2>/dev/null || echo 000)"
	[[ "$code" == "200" ]]
}

if staging_ok "${CF_STAGING}/"; then
	measure "CF staging GET / (n=${SAMPLES})" "${CF_STAGING}/"
	measure "CF staging GET /publishedStrategies (Neon, n=${SAMPLES})" "${CF_STAGING}/publishedStrategies"
	cold_measure "CF staging GET /publishedStrategies" "${CF_STAGING}/publishedStrategies"
else
	echo "CF staging not deployed (GET / != 200) at ${CF_STAGING}/ — run factor-cf-staging/scripts/deploy.sh"
fi

measure "CF spike GET /health (n=${SAMPLES})" "${CF_SPIKE}/health"
cold_measure "CF spike GET /health" "${CF_SPIKE}/health"
