package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/skill"
	"github.com/devs-group/skillbox/internal/store"
)

// ListSkillVersions handles GET /v1/skills/:name/versions.
// It returns every stored version of a skill with its status and active flag.
func ListSkillVersions(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")

		if err := skill.ValidateName(name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		versions, err := s.ListSkillVersions(c.Request.Context(), tenantID, name)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list versions: "+err.Error())
			return
		}
		if versions == nil {
			versions = []store.SkillVersionInfo{}
		}
		c.JSON(http.StatusOK, versions)
	}
}

// setActiveRequest is the JSON body for PUT /v1/skills/:name/active.
type setActiveRequest struct {
	Version string `json:"version" binding:"required"`
}

// SetActiveSkillVersion handles PUT /v1/skills/:name/active.
// It switches the tenant's active version to an existing available version.
func SetActiveSkillVersion(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")

		if err := skill.ValidateName(name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		var req setActiveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
			return
		}
		if err := skill.ValidateVersion(req.Version); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		err := s.SetActiveVersion(c.Request.Context(), tenantID, name, req.Version)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrNotFound):
				response.RespondError(c, http.StatusNotFound, "not_found", "version not found: "+name+"@"+req.Version)
			case errors.Is(err, store.ErrInvalidStatus):
				response.RespondError(c, http.StatusConflict, "invalid_status", err.Error())
			default:
				response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to set active version: "+err.Error())
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{"name": name, "active": req.Version})
	}
}
