package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/store"
)

// FilesHandler groups the handler methods for the /v1/files endpoints.
type FilesHandler struct {
	store     *store.Store
	collector *artifacts.Collector
}

// NewFilesHandler creates a new FilesHandler with the given store and
// artifact collector dependencies.
func NewFilesHandler(st *store.Store, col *artifacts.Collector) *FilesHandler {
	return &FilesHandler{
		store:     st,
		collector: col,
	}
}

// List handles GET /v1/files.
// It returns files matching the query parameters for the authenticated
// tenant, with pagination via limit and offset.
func (h *FilesHandler) List(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := store.FileFilter{
		TenantID:    tenantID,
		SessionID:   c.Query("session_id"),
		ExecutionID: c.Query("execution_id"),
		Limit:       limit,
		Offset:      offset,
	}

	files, err := h.store.ListFiles(c.Request.Context(), filter)
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list files")
		return
	}

	// Always return an array, even if empty.
	if files == nil {
		files = []*store.File{}
	}

	c.JSON(http.StatusOK, files)
}

// Get handles GET /v1/files/:id.
// It retrieves a single file record and enforces tenant isolation.
func (h *FilesHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "file id is required")
		return
	}

	tenantID := middleware.GetTenantID(c)

	file, err := h.store.GetFile(c.Request.Context(), id, tenantID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusNotFound, "not_found", "file not found")
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve file")
		return
	}

	c.JSON(http.StatusOK, file)
}

// Download handles GET /v1/files/:id/download.
// It streams the file content from S3/MinIO directly to the client.
func (h *FilesHandler) Download(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "file id is required")
		return
	}

	tenantID := middleware.GetTenantID(c)

	file, err := h.store.GetFile(c.Request.Context(), id, tenantID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusNotFound, "not_found", "file not found")
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve file")
		return
	}

	reader, size, contentType, dlErr := h.collector.DownloadObject(c.Request.Context(), file.S3Key)
	if dlErr != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to download file from storage")
		return
	}
	defer reader.Close()

	if contentType == "" {
		contentType = file.ContentType
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Name))
	c.DataFromReader(http.StatusOK, size, contentType, reader, nil)
}

// Update handles PUT /v1/files/:id.
// It accepts a multipart form upload with a "file" field, uploads the new
// content to S3, and creates a new version record linked to the original
// via parent_id.
func (h *FilesHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "file id is required")
		return
	}

	tenantID := middleware.GetTenantID(c)

	existing, err := h.store.GetFile(c.Request.Context(), id, tenantID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusNotFound, "not_found", "file not found")
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve file")
		return
	}

	upload, header, formErr := c.Request.FormFile("file")
	if formErr != nil {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "missing 'file' field in multipart form")
		return
	}
	defer upload.Close()

	newVersion := existing.Version + 1
	newS3Key := fmt.Sprintf("%s/%s/v%d/%s", existing.TenantID, existing.ExecutionID, newVersion, existing.Name)
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = detectContentType(existing.Name)
	}

	uploadedSize, uploadErr := h.collector.UploadObject(c.Request.Context(), newS3Key, upload, header.Size, contentType)
	if uploadErr != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to upload file to storage")
		return
	}

	// Determine the root parent for the version chain.
	parentID := id
	if existing.ParentID != nil {
		parentID = *existing.ParentID
	}

	newFile := &store.File{
		TenantID:    existing.TenantID,
		SessionID:   existing.SessionID,
		ExecutionID: existing.ExecutionID,
		Name:        existing.Name,
		ContentType: contentType,
		SizeBytes:   uploadedSize,
		S3Key:       newS3Key,
		Version:     newVersion,
		ParentID:    &parentID,
	}

	created, createErr := h.store.CreateFile(c.Request.Context(), newFile)
	if createErr != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to create file version record")
		return
	}

	c.JSON(http.StatusOK, created)
}

// Delete handles DELETE /v1/files/:id.
// It removes the file record from the database and the object from S3.
func (h *FilesHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "file id is required")
		return
	}

	tenantID := middleware.GetTenantID(c)

	file, err := h.store.GetFile(c.Request.Context(), id, tenantID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusNotFound, "not_found", "file not found")
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve file")
		return
	}

	if deleteErr := h.store.DeleteFile(c.Request.Context(), id, tenantID); deleteErr != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to delete file record")
		return
	}

	// Best-effort removal from S3; the DB record is already gone.
	if h.collector != nil {
		_ = h.collector.DeleteObject(c.Request.Context(), file.S3Key)
	}

	c.Status(http.StatusNoContent)
}

// Versions handles GET /v1/files/:id/versions.
// It returns all versions of the file, ordered by version descending.
func (h *FilesHandler) Versions(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "file id is required")
		return
	}

	tenantID := middleware.GetTenantID(c)

	// Verify the file exists and belongs to the tenant.
	_, err := h.store.GetFile(c.Request.Context(), id, tenantID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			response.RespondError(c, http.StatusNotFound, "not_found", "file not found")
			return
		}
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to retrieve file")
		return
	}

	versions, versErr := h.store.ListFileVersions(c.Request.Context(), id, tenantID)
	if versErr != nil {
		response.RespondError(c, http.StatusInternalServerError, "internal_error", "failed to list file versions")
		return
	}

	if versions == nil {
		versions = []*store.File{}
	}

	c.JSON(http.StatusOK, versions)
}

// detectContentType returns a MIME type based on the file extension.
func detectContentType(name string) string {
	switch {
	case hasAnySuffix(name, ".json"):
		return "application/json"
	case hasAnySuffix(name, ".csv"):
		return "text/csv"
	case hasAnySuffix(name, ".txt", ".log"):
		return "text/plain"
	case hasAnySuffix(name, ".html", ".htm"):
		return "text/html"
	case hasAnySuffix(name, ".xml"):
		return "application/xml"
	case hasAnySuffix(name, ".pdf"):
		return "application/pdf"
	case hasAnySuffix(name, ".png"):
		return "image/png"
	case hasAnySuffix(name, ".jpg", ".jpeg"):
		return "image/jpeg"
	case hasAnySuffix(name, ".gif"):
		return "image/gif"
	case hasAnySuffix(name, ".svg"):
		return "image/svg+xml"
	case hasAnySuffix(name, ".zip"):
		return "application/zip"
	case hasAnySuffix(name, ".tar"):
		return "application/x-tar"
	case hasAnySuffix(name, ".gz", ".tgz"):
		return "application/gzip"
	case hasAnySuffix(name, ".py"):
		return "text/x-python"
	case hasAnySuffix(name, ".js"):
		return "application/javascript"
	case hasAnySuffix(name, ".yaml", ".yml"):
		return "application/x-yaml"
	case hasAnySuffix(name, ".md"):
		return "text/markdown"
	default:
		return "application/octet-stream"
	}
}

// hasAnySuffix returns true if s ends with any of the given suffixes.
func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

