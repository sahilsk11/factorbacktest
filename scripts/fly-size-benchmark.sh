#!/usr/bin/env bash
# Measure public API latency + /backtest wall time against a Fly deployment.
# Usage: ./scripts/fly-size-benchmark.sh [phase_label]
# Writes JSON under /opt/cursor/artifacts/ or ./artifacts/ when not in cloud.
set -euo pipefail

PHASE="${1:-unlabeled}"
API_BASE="${API_BASE:-https://api.factor.trade}"
OUT_DIR="${OUT_DIR:-/opt/cursor/artifacts}"
if [[ ! -d "$OUT_DIR" ]]; then
  OUT_DIR="./artifacts"
  mkdir -p "$OUT_DIR"
fi

PAYLOAD="${PAYLOAD:-/tmp/payload.json}"
if [[ ! -f "$PAYLOAD" ]]; then
  echo "Missing $PAYLOAD — copy sample from docs/latency.md" >&2
  exit 1
fi

latency_endpoint() {
  local name="$1" url="$2" method="${3:-GET}" data="${4:-}"
  local -a times=()
  local i code t
  for i in $(seq 1 25); do
    if [[ "$method" == "POST" ]]; then
      read -r code t < <(curl -sS -o /dev/null -w '%{http_code} %{time_total}' -X POST "$url" -H 'Content-Type: application/json' --data-binary "$data")
    else
      read -r code t < <(curl -sS -o /dev/null -w '%{http_code} %{time_total}' "$url")
    fi
    times+=("$t")
  done
  python3 - "$name" "${times[@]}" <<'PY'
import json, sys, statistics
name, *raw = sys.argv[1:]
times = sorted(float(x) for x in raw)
def pct(p):
    idx = int(round((p/100) * (len(times)-1)))
    return times[idx]
print(json.dumps({
    "endpoint": name,
    "n": len(times),
    "p50_s": pct(50),
    "p95_s": pct(95),
    "mean_s": statistics.mean(times),
}))
PY
}

echo "== Latency ($PHASE) =="
LAT_JSON="["
LAT_JSON+="$(latency_endpoint 'GET /' "$API_BASE/")"
LAT_JSON+=",$(latency_endpoint 'GET /publishedStrategies' "$API_BASE/publishedStrategies")"
LAT_JSON+=",$(latency_endpoint 'GET /assetUniverses' "$API_BASE/assetUniverses")"
LAT_JSON+="]"

echo "== Warm backtest x5 ($PHASE) =="
BT_WALLS=()
for i in $(seq 1 5); do
  nonce="fly-bench-${PHASE}-warm-$(date +%s)-$i"
  tagged=$(mktemp)
  jq --arg n "$nonce" '.factorOptions.name = $n' "$PAYLOAD" > "$tagged"
  read -r code wall < <(curl -sS -o /dev/null -w '%{http_code} %{time_total}' -X POST "$API_BASE/backtest" -H 'Content-Type: application/json' --data-binary @"$tagged")
  rm -f "$tagged"
  echo "  warm $i: http=$code total=${wall}s"
  BT_WALLS+=("$wall")
done

OUT_FILE="$OUT_DIR/fly_benchmark_${PHASE// /_}.json"
python3 - "$PHASE" "$LAT_JSON" "${BT_WALLS[@]}" <<'PY' > "$OUT_FILE"
import json, sys, statistics, time
phase, lat_json, *walls = sys.argv[1:]
walls = [float(w) for w in walls]
ws = sorted(walls)
def pct(arr, p):
    idx = int(round((p/100) * (len(arr)-1)))
    return arr[idx]
doc = {
    "phase": phase,
    "timestamp": time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
    "api_base": "https://api.factor.trade",
    "latency": json.loads(lat_json),
    "backtest_warm": {
        "n": len(walls),
        "p50_s": pct(ws, 50),
        "p95_s": pct(ws, 95),
        "mean_s": statistics.mean(walls),
        "walls_s": walls,
    },
}
print(json.dumps(doc, indent=2))
PY

echo "Wrote $OUT_FILE"
