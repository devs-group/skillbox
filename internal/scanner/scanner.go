package scanner

import (
	"archive/zip"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devs-group/skillbox/internal/skill"
)

// Scanner is the public interface for security scanning.
//
// Error contract:
//   - (result, nil): scan completed — check result.Pass for verdict
//   - (nil, error): infrastructure failure — caller should fail closed (reject upload, return 500)
//
// Never returns (result, error).
type Scanner interface {
	Scan(ctx context.Context, zr *zip.Reader, s *skill.Skill) (*ScanResult, error)
}

// stage is an internal interface for a single scan stage.
type stage interface {
	name() string
	run(ctx context.Context, zr *zip.Reader, priorFlags []Finding) ([]Finding, error)
}

// Pipeline implements Scanner by running tiered scan stages.
type Pipeline struct {
	mu      sync.RWMutex
	tier1   []stage
	tier2   []stage
	tier3   []stage // LLM deep analysis (optional, empty when disabled)
	timeout time.Duration
	logger  *slog.Logger
	metrics *Metrics

	// customPatterns holds the currently active custom pattern entries
	// (user-uploaded, separate from embedded defaults). Protected by mu.
	customPatterns *PatternFile
	ossfFeedDir    string
}

// New creates a new scanner Pipeline configured for Tier 1 and Tier 2 scanning.
// If llmCfg is non-nil, Tier 3 LLM analysis is enabled.
// customPatternsFile and ossfFeedDir are optional paths for custom pattern sources.
func New(timeout time.Duration, logger *slog.Logger, llmCfg *LLMConfig, customPatternsFile, ossfFeedDir string) (*Pipeline, error) {
	lp, err := loadPatterns(customPatternsFile, ossfFeedDir, logger)
	if err != nil {
		if customPatternsFile != "" {
			// Fail-closed: if a custom patterns file was explicitly configured
			// but cannot be loaded, refuse to start with degraded security.
			return nil, fmt.Errorf("load custom patterns file %q: %w", customPatternsFile, err)
		}
		logger.Warn("failed to load external patterns, using embedded defaults", "error", err)
		// Fall back to embedded defaults with no custom overlay.
		lp, _ = loadPatterns("", "", logger)
	}

	p := &Pipeline{
		tier1: []stage{
			newPatternStage(logger, lp),
		},
		tier2: []stage{
			newDepsStage(logger, lp.popularPackages, lp.blocklistPackages),
			newPromptStage(logger),
			newSecurityStage(logger),
		},
		timeout:     timeout,
		logger:      logger,
		metrics:     NewMetrics(),
		ossfFeedDir: ossfFeedDir,
	}

	if llmCfg != nil {
		p.tier3 = []stage{newLLMStage(*llmCfg, logger)}
		logger.Info("scanner LLM analysis enabled", "model", llmCfg.Model)
	}

	return p, nil
}

// Metrics returns the scanner's metrics tracker.
func (p *Pipeline) Metrics() *Metrics {
	return p.metrics
}

// GetCustomPatterns returns the currently active custom pattern definitions.
// Returns nil if no custom patterns have been loaded.
func (p *Pipeline) GetCustomPatterns() *PatternFile {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.customPatterns
}

// SetCustomPatterns replaces the custom pattern overlay and rebuilds
// the Tier 1 and Tier 2 stages with the merged pattern set.
// Pass nil to clear all custom patterns and revert to defaults only.
func (p *Pipeline) SetCustomPatterns(pf *PatternFile) error {
	// Build merged patterns: embedded defaults + custom overlay + OSSF feed.
	base, err := parsePatternFile(defaultPatternsYAML)
	if err != nil {
		return fmt.Errorf("parse embedded defaults: %w", err)
	}

	if pf != nil {
		base = mergePatternFiles(base, pf)
	}

	if p.ossfFeedDir != "" {
		pkgs, err := loadOSSFFeed(p.ossfFeedDir, p.logger)
		if err != nil {
			p.logger.Warn("failed to reload OSSF feed", "error", err)
		} else {
			base.BlocklistPackages = append(base.BlocklistPackages, pkgs...)
		}
	}

	lp, err := compilePatternFile(base)
	if err != nil {
		return fmt.Errorf("compile patterns: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.customPatterns = pf
	p.tier1 = []stage{newPatternStage(p.logger, lp)}
	// Rebuild only deps stage (uses blocklist/popular from patterns).
	// Prompt and security stages are stateless.
	p.tier2 = []stage{
		newDepsStage(p.logger, lp.popularPackages, lp.blocklistPackages),
		newPromptStage(p.logger),
		newSecurityStage(p.logger),
	}

	p.logger.Info("custom patterns reloaded",
		"block_patterns", len(lp.blockPatterns),
		"flag_patterns", len(lp.flagPatterns),
		"blocklist_packages", len(lp.blocklistPackages),
	)

	return nil
}

// Scan runs the tiered scanning pipeline.
//
// Tier 1 (Quick Scan): Runs static patterns + dep blocklist.
//   - No findings → accept immediately (most uploads stop here).
//   - BLOCK findings → reject immediately.
//   - FLAG findings → escalate to Tier 2.
//
// Tier 2 (Deep Scan): Typosquatting, prompt injection, Unicode analysis.
//   - BLOCK findings → reject.
//   - Unresolved flags → escalate to Tier 3 (if enabled).
//   - All resolved → accept.
//
// Tier 3 (LLM Analysis): Contextual judgment by Claude.
//   - Only runs when Tier 2 leaves unresolved FLAG findings and LLM is enabled.
//   - BLOCK findings → reject.
//   - LLM unavailable → fail closed (return error).
//   - LLM says benign → accept.
func (p *Pipeline) Scan(ctx context.Context, zr *zip.Reader, s *skill.Skill) (*ScanResult, error) {
	// Hold read lock for the duration of the scan to prevent
	// stage replacement mid-scan during pattern hot-reload.
	p.mu.RLock()
	defer p.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	result := &ScanResult{Pass: true, Tier: 1}
	start := time.Now()

	// Record metrics on completion (deferred to capture all exit paths).
	recordMetrics := true
	defer func() {
		if recordMetrics && result != nil {
			p.metrics.RecordScan(result)
		}
	}()

	// --- Tier 1: Quick Scan ---
	t1Findings, err := p.runStages(ctx, p.tier1, zr, nil)
	if err != nil {
		recordMetrics = false
		p.metrics.RecordFailure()
		return nil, fmt.Errorf("tier 1: %w", err)
	}
	result.Findings = append(result.Findings, t1Findings...)

	// Any BLOCK → reject immediately.
	if hasBlock(t1Findings) {
		result.Pass = false
		result.Duration = time.Since(start)
		p.logResult(result, s)
		return result, nil
	}

	// Tier 2 always runs: security analysis (secrets, URLs, etc.) applies to all uploads,
	// and dependency/prompt checks run when relevant files are present.
	t1Flags := collectFlags(t1Findings)

	// --- Tier 2: Deep Scan ---
	result.Tier = 2
	t2Findings, err := p.runStages(ctx, p.tier2, zr, t1Flags)
	if err != nil {
		recordMetrics = false
		p.metrics.RecordFailure()
		return nil, fmt.Errorf("tier 2: %w", err)
	}
	result.Findings = append(result.Findings, t2Findings...)

	// Any BLOCK from Tier 2 → reject.
	if hasBlock(t2Findings) {
		result.Pass = false
		result.Duration = time.Since(start)
		p.logResult(result, s)
		return result, nil
	}

	// Collect all unresolved flags from Tier 1 + Tier 2.
	allFlags := collectFlags(result.Findings)

	// --- Tier 3: LLM Analysis (optional) ---
	if len(allFlags) > 0 && len(p.tier3) > 0 {
		result.Tier = 3
		t3Findings, err := p.runStages(ctx, p.tier3, zr, allFlags)
		if err != nil {
			// LLM unavailable → fail closed.
			recordMetrics = false
			p.metrics.RecordFailure()
			return nil, fmt.Errorf("tier 3: %w", err)
		}
		result.Findings = append(result.Findings, t3Findings...)

		if hasBlock(t3Findings) {
			result.Pass = false
			result.Duration = time.Since(start)
			p.logResult(result, s)
			return result, nil
		}
	}

	// Accept with all flags recorded.
	result.Duration = time.Since(start)
	p.logResult(result, s)
	return result, nil
}

// runStages executes a slice of stages sequentially, collecting findings.
func (p *Pipeline) runStages(ctx context.Context, stages []stage, zr *zip.Reader, priorFlags []Finding) ([]Finding, error) {
	var all []Finding
	for _, s := range stages {
		findings, err := s.run(ctx, zr, priorFlags)
		if err != nil {
			return nil, err
		}
		all = append(all, findings...)
	}
	return all, nil
}

// logResult generates the scan summary and emits a structured log entry for the scan verdict.
func (p *Pipeline) logResult(result *ScanResult, s *skill.Skill) {
	result.GenerateSummary()
	categories := make([]string, 0, len(result.Findings))
	seen := make(map[string]bool)
	for _, f := range result.Findings {
		if !seen[f.Category] {
			categories = append(categories, f.Category)
			seen[f.Category] = true
		}
	}

	attrs := []any{
		"skill_name", s.Name,
		"skill_version", s.Version,
		"verdict", verdictString(result.Pass),
		"tier", result.Tier,
		"findings_count", len(result.Findings),
		"categories", categories,
		"duration_ms", result.Duration.Milliseconds(),
	}

	if result.Pass {
		p.logger.Info("security scan passed", attrs...)
	} else {
		p.logger.Warn("security scan blocked", attrs...)
	}
}

// NoopScanner is a no-op scanner for use in tests and when scanning is disabled.
type NoopScanner struct{}

// Scan always returns a passing result with no findings.
func (n *NoopScanner) Scan(_ context.Context, _ *zip.Reader, _ *skill.Skill) (*ScanResult, error) {
	return &ScanResult{Pass: true, Tier: 0}, nil
}

// hasBlock returns true if any finding has BLOCK severity.
func hasBlock(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityBlock {
			return true
		}
	}
	return false
}

// collectFlags returns all FLAG-severity findings from the given slice.
func collectFlags(findings []Finding) []Finding {
	var flags []Finding
	for _, f := range findings {
		if f.Severity == SeverityFlag {
			flags = append(flags, f)
		}
	}
	return flags
}

func verdictString(pass bool) string {
	if pass {
		return "pass"
	}
	return "blocked"
}
