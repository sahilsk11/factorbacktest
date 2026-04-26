package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// requireOrigin rejects state-changing POSTs whose Origin header isn't in
// the allowlist. Defense for endpoints where SameSite=Lax alone isn't
// tight enough; we have no form-encoded endpoints today, but a future
// contributor adding one shouldn't lose this protection.
//
// Origin missing => 403 (browser-issued same-origin XHR/fetch from
// factor.trade always sets Origin; absent = not a browser).
//
// Origin checked, NOT Referer. Referer can be stripped by a referrer
// policy or relaxed by user settings; Origin is the modern, reliable
// header for cross-origin distinction.
func (s *Service) requireOrigin() gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(s.cfg.AllowedOrigins)+1)
	for _, o := range s.cfg.AllowedOrigins {
		allowed[o] = struct{}{}
	}
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
