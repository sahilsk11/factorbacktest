package api

import (
	"encoding/json"
	"fmt"
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

func parseSupabaseJWT(jwtStr string, decodeToken string) (*SupabaseJWT, error) {
	token, err := jwt.Parse(jwtStr, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
		return []byte(decodeToken), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
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
