package scanner

import (
	"archive/zip"
	"context"
	"fmt"
	"log/slog"
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
	tier1   []stage
	timeout time.Duration
	logger  *slog.Logger
}

// New creates a new scanner Pipeline configured for Tier 1 scanning.
// Tier 2 and Tier 3 stages are added in later phases.
func New(timeout time.Duration, logger *slog.Logger) *Pipeline {
	return &Pipeline{
		tier1: []stage{
			newPatternStage(logger),
		},
		timeout: timeout,
		logger:  logger,
	}
}

// Scan runs the tiered scanning pipeline.
//
// Tier 1 (Quick Scan): Runs static patterns + dep blocklist.
// If no findings at all, accepts immediately.
// If BLOCK findings, rejects immediately.
// If only FLAG findings, the result contains the flags (Tier 2/3 will handle in later phases).
func (p *Pipeline) Scan(ctx context.Context, zr *zip.Reader, s *skill.Skill) (*ScanResult, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	result := &ScanResult{Pass: true, Tier: 1}
	start := time.Now()

	// --- Tier 1: Quick Scan ---
	findings, err := p.runStages(ctx, p.tier1, zr, nil)
	if err != nil {
		return nil, fmt.Errorf("tier 1: %w", err)
	}
	result.Findings = append(result.Findings, findings...)

	// Any BLOCK → reject immediately.
	if hasBlock(findings) {
		result.Pass = false
		result.Duration = time.Since(start)
		p.logResult(result, s)
		return result, nil
	}

	// FLAG findings without LLM → for now, pass with flags recorded.
	// In Phase 2+, this is where Tier 2 and Tier 3 escalation will happen.
	// For Phase 1: flags are logged but do not block (LLM not yet available).
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

// logResult emits a structured log entry for the scan verdict.
func (p *Pipeline) logResult(result *ScanResult, s *skill.Skill) {
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
