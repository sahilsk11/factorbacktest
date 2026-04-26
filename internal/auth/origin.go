package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// requireOrigin returns a middleware that rejects state-changing POSTs
// whose Origin header doesn't match the configured allowlist. This is the
// CSRF defense for endpoints where SameSite=Lax alone isn't tight enough
// (we have no form-encoded endpoints, but a future contributor adding one
// shouldn't accidentally lose CSRF protection).
//
// Behavior:
//   - Origin missing  -> 403. Browser-issued same-origin XHR/fetch from
//     factor.trade always sets Origin; absent = not a browser.
//   - Origin set but not in allowlist -> 403.
//   - Origin in allowlist -> pass through.
//
// Origin is checked, NOT Referer. Referer can be stripped by a referrer
// policy or relaxed by user settings; Origin is the modern, reliable
// header for cross-origin distinction.
func (s *Service) requireOrigin() gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(s.cfg.AllowedOrigins))
	for _, o := range s.cfg.AllowedOrigins {
		allowed[o] = struct{}{}
	}
	// PublicBaseURL itself should always be allowed; tests that hit the
	// API directly from a server-side context include the API origin.
	allowed[s.cfg.PublicBaseURL] = struct{}{}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if _, ok := allowed[origin]; !ok {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
