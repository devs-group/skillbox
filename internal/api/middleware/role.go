package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// RequireRole creates middleware that enforces the authenticated user has
// one of the specified roles. API-key authenticated requests are allowed
// through for backwards compatibility (API keys have full access).
func RequireRole(s *store.Store, roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}

	return func(c *gin.Context) {
		// API-key auth bypasses role checks (backwards compat)
		authType, _ := c.Get(ContextKeyAuthType)
		if authType == AuthTypeAPIKey {
			c.Next()
			return
		}

		userID, exists := c.Get(ContextKeyUserID)
		if !exists {
			response.RespondError(c, http.StatusForbidden, "forbidden", "authentication required")
			c.Abort()
			return
		}

		user, err := s.GetUserByID(c.Request.Context(), userID.(string))
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to resolve user role")
			c.Abort()
			return
		}

		if !allowed[user.Role] {
			response.RespondError(c, http.StatusForbidden, "forbidden", "insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}
