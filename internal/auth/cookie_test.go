package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSessionCookie_Attributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	(&Service{}).setSessionCookie(c, "id.mac", 30*24*time.Hour)

	raw := rec.Header().Get("Set-Cookie")
	if raw == "" {
		t.Fatal("missing Set-Cookie")
	}
	// Literal-string asserts: a future edit that drops Secure, sets Domain,
	// or reverts SameSite=Lax would break cross-site sessions from
	// factor.trade to factor-api.ultron.sh (and __Host- would be refused).
	for _, want := range []string{
		sessionCookieName + "=",
		"Path=/",
		"HttpOnly",
		"Secure",
		"SameSite=None",
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("Set-Cookie %q missing %q", raw, want)
		}
	}
	if strings.Contains(strings.ToLower(raw), "domain=") {
		t.Errorf("Set-Cookie %q must not set Domain (__Host- is host-only)", raw)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("parsed %d cookies, want 1", len(cookies))
	}
	ck := cookies[0]
	if ck.SameSite != http.SameSiteNoneMode {
		t.Errorf("SameSite = %v, want SameSiteNoneMode", ck.SameSite)
	}
	if !ck.Secure || !ck.HttpOnly || ck.Path != "/" || ck.Domain != "" {
		t.Errorf("parsed cookie attributes: %+v", ck)
	}
}

func TestClearSessionCookie_Attributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	(&Service{}).clearSessionCookie(c)

	raw := rec.Header().Get("Set-Cookie")
	if !strings.Contains(raw, "SameSite=None") {
		t.Errorf("clear Set-Cookie %q missing SameSite=None (must match the setter to overwrite)", raw)
	}
}
