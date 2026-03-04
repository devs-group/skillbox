package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/devs-group/skillbox/internal/api/middleware"
	"github.com/devs-group/skillbox/internal/api/response"
	"github.com/devs-group/skillbox/internal/sandbox"
)

// SandboxHandler groups sandbox shell HTTP handlers and their dependencies.
type SandboxHandler struct {
	manager *sandbox.SessionManager
}

// NewSandboxHandler creates a handler with the session manager dependency.
func NewSandboxHandler(sm *sandbox.SessionManager) *SandboxHandler {
	return &SandboxHandler{manager: sm}
}

// --- Request / Response types ---

// SandboxExecRequest is the body for POST /v1/sandbox/execute.
type SandboxExecRequest struct {
	Command   string `json:"command"`
	WorkDir   string `json:"workdir,omitempty"`    // default: /sandbox/session
	TimeoutMs int    `json:"timeout_ms,omitempty"` // default: 30000
}

// SandboxExecResponse is the response for POST /v1/sandbox/execute.
type SandboxExecResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// SandboxReadRequest is the body for POST /v1/sandbox/read-file.
type SandboxReadRequest struct {
	Path string `json:"path"`
}

// SandboxReadResponse is the response for POST /v1/sandbox/read-file.
type SandboxReadResponse struct {
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

// SandboxWriteRequest is the body for POST /v1/sandbox/write-file.
type SandboxWriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append,omitempty"`
}

// SandboxListRequest is the body for POST /v1/sandbox/list-dir.
type SandboxListRequest struct {
	Path     string `json:"path"`
	MaxDepth int    `json:"max_depth,omitempty"` // default: 2
}

// SandboxListResponse is the response for POST /v1/sandbox/list-dir.
type SandboxListResponse struct {
	Entries []SandboxDirEntry `json:"entries"`
}

// SandboxDirEntry represents a single directory entry in a listing response.
type SandboxDirEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// --- Handlers ---

// Execute handles POST /v1/sandbox/execute.
func (h *SandboxHandler) Execute(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "X-Session-ID header is required")
		return
	}

	var req SandboxExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
		return
	}
	if req.Command == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "command is required")
		return
	}
	if req.WorkDir == "" {
		req.WorkDir = "/sandbox/session"
	}
	if req.TimeoutMs <= 0 {
		req.TimeoutMs = 30000
	}

	ms, err := h.manager.GetOrCreate(c.Request.Context(), tenantID, sessionID, sandbox.SandboxSessionOpts{})
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "sandbox_error", "failed to get or create sandbox: "+err.Error())
		return
	}

	key := tenantID + ":" + ms.ExternalID
	result, err := h.manager.Execute(c.Request.Context(), key, req.Command, req.WorkDir, req.TimeoutMs)
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "execution_error", "command execution failed: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, SandboxExecResponse{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	})
}

// ReadFile handles POST /v1/sandbox/read-file.
func (h *SandboxHandler) ReadFile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "X-Session-ID header is required")
		return
	}

	var req SandboxReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
		return
	}
	if req.Path == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "path is required")
		return
	}

	ms, err := h.manager.GetOrCreate(c.Request.Context(), tenantID, sessionID, sandbox.SandboxSessionOpts{})
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "sandbox_error", "failed to get or create sandbox: "+err.Error())
		return
	}

	key := tenantID + ":" + ms.ExternalID
	data, err := h.manager.ReadFile(c.Request.Context(), key, req.Path)
	if err != nil {
		response.RespondError(c, http.StatusBadRequest, "read_error", "failed to read file: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, SandboxReadResponse{
		Content: string(data),
		Size:    int64(len(data)),
	})
}

// WriteFile handles POST /v1/sandbox/write-file.
func (h *SandboxHandler) WriteFile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "X-Session-ID header is required")
		return
	}

	var req SandboxWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
		return
	}
	if req.Path == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "path is required")
		return
	}

	ms, err := h.manager.GetOrCreate(c.Request.Context(), tenantID, sessionID, sandbox.SandboxSessionOpts{})
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "sandbox_error", "failed to get or create sandbox: "+err.Error())
		return
	}

	key := tenantID + ":" + ms.ExternalID
	content := req.Content
	if req.Append {
		// For append mode, read existing content first and concatenate.
		existing, err := h.manager.ReadFile(c.Request.Context(), key, req.Path)
		if err == nil {
			content = string(existing) + content
		}
		// If read fails (file doesn't exist), just write the new content.
	}

	if err := h.manager.WriteFile(c.Request.Context(), key, req.Path, content); err != nil {
		response.RespondError(c, http.StatusBadRequest, "write_error", "failed to write file: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListDir handles POST /v1/sandbox/list-dir.
func (h *SandboxHandler) ListDir(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "X-Session-ID header is required")
		return
	}

	var req SandboxListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
		return
	}
	if req.Path == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "path is required")
		return
	}
	if req.MaxDepth <= 0 {
		req.MaxDepth = 2
	}

	ms, err := h.manager.GetOrCreate(c.Request.Context(), tenantID, sessionID, sandbox.SandboxSessionOpts{})
	if err != nil {
		response.RespondError(c, http.StatusInternalServerError, "sandbox_error", "failed to get or create sandbox: "+err.Error())
		return
	}

	key := tenantID + ":" + ms.ExternalID
	entries, err := h.manager.ListDir(c.Request.Context(), key, req.Path, req.MaxDepth)
	if err != nil {
		response.RespondError(c, http.StatusBadRequest, "list_error", "failed to list directory: "+err.Error())
		return
	}

	// Convert from sandbox.DirEntry to SandboxDirEntry.
	apiEntries := make([]SandboxDirEntry, len(entries))
	for i, e := range entries {
		apiEntries[i] = SandboxDirEntry{
			Path:  e.Path,
			IsDir: e.IsDir,
			Size:  e.Size,
		}
	}

	c.JSON(http.StatusOK, SandboxListResponse{Entries: apiEntries})
}

// Sync handles POST /v1/sandbox/sync.
func (h *SandboxHandler) Sync(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	sessionID := c.GetHeader("X-Session-ID")
	if sessionID == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "X-Session-ID header is required")
		return
	}

	key := tenantID + ":" + sessionID
	if err := h.manager.SyncSessionFiles(c.Request.Context(), key); err != nil {
		response.RespondError(c, http.StatusInternalServerError, "sync_error", "failed to sync session files: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Destroy handles DELETE /v1/sandbox/:session.
func (h *SandboxHandler) Destroy(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	sessionID := c.Param("session")
	if sessionID == "" {
		response.RespondError(c, http.StatusBadRequest, "bad_request", "session ID is required")
		return
	}

	key := tenantID + ":" + sessionID
	if err := h.manager.Destroy(c.Request.Context(), key); err != nil {
		response.RespondError(c, http.StatusNotFound, "not_found", "sandbox not found or already destroyed: "+err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}
