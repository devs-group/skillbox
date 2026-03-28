package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// TenantMiddleware reads the tenant_id set by AuthMiddleware and, if the
// client also sends an X-Tenant-ID header, verifies the two match. This
// prevents a client from asserting a different tenant than the one bound
// to their API key.
//
// Downstream handlers may retrieve the canonical tenant ID with:
//
//	tenantID, _ := c.Get(middleware.ContextKeyTenantID)
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get(ContextKeyTenantID)
		if !exists {
			// This should never happen if AuthMiddleware runs first.
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "tenant_id not set in context")
			c.Abort()
			return
		}

		keyTenant, ok := tenantID.(string)
		if !ok || keyTenant == "" {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "invalid tenant_id in context")
			c.Abort()
			return
		}

		headerTenant := c.GetHeader("X-Tenant-ID")

		// Service keys (e.g. VectorChat) may assume any tenant via X-Tenant-ID.
		if headerTenant != "" && headerTenant != keyTenant {
			if isServiceKey(c) {
				c.Set(ContextKeyTenantID, headerTenant)
				c.Next()
				return
			}
			response.RespondError(c, http.StatusForbidden, "forbidden", "X-Tenant-ID header does not match the API key's tenant")
			c.Abort()
			return
		}

		c.Set(ContextKeyTenantID, keyTenant)
		c.Next()
	}
}

// GetTenantID is a convenience helper for handlers to retrieve the
// tenant ID from the Gin context. It panics if the value is missing,
// which indicates a programming error (middleware not configured).
func GetTenantID(c *gin.Context) string {
	v, exists := c.Get(ContextKeyTenantID)
	if !exists {
		panic("middleware: tenant_id not in context — is TenantMiddleware registered?")
	}
	return v.(string)
}

// isServiceKey returns true if the request was authenticated with a service API key.
// Service keys are trusted to assume any tenant via X-Tenant-ID.
func isServiceKey(c *gin.Context) bool {
	v, exists := c.Get(ContextKeyAPIKey)
	if !exists {
		return false
	}
	key, ok := v.(*store.APIKey)
	return ok && key.IsService
}
