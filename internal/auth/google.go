package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// State cookie path: scope it to the OAuth flow so it's never sent on
// non-OAuth requests. TTL is short (10 min) and the cookie is one-time
// use — cleared at the start of the callback regardless of outcome.
//
// Uses the `__Secure-` prefix (browser enforces Secure attribute) rather
// than `__Host-`. `__Host-` would require Path=/, which conflicts with
// scoping the cookie to /auth/google. We get the Secure-prefix
// hardening without giving up path scoping.
const (
	stateCookieName = "__Secure-factor_oauth_state"
	stateCookiePath = "/auth/google"
	stateCookieTTL  = 10 * time.Minute
)

// oauthState is what we stuff into the HMAC-signed state cookie. Holding
// state + nonce + PKCE verifier in the cookie keeps the CSRF defense
// self-contained: on callback we only need to verify the cookie's HMAC
// and parse, no server-side lookup required.
type oauthState struct {
	State    string
	Nonce    string
	Verifier string
	Expires  int64
}

func newOAuthState(now time.Time) (oauthState, error) {
	state, err := randB64URL(32)
	if err != nil {
		return oauthState{}, err
	}
	nonce, err := randB64URL(32)
	if err != nil {
		return oauthState{}, err
	}
	return oauthState{
		State:    state,
		Nonce:    nonce,
		Verifier: oauth2.GenerateVerifier(),
		Expires:  now.Add(stateCookieTTL).Unix(),
	}, nil
}

func (st oauthState) encode(secret []byte) string {
	payload := fmt.Sprintf("%s.%s.%s.%d", st.State, st.Nonce, st.Verifier, st.Expires)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func decodeOAuthState(raw string, secret []byte, now time.Time) (oauthState, error) {
	dot := strings.LastIndexByte(raw, '.')
	if dot <= 0 || dot == len(raw)-1 {
		return oauthState{}, fmt.Errorf("malformed state cookie")
	}
	payloadB64, macHex := raw[:dot], raw[dot+1:]
	got, err := hex.DecodeString(macHex)
	if err != nil {
		return oauthState{}, fmt.Errorf("malformed state mac")
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return oauthState{}, fmt.Errorf("malformed state payload")
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	if subtle.ConstantTimeCompare(got, mac.Sum(nil)) != 1 {
		return oauthState{}, fmt.Errorf("state cookie signature mismatch")
	}
	parts := strings.SplitN(string(payload), ".", 4)
	if len(parts) != 4 {
		return oauthState{}, fmt.Errorf("malformed state body")
	}
	var expires int64
	if _, err := fmt.Sscanf(parts[3], "%d", &expires); err != nil {
		return oauthState{}, fmt.Errorf("bad state expires: %w", err)
	}
	if now.Unix() > expires {
		return oauthState{}, fmt.Errorf("state cookie expired")
	}
	return oauthState{
		State:    parts[0],
		Nonce:    parts[1],
		Verifier: parts[2],
		Expires:  expires,
	}, nil
}

func (s *Service) handleGoogleStart(c *gin.Context) {
	st, err := newOAuthState(s.now())
	if err != nil {
		s.log.Errorw("oauth start", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     stateCookieName,
		Value:    st.encode(s.cfg.SessionSecret),
		Path:     stateCookiePath,
		MaxAge:   int(stateCookieTTL.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, s.oauth2cfg.AuthCodeURL(
		st.State,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("nonce", st.Nonce),
		oauth2.S256ChallengeOption(st.Verifier),
	))
}

// handleGoogleCallback — clear state cookie FIRST so a re-presentation
// (CSRF, stale tab, attacker beating the user to the callback) can't
// reuse a still-valid state. Validation runs on the cookie value we just
// captured; the cookie itself is gone after this call returns.
func (s *Service) handleGoogleCallback(c *gin.Context) {
	clearStateCookie(c)

	cookie, err := c.Request.Cookie(stateCookieName)
	if err != nil || cookie == nil {
		s.log.Warn("callback without state cookie")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	st, err := decodeOAuthState(cookie.Value, s.cfg.SessionSecret, s.now())
	if err != nil {
		s.log.Warnw("callback bad state", "err", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(st.State)) != 1 {
		s.log.Warn("callback state mismatch")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	code := c.Query("code")
	if code == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	tok, err := s.oauth2cfg.Exchange(ctx, code, oauth2.VerifierOption(st.Verifier))
	if err != nil {
		s.log.Warnw("oauth exchange", "err", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		s.log.Warnw("id token verify", "err", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if idToken.Nonce != st.Nonce {
		s.log.Warn("nonce mismatch")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
	}
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	// Only trust email if Google says it's verified — an unverified email
	// might be controlled by someone else.
	email := ""
	if claims.EmailVerified {
		email = claims.Email
	}

	userID, err := s.upsertGoogleUser(ctx, claims.Subject, email, claims.GivenName, claims.FamilyName)
	if err != nil {
		s.log.Errorw("get/create user", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := s.loginUser(ctx, c, userID); err != nil {
		s.log.Errorw("login", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Always redirect to the configured frontend base URL. We never accept
	// a redirect target from query params: doing so would create an
	// open-redirect surface where an attacker tricks the user into
	// authenticating then bouncing them somewhere malicious.
	c.Redirect(http.StatusFound, s.cfg.FrontendBaseURL)
}

func clearStateCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     stateCookiePath,
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func randB64URL(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
