package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/store"
)

// reviewRequest is the JSON body for PUT /v1/admin/skills/:name/:version/review.
type reviewRequest struct {
	Action  string `json:"action" binding:"required"` // "approve" or "decline"
	Comment string `json:"comment"`
}

// ReviewSkill handles PUT /v1/admin/skills/:name/:version/review.
// Allows an admin to approve or decline a skill in 'review' status.
// On approve, the skill is promoted from pending to available in the registry.
func ReviewSkill(reg *registry.Registry, s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")
		version := c.Param("version")

		if name == "" || version == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "skill name and version are required")
			return
		}

		var req reviewRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
			return
		}

		if req.Action != "approve" && req.Action != "decline" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "action must be 'approve' or 'decline'")
			return
		}

		// Get the admin identity for audit trail.
		reviewedBy := "admin" // TODO: extract from auth context when available.

		// Update skill status in DB.
		if err := s.ReviewSkill(c.Request.Context(), tenantID, name, version, req.Action, reviewedBy); err != nil {
			if err == store.ErrNotFound {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found or not in review status")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to review skill")
			return
		}

		// On approve, promote from pending to available in the registry.
		if req.Action == "approve" {
			if err := reg.Promote(c.Request.Context(), tenantID, name, version); err != nil {
				// Non-fatal: DB is already updated. Log the error.
				_ = c.Error(err)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"name":    name,
			"version": version,
			"status":  map[string]string{"approve": "available", "decline": "declined"}[req.Action],
		})
	}
}

// ListSkillsForReview handles GET /v1/admin/skills/review.
// Returns all skills in 'review' status for the tenant.
func ListSkillsForReview(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		skills, err := s.ListSkills(c.Request.Context(), tenantID, store.SkillStatusReview)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list skills for review")
			return
		}

		if skills == nil {
			skills = []store.SkillRecord{}
		}

		c.JSON(http.StatusOK, skills)
	}
}
