package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// BetterAuthJWT mirrors the claims emitted by Better Auth's JWT plugin.
// The plugin signs with EdDSA / Ed25519 by default, and exposes the public
// keys via a JWKS endpoint (default path `${baseURL}${basePath}/jwks`).
type BetterAuthJWT struct {
	Subject              string  `json:"sub"`
	UserID               string  `json:"id"`
	Email                *string `json:"email,omitempty"`
	EmailVerified        bool    `json:"emailVerified,omitempty"`
	Name                 string  `json:"name,omitempty"`
	Image                *string `json:"image,omitempty"`
	PhoneNumber          *string `json:"phoneNumber,omitempty"`
	PhoneNumberVerified  bool    `json:"phoneNumberVerified,omitempty"`
	Issuer               string  `json:"iss,omitempty"`
	Audience             string  `json:"aud,omitempty"`
	IssuedAt             int64   `json:"iat,omitempty"`
	ExpiresAt            int64   `json:"exp,omitempty"`
}

// jwkOKPKey is the subset of a JWKS entry needed to reconstruct an Ed25519
// public key. Better Auth emits `{kty:"OKP", crv:"Ed25519", x:"...", kid:"..."}`.
type jwkOKPKey struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	X   string `json:"x"`
}

type jwkOKPSet struct {
	Keys []jwkOKPKey `json:"keys"`
}

var (
	betterAuthJwksMu    sync.RWMutex
	betterAuthJwksCache = map[string]ed25519.PublicKey{} // key = jwksURL + "|" + kid
)

// fetchBetterAuthEdDSAKey returns the cached Ed25519 public key for `kid` from
// `jwksURL`, fetching the JWKS over HTTP on cache miss.
func fetchBetterAuthEdDSAKey(jwksURL string, kid string) (ed25519.PublicKey, error) {
	cacheKey := jwksURL + "|" + kid
	betterAuthJwksMu.RLock()
	if k, ok := betterAuthJwksCache[cacheKey]; ok {
		betterAuthJwksMu.RUnlock()
		return k, nil
	}
	betterAuthJwksMu.RUnlock()

	resp, err := http.Get(jwksURL) // #nosec G107 - JWKS URL is configured by the operator, not an arbitrary token claim.
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch JWKS: http %d", resp.StatusCode)
	}

	var jwks jwkOKPSet
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}

	for _, k := range jwks.Keys {
		if k.Kid != kid {
			continue
		}
		if k.Kty != "OKP" || k.Crv != "Ed25519" {
			return nil, fmt.Errorf("unsupported JWK key type/curve: kty=%s crv=%s (only OKP/Ed25519 supported)", k.Kty, k.Crv)
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, fmt.Errorf("decode JWK x: %w", err)
		}
		if len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("ed25519 public key has unexpected length %d", len(raw))
		}
		pub := ed25519.PublicKey(raw)

		betterAuthJwksMu.Lock()
		betterAuthJwksCache[cacheKey] = pub
		betterAuthJwksMu.Unlock()
		return pub, nil
	}
	return nil, fmt.Errorf("kid not found in JWKS: %s", kid)
}

// parseBetterAuthJWT validates `jwtStr` against the JWKS at `jwksURL` and
// returns the parsed claims. Only EdDSA / Ed25519 is accepted.
func parseBetterAuthJWT(jwtStr string, jwksURL string) (*BetterAuthJWT, error) {
	parser := jwtv5.NewParser(jwtv5.WithValidMethods([]string{jwtv5.SigningMethodEdDSA.Alg()}))
	token, err := parser.Parse(jwtStr, func(t *jwtv5.Token) (interface{}, error) {
		kidVal, ok := t.Header["kid"]
		if !ok {
			return nil, fmt.Errorf("missing kid")
		}
		kid, ok := kidVal.(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("invalid kid")
		}
		return fetchBetterAuthEdDSAKey(jwksURL, kid)
	})
	if err != nil {
		return nil, fmt.Errorf("parse better-auth jwt: %w", err)
	}

	mc, ok := token.Claims.(jwtv5.MapClaims)
	if !ok {
		return nil, fmt.Errorf("parse better-auth jwt: unexpected claims type")
	}
	raw, err := json.Marshal(mc)
	if err != nil {
		return nil, fmt.Errorf("marshal claims: %w", err)
	}
	var parsed BetterAuthJWT
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	if parsed.ExpiresAt > 0 && time.Now().UTC().Unix() > parsed.ExpiresAt {
		return nil, fmt.Errorf("better-auth jwt expired")
	}
	if strings.TrimSpace(parsed.Subject) == "" {
		return nil, fmt.Errorf("better-auth jwt missing sub")
	}
	return &parsed, nil
}
