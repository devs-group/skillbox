package scanner

import (
	"fmt"
	"strings"
	"time"
)

// Severity classifies how a finding affects the scan verdict.
type Severity string

const (
	// SeverityBlock causes immediate rejection. The skill is never stored.
	SeverityBlock Severity = "BLOCK"

	// SeverityFlag escalates the finding to a deeper tier for contextual judgment.
	SeverityFlag Severity = "FLAG"
)

// Finding represents a single security observation from a scan stage.
type Finding struct {
	Stage       string   `json:"stage"`                  // e.g. "static_patterns", "dependencies"
	Severity    Severity `json:"severity"`                // BLOCK or FLAG
	Category    string   `json:"category"`                // e.g. "reverse_shell", "typosquat"
	FilePath    string   `json:"file_path"`               // relative path within ZIP
	Description string   `json:"description"`             // human-readable, no specific regex details
	Line        int      `json:"line,omitempty"`          // 1-based line number where the finding was detected
	MatchText   string   `json:"match_text,omitempty"`    // the matched text snippet (trimmed, max 120 chars)
	Remediation string   `json:"remediation,omitempty"`   // actionable guidance for the skill author
	IssueCode   string   `json:"issue_code,omitempty"`    // unique issue code (e.g. "E006", "W008")
}

// ScanResult is the outcome of a full scanner pipeline run.
type ScanResult struct {
	Pass     bool          `json:"pass"`
	Findings []Finding     `json:"findings"`
	Duration time.Duration `json:"duration"`
	Tier     int           `json:"tier"` // highest tier reached (1, 2, or 3)
	Summary  string        `json:"summary,omitempty"` // human-readable scan summary
}

// GenerateSummary builds a human-readable summary of the scan result with
// per-finding details including file, line, and remediation guidance.
func (r *ScanResult) GenerateSummary() string {
	if len(r.Findings) == 0 {
		r.Summary = "Security scan passed with no findings."
		return r.Summary
	}

	var b strings.Builder
	if r.Pass {
		b.WriteString("Security scan passed with warnings:\n\n")
	} else {
		b.WriteString("Security scan BLOCKED — issues must be fixed before upload:\n\n")
	}

	for i, f := range r.Findings {
		fmt.Fprintf(&b, "  %d. [%s] %s", i+1, f.Severity, f.Description)
		if f.IssueCode != "" {
			fmt.Fprintf(&b, " (%s)", f.IssueCode)
		}
		b.WriteString("\n")
		if f.FilePath != "" {
			fmt.Fprintf(&b, "     File: %s", f.FilePath)
			if f.Line > 0 {
				fmt.Fprintf(&b, ":%d", f.Line)
			}
			b.WriteString("\n")
		}
		if f.MatchText != "" {
			b.WriteString(fmt.Sprintf("     Match: %s\n", f.MatchText))
		}
		if f.Remediation != "" {
			b.WriteString(fmt.Sprintf("     Fix: %s\n", f.Remediation))
		}
		b.WriteString("\n")
	}

	r.Summary = b.String()
	return r.Summary
}
