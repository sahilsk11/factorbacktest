package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Cookie name uses the __Host- prefix, which the browser enforces:
//
//   - cookie MUST have Secure
//   - cookie MUST NOT have a Domain attribute (host-only)
//   - cookie MUST have Path=/
//
// If we ever accidentally drop Secure or set a Domain, the browser will
// refuse the cookie. Catches a class of cookie-attribute downgrade bugs.
const sessionCookieName = "__Host-factor_session"

// State cookie used by the OAuth flow. Short-lived (10 min) and one-time
// use: cleared on the callback regardless of outcome.
const stateCookieName = "__Host-factor_oauth_state"
const stateCookieTTL = 10 * time.Minute

// sessionIDBytes is the entropy of a session id before HMAC. 32 bytes =
// 256 bits, well above the floor for unguessable random tokens.
const sessionIDBytes = 32

// newSessionID returns a fresh hex-encoded session id from crypto/rand.
// Used on every successful login (Google callback or SMS verify) — never
// reused, never derived from any prior identifier. This is what prevents
// session fixation: a pre-login cookie value can never become a post-login
// session id.
func newSessionID() (string, error) {
	b := make([]byte, sessionIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// signCookieValue combines the session id with an HMAC-SHA256 of the id
// using the configured SessionSecret. Cookie value format: "<id>.<mac>".
// The HMAC binds the cookie to this server's secret; tampering produces a
// MAC mismatch which we reject in constant time.
func (s *Service) signCookieValue(sessionID string) string {
	mac := hmac.New(sha256.New, s.cfg.SessionSecret)
	mac.Write([]byte(sessionID))
	return sessionID + "." + hex.EncodeToString(mac.Sum(nil))
}

// verifyCookieValue parses a signed cookie value and returns the session
// id IFF the HMAC matches. Comparison is constant-time (subtle.ConstantTimeCompare).
// Returns ("", false) on any tampering, missing-dot, or bad-hex error.
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
	want := mac.Sum(nil)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return "", false
	}
	return id, true
}

// setSessionCookie is the ONLY function in this package that emits a
// Set-Cookie header for the session cookie. Centralized so an accidental
// future edit can't downgrade attributes silently. Every test in
// TestSessionCookie_Attributes asserts on the literal Set-Cookie string
// emitted here.
func (s *Service) setSessionCookie(c *gin.Context, signedValue string, maxAge time.Duration) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    signedValue,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		Secure:   true,                 // forced by __Host- prefix; explicit for clarity
		HttpOnly: true,                 // JS cannot read; mitigates XSS-based theft
		SameSite: http.SameSiteLaxMode, // GET-style cross-site sends, POST does not
		// Domain MUST be empty for __Host-; setting it makes the browser refuse the cookie.
	}
	http.SetCookie(c.Writer, cookie)
}

// clearSessionCookie sends a Set-Cookie that immediately invalidates the
// browser's stored session cookie. Used on sign-out and on any session
// rejection path so a stale cookie isn't repeatedly re-presented.
func (s *Service) clearSessionCookie(c *gin.Context) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(c.Writer, cookie)
}

// loginUser is the shared post-identity-verified path: generate a fresh
// session id, persist the row, and set the cookie. Called from both the
// Google callback and SMS verify flows. Returning the SessionRow gives
// tests a way to inspect what was created.
func (s *Service) loginUser(ctx context.Context, c *gin.Context, userID uuid.UUID) (*SessionRow, error) {
	id, err := newSessionID()
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	row := SessionRow{
		ID:            id,
		UserAccountID: userID,
		CreatedAt:     now,
		ExpiresAt:     now.Add(s.cfg.SessionTTL),
		LastSeenAt:    now,
		IP:            clientIP(c),
		UserAgent:     truncate(c.GetHeader("User-Agent"), 512),
	}
	if err := s.sessions.Create(ctx, row); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	s.setSessionCookie(c, s.signCookieValue(id), s.cfg.SessionTTL)
	return &row, nil
}

// resolveSession is the auth middleware's lookup. Performs (in order):
//  1. Cookie present?         -> false: anonymous request, return false
//  2. HMAC verifies?          -> false: anonymous, clear stale cookie
//  3. Row exists + not expired-> false: anonymous, clear stale cookie
//  4. Within absolute max-age?-> false: delete row, clear cookie, anonymous
//  5. Slide expires_at        -> bump last_seen, return user id
//
// The clear-on-failure behavior is intentional: a stale or tampered cookie
// stays on the client otherwise and gets rejected on every request. Better
// to nuke it once and let the FE re-authenticate.
func (s *Service) resolveSession(ctx context.Context, c *gin.Context) (uuid.UUID, bool) {
	raw, err := c.Request.Cookie(sessionCookieName)
	if err != nil || raw == nil {
		return uuid.Nil, false
	}
	id, ok := s.verifyCookieValue(raw.Value)
	if !ok {
		s.clearSessionCookie(c)
		return uuid.Nil, false
	}
	row, err := s.sessions.Get(ctx, id)
	if err != nil {
		if !errors.Is(err, ErrSessionNotFound) {
			logf("session lookup error: %v", err)
		}
		s.clearSessionCookie(c)
		return uuid.Nil, false
	}
	now := s.now().UTC()
	if now.Sub(row.CreatedAt) >= s.cfg.SessionAbsoluteMaxAge {
		// Past absolute cap. Force re-auth even if expires_at hasn't tripped.
		_ = s.sessions.Delete(ctx, id)
		s.clearSessionCookie(c)
		return uuid.Nil, false
	}
	// Slide the window. Don't fail the request if Touch errors; we have a
	// valid identity, the bump is best-effort.
	newExpires := now.Add(s.cfg.SessionTTL)
	if newExpires.After(row.ExpiresAt) {
		if err := s.sessions.Touch(ctx, id, newExpires); err != nil {
			logf("session touch failed: %v", err)
		}
	}
	return row.UserAccountID, true
}

// handleGetSession returns the current user's id (and a couple of profile
// hints if cheap to surface) so the FE can decide whether to show a
// signed-in or signed-out UI on page load. We deliberately return 200 with
// `{"user": null}` for unauthenticated rather than 401 — signed-out is a
// normal state, not an error.
func (s *Service) handleGetSession(c *gin.Context) {
	userID, ok := s.resolveSession(c.Request.Context(), c)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"user": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{"id": userID.String()},
	})
}

// handleSignOut deletes the session row AND clears the cookie. Once the row
// is deleted, the same cookie value can never re-authenticate even if it's
// somehow re-presented (e.g. a stale browser tab).
func (s *Service) handleSignOut(c *gin.Context) {
	if raw, err := c.Request.Cookie(sessionCookieName); err == nil && raw != nil {
		if id, ok := s.verifyCookieValue(raw.Value); ok {
			if err := s.sessions.Delete(c.Request.Context(), id); err != nil {
				logf("delete session on signout: %v", err)
			}
		}
	}
	s.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// clientIP extracts the best-effort client IP for logging/audit. Trusts
// gin.Context's ClientIP which respects X-Forwarded-For when behind a
// known proxy. Returns empty string on parse failure (NULLIF in the
// repo turns that into a SQL NULL).
func clientIP(c *gin.Context) string {
	return c.ClientIP()
}
