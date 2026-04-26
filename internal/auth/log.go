package auth

import "log"

// logf wraps the standard log package so the rest of the auth package
// has a single chokepoint for diagnostic output. We deliberately do NOT
// log request bodies, cookie values, headers, or anything that could
// contain a session id, OTP, or auth token. Anything sensitive must
// either be summarized to non-secret form (e.g. "ok"/"fail") or omitted.
func logf(format string, args ...any) {
	log.Printf("[auth] "+format, args...)
}
