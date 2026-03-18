package handlers

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/skill"
	"github.com/devs-group/skillbox/internal/store"
)

// CreateFromFieldsRequest is the JSON body for POST /v1/skills/from-fields.
type CreateFromFieldsRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description" binding:"required"`
	Lang         string `json:"lang"`
	Code         string `json:"code" binding:"required"`
	Instructions string `json:"instructions"`
	Version      string `json:"version"`
}

// CreateFromFields handles POST /v1/skills/from-fields.
//
// It accepts structured JSON fields, builds a SKILL.md and zip archive
// server-side, then uploads to the registry. Upsert semantics: if the
// skill already exists for this tenant, it is replaced.
func CreateFromFields(reg *registry.Registry, s *store.Store, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		var req CreateFromFieldsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
			return
		}

		if err := skill.ValidateName(req.Name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		lang := req.Lang
		if lang == "" {
			lang = skill.LangPython
		}

		version := req.Version
		if version == "" {
			version = "1.0.0"
		}
		if err := skill.ValidateVersion(version); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		skillMD := skill.BuildSkillMD(req.Name, req.Description, lang, version, req.Instructions)

		zipData, err := skill.PackageSkillZip(skillMD, req.Code, lang)
		if err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "failed to package skill: "+err.Error())
			return
		}

		// Enforce max skill size.
		if cfg.MaxSkillSize > 0 && int64(len(zipData)) > cfg.MaxSkillSize {
			response.RespondError(c, http.StatusRequestEntityTooLarge, "too_large", "skill zip exceeds maximum allowed size")
			return
		}

		// Upload to registry (MinIO/S3).
		err = reg.Upload(c.Request.Context(), tenantID, req.Name, version, bytes.NewReader(zipData), int64(len(zipData)))
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to upload skill")
			return
		}

		// Persist metadata in PostgreSQL for fast listing.
		err = s.UpsertSkill(c.Request.Context(), &store.SkillRecord{
			TenantID:    tenantID,
			Name:        req.Name,
			Version:     version,
			Description: req.Description,
			Lang:        lang,
		})
		if err != nil {
			// Log but don't fail — the skill is already in the registry.
			_ = c.Error(err)
		}

		c.JSON(http.StatusCreated, skill.SkillSummary{
			Name:        req.Name,
			Version:     version,
			Description: req.Description,
			Lang:        lang,
			Mode:        "executable",
		})
	}
}
