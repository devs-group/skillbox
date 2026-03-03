package handlers

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/skill"
	"github.com/devs-group/skillbox/internal/store"
)

// UploadSkill handles POST /v1/skills.
//
// It accepts skill zip data via two content types:
//   - application/zip: raw zip body
//   - multipart/form-data: zip in a "file" form field
//
// The zip is validated (must contain SKILL.md with valid frontmatter),
// then uploaded to the registry. Skill metadata is also persisted in
// PostgreSQL so that list operations can return descriptions without
// downloading every zip archive. Returns 201 with skill metadata on success.
func UploadSkill(reg *registry.Registry, s *store.Store, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		var zipData []byte
		var err error

		contentType := c.ContentType()
		switch {
		case strings.HasPrefix(contentType, "application/zip"),
			strings.HasPrefix(contentType, "application/octet-stream"):
			// Read raw body, enforcing size limit.
			limited := io.LimitReader(c.Request.Body, cfg.MaxSkillSize+1)
			zipData, err = io.ReadAll(limited)
			if err != nil {
				response.RespondError(c, http.StatusBadRequest, "bad_request", "failed to read request body: "+err.Error())
				return
			}
			if int64(len(zipData)) > cfg.MaxSkillSize {
				response.RespondError(c, http.StatusRequestEntityTooLarge, "too_large",
					"skill zip exceeds maximum allowed size")
				return
			}

		case strings.HasPrefix(contentType, "multipart/form-data"):
			file, _, ferr := c.Request.FormFile("file")
			if ferr != nil {
				response.RespondError(c, http.StatusBadRequest, "bad_request", "missing 'file' field in multipart form")
				return
			}
			defer file.Close() //nolint:errcheck

			limited := io.LimitReader(file, cfg.MaxSkillSize+1)
			zipData, err = io.ReadAll(limited)
			if err != nil {
				response.RespondError(c, http.StatusBadRequest, "bad_request", "failed to read uploaded file: "+err.Error())
				return
			}
			if int64(len(zipData)) > cfg.MaxSkillSize {
				response.RespondError(c, http.StatusRequestEntityTooLarge, "too_large",
					"skill zip exceeds maximum allowed size")
				return
			}

		default:
			response.RespondError(c, http.StatusUnsupportedMediaType, "unsupported_media_type",
				"expected Content-Type application/zip or multipart/form-data")
			return
		}

		if len(zipData) == 0 {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "empty zip data")
			return
		}

		// Validate zip structure: must contain SKILL.md with valid frontmatter.
		parsedSkill, err := validateSkillZip(zipData)
		if err != nil {
			response.RespondError(c, http.StatusBadRequest, "invalid_skill", err.Error())
			return
		}

		if err := parsedSkill.Validate(); err != nil {
			response.RespondError(c, http.StatusBadRequest, "invalid_skill", err.Error())
			return
		}

		// Upload to registry (MinIO/S3).
		err = reg.Upload(c.Request.Context(), tenantID, parsedSkill.Name, parsedSkill.Version, bytes.NewReader(zipData), int64(len(zipData)))
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to upload skill: "+err.Error())
			return
		}

		// Persist metadata in PostgreSQL for fast listing with descriptions.
		err = s.UpsertSkill(c.Request.Context(), &store.SkillRecord{
			TenantID:    tenantID,
			Name:        parsedSkill.Name,
			Version:     parsedSkill.Version,
			Description: parsedSkill.Description,
			Lang:        parsedSkill.Lang,
		})
		if err != nil {
			// Log but don't fail — the skill is already in the registry.
			// The list endpoint falls back to registry listing if needed.
			_ = c.Error(err)
		}

		c.JSON(http.StatusCreated, skill.SkillSummary{
			Name:        parsedSkill.Name,
			Version:     parsedSkill.Version,
			Description: parsedSkill.Description,
			Lang:        parsedSkill.Lang,
		})
	}
}

// validateSkillZip opens a zip archive from the raw bytes, locates SKILL.md,
// parses it, and returns the resulting Skill. It rejects archives with path
// traversal entries (containing "..").
func validateSkillZip(data []byte) (*skill.Skill, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("invalid zip archive: " + err.Error())
	}

	var skillMDData []byte
	for _, f := range reader.File {
		// Reject path traversal.
		if strings.Contains(f.Name, "..") {
			return nil, errors.New("zip contains path traversal entry: " + f.Name)
		}

		// Look for SKILL.md at the root of the archive.
		name := strings.TrimPrefix(f.Name, "./")
		if name == "SKILL.md" || strings.HasSuffix(name, "/SKILL.md") && strings.Count(name, "/") == 1 {
			rc, err := f.Open()
			if err != nil {
				return nil, errors.New("failed to open SKILL.md in zip: " + err.Error())
			}
			skillMDData, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, errors.New("failed to read SKILL.md from zip: " + err.Error())
			}
			break
		}
	}

	if skillMDData == nil {
		return nil, errors.New("zip archive must contain a SKILL.md file")
	}

	parsed, err := skill.ParseSkillMD(skillMDData)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}

// ListSkills handles GET /v1/skills.
// It returns all skill metadata for the authenticated tenant, including
// descriptions so agents can decide which skill to use.
func ListSkills(s *store.Store, reg *registry.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)

		// Try the database first — it has descriptions.
		records, err := s.ListSkills(c.Request.Context(), tenantID)
		if err == nil && len(records) > 0 {
			summaries := make([]skill.SkillSummary, len(records))
			for i, rec := range records {
				summaries[i] = skill.SkillSummary{
					Name:        rec.Name,
					Version:     rec.Version,
					Description: rec.Description,
					Lang:        rec.Lang,
				}
			}
			c.JSON(http.StatusOK, summaries)
			return
		}

		// Fall back to registry listing (no descriptions, for backward compat).
		skills, err := reg.List(c.Request.Context(), tenantID)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list skills: "+err.Error())
			return
		}

		// Always return an array, even if empty.
		if skills == nil {
			skills = []registry.SkillMeta{}
		}

		c.JSON(http.StatusOK, skills)
	}
}

// GetSkill handles GET /v1/skills/:name/:version.
// It downloads the skill zip from the registry, parses SKILL.md, and
// returns the full metadata including the SKILL.md body content (instructions).
// When version is "latest", it resolves to the most recently uploaded version
// via the database before downloading from the registry.
func GetSkill(reg *registry.Registry, s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")
		version := c.Param("version")

		if name == "" || version == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "skill name and version are required")
			return
		}

		// Validate name and version to prevent S3 path traversal.
		if err := skill.ValidateName(name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := skill.ValidateVersion(version); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		// Resolve "latest" to the most recently uploaded version.
		if version == "latest" {
			resolved, err := s.ResolveLatestVersion(c.Request.Context(), tenantID, name)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					response.RespondError(c, http.StatusNotFound, "not_found", "skill not found: "+name+"@latest")
					return
				}
				response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to resolve latest version: "+err.Error())
				return
			}
			version = resolved
		}

		rc, err := reg.Download(c.Request.Context(), tenantID, name, version)
		if err != nil {
			if errors.Is(err, registry.ErrSkillNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found: "+name+"@"+version)
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve skill: "+err.Error())
			return
		}
		defer rc.Close() //nolint:errcheck

		zipBytes, err := io.ReadAll(rc)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to read skill archive: "+err.Error())
			return
		}

		parsed, err := validateSkillZip(zipBytes)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to parse skill archive: "+err.Error())
			return
		}

		var timeout string
		if parsed.Timeout > 0 {
			timeout = parsed.Timeout.String()
		}

		c.JSON(http.StatusOK, skill.SkillMetadata{
			Name:         parsed.Name,
			Version:      parsed.Version,
			Description:  parsed.Description,
			Lang:         parsed.Lang,
			Image:        parsed.Image,
			Instructions: parsed.Instructions,
			Timeout:      timeout,
			Resources:    parsed.Resources,
		})
	}
}

// SkillFileEntry represents a single file extracted from a skill archive.
type SkillFileEntry struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	SizeBytes int    `json:"size_bytes"`
}

// GetSkillFiles handles GET /v1/skills/:name/:version/files.
// It downloads the skill zip from the registry, extracts all files, and
// returns them as a JSON array. An optional "path" query parameter returns
// only the matching file.
func GetSkillFiles(reg *registry.Registry, s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")
		version := c.Param("version")

		if name == "" || version == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "skill name and version are required")
			return
		}

		if err := skill.ValidateName(name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := skill.ValidateVersion(version); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		if version == "latest" {
			resolved, err := s.ResolveLatestVersion(c.Request.Context(), tenantID, name)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					response.RespondError(c, http.StatusNotFound, "not_found", "skill not found: "+name+"@latest")
					return
				}
				response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to resolve latest version: "+err.Error())
				return
			}
			version = resolved
		}

		rc, err := reg.Download(c.Request.Context(), tenantID, name, version)
		if err != nil {
			if errors.Is(err, registry.ErrSkillNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found: "+name+"@"+version)
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve skill: "+err.Error())
			return
		}
		defer rc.Close() //nolint:errcheck

		zipBytes, err := io.ReadAll(rc)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to read skill archive: "+err.Error())
			return
		}

		reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to open skill archive: "+err.Error())
			return
		}

		filterPath := c.Query("path")
		var entries []SkillFileEntry

		for _, f := range reader.File {
			if f.FileInfo().IsDir() {
				continue
			}
			if strings.Contains(f.Name, "..") {
				continue
			}

			filePath := strings.TrimPrefix(f.Name, "./")
			if filterPath != "" && filePath != filterPath {
				continue
			}

			frc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(frc)
			frc.Close() //nolint:errcheck
			if err != nil {
				continue
			}

			entries = append(entries, SkillFileEntry{
				Path:      filePath,
				Content:   string(data),
				SizeBytes: len(data),
			})
		}

		if filterPath != "" && len(entries) == 0 {
			response.RespondError(c, http.StatusNotFound, "not_found", "file not found: "+filterPath)
			return
		}

		if entries == nil {
			entries = []SkillFileEntry{}
		}
		c.JSON(http.StatusOK, entries)
	}
}

// DeleteSkill handles DELETE /v1/skills/:name/:version.
// It removes the skill from both the registry and the metadata store,
// then returns 204 No Content.
func DeleteSkill(reg *registry.Registry, s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")
		version := c.Param("version")

		if name == "" || version == "" {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "skill name and version are required")
			return
		}

		// Validate name and version to prevent S3 path traversal.
		if err := skill.ValidateName(name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := skill.ValidateVersion(version); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		if err := reg.Delete(c.Request.Context(), tenantID, name, version); err != nil {
			if errors.Is(err, registry.ErrSkillNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found: "+name+"@"+version)
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to delete skill: "+err.Error())
			return
		}

		// Best-effort cleanup of the metadata record.
		_ = s.DeleteSkill(c.Request.Context(), tenantID, name, version)

		c.Status(http.StatusNoContent)
	}
}
