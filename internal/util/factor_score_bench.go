package util

import (
	"context"
	"factorbacktest/internal/domain"
	"os"
)

// FactorScoreDBDisabled is true when factor_score DB cache read/write must be
// skipped for a true compute dry run (CF vs Fly benchmarks).
//
// Enable globally:
//
//	FB_DISABLE_FACTOR_SCORE_DB=1
//
// Enable per request on servers that set:
//
//	FB_BENCH_ALLOW_SCORE_CACHE_BYPASS=1
//
// and send header X-FB-Disable-Factor-Score-DB: 1 on POST /backtest.
func FactorScoreDBDisabled(ctx context.Context) bool {
	if os.Getenv("FB_DISABLE_FACTOR_SCORE_DB") == "1" {
		return true
	}
	if os.Getenv("FB_BENCH_ALLOW_SCORE_CACHE_BYPASS") == "1" {
		return domain.FactorScoreDBDisabledFromContext(ctx)
	}
	return false
}
