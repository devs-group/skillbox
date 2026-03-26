package scanner

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks scan statistics for monitoring and observability.
// All operations are safe for concurrent use.
type Metrics struct {
	TotalScans   atomic.Int64
	PassedScans  atomic.Int64
	BlockedScans atomic.Int64
	FailedScans  atomic.Int64 // infrastructure failures (returned error)

	// Per-tier counts: how many scans reached each tier.
	Tier1Scans atomic.Int64
	Tier2Scans atomic.Int64
	Tier3Scans atomic.Int64

	// Category counts — which threat categories triggered BLOCKs.
	mu         sync.RWMutex
	categories map[string]int64

	// Timing histogram (simple: average + max).
	totalDuration atomic.Int64 // nanoseconds
	maxDuration   atomic.Int64 // nanoseconds
}

// NewMetrics creates a new metrics tracker.
func NewMetrics() *Metrics {
	return &Metrics{
		categories: make(map[string]int64),
	}
}

// RecordScan records a completed scan result.
func (m *Metrics) RecordScan(result *ScanResult) {
	m.TotalScans.Add(1)

	if result.Pass {
		m.PassedScans.Add(1)
	} else {
		m.BlockedScans.Add(1)

		// Track blocked categories.
		m.mu.Lock()
		seen := make(map[string]bool)
		for _, f := range result.Findings {
			if f.Severity == SeverityBlock && !seen[f.Category] {
				m.categories[f.Category]++
				seen[f.Category] = true
			}
		}
		m.mu.Unlock()
	}

	// Track tier usage.
	switch result.Tier {
	case 1:
		m.Tier1Scans.Add(1)
	case 2:
		m.Tier2Scans.Add(1)
	case 3:
		m.Tier3Scans.Add(1)
	}

	// Track timing.
	dur := result.Duration.Nanoseconds()
	m.totalDuration.Add(dur)
	for {
		old := m.maxDuration.Load()
		if dur <= old || m.maxDuration.CompareAndSwap(old, dur) {
			break
		}
	}
}

// RecordFailure records an infrastructure failure (scanner returned error).
func (m *Metrics) RecordFailure() {
	m.TotalScans.Add(1)
	m.FailedScans.Add(1)
}

// Snapshot returns a point-in-time copy of the metrics as a serializable struct.
func (m *Metrics) Snapshot() MetricsSnapshot {
	total := m.TotalScans.Load()

	var avgDurationMs float64
	if total > 0 {
		avgDurationMs = float64(m.totalDuration.Load()) / float64(total) / float64(time.Millisecond)
	}

	m.mu.RLock()
	cats := make(map[string]int64, len(m.categories))
	for k, v := range m.categories {
		cats[k] = v
	}
	m.mu.RUnlock()

	return MetricsSnapshot{
		TotalScans:     total,
		PassedScans:    m.PassedScans.Load(),
		BlockedScans:   m.BlockedScans.Load(),
		FailedScans:    m.FailedScans.Load(),
		Tier1Scans:     m.Tier1Scans.Load(),
		Tier2Scans:     m.Tier2Scans.Load(),
		Tier3Scans:     m.Tier3Scans.Load(),
		AvgDurationMs:  avgDurationMs,
		MaxDurationMs:  float64(m.maxDuration.Load()) / float64(time.Millisecond),
		BlockCategories: cats,
	}
}

// MetricsSnapshot is a serializable point-in-time view of scanner metrics.
type MetricsSnapshot struct {
	TotalScans      int64              `json:"total_scans"`
	PassedScans     int64              `json:"passed_scans"`
	BlockedScans    int64              `json:"blocked_scans"`
	FailedScans     int64              `json:"failed_scans"`
	Tier1Scans      int64              `json:"tier1_scans"`
	Tier2Scans      int64              `json:"tier2_scans"`
	Tier3Scans      int64              `json:"tier3_scans"`
	AvgDurationMs   float64            `json:"avg_duration_ms"`
	MaxDurationMs   float64            `json:"max_duration_ms"`
	BlockCategories map[string]int64   `json:"block_categories"`
}
