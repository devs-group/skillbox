package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/store"
)

// createApprovalRequestBody is the request body for CreateApprovalRequest.
type createApprovalRequestBody struct {
	SkillName    string `json:"skill_name"`
	SkillVersion string `json:"skill_version"`
}

// CreateApprovalRequest handles POST /v1/approvals.
// Creates or updates an approval request for a skill.
func CreateApprovalRequest(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		userID, exists := c.Get(middleware.ContextKeyUserID)
		if !exists {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		var req createApprovalRequestBody
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		if req.SkillName == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "skill_name is required")
			return
		}

		if req.SkillVersion == "" {
			req.SkillVersion = "latest"
		}

		ar := &store.ApprovalRequest{
			TenantID:     tenantID,
			UserID:       userID.(string),
			SkillName:    req.SkillName,
			SkillVersion: req.SkillVersion,
		}

		result, err := s.CreateApprovalRequest(c.Request.Context(), ar)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to create approval request")
			return
		}

		c.JSON(http.StatusCreated, result)
	}
}

// ListApprovalRequests handles GET /v1/approvals?status=pending.
// Lists approval requests for the authenticated tenant.
func ListApprovalRequests(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		status := c.Query("status")

		requests, err := s.ListApprovalRequests(c.Request.Context(), tenantID, status)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list approval requests")
			return
		}

		if requests == nil {
			requests = []*store.ApprovalRequest{}
		}

		c.JSON(http.StatusOK, requests)
	}
}

// updateApprovalRequestBody is the request body for UpdateApprovalRequest.
type updateApprovalRequestBody struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
}

// UpdateApprovalRequest handles PUT /v1/approvals/:id (admin only).
// Updates the status of an approval request. When approving, also registers
// the skill as approved for the tenant.
func UpdateApprovalRequest(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "approval request id is required")
			return
		}

		reviewerID, exists := c.Get(middleware.ContextKeyUserID)
		if !exists {
			response.RespondError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		var req updateApprovalRequestBody
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		if req.Status != "approved" && req.Status != "rejected" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "status must be one of: approved, rejected")
			return
		}

		tenantID := middleware.GetTenantID(c)

		// Fetch the approval request to get tenant and skill details.
		ar, err := s.GetApprovalRequest(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "approval request not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve approval request")
			return
		}

		// Enforce tenant isolation — admin can only approve requests in their own tenant.
		if ar.TenantID != tenantID {
			response.RespondError(c, http.StatusNotFound, "not_found", "approval request not found")
			return
		}

		if err := s.UpdateApprovalStatus(c.Request.Context(), id, req.Status, reviewerID.(string), req.Comment); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "approval request not found")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to update approval status")
			return
		}

		// When approved, also register the skill as approved for the tenant.
		if req.Status == "approved" {
			if err := s.ApproveSkillForTenant(c.Request.Context(), ar.TenantID, ar.SkillName, ar.SkillVersion, reviewerID.(string)); err != nil {
				response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to approve skill for tenant")
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
