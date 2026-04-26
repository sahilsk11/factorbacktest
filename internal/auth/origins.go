package auth

import (
	"os"
	"strings"
)

// AppOrigins returns the list of browser origins the API trusts. This is
// the single source of truth for both CORS (api/api.go) and the
// `requireOrigin` middleware here in the auth package — having one
// builder ensures the two checks can't drift.
//
// Sources, in order:
//   - the static prod / dev FE origins (factor.trade, factorbacktest.net,
//     localhost:3000, 127.0.0.1:3000)
//   - any value in $EXTRA_ALLOWED_ORIGINS (CSV) — e.g. for Playwright
//     harnesses that pick a random port
//
// Deliberately omitted: the API's own public base URL. It's a server
// origin, not a browser origin; legitimate browser traffic never has
// it in the Origin header.
func AppOrigins() []string {
	out := []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"https://factorbacktest.net",
		"https://www.factorbacktest.net",
		"https://factor.trade",
		"https://www.factor.trade",
	}
	if extra := os.Getenv("EXTRA_ALLOWED_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			if t := strings.TrimSpace(o); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}
