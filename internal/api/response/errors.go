package response

import "github.com/gin-gonic/gin"

// APIError is the standard JSON error response body returned by all API
// endpoints. It provides a machine-readable error code and a human-readable
// message, with an optional details field for structured validation errors.
type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// RespondError writes a JSON error response with the given HTTP status code.
// The code parameter is a machine-readable slug (e.g. "not_found"), while
// message is a human-readable description of the problem.
func RespondError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, APIError{Error: code, Message: message})
}

// RespondErrorWithDetails writes a JSON error response that includes
// structured details, useful for validation errors where callers benefit
// from knowing exactly which fields failed.
func RespondErrorWithDetails(c *gin.Context, status int, code string, message string, details any) {
	c.JSON(status, APIError{Error: code, Message: message, Details: details})
}
