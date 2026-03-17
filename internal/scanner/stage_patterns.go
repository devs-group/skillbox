package scanner

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const (
	stageNamePatterns = "static_patterns"

	// maxFileSizeForScan is the per-file byte cap for regex scanning (1MB).
	maxFileSizeForScan int64 = 1 << 20
)

// patternStage implements the stage interface for Tier 1 static pattern scanning.
type patternStage struct {
	logger              *slog.Logger
	blockPatterns       []pattern
	flagPatterns        []pattern
	commonBlockPatterns []pattern
	commonFlagPatterns  []pattern
	blocklistPackages   map[string]bool
}

func newPatternStage(logger *slog.Logger, lp *loadedPatterns) *patternStage {
	return &patternStage{
		logger:              logger,
		blockPatterns:       lp.blockPatterns,
		flagPatterns:        lp.flagPatterns,
		commonBlockPatterns: lp.commonBlockPatterns,
		commonFlagPatterns:  lp.commonFlagPatterns,
		blocklistPackages:   lp.blocklistPackages,
	}
}

func (ps *patternStage) name() string {
	return stageNamePatterns
}

func (ps *patternStage) run(ctx context.Context, zr *zip.Reader, _ []Finding) ([]Finding, error) {
	var findings []Finding

	for _, f := range zr.File {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s: %w", stageNamePatterns, ctx.Err())
		}

		// Skip directories.
		if f.FileInfo().IsDir() {
			continue
		}

		// For files exceeding size cap, scan the first 1MB instead of skipping.
		// This prevents evasion by padding malicious files beyond the size limit.
		oversized := f.UncompressedSize64 > uint64(maxFileSizeForScan)

		// Read file content.
		content, err := readZipFileContent(f)
		if err != nil {
			return nil, fmt.Errorf("%s: read %s: %w", stageNamePatterns, f.Name, err)
		}

		// Skip binary files.
		if isBinaryContent(content) {
			continue
		}

		text := string(content)

		// Emit a flag for oversized files so deeper tiers can scrutinize them.
		if oversized {
			findings = append(findings, Finding{
				Stage:       stageNamePatterns,
				Severity:    SeverityFlag,
				Category:    "oversized_file",
				FilePath:    f.Name,
				Description: fmt.Sprintf("file exceeds %d bytes (actual: %d), only first %d bytes scanned", maxFileSizeForScan, f.UncompressedSize64, len(content)),
			})
		}

		// Check block patterns.
		for _, p := range ps.blockPatterns {
			if p.re.MatchString(text) {
				line, snippet := regexMatchLine(text, p)
				findings = append(findings, Finding{
					Stage:       stageNamePatterns,
					Severity:    SeverityBlock,
					Category:    p.category,
					FilePath:    f.Name,
					Description: p.desc,
					Line:        line,
					MatchText:   snippet,
					Remediation: remediationForCategory(p.category),
					IssueCode:   "E006",
				})
				// Short-circuit: one BLOCK per file is enough to reject.
				break
			}
		}

		// Check common block patterns.
		for _, p := range ps.commonBlockPatterns {
			if p.re.MatchString(text) {
				line, snippet := regexMatchLine(text, p)
				findings = append(findings, Finding{
					Stage:       stageNamePatterns,
					Severity:    SeverityBlock,
					Category:    p.category,
					FilePath:    f.Name,
					Description: p.desc,
					Line:        line,
					MatchText:   snippet,
					Remediation: remediationForCategory(p.category),
					IssueCode:   "E006",
				})
				break
			}
		}

		// Check flag patterns (always, even if block was found — for audit logging).
		for _, p := range ps.flagPatterns {
			if p.re.MatchString(text) {
				line, snippet := regexMatchLine(text, p)
				findings = append(findings, Finding{
					Stage:       stageNamePatterns,
					Severity:    SeverityFlag,
					Category:    p.category,
					FilePath:    f.Name,
					Description: p.desc,
					Line:        line,
					MatchText:   snippet,
					Remediation: remediationForCategory(p.category),
				})
			}
		}

		// Check common flag patterns.
		for _, p := range ps.commonFlagPatterns {
			if p.re.MatchString(text) {
				line, snippet := regexMatchLine(text, p)
				findings = append(findings, Finding{
					Stage:       stageNamePatterns,
					Severity:    SeverityFlag,
					Category:    p.category,
					FilePath:    f.Name,
					Description: p.desc,
					Line:        line,
					MatchText:   snippet,
					Remediation: remediationForCategory(p.category),
				})
			}
		}
	}

	// Check dependency files for blocklisted packages.
	depFindings := ps.checkDepBlocklist(zr)
	findings = append(findings, depFindings...)

	return findings, nil
}

// checkDepBlocklist scans requirements.txt, package.json, and pyproject.toml
// for known-malicious package names (exact match).
func (ps *patternStage) checkDepBlocklist(zr *zip.Reader) []Finding {
	var findings []Finding

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		name := strings.TrimPrefix(f.Name, "./")
		switch {
		case name == "requirements.txt" || strings.HasSuffix(name, "/requirements.txt"):
			content, err := readZipFileContent(f)
			if err != nil {
				continue
			}
			findings = append(findings, ps.checkRequirementsTxt(string(content), f.Name)...)

		case name == "package.json" || strings.HasSuffix(name, "/package.json"):
			content, err := readZipFileContent(f)
			if err != nil {
				continue
			}
			findings = append(findings, ps.checkPackageJSON(string(content), f.Name)...)
		}
	}

	return findings
}

// checkRequirementsTxt checks each line of a requirements.txt for blocklisted packages.
func (ps *patternStage) checkRequirementsTxt(content, filePath string) []Finding {
	var findings []Finding
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		// Extract package name: strip version specifiers.
		pkg := line
		for _, sep := range []string{"==", ">=", "<=", "!=", "~=", ">", "<", "["} {
			if idx := strings.Index(pkg, sep); idx >= 0 {
				pkg = pkg[:idx]
			}
		}
		pkg = strings.TrimSpace(strings.ToLower(pkg))
		if pkg != "" && ps.blocklistPackages[pkg] {
			findings = append(findings, Finding{
				Stage:       stageNamePatterns,
				Severity:    SeverityBlock,
				Category:    "malicious_package",
				FilePath:    filePath,
				Description: fmt.Sprintf("known-malicious package: %s", pkg),
				Line:        findLineNumberCI(content, pkg),
				Remediation: fmt.Sprintf("Remove '%s' from your dependencies. This is a known-malicious package. Check if you meant a similarly-named legitimate package.", pkg),
				IssueCode:   "E006",
			})
		}
	}
	return findings
}

// checkPackageJSON does a simple string search for blocklisted package names
// in package.json content. This avoids a JSON parse dependency and catches
// packages in dependencies, devDependencies, and peerDependencies.
func (ps *patternStage) checkPackageJSON(content, filePath string) []Finding {
	var findings []Finding
	lower := strings.ToLower(content)
	for pkg := range ps.blocklistPackages {
		// Look for "package-name" pattern in the JSON.
		needle := fmt.Sprintf(`"%s"`, pkg)
		if strings.Contains(lower, needle) {
			findings = append(findings, Finding{
				Stage:       stageNamePatterns,
				Severity:    SeverityBlock,
				Category:    "malicious_package",
				FilePath:    filePath,
				Description: fmt.Sprintf("known-malicious package: %s", pkg),
				Line:        findLineNumberCI(content, pkg),
				Remediation: fmt.Sprintf("Remove '%s' from your dependencies. This is a known-malicious package.", pkg),
				IssueCode:   "E006",
			})
		}
	}
	return findings
}

// readZipFileContent reads the full content of a zip file entry, capped at maxFileSizeForScan.
func readZipFileContent(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close() //nolint:errcheck

	lr := io.LimitReader(rc, maxFileSizeForScan+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// remediationForCategory returns actionable fix guidance for common finding categories.
func remediationForCategory(category string) string {
	switch category {
	case "reverse_shell":
		return "Remove shell commands that establish remote connections. If you need network access, use high-level HTTP libraries instead."
	case "piped_execution":
		return "Do not pipe downloaded content to a shell. Download files explicitly and validate them before execution."
	case "crypto_miner":
		return "Remove cryptocurrency mining code. Mining is not permitted in uploaded skills."
	case "sandbox_escape":
		return "Remove namespace, mount, or container escape commands. Skills run in a sandboxed environment for security."
	case "destructive_command":
		return "Remove destructive system commands (rm -rf /, fork bombs). Use targeted file operations instead."
	case "fork_bomb":
		return "Remove the fork bomb. This pattern causes denial of service."
	case "obfuscation":
		return "Remove or decode the large base64 blob. Obfuscated payloads cannot be security-reviewed."
	case "malicious_package":
		return "Remove the known-malicious package from your dependencies. Check the package name for typos."
	case "process_execution":
		return "Subprocess usage is flagged for review. Ensure commands are not user-controlled and cannot be injected."
	case "dynamic_execution":
		return "Avoid eval()/exec() with dynamic input. Use safer alternatives for code generation."
	case "network_access":
		return "Network access is flagged for review. Ensure URLs are not user-controlled."
	case "sensitive_file_access":
		return "Avoid accessing system-sensitive files. Use application-specific paths instead."
	case "hardcoded_ip":
		return "Replace hardcoded IP addresses with configurable hostname variables."
	default:
		return ""
	}
}

// isBinaryContent checks if the content is binary using Go's built-in
// content type detection on the first 512 bytes.
func isBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	contentType := http.DetectContentType(sample)
	return !strings.HasPrefix(contentType, "text/")
}
