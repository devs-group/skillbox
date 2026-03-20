package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// GetCurrentUser handles GET /v1/users/me.
// Returns the authenticated user's profile.
func GetCurrentUser(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get(middleware.ContextKeyUserID)
		if !exists {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		user, err := s.GetUserByID(c.Request.Context(), userID.(string))
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "user not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve user")
			return
		}

		c.JSON(http.StatusOK, user)
	}
}

// ListUsers handles GET /v1/users (admin only).
// Returns all users for the authenticated tenant.
func ListUsers(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		users, err := s.ListUsers(c.Request.Context(), tenantID)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list users")
			return
		}

		if users == nil {
			users = []*store.User{}
		}

		c.JSON(http.StatusOK, users)
	}
}

// updateUserRoleRequest is the request body for UpdateUserRole.
type updateUserRoleRequest struct {
	Role string `json:"role"`
}

// validRoles contains the allowed user role values.
var validRoles = map[string]bool{
	"admin":    true,
	"publisher": true,
	"consumer": true,
}

// UpdateUserRole handles PUT /v1/users/:id/role (admin only).
// Updates the role for a given user.
func UpdateUserRole(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "user id is required")
			return
		}

		var req updateUserRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		if !validRoles[req.Role] {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "role must be one of: admin, publisher, consumer")
			return
		}

		if err := s.UpdateUserRole(c.Request.Context(), id, req.Role); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "user not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to update user role")
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
