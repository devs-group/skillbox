package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/store"
)

type reviewRequest struct {
	Action  string `json:"action" binding:"required"`
	Comment string `json:"comment"`
}

var reviewActions = map[string]string{"approve": "available", "decline": "declined", "decline_forever": "declined", "reopen": "review"}

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

		if _, ok := reviewActions[req.Action]; !ok {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "action must be 'approve', 'decline', 'decline_forever', or 'reopen'")
			return
		}

		reviewedBy := "admin"

		if err := s.ReviewSkill(c.Request.Context(), tenantID, name, version, req.Action, reviewedBy, req.Comment); err != nil {
			if err == store.ErrNotFound {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found or not in review status")
				return
			}
			if err == store.ErrBlocked {
				response.RespondError(c, http.StatusForbidden, "blocked", "skill is blocked: reopen it first")
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to review skill")
			return
		}

		// On approve, promote from pending to main when applicable. Skill may
		// already be in main (declined-from-available transition) — the
		// pending source will be missing and Promote returns an error; that's
		// fine, DB status is the source of truth for runtime gating.
		if req.Action == "approve" {
			if err := reg.Promote(c.Request.Context(), tenantID, name, version); err != nil {
				_ = c.Error(err)
			}
			// Advance active pointer to the approved version, mirroring scanner auto-promote.
			if err := s.SetActiveVersion(c.Request.Context(), tenantID, name, version); err != nil {
				_ = c.Error(err)
			}
		}


		c.JSON(http.StatusOK, gin.H{
			"name":    name,
			"version": version,
			"status":  reviewActions[req.Action],
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
