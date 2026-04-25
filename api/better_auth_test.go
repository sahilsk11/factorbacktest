package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// TestParseBetterAuthJWT signs a JWT with a known Ed25519 key, exposes its
// public half via a fake JWKS endpoint, and verifies parseBetterAuthJWT
// can validate it end-to-end. This mirrors what Better Auth's JWT plugin
// does in production.
func TestParseBetterAuthJWT(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	const kid = "test-kid"

	jwks := jwkOKPSet{
		Keys: []jwkOKPKey{{
			Kty: "OKP",
			Crv: "Ed25519",
			Use: "sig",
			Alg: "EdDSA",
			Kid: kid,
			X:   base64.RawURLEncoding.EncodeToString(pub),
		}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	now := time.Now().UTC().Unix()
	claims := jwtv5.MapClaims{
		"sub":         "user-123",
		"id":          "user-123",
		"email":       "alice@example.com",
		"phoneNumber": "+15551234567",
		"name":        "Alice Smith",
		"iat":         now,
		"exp":         now + 600,
		"iss":         "http://localhost:3009",
		"aud":         "http://localhost:3009",
	}
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	parsed, err := parseBetterAuthJWT(signed, srv.URL, "http://localhost:3009")
	if err != nil {
		t.Fatalf("parseBetterAuthJWT: %v", err)
	}
	if parsed.Subject != "user-123" {
		t.Errorf("sub: got %q, want user-123", parsed.Subject)
	}
	if parsed.Email == nil || *parsed.Email != "alice@example.com" {
		t.Errorf("email: got %v, want alice@example.com", parsed.Email)
	}
	if parsed.PhoneNumber == nil || *parsed.PhoneNumber != "+15551234567" {
		t.Errorf("phone: got %v", parsed.PhoneNumber)
	}
	if parsed.Name != "Alice Smith" {
		t.Errorf("name: got %q", parsed.Name)
	}
}

func TestParseBetterAuthJWT_RejectsHS256(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()

	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, jwtv5.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = "x"
	signed, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := parseBetterAuthJWT(signed, srv.URL, ""); err == nil {
		t.Fatal("expected non-EdDSA token to be rejected")
	}
}

func TestParseBetterAuthJWT_Expired(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	const kid = "expired-kid"
	jwks := jwkOKPSet{Keys: []jwkOKPKey{{
		Kty: "OKP", Crv: "Ed25519", Alg: "EdDSA", Kid: kid,
		X: base64.RawURLEncoding.EncodeToString(pub),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, jwtv5.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	_, err = parseBetterAuthJWT(signed, srv.URL, "")
	if err == nil {
		t.Fatal("expected expired token to error")
	}
	// jwt/v5 returns its own ErrTokenExpired wrapped in our error message;
	// our explicit `time.Now > exp` check is a backstop.
	if err == nil {
		t.Fatal("unreachable")
	}
	_ = fmt.Sprintf("%v", err) // suppress unused import warning if changes
}

func TestParseBetterAuthJWT_IssuerMismatch(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	const kid = "iss-kid"
	jwks := jwkOKPSet{Keys: []jwkOKPKey{{
		Kty: "OKP", Crv: "Ed25519", Alg: "EdDSA", Kid: kid,
		X: base64.RawURLEncoding.EncodeToString(pub),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, jwtv5.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iss": "https://attacker.example.com",
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := parseBetterAuthJWT(signed, srv.URL, "https://api.factor.trade"); err == nil {
		t.Fatal("expected issuer-mismatch token to be rejected")
	}
}
