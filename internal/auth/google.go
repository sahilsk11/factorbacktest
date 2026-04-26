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

// State cookie format: base64url("<state>.<nonce>.<verifier>.<expires_unix>") + "." + hex(HMAC).
//
// Why everything in one cookie? It keeps the state-CSRF defense self-contained.
// On callback we don't have to look anything up server-side; we read this
// cookie, verify HMAC, parse parts, and confirm `state` matches the query
// param + verify nonce in the ID token + use verifier for PKCE. After
// successful verification we delete the cookie (one-time use).
//
// Why HMAC the cookie at all if it's already opaque? Because without it,
// an attacker who controls a sibling subdomain (or any future XSS that
// can write cookies via `document.cookie`) could forge a state cookie
// matching a state param they also forged. HMAC ensures the cookie was
// produced by us.

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
	verifier := oauth2.GenerateVerifier()
	return oauthState{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
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

// handleGoogleStart begins the OAuth flow: generate state+nonce+PKCE,
// store them in an HMAC-signed short-lived state cookie, redirect to
// Google's authorize URL.
func (s *Service) handleGoogleStart(c *gin.Context) {
	st, err := newOAuthState(s.now())
	if err != nil {
		logf("oauth start: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	encoded := st.encode(s.cfg.SessionSecret)

	cookie := &http.Cookie{
		Name:     stateCookieName,
		Value:    encoded,
		Path:     "/auth/google",
		MaxAge:   int(stateCookieTTL.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(c.Writer, cookie)

	authURL := s.oauth2cfg.AuthCodeURL(
		st.State,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("nonce", st.Nonce),
		oauth2.S256ChallengeOption(st.Verifier),
	)
	c.Redirect(http.StatusFound, authURL)
}

// handleGoogleCallback validates state + nonce, exchanges the code, verifies
// the ID token, finds-or-creates the user, creates the session, and
// redirects to FrontendBaseURL.
//
// Resolution order matters here. We FIRST clear the state cookie so any
// future re-presentation of the same state is rejected (one-time use),
// then validate. If we cleared only on success, an attacker's first
// request would consume the user's pending state.
func (s *Service) handleGoogleCallback(c *gin.Context) {
	clearStateCookie(c)

	cookie, err := c.Request.Cookie(stateCookieName)
	if err != nil || cookie == nil {
		// Most common cause: user opened /auth/google/callback?... without
		// going through /auth/google/start first (e.g. an attacker's CSRF
		// attempt, or a stale tab). Refuse, don't issue a session.
		logf("callback without state cookie")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	st, err := decodeOAuthState(cookie.Value, s.cfg.SessionSecret, s.now())
	if err != nil {
		logf("callback bad state: %v", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	queryState := c.Query("state")
	if subtle.ConstantTimeCompare([]byte(queryState), []byte(st.State)) != 1 {
		logf("callback state mismatch")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	code := c.Query("code")
	if code == "" {
		logf("callback missing code")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	tok, err := s.oauth2cfg.Exchange(ctx, code, oauth2.VerifierOption(st.Verifier))
	if err != nil {
		logf("oauth exchange: %v", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		logf("oauth response missing id_token")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		logf("id token verify: %v", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if idToken.Nonce != st.Nonce {
		logf("nonce mismatch")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		logf("id token claims: %v", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if claims.Subject == "" {
		logf("id token missing sub")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	// Email is the only profile field where we have a meaningful trust
	// decision: only accept it if Google says it's verified. An unverified
	// email might be controlled by someone else.
	email := ""
	if claims.EmailVerified {
		email = claims.Email
	}
	first := claims.GivenName
	last := claims.FamilyName

	userID, err := s.users.GetOrCreateByGoogle(ctx, claims.Subject, email, first, last)
	if err != nil {
		logf("get/create user: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if _, err := s.loginUser(ctx, c, userID); err != nil {
		logf("login: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Always redirect to the configured frontend base URL. Never accept a
	// `redirect_uri` / `next` / `return_to` from query params: doing so
	// would create an open-redirect surface where an attacker tricks the
	// user into authenticating then bouncing them somewhere else.
	c.Redirect(http.StatusFound, s.cfg.FrontendBaseURL)
}

func clearStateCookie(c *gin.Context) {
	cookie := &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/auth/google",
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(c.Writer, cookie)
}

func randB64URL(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
