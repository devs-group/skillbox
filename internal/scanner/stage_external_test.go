package scanner

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// --- Mock ExternalScanner ---

type mockExternalScanner struct {
	name     string
	findings []Finding
	err      error
}

func (m *mockExternalScanner) ScanFile(_ context.Context, filePath string, _ []byte) ([]Finding, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return findings with the actual filePath populated.
	var result []Finding
	for _, f := range m.findings {
		f.FilePath = filePath
		result = append(result, f)
	}
	return result, nil
}

func (m *mockExternalScanner) Name() string {
	return m.name
}

// --- ExternalStage Tests ---

func TestExternalStage_CleanFiles(t *testing.T) {
	mock := &mockExternalScanner{name: "test", findings: nil, err: nil}
	stage := newExternalStage(mock, testLogger)

	zr := createTestZip(t, map[string]string{
		"SKILL.md": "---\nname: test-skill\nversion: \"1.0\"\nlang: python\n---\nHello",
		"main.py":  "print('hello')",
	})

	findings, err := stage.run(context.Background(), zr, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestExternalStage_MalwareDetected(t *testing.T) {
	mock := &mockExternalScanner{
		name: "test",
		findings: []Finding{{
			Stage:       stageNameExternal,
			Severity:    SeverityBlock,
			Category:    "malware_detected",
			Description: "test virus found",
		}},
	}
	stage := newExternalStage(mock, testLogger)

	zr := createTestZip(t, map[string]string{
		"SKILL.md":  "---\nname: test-skill\nversion: \"1.0\"\nlang: python\n---\nHello",
		"malware.py": "X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*",
	})

	findings, err := stage.run(context.Background(), zr, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 { // One per file (SKILL.md + malware.py)
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if findings[0].Severity != SeverityBlock {
		t.Fatalf("expected BLOCK, got %s", findings[0].Severity)
	}
}

func TestExternalStage_ScannerUnavailable_FailsClosed(t *testing.T) {
	mock := &mockExternalScanner{
		name: "test",
		err:  fmt.Errorf("connection refused"),
	}
	stage := newExternalStage(mock, testLogger)

	zr := createTestZip(t, map[string]string{
		"SKILL.md": "---\nname: test-skill\nversion: \"1.0\"\nlang: python\n---\nHello",
	})

	_, err := stage.run(context.Background(), zr, nil)
	if err == nil {
		t.Fatal("expected error on scanner unavailability")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("expected 'connection refused' in error, got: %v", err)
	}
}

func TestExternalStage_ContextCancellation(t *testing.T) {
	mock := &mockExternalScanner{name: "test"}
	stage := newExternalStage(mock, testLogger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	zr := createTestZip(t, map[string]string{
		"SKILL.md": "---\nname: test-skill\nversion: \"1.0\"\nlang: python\n---\nHello",
	})

	_, err := stage.run(ctx, zr, nil)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

// --- Pipeline Integration with External Scanner ---

func TestPipeline_ExternalScanner_Integration(t *testing.T) {
	mock := &mockExternalScanner{
		name: "clamav",
		findings: []Finding{{
			Stage:       stageNameExternal,
			Severity:    SeverityBlock,
			Category:    "malware_detected",
			Description: "Eicar-Signature",
		}},
	}

	// Create pipeline with external scanner.
	p := New(30*time.Second, testLogger, nil, mock)

	zr := createTestZip(t, map[string]string{
		"SKILL.md": "---\nname: test-skill\nversion: \"1.0\"\nlang: python\n---\nHello",
		// Include a deps file to trigger Tier 2.
		"requirements.txt": "requests==2.28.0\n",
	})

	result, err := p.Scan(context.Background(), zr, testSkill())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Fatal("expected blocked result")
	}
	if result.Tier != 2 {
		t.Fatalf("expected tier 2, got %d", result.Tier)
	}

	// Check metrics recorded the block.
	snap := p.Metrics().Snapshot()
	if snap.BlockedScans != 1 {
		t.Fatalf("expected 1 blocked scan, got %d", snap.BlockedScans)
	}
}

// --- ClamAV INSTREAM Protocol Tests ---

// mockClamdServer creates a test TCP server that speaks the ClamAV INSTREAM protocol.
func mockClamdServer(t *testing.T, response string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock clamd: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // Listener closed.
			}
			go handleClamdConn(conn, response)
		}
	}()

	return ln.Addr().String(), func() { ln.Close() }
}

func handleClamdConn(conn net.Conn, response string) {
	defer conn.Close()

	// Read the "zINSTREAM\0" command byte-by-byte until we see the null terminator.
	var cmd []byte
	oneByte := make([]byte, 1)
	for {
		n, err := conn.Read(oneByte)
		if err != nil || n == 0 {
			return
		}
		cmd = append(cmd, oneByte[0])
		if oneByte[0] == 0 {
			break
		}
	}

	if string(cmd) != "zINSTREAM\x00" {
		return
	}

	// Drain INSTREAM data chunks.
	for {
		var lenBuf [4]byte
		if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
			return
		}
		chunkLen := binary.BigEndian.Uint32(lenBuf[:])
		if chunkLen == 0 {
			break // End of stream.
		}
		if _, err := io.CopyN(io.Discard, conn, int64(chunkLen)); err != nil {
			return
		}
	}

	// Send response and close write side so client sees EOF.
	conn.Write([]byte(response + "\x00"))
}

func TestClamAV_CleanFile(t *testing.T) {
	addr, cleanup := mockClamdServer(t, "stream: OK")
	defer cleanup()

	scanner, err := NewClamAVScanner("tcp://" + addr)
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	findings, err := scanner.ScanFile(context.Background(), "test.py", []byte("print('hello')"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestClamAV_EICAR_Detection(t *testing.T) {
	addr, cleanup := mockClamdServer(t, "stream: Win.Test.EICAR_HDB-1 FOUND")
	defer cleanup()

	scanner, err := NewClamAVScanner("tcp://" + addr)
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	// EICAR test file (standard antivirus test pattern).
	eicar := `X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`

	findings, err := scanner.ScanFile(context.Background(), "eicar.com", []byte(eicar))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != SeverityBlock {
		t.Fatalf("expected BLOCK, got %s", findings[0].Severity)
	}
	if findings[0].Category != "malware_detected" {
		t.Fatalf("expected malware_detected, got %s", findings[0].Category)
	}
	if !strings.Contains(findings[0].Description, "Win.Test.EICAR_HDB-1") {
		t.Fatalf("expected virus name in description, got: %s", findings[0].Description)
	}
}

func TestClamAV_ScanError(t *testing.T) {
	addr, cleanup := mockClamdServer(t, "stream: INSTREAM size limit exceeded ERROR")
	defer cleanup()

	scanner, err := NewClamAVScanner("tcp://" + addr)
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	_, err = scanner.ScanFile(context.Background(), "big.bin", []byte("data"))
	if err == nil {
		t.Fatal("expected error on clamd scan error")
	}
	if !strings.Contains(err.Error(), "ERROR") {
		t.Fatalf("expected ERROR in message, got: %v", err)
	}
}

func TestClamAV_ConnectionRefused(t *testing.T) {
	scanner, err := NewClamAVScanner("tcp://127.0.0.1:1") // Nothing listening on port 1.
	if err != nil {
		t.Fatalf("failed to create scanner: %v", err)
	}

	_, err = scanner.ScanFile(context.Background(), "test.py", []byte("data"))
	if err == nil {
		t.Fatal("expected error on connection refused")
	}
}

func TestClamAV_ParseAddress(t *testing.T) {
	tests := []struct {
		addr    string
		network string
		address string
		wantErr bool
	}{
		{"tcp://127.0.0.1:3310", "tcp", "127.0.0.1:3310", false},
		{"unix:/run/clamav/clamd.ctl", "unix", "/run/clamav/clamd.ctl", false},
		{"127.0.0.1:3310", "tcp", "127.0.0.1:3310", false},
		{"/no/scheme/no/colon", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			net, addr, err := parseClamAVAddress(tt.addr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if net != tt.network || addr != tt.address {
				t.Fatalf("got %s://%s, want %s://%s", net, addr, tt.network, tt.address)
			}
		})
	}
}

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

// --- YARA Tests ---

func TestYARAScanner_MissingBinary(t *testing.T) {
	// Save and restore PATH.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", origPath)

	tmpDir := t.TempDir()
	os.WriteFile(tmpDir+"/test.yar", []byte(`rule test { condition: true }`), 0644)

	_, err := NewYARAScanner(tmpDir)
	if err == nil {
		t.Fatal("expected error when yara binary not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestYARAScanner_MissingRulesDir(t *testing.T) {
	_, err := NewYARAScanner("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing rules directory")
	}
}

func TestYARAScanner_EmptyRulesDir(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := NewYARAScanner(tmpDir)
	if err == nil {
		t.Fatal("expected error for empty rules directory")
	}
	if !strings.Contains(err.Error(), "no .yar") {
		t.Fatalf("expected 'no .yar' error, got: %v", err)
	}
}

// createTestZip and testSkill are defined in stage_deps_test.go and scanner_test.go.
