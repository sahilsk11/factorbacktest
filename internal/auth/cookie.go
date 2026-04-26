package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The __Host- prefix is a browser-enforced contract: cookie MUST be
// Secure, MUST NOT have a Domain attribute (host-only), MUST have Path=/.
// If a future edit accidentally drops Secure or sets Domain, the browser
// will refuse the cookie. Catches a class of cookie-attribute downgrade bugs.
const sessionCookieName = "__Host-factor_session"

// newSessionID returns a fresh UUIDv4 from crypto/rand. Generated on
// every successful login so a pre-login cookie value can never become a
// post-login session id (session-fixation defense). UUIDv4 carries 122
// bits of entropy — well above NIST SP 800-63B's 64-bit floor for
// session identifiers.
func newSessionID() uuid.UUID {
	return uuid.New()
}

// signCookieValue produces "<id>.<HMAC>". Tampering the id without the
// secret produces a MAC mismatch we reject in constant time below.
func (s *Service) signCookieValue(id uuid.UUID) string {
	idStr := id.String()
	mac := hmac.New(sha256.New, s.cfg.SessionSecret)
	mac.Write([]byte(idStr))
	return idStr + "." + hex.EncodeToString(mac.Sum(nil))
}

// verifyCookieValue parses "<id>.<HMAC>" and returns the parsed UUID IFF
// the HMAC matches. Constant-time comparison via subtle.ConstantTimeCompare.
// Returns (uuid.Nil, false) on any tampering, malformed value, or
// non-UUID id portion.
func (s *Service) verifyCookieValue(raw string) (uuid.UUID, bool) {
	dot := strings.LastIndexByte(raw, '.')
	if dot <= 0 || dot == len(raw)-1 {
		return uuid.Nil, false
	}
	idStr := raw[:dot]
	got, err := hex.DecodeString(raw[dot+1:])
	if err != nil {
		return uuid.Nil, false
	}
	mac := hmac.New(sha256.New, s.cfg.SessionSecret)
	mac.Write([]byte(idStr))
	if subtle.ConstantTimeCompare(got, mac.Sum(nil)) != 1 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, false
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
