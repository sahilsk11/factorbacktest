package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
)

type SupabaseJWT struct {
	Aal                   string                 `json:"aal"`
	AuthenticationMethods []AuthenticationMethod `json:"amr"`
	AppMetadata           AppMetadata            `json:"app_metadata"`
	Audience              string                 `json:"aud"`
	Email                 *string                `json:"email"`
	ExpiresAt             int64                  `json:"exp"`
	IssuedAt              int64                  `json:"iat"`
	IsAnonymous           bool                   `json:"is_anonymous"`
	Issuer                string                 `json:"iss"`
	PhoneNumber           *string                `json:"phone"`
	Role                  string                 `json:"role"`
	SessionID             string                 `json:"session_id"`
	Subject               string                 `json:"sub"`
	UserMetadata          UserMetadata           `json:"user_metadata"`
	Name                  string                 `json:"name"`
}

type AuthenticationMethod struct {
	Method    string `json:"method"`
	Timestamp int64  `json:"timestamp"`
}

type AppMetadata struct {
	Provider  string   `json:"provider"`
	Providers []string `json:"providers"`
}

type UserMetadata struct {
	EmailVerified bool   `json:"email_verified"`
	PhoneVerified bool   `json:"phone_verified"`
	Subject       string `json:"sub"`
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

// Minimal subset of JWK fields needed for ES256 verification.
type jwkKey struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Alg string `json:"alg"`
}

var (
	jwksCacheMu sync.RWMutex
	// cache key: jwksURL + "|" + kid
	jwksKeyCache = map[string]*ecdsa.PublicKey{}
)

func base64URLDecodeToBigInt(s string) (*big.Int, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
}

func getES256PublicKey(jwksURL string, kid string) (*ecdsa.PublicKey, error) {
	cacheKey := jwksURL + "|" + kid
	jwksCacheMu.RLock()
	if k, ok := jwksKeyCache[cacheKey]; ok {
		jwksCacheMu.RUnlock()
		return k, nil
	}
	jwksCacheMu.RUnlock()

	resp, err := http.Get(jwksURL) // #nosec G107 - JWKS URL derived from token issuer; network call is expected.
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("failed to fetch JWKS: http %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	for _, k := range jwks.Keys {
		if k.Kid != kid {
			continue
		}
		if k.Kty != "EC" || k.Crv != "P-256" {
			return nil, fmt.Errorf("unsupported JWK key type/curve: kty=%s crv=%s", k.Kty, k.Crv)
		}
		x, err := base64URLDecodeToBigInt(k.X)
		if err != nil {
			return nil, fmt.Errorf("failed to decode JWK x: %w", err)
		}
		y, err := base64URLDecodeToBigInt(k.Y)
		if err != nil {
			return nil, fmt.Errorf("failed to decode JWK y: %w", err)
		}
		pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

		jwksCacheMu.Lock()
		jwksKeyCache[cacheKey] = pub
		jwksCacheMu.Unlock()

		return pub, nil
	}

	return nil, fmt.Errorf("kid not found in JWKS: %s", kid)
}

func decodeJWTHeaderAndClaimsUnverified(jwtStr string) (map[string]any, *SupabaseJWT, error) {
	parts := strings.Split(jwtStr, ".")
	if len(parts) < 2 {
		return nil, nil, fmt.Errorf("invalid JWT format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}
	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, fmt.Errorf("failed to parse JWT header: %w", err)
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode JWT claims: %w", err)
	}
	var parsedJWT SupabaseJWT
	if err := json.Unmarshal(claimsBytes, &parsedJWT); err != nil {
		return nil, nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return header, &parsedJWT, nil
}

func parseSupabaseJWT(jwtStr string, decodeToken string) (*SupabaseJWT, error) {
	// First attempt: legacy HS256 (shared secret).
	token, err := jwt.Parse(jwtStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(decodeToken), nil
	})

	// If the token isn't HS*, try ES256 verification via Supabase JWKS.
	if err != nil {
		// Decode unverified to get issuer + kid for JWKS URL.
		header, unverifiedClaims, decodeErr := decodeJWTHeaderAndClaimsUnverified(jwtStr)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to parse token: %w", err)
		}
		alg, _ := header["alg"].(string)
		if alg != "ES256" {
			return nil, fmt.Errorf("failed to parse token: %w", err)
		}
		kid, _ := header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("failed to parse token: missing kid")
		}
		if unverifiedClaims.Issuer == "" {
			return nil, fmt.Errorf("failed to parse token: missing iss")
		}

		jwksURL := strings.TrimRight(unverifiedClaims.Issuer, "/") + "/.well-known/jwks.json"
		esToken, esErr := jwt.Parse(jwtStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}
			return getES256PublicKey(jwksURL, kid)
		})
		if esErr != nil {
			return nil, fmt.Errorf("failed to parse token: %w", esErr)
		}
		token = esToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("Failed to parse claims")
	}
	// Convert the MapClaims to JSON bytes
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("Error marshalling claims: %w", err)
	}

	// Unmarshal the JSON bytes into your JWT struct
	var parsedJWT SupabaseJWT
	if err := json.Unmarshal(claimsJSON, &parsedJWT); err != nil {
		return nil, fmt.Errorf("Error unmarshalling into JWT struct: %w", err)
	}

	if time.Now().UTC().Unix() > parsedJWT.ExpiresAt {
		return nil, fmt.Errorf("jwt is expired")
	}

	return &parsedJWT, nil
}
