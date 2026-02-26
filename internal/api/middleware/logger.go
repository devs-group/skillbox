package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HeaderRequestID is the HTTP header name used to propagate request IDs.
const HeaderRequestID = "X-Request-ID"

// ContextKeyRequestID is the Gin context key for the request ID.
const ContextKeyRequestID = "request_id"

// RequestLogger returns a Gin middleware that emits a structured log line
// for every request. Each request receives a unique UUID stored in both the
// Gin context and the X-Request-ID response header.
//
// Log fields: method, path, status, duration_ms, tenant_id, request_id.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Generate or propagate a request ID.
		requestID := c.GetHeader(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set(ContextKeyRequestID, requestID)
		c.Header(HeaderRequestID, requestID)

		// Process the request.
		c.Next()

		// Compute duration.
		duration := time.Since(start)

		// Build log attributes.
		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.String("request_id", requestID),
		}

		// Include tenant_id if available (set by AuthMiddleware).
		if tenantID, exists := c.Get(ContextKeyTenantID); exists {
			if tid, ok := tenantID.(string); ok && tid != "" {
				attrs = append(attrs, slog.String("tenant_id", tid))
			}
		}

		// Convert to []any for the slog API.
		logArgs := make([]any, len(attrs))
		for i, a := range attrs {
			logArgs[i] = a
		}

		status := c.Writer.Status()
		msg := "request completed"

		switch {
		case status >= 500:
			slog.Error(msg, logArgs...)
		case status >= 400:
			slog.Warn(msg, logArgs...)
		default:
			slog.Info(msg, logArgs...)
		}
	}
}
