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
	"github.com/devs-group/skillbox/internal/scanner"
	"github.com/devs-group/skillbox/internal/skill"
	"github.com/devs-group/skillbox/internal/store"
)

// writeFileRequest is the JSON body for PUT /v1/skills/:name/files.
type writeFileRequest struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content"`
}

// WriteSkillFile handles PUT /v1/skills/:name/files.
//
// It replaces or adds a single file in the skill's active version, repackages
// the archive as a new derived version, and submits it to the scanner. The new
// version only becomes active once it passes scanning (see the worker's
// onAvailable hook).
func WriteSkillFile(reg *registry.Registry, s *store.Store, cfg *config.Config, worker *scanner.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")

		if err := skill.ValidateName(name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		var req writeFileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
			return
		}
		filePath := strings.TrimPrefix(req.Path, "./")
		if filePath == "" || strings.Contains(filePath, "..") || strings.HasPrefix(filePath, "/") || strings.HasPrefix(filePath, "\\") {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid file path")
			return
		}

		if blocked, _ := s.IsSkillBlocked(c.Request.Context(), tenantID, name); blocked {
			response.RespondError(c, http.StatusForbidden, "blocked", "skill is blocked: no new versions can be submitted")
			return
		}

		active, err := s.ResolveActiveVersion(c.Request.Context(), tenantID, name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found: "+name)
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to resolve active version: "+err.Error())
			return
		}

		rc, err := reg.Download(c.Request.Context(), tenantID, name, active)
		if err != nil {
			if errors.Is(err, registry.ErrSkillNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill archive not found: "+name+"@"+active)
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

		next := s.NextFreeVersion(c.Request.Context(), tenantID, name, active)

		content := req.Content
		// Keep the SKILL.md frontmatter version in sync when SKILL.md itself is edited (no-op if absent/invalid).
		if filePath == "SKILL.md" {
			content = skill.SetFrontmatterVersion(content, next)
		}

		newZip, desc, lang, err := repackageWithFile(zipBytes, filePath, content)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to repackage skill: "+err.Error())
			return
		}
		if cfg.MaxSkillSize > 0 && int64(len(newZip)) > cfg.MaxSkillSize {
			response.RespondError(c, http.StatusRequestEntityTooLarge, "too_large", "skill zip exceeds maximum allowed size")
			return
		}

		if err := reg.Submit(c.Request.Context(), tenantID, name, next, bytes.NewReader(newZip), int64(len(newZip))); err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to submit skill: "+err.Error())
			return
		}

		if err := s.UpsertSkill(c.Request.Context(), &store.SkillRecord{
			TenantID:    tenantID,
			Name:        name,
			Version:     next,
			Description: desc,
			Lang:        lang,
			Status:      store.SkillStatusPending,
		}); err != nil {
			_ = c.Error(err)
		}

		if worker != nil {
			worker.Submit(scanner.ScanJob{TenantID: tenantID, Skill: name, Version: next})
		}

		c.JSON(http.StatusAccepted, gin.H{
			"name":    name,
			"version": next,
			"status":  store.SkillStatusPending,
		})
	}
}

// batchFile is one entry in a full-tree write.
type batchFile struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content"`
}

// writeFilesRequest is the JSON body for PUT /v1/skills/:name/files-batch.
type writeFilesRequest struct {
	Files []batchFile `json:"files" binding:"required"`
}

// WriteSkillFiles handles PUT /v1/skills/:name/files-batch — full file-tree snapshot as one new scanned version.
func WriteSkillFiles(reg *registry.Registry, s *store.Store, cfg *config.Config, worker *scanner.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := middleware.GetTenantID(c)
		name := c.Param("name")

		if err := skill.ValidateName(name); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		var req writeFilesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
			return
		}
		if len(req.Files) == 0 {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "files must not be empty")
			return
		}

		clean := make([]batchFile, 0, len(req.Files))
		hasSkillMd := false
		for _, f := range req.Files {
			p := strings.TrimPrefix(f.Path, "./")
			if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
				response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid file path: "+f.Path)
				return
			}
			if p == "SKILL.md" {
				hasSkillMd = true
			}
			clean = append(clean, batchFile{Path: p, Content: f.Content})
		}
		if !hasSkillMd {
			response.RespondError(c, http.StatusBadRequest, "bad_request", "SKILL.md is required")
			return
		}

		if blocked, _ := s.IsSkillBlocked(c.Request.Context(), tenantID, name); blocked {
			response.RespondError(c, http.StatusForbidden, "blocked", "skill is blocked: no new versions can be submitted")
			return
		}

		active, err := s.ResolveActiveVersion(c.Request.Context(), tenantID, name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				response.RespondError(c, http.StatusNotFound, "not_found", "skill not found: "+name)
				return
			}
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to resolve active version: "+err.Error())
			return
		}

		next := s.NextFreeVersion(c.Request.Context(), tenantID, name, active)

		// Keep the SKILL.md frontmatter version in sync with the minted version (no-op if absent/invalid).
		for i := range clean {
			if clean[i].Path == "SKILL.md" {
				clean[i].Content = skill.SetFrontmatterVersion(clean[i].Content, next)
			}
		}

		newZip, desc, lang, err := buildZipFromTree(clean)
		if err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to package skill: "+err.Error())
			return
		}
		if cfg.MaxSkillSize > 0 && int64(len(newZip)) > cfg.MaxSkillSize {
			response.RespondError(c, http.StatusRequestEntityTooLarge, "too_large", "skill zip exceeds maximum allowed size")
			return
		}

		if err := reg.Submit(c.Request.Context(), tenantID, name, next, bytes.NewReader(newZip), int64(len(newZip))); err != nil {
			response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to submit skill: "+err.Error())
			return
		}

		if err := s.UpsertSkill(c.Request.Context(), &store.SkillRecord{
			TenantID:    tenantID,
			Name:        name,
			Version:     next,
			Description: desc,
			Lang:        lang,
			Status:      store.SkillStatusPending,
		}); err != nil {
			_ = c.Error(err)
		}

		if worker != nil {
			worker.Submit(scanner.ScanJob{TenantID: tenantID, Skill: name, Version: next})
		}

		c.JSON(http.StatusAccepted, gin.H{
			"name":    name,
			"version": next,
			"status":  store.SkillStatusPending,
		})
	}
}

// buildZipFromTree packages an exact file set into a skill zip; returns archive + desc/lang from SKILL.md.
func buildZipFromTree(files []batchFile) ([]byte, string, string, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.Create(f.Path)
		if err != nil {
			return nil, "", "", err
		}
		if _, err := fw.Write([]byte(f.Content)); err != nil {
			return nil, "", "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", "", err
	}
	desc, lang := "", ""
	if parsed, err := validateSkillZip(buf.Bytes()); err == nil {
		desc = parsed.Description
		lang = parsed.Lang
	}
	return buf.Bytes(), desc, lang, nil
}

// repackageWithFile rebuilds a skill zip with one file replaced or added.
// It returns the new archive plus the skill's description and lang parsed from
// SKILL.md so the metadata row can be carried forward.
func repackageWithFile(zipBytes []byte, filePath, content string) ([]byte, string, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, "", "", err
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	replaced := false

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		entryName := strings.TrimPrefix(f.Name, "./")
		if strings.Contains(entryName, "..") {
			continue
		}
		data := []byte(nil)
		if entryName == filePath {
			data = []byte(content)
			replaced = true
		} else {
			rc, err := f.Open()
			if err != nil {
				return nil, "", "", err
			}
			data, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return nil, "", "", err
			}
		}
		fw, err := w.Create(entryName)
		if err != nil {
			return nil, "", "", err
		}
		if _, err := fw.Write(data); err != nil {
			return nil, "", "", err
		}
	}

	if !replaced {
		fw, err := w.Create(filePath)
		if err != nil {
			return nil, "", "", err
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			return nil, "", "", err
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", "", err
	}

	desc, lang := "", ""
	if parsed, err := validateSkillZip(buf.Bytes()); err == nil {
		desc = parsed.Description
		lang = parsed.Lang
	}
	return buf.Bytes(), desc, lang, nil
}
