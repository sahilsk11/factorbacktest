package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// userAccountIDKey matches the existing key the rest of the API
// (api/api.go's getGoogleAuthMiddleware) reads, so the wider codebase
// doesn't have to know which middleware set it.
const userAccountIDKey = "userAccountID"

// Middleware reads the session cookie, resolves it to a user, and stores
// the user id on the gin context. Anonymous requests pass through unset.
// The legacy getGoogleAuthMiddleware in api.go skips its own work when
// this middleware has already set the key — cookie wins.
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID, ok := s.resolveSession(c.Request.Context(), c); ok {
			c.Set(userAccountIDKey, userID.String())
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user's id for the current request,
// or (uuid.Nil, false) if the request is unauthenticated.
func CurrentUser(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(userAccountIDKey)
	if !ok {
		return uuid.Nil, false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// RequireAuth aborts with 401 when the request is unauthenticated. Use as
// a per-route guard on new endpoints; existing handlers in api/api.go
// have their own auth-checking patterns.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := CurrentUser(c); !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}
