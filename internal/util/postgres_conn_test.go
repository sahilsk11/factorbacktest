package util

import (
	"strings"
	"testing"
)

func TestEnsurePostgresConnectTimeoutURL(t *testing.T) {
	in := "postgresql://user:pass@ep-test.us-east-1.aws.neon.tech/neondb?sslmode=require"
	got := EnsurePostgresConnectTimeout(in, 10)
	if !strings.Contains(got, "connect_timeout=10") {
		t.Fatalf("missing connect_timeout: %q", got)
	}
	if !strings.Contains(got, "sslmode=require") {
		t.Fatalf("lost sslmode: %q", got)
	}

	again := EnsurePostgresConnectTimeout(got, 99)
	if strings.Contains(again, "connect_timeout=99") {
		t.Fatalf("should not overwrite existing connect_timeout: %q", again)
	}
}

func TestEnsurePostgresConnectTimeoutKeyValue(t *testing.T) {
	in := "host=db.example.com port=5432 user=u password=p dbname=app sslmode=disable"
	got := EnsurePostgresConnectTimeout(in, 10)
	if !strings.Contains(got, "connect_timeout=10") {
		t.Fatalf("missing connect_timeout: %q", got)
	}
}
