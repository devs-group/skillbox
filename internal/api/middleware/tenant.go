package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/response"
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

		// If the client explicitly sends X-Tenant-ID, it must match the
		// tenant derived from the API key. This guards against accidental
		// or malicious cross-tenant requests.
		if headerTenant := c.GetHeader("X-Tenant-ID"); headerTenant != "" && headerTenant != keyTenant {
			response.RespondError(c, http.StatusForbidden, "forbidden", "X-Tenant-ID header does not match the API key's tenant")
			c.Abort()
			return
		}

		// Re-set the canonical tenant_id so downstream handlers have a
		// single, authoritative source.
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
