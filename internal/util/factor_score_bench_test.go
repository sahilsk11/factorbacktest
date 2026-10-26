package util_test

import (
	"context"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/util"
	"testing"
)

func TestFactorScoreDBDisabled_env(t *testing.T) {
	t.Setenv("FB_DISABLE_FACTOR_SCORE_DB", "1")
	t.Setenv("FB_BENCH_ALLOW_SCORE_CACHE_BYPASS", "0")
	if !util.FactorScoreDBDisabled(context.Background()) {
		t.Fatal("expected disabled from env")
	}
}

func TestFactorScoreDBDisabled_headerBypass(t *testing.T) {
	t.Setenv("FB_DISABLE_FACTOR_SCORE_DB", "0")
	t.Setenv("FB_BENCH_ALLOW_SCORE_CACHE_BYPASS", "1")
	ctx := domain.WithFactorScoreDBDisabled(context.Background())
	if !util.FactorScoreDBDisabled(ctx) {
		t.Fatal("expected disabled from context when bypass allowed")
	}
	if util.FactorScoreDBDisabled(context.Background()) {
		t.Fatal("expected enabled without context flag")
	}
}
