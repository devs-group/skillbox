package scanner

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPatterns_EmbeddedDefaults(t *testing.T) {
	logger := slog.Default()
	lp, err := loadPatterns("", "", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lp.blockPatterns) == 0 {
		t.Error("expected non-empty block patterns from embedded defaults")
	}
	if len(lp.flagPatterns) == 0 {
		t.Error("expected non-empty flag patterns from embedded defaults")
	}
	if len(lp.blocklistPackages) == 0 {
		t.Error("expected non-empty blocklist packages from embedded defaults")
	}
	if len(lp.popularPackages) == 0 {
		t.Error("expected non-empty popular packages from embedded defaults")
	}

	// Verify specific known entries.
	if !lp.blocklistPackages["colourfool"] {
		t.Error("expected 'colourfool' in blocklist packages")
	}
	if !lp.popularPackages["requests"] {
		t.Error("expected 'requests' in popular packages")
	}
}

func TestLoadPatterns_CustomFile_Merges(t *testing.T) {
	logger := slog.Default()

	// Write a custom patterns file with one extra block pattern.
	customYAML := `version: 1
block_patterns:
  - regex: 'custom_evil_pattern'
    category: custom_block
    description: "custom block pattern"
flag_patterns:
  - regex: 'custom_suspicious'
    category: custom_flag
    description: "custom flag pattern"
blocklist_packages:
  - evil-custom-pkg
popular_packages:
  - my-internal-lib
`
	tmpFile := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(tmpFile, []byte(customYAML), 0644); err != nil {
		t.Fatalf("write custom file: %v", err)
	}

	lp, err := loadPatterns(tmpFile, "", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Custom patterns should be merged on top of defaults.
	foundCustomBlock := false
	for _, p := range lp.blockPatterns {
		if p.category == "custom_block" {
			foundCustomBlock = true
			break
		}
	}
	if !foundCustomBlock {
		t.Error("expected custom block pattern to be merged")
	}

	foundCustomFlag := false
	for _, p := range lp.flagPatterns {
		if p.category == "custom_flag" {
			foundCustomFlag = true
			break
		}
	}
	if !foundCustomFlag {
		t.Error("expected custom flag pattern to be merged")
	}

	if !lp.blocklistPackages["evil-custom-pkg"] {
		t.Error("expected custom blocklist package")
	}
	if !lp.popularPackages["my-internal-lib"] {
		t.Error("expected custom popular package")
	}

	// Defaults should still be present.
	if !lp.blocklistPackages["colourfool"] {
		t.Error("expected default blocklist package 'colourfool' to still be present")
	}
}

func TestLoadPatterns_CustomFile_InvalidYAML(t *testing.T) {
	logger := slog.Default()

	tmpFile := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(tmpFile, []byte("not: [valid: yaml: {{{"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := loadPatterns(tmpFile, "", logger)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadPatterns_CustomFile_InvalidRegex(t *testing.T) {
	logger := slog.Default()

	customYAML := `version: 1
block_patterns:
  - regex: '[invalid'
    category: bad
    description: "bad regex"
`
	tmpFile := filepath.Join(t.TempDir(), "bad_regex.yaml")
	if err := os.WriteFile(tmpFile, []byte(customYAML), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := loadPatterns(tmpFile, "", logger)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestLoadPatterns_CustomFile_NotFound(t *testing.T) {
	logger := slog.Default()

	_, err := loadPatterns("/nonexistent/file.yaml", "", logger)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadPatterns_OSSFFeed(t *testing.T) {
	logger := slog.Default()

	// Create a temp dir with OSV JSON files.
	feedDir := t.TempDir()
	osvJSON := `{
  "id": "MAL-2023-001",
  "affected": [
    {"package": {"name": "evil-ossf-pkg", "ecosystem": "PyPI"}},
    {"package": {"name": "another-bad-pkg", "ecosystem": "npm"}}
  ]
}`
	if err := os.WriteFile(filepath.Join(feedDir, "MAL-2023-001.json"), []byte(osvJSON), 0644); err != nil {
		t.Fatalf("write OSV file: %v", err)
	}

	lp, err := loadPatterns("", feedDir, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !lp.blocklistPackages["evil-ossf-pkg"] {
		t.Error("expected OSSF package 'evil-ossf-pkg' in blocklist")
	}
	if !lp.blocklistPackages["another-bad-pkg"] {
		t.Error("expected OSSF package 'another-bad-pkg' in blocklist")
	}

	// Defaults should still be present.
	if !lp.blocklistPackages["colourfool"] {
		t.Error("expected default blocklist to still be present")
	}
}

func TestLoadPatterns_OSSFFeed_MalformedJSON(t *testing.T) {
	logger := slog.Default()

	feedDir := t.TempDir()
	// Malformed JSON should be skipped (not cause failure).
	if err := os.WriteFile(filepath.Join(feedDir, "bad.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// Valid file alongside.
	osvJSON := `{"id": "MAL-2023-002", "affected": [{"package": {"name": "valid-pkg", "ecosystem": "npm"}}]}`
	if err := os.WriteFile(filepath.Join(feedDir, "good.json"), []byte(osvJSON), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	lp, err := loadPatterns("", feedDir, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !lp.blocklistPackages["valid-pkg"] {
		t.Error("expected valid OSSF package despite malformed sibling")
	}
}

func TestLoadPatterns_OSSFFeed_NonexistentDir(t *testing.T) {
	logger := slog.Default()

	_, err := loadPatterns("", "/nonexistent/dir", logger)
	if err == nil {
		t.Fatal("expected error for nonexistent OSSF dir")
	}
}

func TestParsePatternFile_WrongVersion(t *testing.T) {
	yaml := `version: 99
block_patterns: []
`
	_, err := parsePatternFile([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestMergePatternFiles(t *testing.T) {
	base := &PatternFile{
		Version:       1,
		BlockPatterns: []PatternEntry{{Regex: "a", Category: "a", Description: "a"}},
		BlocklistPackages: []string{"pkg-a"},
	}
	overlay := &PatternFile{
		Version:       1,
		BlockPatterns: []PatternEntry{{Regex: "b", Category: "b", Description: "b"}},
		BlocklistPackages: []string{"pkg-b"},
	}

	merged := mergePatternFiles(base, overlay)
	if len(merged.BlockPatterns) != 2 {
		t.Fatalf("expected 2 block patterns, got %d", len(merged.BlockPatterns))
	}
	if len(merged.BlocklistPackages) != 2 {
		t.Fatalf("expected 2 blocklist packages, got %d", len(merged.BlocklistPackages))
	}
}
