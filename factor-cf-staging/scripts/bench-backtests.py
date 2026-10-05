#!/usr/bin/env python3
"""
Run published-strategy backtests on Fly prod and/or CF staging for CF vs Fly bench.

Requires API build with factor-score DB bypass (this PR):
  FB_DISABLE_FACTOR_SCORE_DB=1  OR
  FB_BENCH_ALLOW_SCORE_CACHE_BYPASS=1 + header X-FB-Disable-Factor-Score-DB: 1

Usage:
  python3 factor-cf-staging/scripts/bench-backtests.py
  FLY_URL=https://api.factor.trade CF_STAGING_URL=https://factor-api-staging.... python3 ...
"""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, asdict
from datetime import date, timedelta
from typing import Any

FLY_URL = os.environ.get("FLY_URL", "https://api.factor.trade").rstrip("/")
CF_STAGING_URL = os.environ.get(
    "CF_STAGING_URL", "https://factor-api-staging.sahilkapur-a.workers.dev"
).rstrip("/")
HORIZONS_YEARS = [int(x) for x in os.environ.get("HORIZONS", "1,3,5,10").split(",")]
BENCH_HEADER = os.environ.get("BENCH_NO_SCORE_CACHE_HEADER", "X-FB-Disable-Factor-Score-DB")
RUN_CF = os.environ.get("RUN_CF", "1") == "1"
RUN_FLY = os.environ.get("RUN_FLY", "1") == "1"
END_DATE = date.fromisoformat(os.environ.get("BENCH_END_DATE", date.today().isoformat()))

# From GET /publishedStrategies on 2026-10-05 (safe prod strategies).
STRATEGIES = [
    {
        "key": "seven_day_momentum",
        "name": "7-day momentum",
        "expression": "pricePercentChange(\n  nDaysAgo(7),\n  currentDate\n)",
        "assetUniverse": "SPY_TOP_300",
        "numSymbols": 10,
        "samplingIntervalUnit": "monthly",
    },
    {
        "key": "exploding_rockets",
        "name": "exploding rockets",
        "expression": (
            "(\n        (\n          pricePercentChange(\n            addDate(currentDate, 0, -6, 0),\n"
            "            currentDate\n          ) + pricePercentChange(\n            addDate(currentDate, 0, -12, 0),\n"
            "            currentDate\n          ) + pricePercentChange(\n            addDate(currentDate, 0, -18, 0),\n"
            "            currentDate\n          ) - 5 * pricePercentChange(\n            addDate(currentDate, 0,0,  -7),\n"
            "            currentDate\n          )\n        )\n) * (2*stdev(addDate(currentDate, -1, 0, 0), currentDate))\n"
        ),
        "assetUniverse": "SPY_TOP_300",
        "numSymbols": 3,
        "samplingIntervalUnit": "monthly",
    },
]


@dataclass
class BenchResult:
    platform: str
    strategy: str
    horizon_years: int
    backtest_start: str
    backtest_end: str
    wall_seconds: float
    http_code: int
    ok: bool
    error: str | None
    strategy_id: str | None


def horizon_start(years: int) -> str:
    # Calendar approximation matching typical FE backtest picks.
    return (END_DATE - timedelta(days=365 * years)).isoformat()


def post_backtest(base_url: str, platform: str, strat: dict[str, Any], years: int) -> BenchResult:
    start = horizon_start(years)
    end = END_DATE.isoformat()
    payload = {
        "factorOptions": {
            "expression": strat["expression"],
            "name": f"bench-{platform}-{strat['key']}-{years}y-{int(time.time())}",
        },
        "backtestStart": start,
        "backtestEnd": end,
        "samplingIntervalUnit": strat["samplingIntervalUnit"],
        "startCash": 10000,
        "numSymbols": strat["numSymbols"],
        "assetUniverse": strat["assetUniverse"],
    }
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        f"{base_url}/backtest",
        data=body,
        method="POST",
        headers={
            "Content-Type": "application/json",
            BENCH_HEADER: "1",
        },
    )
    t0 = time.perf_counter()
    http_code = 0
    err: str | None = None
    strategy_id: str | None = None
    ok = False
    try:
        with urllib.request.urlopen(req, timeout=3600) as resp:
            http_code = resp.getcode()
            raw = resp.read().decode("utf-8")
            wall = time.perf_counter() - t0
            data = json.loads(raw)
            if isinstance(data, dict) and data.get("error"):
                err = str(data["error"])
            else:
                ok = http_code == 200 and err is None
                if isinstance(data, dict):
                    strategy_id = data.get("strategyID")
            return BenchResult(
                platform=platform,
                strategy=strat["name"],
                horizon_years=years,
                backtest_start=start,
                backtest_end=end,
                wall_seconds=wall,
                http_code=http_code,
                ok=ok,
                error=err,
                strategy_id=strategy_id,
            )
    except urllib.error.HTTPError as e:
        wall = time.perf_counter() - t0
        raw = e.read().decode("utf-8", errors="replace")
        try:
            err = json.loads(raw).get("error", raw[:500])
        except json.JSONDecodeError:
            err = raw[:500]
        return BenchResult(
            platform=platform,
            strategy=strat["name"],
            horizon_years=years,
            backtest_start=start,
            backtest_end=end,
            wall_seconds=wall,
            http_code=e.code,
            ok=False,
            error=str(err),
            strategy_id=None,
        )
    except Exception as e:  # noqa: BLE001
        wall = time.perf_counter() - t0
        return BenchResult(
            platform=platform,
            strategy=strat["name"],
            horizon_years=years,
            backtest_start=start,
            backtest_end=end,
            wall_seconds=wall,
            http_code=http_code,
            ok=False,
            error=str(e),
            strategy_id=None,
        )


def reachable(base: str) -> bool:
    try:
        req = urllib.request.Request(f"{base}/", method="GET")
        with urllib.request.urlopen(req, timeout=20) as resp:
            return resp.getcode() == 200
    except Exception:
        return False


def main() -> int:
    platforms: list[tuple[str, str]] = []
    if RUN_FLY:
        platforms.append(("fly", FLY_URL))
    if RUN_CF and reachable(CF_STAGING_URL):
        platforms.append(("cf_staging", CF_STAGING_URL))
    elif RUN_CF:
        print(f"skip CF staging: {CF_STAGING_URL}/ not HTTP 200", file=sys.stderr)

    results: list[BenchResult] = []
    for platform, url in platforms:
        for strat in STRATEGIES:
            for years in HORIZONS_YEARS:
                print(f"==> {platform} {strat['name']} {years}y ...", flush=True)
                results.append(post_backtest(url, platform, strat, years))
                r = results[-1]
                status = "ok" if r.ok else "FAIL"
                print(
                    f"    {status} wall={r.wall_seconds:.1f}s http={r.http_code} err={r.error}",
                    flush=True,
                )

    out_path = os.environ.get(
        "BENCH_OUT", "factor-cf-staging/bench-results-latest.json"
    )
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump([asdict(r) for r in results], f, indent=2)
    print(f"wrote {out_path}")
    return 0 if all(r.ok for r in results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
