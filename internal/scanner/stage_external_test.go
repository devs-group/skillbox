package scanner

import (
	"testing"
	"time"
)

// --- Metrics Tests ---

func TestMetrics_RecordScan(t *testing.T) {
	m := NewMetrics()

	// Record a passed scan.
	m.RecordScan(&ScanResult{
		Pass:     true,
		Tier:     1,
		Duration: 5 * time.Millisecond,
	})

	// Record a blocked scan.
	m.RecordScan(&ScanResult{
		Pass: false,
		Tier: 2,
		Findings: []Finding{
			{Severity: SeverityBlock, Category: "reverse_shell"},
			{Severity: SeverityBlock, Category: "reverse_shell"}, // Duplicate — should count once.
			{Severity: SeverityFlag, Category: "network_access"}, // FLAG — not counted in block categories.
		},
		Duration: 50 * time.Millisecond,
	})

	snap := m.Snapshot()

	if snap.TotalScans != 2 {
		t.Fatalf("expected 2 total scans, got %d", snap.TotalScans)
	}
	if snap.PassedScans != 1 {
		t.Fatalf("expected 1 passed scan, got %d", snap.PassedScans)
	}
	if snap.BlockedScans != 1 {
		t.Fatalf("expected 1 blocked scan, got %d", snap.BlockedScans)
	}
	if snap.Tier1Scans != 1 {
		t.Fatalf("expected 1 tier1 scan, got %d", snap.Tier1Scans)
	}
	if snap.Tier2Scans != 1 {
		t.Fatalf("expected 1 tier2 scan, got %d", snap.Tier2Scans)
	}
	if count, ok := snap.BlockCategories["reverse_shell"]; !ok || count != 1 {
		t.Fatalf("expected reverse_shell count 1, got %d", count)
	}
	if _, ok := snap.BlockCategories["network_access"]; ok {
		t.Fatal("FLAG category should not be in block categories")
	}
}

func TestMetrics_RecordFailure(t *testing.T) {
	m := NewMetrics()
	m.RecordFailure()
	m.RecordFailure()

	snap := m.Snapshot()
	if snap.TotalScans != 2 {
		t.Fatalf("expected 2 total, got %d", snap.TotalScans)
	}
	if snap.FailedScans != 2 {
		t.Fatalf("expected 2 failed, got %d", snap.FailedScans)
	}
}

func TestMetrics_Timing(t *testing.T) {
	m := NewMetrics()

	m.RecordScan(&ScanResult{Pass: true, Tier: 1, Duration: 10 * time.Millisecond})
	m.RecordScan(&ScanResult{Pass: true, Tier: 1, Duration: 30 * time.Millisecond})

	snap := m.Snapshot()

	// Average should be ~20ms.
	if snap.AvgDurationMs < 15 || snap.AvgDurationMs > 25 {
		t.Fatalf("expected avg ~20ms, got %.2fms", snap.AvgDurationMs)
	}
	// Max should be ~30ms.
	if snap.MaxDurationMs < 25 || snap.MaxDurationMs > 35 {
		t.Fatalf("expected max ~30ms, got %.2fms", snap.MaxDurationMs)
	}
}
