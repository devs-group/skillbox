package scanner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// YARAScanner implements ExternalScanner by shelling out to the `yara`
// command-line tool. This avoids CGO dependencies and works with any
// system-installed YARA version.
//
// The scanner discovers all .yar/.yara rule files in the configured
// directory and runs them against each file extracted from the ZIP.
type YARAScanner struct {
	rulesDir string
	rules    []string // resolved paths to .yar/.yara files
}

// NewYARAScanner creates a YARA scanner that loads rules from the given directory.
// It validates that the directory exists and contains at least one rule file.
func NewYARAScanner(rulesDir string) (*YARAScanner, error) {
	info, err := os.Stat(rulesDir)
	if err != nil {
		return nil, fmt.Errorf("YARA rules directory %q: %w", rulesDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("YARA rules path %q is not a directory", rulesDir)
	}

	// Find all rule files.
	var rules []string
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return nil, fmt.Errorf("read YARA rules directory: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".yar" || ext == ".yara" {
			rules = append(rules, filepath.Join(rulesDir, e.Name()))
		}
	}

	if len(rules) == 0 {
		return nil, fmt.Errorf("no .yar or .yara rule files found in %q", rulesDir)
	}

	// Verify yara binary is available.
	if _, err := exec.LookPath("yara"); err != nil {
		return nil, fmt.Errorf("yara binary not found in PATH: %w", err)
	}

	return &YARAScanner{rulesDir: rulesDir, rules: rules}, nil
}

func (y *YARAScanner) Name() string {
	return "yara"
}

// ScanFile writes the file content to a temp file and runs YARA rules against it.
func (y *YARAScanner) ScanFile(ctx context.Context, filePath string, data []byte) ([]Finding, error) {
	// Write content to a temporary file for YARA to scan.
	tmpFile, err := os.CreateTemp("", "yara-scan-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck
	defer tmpFile.Close()           //nolint:errcheck

	if _, err := tmpFile.Write(data); err != nil {
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	var findings []Finding

	// Run each rule file against the temp file.
	for _, rulePath := range y.rules {
		ruleFindings, err := y.runYARA(ctx, rulePath, tmpFile.Name(), filePath)
		if err != nil {
			return nil, err
		}
		findings = append(findings, ruleFindings...)
	}

	return findings, nil
}

// runYARA executes the yara binary and parses its output.
func (y *YARAScanner) runYARA(ctx context.Context, rulePath, targetPath, origPath string) ([]Finding, error) {
	cmd := exec.CommandContext(ctx, "yara", "--no-warnings", rulePath, targetPath)

	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no matches (normal). Other codes are errors.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil, nil // No matches.
			}
			return nil, fmt.Errorf("yara failed (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("run yara: %w", err)
	}

	// Parse output: each line is "RULE_NAME TARGET_FILE"
	var findings []Finding
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Extract rule name (first field).
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ruleName := parts[0]

		findings = append(findings, Finding{
			Stage:       stageNameExternal,
			Severity:    SeverityBlock,
			Category:    "yara_rule_match",
			FilePath:    origPath,
			Description: fmt.Sprintf("YARA rule matched: %s", ruleName),
		})
	}

	return findings, nil
}
