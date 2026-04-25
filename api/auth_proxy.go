package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// betterAuthInternalURL is the address of the local Better Auth (Node) sidecar.
// In production it runs in the same Fly machine as the Go API and binds to
// localhost only, so we don't need any TLS or auth on this hop.
func betterAuthInternalURL() string {
	if v := os.Getenv("BETTER_AUTH_INTERNAL_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:3001"
}

// newBetterAuthProxy constructs a single-host reverse proxy that forwards
// /api/auth/* to the local Node Better Auth service. We deliberately do NOT
// inject any auth headers, do not read the JWT, and do not modify the
// response body. Cookies and Set-Cookie headers pass through untouched so
// session cookies work on the same domain.
func newBetterAuthProxy() (gin.HandlerFunc, error) {
	target, err := url.Parse(betterAuthInternalURL())
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Tighter transport timeouts so a hung sidecar doesn't tie up Go workers.
	proxy.Transport = &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "auth service unavailable: "+err.Error(), http.StatusBadGateway)
	}

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}, nil
}
