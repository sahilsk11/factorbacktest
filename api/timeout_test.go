package api

import (
	"net/http"
	"testing"
	"time"
)

func TestRequestTimeout(t *testing.T) {
	if requestTimeout(http.MethodGet, "/") != 10*time.Second {
		t.Fatalf("unexpected / timeout")
	}
	if requestTimeout(http.MethodPost, "/internal/cron/rebalance") != 2*time.Hour {
		t.Fatalf("unexpected rebalance timeout")
	}
	if requestTimeout(http.MethodPost, "/internal/cron/updateOrders") != 12*time.Minute {
		t.Fatalf("unexpected updateOrders timeout")
	}
	if requestTimeout(http.MethodGet, "/publishedStrategies") != defaultRequestTimeout {
		t.Fatalf("unexpected default timeout")
	}
}
