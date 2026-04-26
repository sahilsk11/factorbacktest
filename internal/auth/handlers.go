package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetSession returns the current user's id (or null) so the FE can
// decide whether to render signed-in vs signed-out UI on bootstrap.
// Returns 200 in both cases — signed-out is a normal state, not an error.
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

// handleSignOut deletes the session row AND clears the cookie. After the
// row is deleted the same cookie value can never re-authenticate, even
// if it's somehow re-presented from a stale tab.
func (s *Service) handleSignOut(c *gin.Context) {
	if raw, err := c.Request.Cookie(sessionCookieName); err == nil && raw != nil {
		if id, ok := s.verifyCookieValue(raw.Value); ok {
			if err := s.sessions.Delete(c.Request.Context(), id); err != nil {
				s.log.Errorw("delete session on signout", "err", err)
			}
		}
	}
	s.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}
