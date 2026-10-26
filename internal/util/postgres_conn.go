package util

import (
	"net/url"
	"strconv"
	"strings"
)

// EnsurePostgresConnectTimeout adds lib/pq connect_timeout when missing so new
// connections fail fast instead of hanging when Neon compute is suspended.
func EnsurePostgresConnectTimeout(connStr string, timeoutSec int) string {
	if timeoutSec <= 0 {
		return connStr
	}
	param := "connect_timeout"
	value := strconv.Itoa(timeoutSec)

	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		u, err := url.Parse(connStr)
		if err != nil {
			return connStr
		}
		q := u.Query()
		if q.Get(param) == "" {
			q.Set(param, value)
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	if strings.Contains(connStr, param+"=") {
		return connStr
	}
	return strings.TrimSpace(connStr) + " " + param + "=" + value
}
