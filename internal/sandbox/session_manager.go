package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/store"
)

// maxSessionFileSize is the maximum size of a single file that the session
// manager will read into memory during mount or sync operations.
const maxSessionFileSize = 100 << 20 // 100 MiB

// DirEntry describes a single entry returned by ListDir.
type DirEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ManagedSandbox tracks a long-lived sandbox tied to a session.
type ManagedSandbox struct {
	SandboxID  string
	ExecDURL   string
	SessionID  string // DB session ID
	TenantID   string
	ExternalID string // VectorChat session UUID
	CreatedAt  time.Time
	LastUsedAt time.Time
	Image      string
}

// SandboxSessionOpts configures sandbox creation for a session.
type SandboxSessionOpts struct {
	Image   string
	Memory  string
	CPU     string
	Timeout int // sandbox TTL in seconds
}

// SessionManager manages long-lived sandboxes tied to sessions.
type SessionManager struct {
	client    *Client
	store     *store.Store
	artifacts *artifacts.Collector
	config    *config.Config

	mu          sync.Mutex
	sessions    map[string]*ManagedSandbox // keyed by "{tenantID}:{sessionExternalID}"
	createGroup singleflight.Group         // coalesces concurrent GetOrCreate for same key
}

// NewSessionManager creates a SessionManager with all required dependencies.
func NewSessionManager(client *Client, s *store.Store, col *artifacts.Collector, cfg *config.Config) *SessionManager {
	return &SessionManager{
		client:    client,
		store:     s,
		artifacts: col,
		config:    cfg,
		sessions:  make(map[string]*ManagedSandbox),
	}
}

// sessionKey builds the map key for a tenant + external session ID pair.
func sessionKey(tenantID, externalID string) string {
	return tenantID + ":" + externalID
}

// GetOrCreate finds an existing managed sandbox by key or creates a new one.
// When creating: calls store.GetOrCreateSession, creates sandbox with OpenSandbox
// API, waits for ready, discovers ExecD, mounts session files from MinIO, and
// creates placeholder directories. Returns cached sandbox on subsequent calls.
// Concurrent calls for the same key are coalesced via singleflight.
func (sm *SessionManager) GetOrCreate(ctx context.Context, tenantID, externalID string, opts SandboxSessionOpts) (*ManagedSandbox, error) {
	key := sessionKey(tenantID, externalID)

	// Fast path: already cached.
	sm.mu.Lock()
	if ms, ok := sm.sessions[key]; ok {
		ms.LastUsedAt = time.Now()
		sm.mu.Unlock()

		// Health check: verify the cached sandbox is still reachable.
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := sm.client.Ping(pingCtx, ms.ExecDURL); err != nil {
			slog.Warn("session manager: stale sandbox detected, recreating",
				"key", key,
				"sandbox_id", ms.SandboxID,
				"error", err,
			)
			// Sync files before removing (best-effort).
			_ = sm.SyncSessionFiles(ctx, key)
			sm.mu.Lock()
			delete(sm.sessions, key)
			sm.mu.Unlock()
			// Fall through to create a new sandbox below.
		} else {
			return ms, nil
		}
	} else {
		// Check capacity while still holding the lock.
		if len(sm.sessions) >= sm.config.MaxSessionSandboxes {
			sm.mu.Unlock()
			return nil, fmt.Errorf("session manager: max concurrent session sandboxes reached (%d)", sm.config.MaxSessionSandboxes)
		}
		sm.mu.Unlock()
	}

	// Coalesce concurrent creations for the same key.
	// Use a detached context with a generous timeout so that sandbox
	// creation is not cancelled by a short-lived HTTP request context.
	val, err, _ := sm.createGroup.Do(key, func() (any, error) {
		createCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		return sm.createSandbox(createCtx, tenantID, externalID, key, opts)
	})
	if err != nil {
		return nil, err
	}
	return val.(*ManagedSandbox), nil
}

// createSandbox performs the actual sandbox creation. Called at most once
// per key due to singleflight coalescing.
func (sm *SessionManager) createSandbox(ctx context.Context, tenantID, externalID, key string, opts SandboxSessionOpts) (*ManagedSandbox, error) {
	// Re-check cache in case another singleflight call just completed.
	sm.mu.Lock()
	if ms, ok := sm.sessions[key]; ok {
		ms.LastUsedAt = time.Now()
		sm.mu.Unlock()
		return ms, nil
	}
	sm.mu.Unlock()

	// Create or retrieve the DB session record.
	sess, err := sm.store.GetOrCreateSession(ctx, tenantID, externalID)
	if err != nil {
		return nil, fmt.Errorf("session manager: get or create session: %w", err)
	}

	// Resolve defaults.
	image := opts.Image
	if image == "" {
		image = sm.config.SandboxSessionImage
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = int(sm.config.SandboxSessionTTL.Seconds())
	}
	memory := opts.Memory
	if memory == "" {
		memory = sm.config.DefaultMemoryStr()
	}
	cpu := opts.CPU
	if cpu == "" {
		cpu = sm.config.DefaultCPUStr()
	}

	// Create sandbox via OpenSandbox API. Entrypoint defaults to
	// "sleep <timeout>" inside CreateSandbox.
	sbResp, err := sm.client.CreateSandbox(ctx, SandboxOpts{
		Image:   image,
		Timeout: timeout,
		ResourceLimits: map[string]string{
			"memory": memory,
			"cpu":    cpu,
		},
		Metadata: map[string]string{
			"tenant_id":   tenantID,
			"session_id":  sess.ID,
			"external_id": externalID,
			"managed_by":  "session_manager",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("session manager: create sandbox: %w", err)
	}

	slog.Info("session sandbox created",
		"sandbox_id", sbResp.ID,
		"tenant_id", tenantID,
		"external_id", externalID,
		"image", image,
	)

	// Wait for sandbox to be running.
	if _, err := sm.client.WaitReady(ctx, sbResp.ID); err != nil {
		_ = sm.client.DeleteSandbox(context.Background(), sbResp.ID)
		return nil, fmt.Errorf("session manager: wait ready: %w", err)
	}

	// Discover ExecD endpoint.
	execdURL, _, err := sm.client.DiscoverExecD(ctx, sbResp.ID)
	if err != nil {
		_ = sm.client.DeleteSandbox(context.Background(), sbResp.ID)
		return nil, fmt.Errorf("session manager: discover execd: %w", err)
	}

	// Wait for ExecD to be reachable before using it. The container may be
	// "Running" but the ExecD process inside needs a few hundred ms to bind
	// its port. Without this gate, the first operation (placeholder dirs)
	// fails with EOF because the proxy has nothing to connect to yet.
	readyCtx, readyCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readyCancel()
	if err := sm.client.WaitExecDReady(readyCtx, execdURL); err != nil {
		slog.Warn("session manager: execd readiness timeout, proceeding anyway",
			"sandbox_id", sbResp.ID,
			"error", err,
		)
	}

	// Create sandbox directory structure — everything under /sandbox/session/.
	placeholders := []FileUpload{
		{Path: "/sandbox/session/.keep", Content: []byte(""), Mode: 0o644},
		{Path: "/sandbox/session/skills/.keep", Content: []byte(""), Mode: 0o644},
		{Path: "/sandbox/session/outputs/.keep", Content: []byte(""), Mode: 0o644},
	}
	if err := sm.client.UploadFiles(ctx, execdURL, placeholders); err != nil {
		slog.Warn("session manager: failed to create placeholder dirs",
			"sandbox_id", sbResp.ID,
			"error", err,
		)
	}

	// Mount existing session files from MinIO into the sandbox.
	if err := sm.mountSessionFiles(ctx, tenantID, sess.ID, execdURL); err != nil {
		slog.Warn("session manager: failed to mount session files",
			"sandbox_id", sbResp.ID,
			"error", err,
		)
	}

	ms := &ManagedSandbox{
		SandboxID:  sbResp.ID,
		ExecDURL:   execdURL,
		SessionID:  sess.ID,
		TenantID:   tenantID,
		ExternalID: externalID,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		Image:      image,
	}

	sm.mu.Lock()
	sm.sessions[key] = ms
	sm.mu.Unlock()

	return ms, nil
}

// mountSessionFiles downloads existing session files from MinIO and uploads
// them into the sandbox's /sandbox/session/ directory.
func (sm *SessionManager) mountSessionFiles(ctx context.Context, tenantID, sessionID, execdURL string) error {
	files, err := sm.store.ListSessionFiles(ctx, tenantID, sessionID)
	if err != nil {
		return fmt.Errorf("list session files: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	var uploads []FileUpload
	for _, f := range files {
		rc, _, _, err := sm.artifacts.DownloadObject(ctx, f.S3Key)
		if err != nil {
			slog.Warn("session manager: failed to download file for mount",
				"s3_key", f.S3Key,
				"error", err,
			)
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxSessionFileSize))
		_ = rc.Close()
		if err != nil {
			slog.Warn("session manager: failed to read file for mount",
				"s3_key", f.S3Key,
				"error", err,
			)
			continue
		}
		// Validate the file name from DB before using in a path.
		if strings.Contains(f.Name, "..") || strings.HasPrefix(f.Name, "/") {
			slog.Warn("session manager: skipping file with unsafe name",
				"name", f.Name,
			)
			continue
		}
		uploads = append(uploads, FileUpload{
			Path:    "/sandbox/session/" + f.Name,
			Content: data,
			Mode:    0o644,
		})
	}

	if len(uploads) == 0 {
		return nil
	}

	slog.Info("session manager: restoring session files",
		"tenant_id", tenantID,
		"session_id", sessionID,
		"files_found", len(files),
		"files_restored", len(uploads),
	)

	return sm.client.UploadFiles(ctx, execdURL, uploads)
}

// Execute runs a command in the managed sandbox identified by key.
// If the sandbox is unreachable, evicts it so GetOrCreate recreates on next call.
func (sm *SessionManager) Execute(ctx context.Context, key string, command, workdir string, timeout int) (*CommandResult, error) {
	ms, err := sm.getSession(key)
	if err != nil {
		return nil, err
	}

	result, err := sm.client.RunCommand(ctx, ms.ExecDURL, command, workdir, timeout)
	if err != nil {
		if isConnectionError(err) {
			sm.evictStale(key, ms.SandboxID, err)
		}
		return nil, fmt.Errorf("session manager: execute: %w", err)
	}
	return result, nil
}

// ReadFile downloads a file from the managed sandbox, validating the path
// for read access. Evicts the sandbox on connection errors.
func (sm *SessionManager) ReadFile(ctx context.Context, key string, filePath string) ([]byte, error) {
	if err := ValidateSandboxPath(filePath); err != nil {
		return nil, err
	}

	ms, err := sm.getSession(key)
	if err != nil {
		return nil, err
	}

	rc, err := sm.client.DownloadFile(ctx, ms.ExecDURL, filePath)
	if err != nil {
		if isConnectionError(err) {
			sm.evictStale(key, ms.SandboxID, err)
		}
		return nil, fmt.Errorf("session manager: read file: %w", err)
	}
	defer rc.Close() //nolint:errcheck

	data, err := io.ReadAll(io.LimitReader(rc, maxSessionFileSize))
	if err != nil {
		return nil, fmt.Errorf("session manager: read file body: %w", err)
	}
	return data, nil
}

// WriteFile uploads a file to the managed sandbox, validating the path
// for write access. Evicts the sandbox on connection errors.
func (sm *SessionManager) WriteFile(ctx context.Context, key string, filePath, content string) error {
	if err := ValidateSandboxPath(filePath); err != nil {
		return err
	}

	ms, err := sm.getSession(key)
	if err != nil {
		return err
	}

	if uploadErr := sm.client.UploadFiles(ctx, ms.ExecDURL, []FileUpload{
		{Path: filePath, Content: []byte(content), Mode: 0o644},
	}); uploadErr != nil {
		if isConnectionError(uploadErr) {
			sm.evictStale(key, ms.SandboxID, uploadErr)
		}
		return uploadErr
	}
	return nil
}

// UploadFiles uploads multiple files to the managed sandbox in a single
// multipart request. Unlike WriteFile, this skips path validation so that
// internal callers (e.g. skill mounting) can write to /sandbox/scripts/.
// Evicts the sandbox on connection errors.
func (sm *SessionManager) UploadFiles(ctx context.Context, key string, files []FileUpload) error {
	if len(files) == 0 {
		return nil
	}

	ms, err := sm.getSession(key)
	if err != nil {
		return err
	}

	if uploadErr := sm.client.UploadFiles(ctx, ms.ExecDURL, files); uploadErr != nil {
		if isConnectionError(uploadErr) {
			sm.evictStale(key, ms.SandboxID, uploadErr)
		}
		return uploadErr
	}
	return nil
}

// ListDir lists directory entries in the sandbox using SearchFiles, validating
// the path for read access. It infers directories from file paths.
// Evicts the sandbox on connection errors.
func (sm *SessionManager) ListDir(ctx context.Context, key string, dirPath string, maxDepth int) ([]DirEntry, error) {
	if err := ValidateSandboxPath(dirPath); err != nil {
		return nil, err
	}

	ms, err := sm.getSession(key)
	if err != nil {
		return nil, err
	}

	if maxDepth <= 0 {
		maxDepth = 2
	}

	// Use glob pattern "**" to find all files under the directory.
	files, searchErr := sm.client.SearchFiles(ctx, ms.ExecDURL, dirPath, "**")
	if searchErr != nil {
		if isConnectionError(searchErr) {
			sm.evictStale(key, ms.SandboxID, searchErr)
		}
		return nil, fmt.Errorf("session manager: list dir: %w", searchErr)
	}

	// Build entries: collect files and infer directories from paths.
	dirSet := make(map[string]struct{})
	var entries []DirEntry

	cleanDir := path.Clean(dirPath)

	for _, f := range files {
		cleanFile := path.Clean(f.Path)
		rel := strings.TrimPrefix(cleanFile, cleanDir+"/")
		if rel == cleanFile {
			// File is not under dirPath.
			continue
		}

		// Count depth (number of "/" in relative path).
		depth := strings.Count(rel, "/") + 1
		if depth > maxDepth {
			continue
		}

		// Add parent directories that haven't been added yet.
		parts := strings.Split(rel, "/")
		for i := 0; i < len(parts)-1 && i < maxDepth; i++ {
			dirRel := strings.Join(parts[:i+1], "/")
			dirAbs := cleanDir + "/" + dirRel
			if _, exists := dirSet[dirAbs]; !exists {
				dirSet[dirAbs] = struct{}{}
				entries = append(entries, DirEntry{
					Path:  dirAbs,
					IsDir: true,
					Size:  0,
				})
			}
		}

		// Add the file entry if within depth.
		entries = append(entries, DirEntry{
			Path:  cleanFile,
			IsDir: false,
			Size:  f.Size,
		})
	}

	if entries == nil {
		entries = []DirEntry{}
	}

	return entries, nil
}

// getSession looks up a managed sandbox by key and updates LastUsedAt.
func (sm *SessionManager) getSession(key string) (*ManagedSandbox, error) {
	sm.mu.Lock()
	ms, ok := sm.sessions[key]
	if ok {
		ms.LastUsedAt = time.Now()
	}
	sm.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("session manager: no sandbox for key %q", key)
	}
	return ms, nil
}

// evictStale removes a sandbox from the cache when a connection error is
// detected. The next GetOrCreate call will recreate the sandbox transparently.
func (sm *SessionManager) evictStale(key, sandboxID string, err error) {
	slog.Warn("session manager: evicting stale sandbox after connection error",
		"key", key,
		"sandbox_id", sandboxID,
		"error", err,
	)
	sm.mu.Lock()
	if ms, ok := sm.sessions[key]; ok && ms.SandboxID == sandboxID {
		delete(sm.sessions, key)
	}
	sm.mu.Unlock()
}

// isConnectionError checks if the error indicates the sandbox is unreachable.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "EOF")
}

// SyncSessionFiles downloads files from /sandbox/out/session/ and
// /sandbox/session/ in the sandbox, uploads them to MinIO, and creates
// or updates file records in the DB.
func (sm *SessionManager) SyncSessionFiles(ctx context.Context, key string) error {
	sm.mu.Lock()
	ms, ok := sm.sessions[key]
	sm.mu.Unlock()

	if !ok {
		return fmt.Errorf("session manager: no sandbox for key %q", key)
	}

	// Sync everything from the single session directory.
	syncDirs := []string{"/sandbox/session"}
	syncedCount := 0

	for _, dir := range syncDirs {
		files, err := sm.client.SearchFiles(ctx, ms.ExecDURL, dir, "**")
		if err != nil {
			slog.Warn("session manager: sync search failed",
				"dir", dir,
				"sandbox_id", ms.SandboxID,
				"error", err,
			)
			continue
		}

		for _, f := range files {
			// Skip placeholder files.
			if strings.HasSuffix(f.Path, "/.keep") {
				continue
			}
			if f.Size == 0 {
				continue
			}

			rc, err := sm.client.DownloadFile(ctx, ms.ExecDURL, f.Path)
			if err != nil {
				slog.Warn("session manager: sync download failed",
					"path", f.Path,
					"error", err,
				)
				continue
			}
			data, err := io.ReadAll(io.LimitReader(rc, maxSessionFileSize))
			_ = rc.Close()
			if err != nil {
				slog.Warn("session manager: sync read failed",
					"path", f.Path,
					"error", err,
				)
				continue
			}

			// Determine the file name (relative to the sync dir).
			cleanDir := path.Clean(dir)
			cleanPath := path.Clean(f.Path)
			name := strings.TrimPrefix(cleanPath, cleanDir+"/")
			if name == "" || name == cleanPath {
				continue
			}

			// Upload to MinIO.
			s3Key := fmt.Sprintf("%s/sessions/%s/%s", ms.TenantID, ms.ExternalID, name)
			_, err = sm.artifacts.UploadObject(ctx, s3Key, bytes.NewReader(data), int64(len(data)), "application/octet-stream")
			if err != nil {
				slog.Warn("session manager: sync upload to minio failed",
					"s3_key", s3Key,
					"error", err,
				)
				continue
			}

			// Create or update file record in DB.
			dbFile := &store.File{
				TenantID:    ms.TenantID,
				SessionID:   ms.SessionID,
				Name:        name,
				ContentType: "application/octet-stream",
				SizeBytes:   int64(len(data)),
				S3Key:       s3Key,
				Version:     1,
			}
			if _, err := sm.store.CreateFile(ctx, dbFile); err != nil {
				slog.Warn("session manager: sync create file record failed",
					"name", name,
					"error", err,
				)
			} else {
				syncedCount++
			}
		}
	}

	if syncedCount > 0 {
		slog.Info("session manager: sync completed",
			"key", key,
			"tenant_id", ms.TenantID,
			"session_id", ms.SessionID,
			"files_synced", syncedCount,
		)
	}

	return nil
}

// Cleanup finds sandboxes that have been idle longer than maxIdle, syncs
// their files, then deletes them from OpenSandbox and removes them from
// the managed map. Called by a background goroutine.
func (sm *SessionManager) Cleanup(ctx context.Context, maxIdle time.Duration) {
	sm.mu.Lock()
	var expired []string
	for key, ms := range sm.sessions {
		if time.Since(ms.LastUsedAt) > maxIdle {
			expired = append(expired, key)
		}
	}
	sm.mu.Unlock()

	for _, key := range expired {
		slog.Info("session manager: cleaning up idle sandbox", "key", key)
		sm.syncAndRemove(ctx, key, "cleanup")
	}

	if len(expired) > 0 {
		slog.Info("session manager: cleanup complete", "cleaned", len(expired))
	}
}

// Shutdown syncs all managed session files and deletes all managed sandboxes.
// Called during graceful server shutdown.
func (sm *SessionManager) Shutdown(ctx context.Context) {
	sm.mu.Lock()
	keys := make([]string, 0, len(sm.sessions))
	for key := range sm.sessions {
		keys = append(keys, key)
	}
	sm.mu.Unlock()

	slog.Info("session manager: shutting down", "active_sandboxes", len(keys))

	for _, key := range keys {
		sm.syncAndRemove(ctx, key, "shutdown")
	}

	slog.Info("session manager: shutdown complete")
}

// syncAndRemove syncs session files then removes the sandbox from the managed map
// and deletes it from OpenSandbox. The caller label is used for log messages.
func (sm *SessionManager) syncAndRemove(ctx context.Context, key, caller string) {
	if err := sm.SyncSessionFiles(ctx, key); err != nil {
		slog.Warn("session manager: "+caller+" sync failed",
			"key", key,
			"error", err,
		)
	}

	sm.mu.Lock()
	ms, ok := sm.sessions[key]
	if ok {
		delete(sm.sessions, key)
	}
	sm.mu.Unlock()

	if ok {
		if err := sm.client.DeleteSandbox(ctx, ms.SandboxID); err != nil {
			// OpenSandbox may garbage-collect containers before our idle
			// TTL fires. A 404 means the sandbox is already gone — that's
			// the outcome we wanted, so just log at debug level.
			if isSandboxGone(err) {
				slog.Debug("session manager: "+caller+" sandbox already removed",
					"sandbox_id", ms.SandboxID,
				)
			} else {
				slog.Warn("session manager: "+caller+" delete sandbox failed",
					"sandbox_id", ms.SandboxID,
					"error", err,
				)
			}
		}
	}
}

// isSandboxGone returns true if the error indicates the sandbox no longer
// exists in OpenSandbox (HTTP 404 / SANDBOX_NOT_FOUND).
func isSandboxGone(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "status 404") || strings.Contains(msg, "SANDBOX_NOT_FOUND")
}

// Destroy tears down a specific session sandbox. It syncs files first,
// then deletes the sandbox and removes it from the managed map.
func (sm *SessionManager) Destroy(ctx context.Context, key string) error {
	// Sync files before destroying.
	if err := sm.SyncSessionFiles(ctx, key); err != nil {
		slog.Warn("session manager: destroy sync failed",
			"key", key,
			"error", err,
		)
	}

	sm.mu.Lock()
	ms, ok := sm.sessions[key]
	if ok {
		delete(sm.sessions, key)
	}
	sm.mu.Unlock()

	if !ok {
		return fmt.Errorf("session manager: no sandbox for key %q", key)
	}

	if err := sm.client.DeleteSandbox(ctx, ms.SandboxID); err != nil {
		return fmt.Errorf("session manager: destroy sandbox: %w", err)
	}

	slog.Info("session manager: sandbox destroyed",
		"sandbox_id", ms.SandboxID,
		"key", key,
	)

	return nil
}
