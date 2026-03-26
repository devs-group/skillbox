package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminMiddleware returns middleware that validates requests carry a static
// admin token in the X-Admin-Token header. When expectedToken is empty every
// request is allowed through (admin auth disabled).
func AdminMiddleware(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expectedToken == "" {
			c.Next()
			return
		}

		got := c.GetHeader("X-Admin-Token")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(expectedToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized", "message": "valid admin token required",
			})
			return
		}

		c.Next()
	}
}
