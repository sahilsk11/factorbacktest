package api

import (
	"net/http"
	"testing"
)

func TestSkipAPIRequestAudit(t *testing.T) {
	tests := []struct {
		method string
		path   string
		skip   bool
	}{
		{http.MethodGet, "/", true},
		{http.MethodGet, "/health", true},
		{http.MethodGet, "/publishedStrategies", false},
		{http.MethodPost, "/", false},
		{http.MethodGet, "/usageStats", false},
		{http.MethodGet, "//", false},
	}

	for _, tc := range tests {
		got := skipAPIRequestAudit(tc.method, tc.path)
		if got != tc.skip {
			t.Errorf("skipAPIRequestAudit(%q, %q) = %v, want %v", tc.method, tc.path, got, tc.skip)
		}
	}
}
