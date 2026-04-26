package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// The __Host- prefix is a browser-enforced contract: cookie MUST be
// Secure, MUST NOT have a Domain attribute (host-only), MUST have Path=/.
// If a future edit accidentally drops Secure or sets Domain, the browser
// will refuse the cookie. Catches a class of cookie-attribute downgrade bugs.
const sessionCookieName = "__Host-factor_session"

const sessionIDBytes = 32

// newSessionID returns a fresh hex-encoded session id from crypto/rand.
// Generated on every successful login so a pre-login cookie value can
// never become a post-login session id (session-fixation defense).
func newSessionID() (string, error) {
	b := make([]byte, sessionIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// signCookieValue produces "<id>.<HMAC>". Tampering the id without the
// secret produces a MAC mismatch we reject in constant time below.
func (s *Service) signCookieValue(sessionID string) string {
	mac := hmac.New(sha256.New, s.cfg.SessionSecret)
	mac.Write([]byte(sessionID))
	return sessionID + "." + hex.EncodeToString(mac.Sum(nil))
}

// verifyCookieValue parses "<id>.<HMAC>" and returns the id IFF the HMAC
// matches. Constant-time comparison via subtle.ConstantTimeCompare.
func (s *Service) verifyCookieValue(raw string) (string, bool) {
	dot := strings.LastIndexByte(raw, '.')
	if dot <= 0 || dot == len(raw)-1 {
		return "", false
	}
	id := raw[:dot]
	got, err := hex.DecodeString(raw[dot+1:])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, s.cfg.SessionSecret)
	mac.Write([]byte(id))
	if subtle.ConstantTimeCompare(got, mac.Sum(nil)) != 1 {
		return "", false
	}
	return id, true
}

// setSessionCookie is the ONLY emitter of the session Set-Cookie header.
// Centralizing keeps cookie attributes in one place; a literal-string
// assertion in tests catches accidental future downgrades.
func (s *Service) setSessionCookie(c *gin.Context, signedValue string, maxAge time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signedValue,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
