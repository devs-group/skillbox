package scanner

import "time"

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
	Stage       string   `json:"stage"`       // e.g. "static_patterns", "dependencies"
	Severity    Severity `json:"severity"`    // BLOCK or FLAG
	Category    string   `json:"category"`    // e.g. "reverse_shell", "typosquat"
	FilePath    string   `json:"file_path"`   // relative path within ZIP
	Description string   `json:"description"` // human-readable, no specific regex details
}

// ScanResult is the outcome of a full scanner pipeline run.
type ScanResult struct {
	Pass     bool          `json:"pass"`
	Findings []Finding     `json:"findings"`
	Duration time.Duration `json:"duration"`
	Tier     int           `json:"tier"` // highest tier reached (1, 2, or 3)
}
