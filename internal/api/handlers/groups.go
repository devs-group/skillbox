package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// createGroupRequest is the request body for CreateGroup.
type createGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateGroup handles POST /v1/groups (admin only).
func CreateGroup(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		var req createGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		if req.Name == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "name is required")
			return
		}

		g := &store.Group{
			TenantID:    tenantID,
			Name:        req.Name,
			Description: req.Description,
		}

		if err := s.CreateGroup(c.Request.Context(), g); err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to create group")
			return
		}

		c.JSON(http.StatusCreated, g)
	}
}

// ListGroups handles GET /v1/groups (admin only).
func ListGroups(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		groups, err := s.ListGroups(c.Request.Context(), tenantID)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list groups")
			return
		}

		if groups == nil {
			groups = []*store.Group{}
		}

		c.JSON(http.StatusOK, groups)
	}
}

// updateGroupRequest is the request body for UpdateGroup.
type updateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateGroup handles PUT /v1/groups/:id (admin only).
func UpdateGroup(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		id := c.Param("id")
		if id == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "group id is required")
			return
		}

		var req updateGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		g := &store.Group{
			ID:          id,
			TenantID:    tenantID,
			Name:        req.Name,
			Description: req.Description,
		}

		if err := s.UpdateGroup(c.Request.Context(), g); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "group not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to update group")
			return
		}

		c.JSON(http.StatusOK, g)
	}
}

// DeleteGroup handles DELETE /v1/groups/:id (admin only).
func DeleteGroup(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "group id is required")
			return
		}

		if err := s.DeleteGroup(c.Request.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "group not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to delete group")
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// addGroupMemberRequest is the request body for AddGroupMember.
type addGroupMemberRequest struct {
	UserID string `json:"user_id"`
}

// AddGroupMember handles POST /v1/groups/:id/members (admin only).
func AddGroupMember(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Param("id")
		if groupID == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "group id is required")
			return
		}

		var req addGroupMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		if req.UserID == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "user_id is required")
			return
		}

		if err := s.AddUserToGroup(c.Request.Context(), req.UserID, groupID); err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to add member to group")
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// RemoveGroupMember handles DELETE /v1/groups/:id/members/:userId (admin only).
func RemoveGroupMember(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Param("id")
		userID := c.Param("userId")
		if groupID == "" || userID == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "group id and user id are required")
			return
		}

		if err := s.RemoveUserFromGroup(c.Request.Context(), userID, groupID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "membership not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to remove member from group")
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// ListGroupMembers handles GET /v1/groups/:id/members (admin only).
func ListGroupMembers(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Param("id")
		if groupID == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "group id is required")
			return
		}

		members, err := s.ListGroupMembers(c.Request.Context(), groupID)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list group members")
			return
		}

		if members == nil {
			members = []*store.User{}
		}

		c.JSON(http.StatusOK, members)
	}
}
