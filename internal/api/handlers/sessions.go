package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/store"
)

// SessionsHandler groups session-related HTTP handlers and their dependencies.
type SessionsHandler struct {
	store     *store.Store
	artifacts *artifacts.Collector
}

// NewSessionsHandler creates a handler with all required dependencies.
func NewSessionsHandler(s *store.Store, col *artifacts.Collector) *SessionsHandler {
	return &SessionsHandler{store: s, artifacts: col}
}

// ListFiles handles GET /v1/sessions/:external_id/files.
func (h *SessionsHandler) ListFiles(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	externalID := c.Param("external_id")
	if externalID == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "session external_id is required")
		return
	}

	sess, err := h.store.GetSessionByExternalID(c.Request.Context(), tenantID, externalID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// No session yet — return empty list.
			c.JSON(http.StatusOK, []*store.File{})
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to find session: "+err.Error())
		return
	}

	files, err := h.store.ListSessionFiles(c.Request.Context(), tenantID, sess.ID)
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list session files: "+err.Error())
		return
	}
	if files == nil {
		files = []*store.File{}
	}
	c.JSON(http.StatusOK, files)
}

// DownloadFile handles GET /v1/sessions/:external_id/files/:filename.
func (h *SessionsHandler) DownloadFile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	externalID := c.Param("external_id")
	filename := c.Param("filename")

	if externalID == "" || filename == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "session external_id and filename are required")
		return
	}

	sess, err := h.store.GetSessionByExternalID(c.Request.Context(), tenantID, externalID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusNotFound, "not_found", "session not found")
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to find session: "+err.Error())
		return
	}

	files, err := h.store.ListSessionFiles(c.Request.Context(), tenantID, sess.ID)
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list session files: "+err.Error())
		return
	}

	var target *store.File
	for _, f := range files {
		if f.Name == filename {
			target = f
			break
		}
	}
	if target == nil {
		response.RespondError(c, http.StatusNotFound, "not_found", "file not found: "+filename)
		return
	}

	rc, size, contentType, err := h.artifacts.DownloadObject(c.Request.Context(), target.S3Key)
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to download file: "+err.Error())
		return
	}
	defer rc.Close() //nolint:errcheck

	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.DataFromReader(http.StatusOK, size, contentType, rc, nil)
}

// DeleteFile handles DELETE /v1/sessions/:external_id/files/:filename.
func (h *SessionsHandler) DeleteFile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	externalID := c.Param("external_id")
	filename := c.Param("filename")

	if externalID == "" || filename == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "session external_id and filename are required")
		return
	}

	sess, err := h.store.GetSessionByExternalID(c.Request.Context(), tenantID, externalID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusNotFound, "not_found", "session not found")
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to find session: "+err.Error())
		return
	}

	files, err := h.store.ListSessionFiles(c.Request.Context(), tenantID, sess.ID)
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list session files: "+err.Error())
		return
	}

	var target *store.File
	for _, f := range files {
		if f.Name == filename {
			target = f
			break
		}
	}
	if target == nil {
		response.RespondError(c, http.StatusNotFound, "not_found", "file not found: "+filename)
		return
	}

	// Delete from MinIO.
	if h.artifacts != nil {
		_ = h.artifacts.DeleteObject(c.Request.Context(), target.S3Key)
	}

	// Delete from DB.
	if err := h.store.DeleteFile(c.Request.Context(), target.ID, tenantID); err != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to delete file record: "+err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// Delete handles DELETE /v1/sessions/:external_id.
func (h *SessionsHandler) Delete(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	externalID := c.Param("external_id")

	if externalID == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "session external_id is required")
		return
	}

	sess, err := h.store.GetSessionByExternalID(c.Request.Context(), tenantID, externalID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusNotFound, "not_found", "session not found")
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to find session: "+err.Error())
		return
	}

	// Delete all session files from MinIO and DB.
	files, _ := h.store.ListSessionFiles(c.Request.Context(), tenantID, sess.ID)
	for _, f := range files {
		if h.artifacts != nil {
			_ = h.artifacts.DeleteObject(c.Request.Context(), f.S3Key)
		}
		_ = h.store.DeleteFile(c.Request.Context(), f.ID, tenantID)
	}

	// Delete the session record.
	if err := h.store.DeleteSession(c.Request.Context(), tenantID, externalID); err != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to delete session: "+err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}
