package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// ContextKeyAPIKey is the Gin context key for the authenticated API key record.
const ContextKeyAPIKey = "api_key"

// ContextKeyTenantID is the Gin context key for the resolved tenant ID.
const ContextKeyTenantID = "tenant_id"

// AuthMiddleware extracts a Bearer token from the Authorization header,
// hashes it with SHA-256, and looks up the hash in the store. If the key
// is not found or has been revoked, the request is rejected with 401.
//
// On success the middleware sets two context values:
//   - "api_key":   the *store.APIKey record
//   - "tenant_id": the tenant ID derived from the key (NOT from a client header)
func AuthMiddleware(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "missing Authorization header")
			c.Abort()
			return
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "Authorization header must use Bearer scheme")
			c.Abort()
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "empty bearer token")
			c.Abort()
			return
		}

		// SHA-256 hash the raw token to look up the stored key.
		hash := sha256.Sum256([]byte(token))
		hashHex := hex.EncodeToString(hash[:])

		key, err := s.GetAPIKeyByHash(c.Request.Context(), hashHex)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusUnauthorized, "unauthorized", "invalid or revoked API key")
				c.Abort()
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to validate API key")
			c.Abort()
			return
		}

		// Reject revoked keys.
		if key.RevokedAt != nil {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "invalid or revoked API key")
			c.Abort()
			return
		}

		// Store the key and tenant ID in the context. The tenant is derived
		// from the key record, never from a client-supplied header.
		c.Set(ContextKeyAPIKey, key)
		c.Set(ContextKeyTenantID, key.TenantID)
		c.Next()
	}
}
