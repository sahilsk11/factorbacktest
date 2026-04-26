package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// userAccountIDKey is the gin.Context key the rest of the API reads from.
// Matches the existing string used by api/api.go's getGoogleAuthMiddleware,
// so the wider codebase doesn't have to know which middleware set it.
const userAccountIDKey = "userAccountID"

// Middleware returns the gin middleware that resolves the session cookie
// to a userAccountID and stores it on the request context.
//
//   - No cookie / invalid / expired: c.Next() with no context value set,
//     letting downstream handlers treat the request as anonymous OR letting
//     a later middleware (e.g. the legacy Bearer-JWT path) fill it in.
//   - Valid cookie: sets userAccountID on the context BEFORE c.Next(). The
//     legacy middleware in api/api.go is updated to skip its own work when
//     the key is already set, so cookie-auth wins over Bearer when both
//     are present.
//
// This middleware never returns 401 itself. RequireAuth (below) is the
// gate that does that, applied per-route by handlers that need login.
func (s *Service) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := s.resolveSession(c.Request.Context(), c)
		if ok {
			c.Set(userAccountIDKey, userID.String())
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user's ID for the current request,
// or (uuid.Nil, false) if the request is unauthenticated. Reads the same
// gin context key the rest of the codebase already uses.
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

// RequireAuth returns a middleware that aborts with 401 when the request
// has no authenticated user. Useful as a per-route guard for endpoints
// that always require login. Most existing handlers in api/api.go check
// for the user themselves and return their own error envelope; use this
// only on new routes added going forward.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := CurrentUser(c); !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}
