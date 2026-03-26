package scanner

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/devs-group/skillbox/internal/skill"
)

// ScanJob represents a skill queued for async scanning.
type ScanJob struct {
	TenantID string
	Skill    string
	Version  string
}

// RegistryForWorker is the subset of registry.Registry the worker needs.
type RegistryForWorker interface {
	DownloadPending(ctx context.Context, tenantID, skillName, version string) (io.ReadCloser, error)
	Promote(ctx context.Context, tenantID, skillName, version string) error
	Quarantine(ctx context.Context, tenantID, skillName, version string) error
}

// StoreForWorker is the subset of store.Store the worker needs.
type StoreForWorker interface {
	UpdateSkillStatus(ctx context.Context, tenantID, name, version, newStatus string, scanResult json.RawMessage) error
	ListPendingSkills(ctx context.Context) ([]struct {
		TenantID string
		Name     string
		Version  string
	}, error)
	GetScannerConfig(ctx context.Context, tenantID string) (ApprovalConfig, error)
}

// ApprovalConfig is the subset of scanner config the worker needs.
type ApprovalConfig struct {
	ApprovalPolicy string
}

// Worker processes pending skills asynchronously via a background goroutine.
type Worker struct {
	registry RegistryForWorker
	scanner  Scanner
	scanCh   chan ScanJob
	logger   *slog.Logger

	// store operations are done via callbacks to avoid circular imports.
	updateStatus    func(ctx context.Context, tenantID, name, version, status string, result json.RawMessage) error
	listPending     func(ctx context.Context) ([]ScanJob, error)
	getApprovalPolicy func(ctx context.Context, tenantID string) (string, error)
}

// WorkerConfig holds the dependencies for creating a Worker.
type WorkerConfig struct {
	Registry        RegistryForWorker
	Scanner         Scanner
	Logger          *slog.Logger
	BufferSize      int // channel buffer size (default 100)
	UpdateStatus    func(ctx context.Context, tenantID, name, version, status string, result json.RawMessage) error
	ListPending     func(ctx context.Context) ([]ScanJob, error)
	GetApprovalPolicy func(ctx context.Context, tenantID string) (string, error)
}

// NewWorker creates a scan worker with the given dependencies.
func NewWorker(cfg WorkerConfig) *Worker {
	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = 100
	}
	return &Worker{
		registry:          cfg.Registry,
		scanner:           cfg.Scanner,
		scanCh:            make(chan ScanJob, bufSize),
		logger:            cfg.Logger,
		updateStatus:      cfg.UpdateStatus,
		listPending:       cfg.ListPending,
		getApprovalPolicy: cfg.GetApprovalPolicy,
	}
}

// Submit queues a scan job. Non-blocking — drops the job if the channel
// is full (the startup recovery loop will pick it up).
func (w *Worker) Submit(job ScanJob) {
	select {
	case w.scanCh <- job:
		w.logger.Debug("scan job queued", "skill", job.Skill, "version", job.Version, "tenant", job.TenantID)
	default:
		w.logger.Warn("scan job channel full, job will be recovered on next poll",
			"skill", job.Skill, "version", job.Version, "tenant", job.TenantID)
	}
}

// Start launches the background worker goroutine. It first recovers any
// pending jobs from the database, then processes jobs from the channel.
// Blocks until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	// Startup recovery: re-queue skills stuck in pending/scanning.
	w.recoverPendingJobs(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("scan worker shutting down")
			return
		case job := <-w.scanCh:
			w.processJob(ctx, job)
		}
	}
}

// recoverPendingJobs queries for skills stuck in pending/scanning and re-queues them.
func (w *Worker) recoverPendingJobs(ctx context.Context) {
	if w.listPending == nil {
		return
	}

	jobs, err := w.listPending(ctx)
	if err != nil {
		w.logger.Error("failed to recover pending scan jobs", "error", err)
		return
	}

	for _, job := range jobs {
		w.Submit(job)
	}

	if len(jobs) > 0 {
		w.logger.Info("recovered pending scan jobs", "count", len(jobs))
	}
}

// processJob handles a single scan job: download → scan → evaluate → transition.
func (w *Worker) processJob(ctx context.Context, job ScanJob) {
	logger := w.logger.With("skill", job.Skill, "version", job.Version, "tenant", job.TenantID)

	// Transition to scanning.
	if err := w.updateStatus(ctx, job.TenantID, job.Skill, job.Version, "scanning", nil); err != nil {
		logger.Error("failed to mark skill as scanning", "error", err)
		return
	}

	// Download from pending prefix.
	reader, err := w.registry.DownloadPending(ctx, job.TenantID, job.Skill, job.Version)
	if err != nil {
		logger.Error("failed to download pending skill", "error", err)
		w.failJob(ctx, job, "quarantined", nil, "download failed: "+err.Error())
		return
	}
	defer reader.Close() //nolint:errcheck

	zipBytes, err := io.ReadAll(reader)
	if err != nil {
		logger.Error("failed to read pending skill data", "error", err)
		w.failJob(ctx, job, "quarantined", nil, "read failed: "+err.Error())
		return
	}

	// Build zip reader.
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		logger.Error("invalid zip in pending skill", "error", err)
		w.failJob(ctx, job, "quarantined", nil, "invalid zip: "+err.Error())
		return
	}

	// Check ZIP safety first.
	if err := CheckZIPSafety(zr); err != nil {
		result := &ScanResult{
			Pass: false,
			Tier: 0,
			Findings: []Finding{{
				Stage:       "zip_safety",
				Severity:    SeverityBlock,
				Category:    "zip_bomb",
				Description: err.Error(),
			}},
		}
		resultJSON, _ := json.Marshal(result)
		w.failJob(ctx, job, "quarantined", resultJSON, "zip safety check failed")
		_ = w.registry.Quarantine(ctx, job.TenantID, job.Skill, job.Version)
		return
	}

	// Parse SKILL.md for scan context.
	parsedSkill, err := parseSkillFromZip(zr)
	if err != nil {
		logger.Warn("could not parse SKILL.md for scan context", "error", err)
		// Continue with a minimal skill object — scanning can still work.
		parsedSkill = &skill.Skill{Name: job.Skill, Version: job.Version}
	}

	// Run the scanner pipeline.
	scanResult, err := w.scanner.Scan(ctx, zr, parsedSkill)
	if err != nil {
		// Infrastructure failure — fail closed (quarantine).
		logger.Error("scanner infrastructure failure", "error", err)
		w.failJob(ctx, job, "quarantined", nil, "scan infrastructure failure: "+err.Error())
		_ = w.registry.Quarantine(ctx, job.TenantID, job.Skill, job.Version)
		return
	}

	resultJSON, _ := json.Marshal(scanResult)

	// Get the tenant's approval policy.
	policy := "auto"
	if w.getApprovalPolicy != nil {
		if p, err := w.getApprovalPolicy(ctx, job.TenantID); err == nil && p != "" {
			policy = p
		}
	}

	// Evaluate result against policy.
	w.evaluateAndTransition(ctx, job, scanResult, resultJSON, policy, logger)
}

// evaluateAndTransition applies the approval policy to determine the skill's final status.
func (w *Worker) evaluateAndTransition(ctx context.Context, job ScanJob, result *ScanResult, resultJSON json.RawMessage, policy string, logger *slog.Logger) {
	hasBlocks := !result.Pass
	hasFlags := len(collectFlags(result.Findings)) > 0

	switch {
	case hasBlocks:
		// BLOCK → quarantine regardless of policy.
		logger.Warn("scan blocked — quarantining", "duration", result.Duration)
		_ = w.updateStatus(ctx, job.TenantID, job.Skill, job.Version, "quarantined", resultJSON)
		_ = w.registry.Quarantine(ctx, job.TenantID, job.Skill, job.Version)

	case policy == "always":
		// Policy requires manual review for everything.
		logger.Info("scan passed but policy=always — sending to review", "duration", result.Duration)
		_ = w.updateStatus(ctx, job.TenantID, job.Skill, job.Version, "review", resultJSON)

	case hasFlags && policy == "auto":
		// FLAGS with auto policy → manual review.
		logger.Info("scan has flags — sending to review", "duration", result.Duration, "flags", len(collectFlags(result.Findings)))
		_ = w.updateStatus(ctx, job.TenantID, job.Skill, job.Version, "review", resultJSON)

	default:
		// CLEAN (or flags with policy=none) → promote to available.
		logger.Info("scan passed — promoting to available", "duration", result.Duration)
		if err := w.registry.Promote(ctx, job.TenantID, job.Skill, job.Version); err != nil {
			logger.Error("failed to promote skill in registry", "error", err)
			_ = w.updateStatus(ctx, job.TenantID, job.Skill, job.Version, "review", resultJSON)
			return
		}
		_ = w.updateStatus(ctx, job.TenantID, job.Skill, job.Version, "available", resultJSON)
	}
}

// failJob transitions a skill to a failure status and logs the reason.
func (w *Worker) failJob(ctx context.Context, job ScanJob, status string, resultJSON json.RawMessage, reason string) {
	w.logger.Warn("scan job failed",
		"skill", job.Skill, "version", job.Version,
		"tenant", job.TenantID, "status", status, "reason", reason)
	_ = w.updateStatus(ctx, job.TenantID, job.Skill, job.Version, status, resultJSON)
}

// parseSkillFromZip extracts and parses SKILL.md from a zip archive.
func parseSkillFromZip(zr *zip.Reader) (*skill.Skill, error) {
	for _, f := range zr.File {
		name := f.Name
		if name == "SKILL.md" || name == "./SKILL.md" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open SKILL.md: %w", err)
			}
			defer rc.Close() //nolint:errcheck

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("read SKILL.md: %w", err)
			}

			s, err := skill.ParseSkillMD(data)
			if err != nil {
				return nil, fmt.Errorf("parse SKILL.md: %w", err)
			}
			return s, nil
		}
	}
	return nil, fmt.Errorf("SKILL.md not found in archive")
}

