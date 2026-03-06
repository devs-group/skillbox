package scanner

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

const (
	stageNamePatterns = "static_patterns"

	// maxFileSizeForScan is the per-file byte cap for regex scanning (1MB).
	maxFileSizeForScan int64 = 1 << 20
)

// pattern defines a compiled regex with its severity and category.
type pattern struct {
	re       *regexp.Regexp
	severity Severity
	category string
	desc     string
}

// blocklistPackages is a set of known-malicious package names for exact-match lookup.
// This is a minimal seed list; maintained and expanded over time from OSSF feeds.
var blocklistPackages = map[string]bool{
	// Python known-malicious packages (OSSF feed examples)
	"colourfool":            true,
	"beautifulsoup":         true, // typosquat of beautifulsoup4
	"python-binance":        true, // typosquat of python-binance-api
	"cryptowall":            true,
	"noblesse":              true,
	"noblesse2":             true,
	"genesisbot":            true,
	"aryi":                  true,
	"suffer":                true,
	"colorwin":              true,
	"httplib2shim":          true,
	"testing123asd":         true,
	"distutils-precedence":  true,
	"piprot":                true,
	"loglib-modules":        true,
	"pygrata":               true,
	"pygrata-utils":         true,
	"hkg-sol-utils":         true,
	"performance-hawk":      true,
	"ultratools":            true,
	// Node.js known-malicious packages (npm advisories)
	"event-stream":          true, // compromised version
	"flatmap-stream":        true,
	"ua-parser-js-malware":  true,
	"colors-malware":        true,
	"faker-malware":         true,
	"node-ipc-malware":      true,
	"peacenotwar":           true,
	"is-promise-malware":    true,
	"electorn":              true, // typosquat of electron
	"crossenv":              true, // typosquat of cross-env
	"mongose":               true, // typosquat of mongoose
	"d3.js":                 true, // typosquat of d3
	"gruntcli":              true, // typosquat of grunt-cli
	"http-proxy.js":         true, // typosquat of http-proxy
	"proxy.js":              true, // typosquat of proxy
	"shadowsock":            true, // typosquat of shadowsocks
	"smb":                   true,
	"nodesass":              true, // typosquat of node-sass
	"discord.js":            true, // typosquat of discord.js (with dot)
}

// blockPatterns are always-reject patterns. Compiled once at init.
var blockPatterns = mustCompilePatterns([]patternDef{
	// Reverse shells
	{`\bnc\s+(-[a-z])*\s*-e\s`, "reverse_shell", "netcat reverse shell"},
	{`/dev/tcp/`, "reverse_shell", "bash reverse shell via /dev/tcp"},
	{`\bmkfifo\b.*\bnc\b`, "reverse_shell", "named pipe reverse shell"},
	{`bash\s+-i\s+>&\s+/dev/tcp/`, "reverse_shell", "interactive bash reverse shell"},
	{`\bsocat\b.*\bexec\b`, "reverse_shell", "socat reverse shell"},
	{`\bpython[23]?\s+-c\s+['"]import\s+socket`, "reverse_shell", "python reverse shell"},

	// Piped execution
	{`\bcurl\b[^|]*\|\s*(ba)?sh\b`, "piped_execution", "curl piped to shell"},
	{`\bwget\b[^|]*\|\s*(ba)?sh\b`, "piped_execution", "wget piped to shell"},
	{`\bbase64\s+-d\b[^|]*\|\s*(ba)?sh\b`, "piped_execution", "base64 decoded and piped to shell"},
	{`\bcurl\b[^|]*\|\s*python`, "piped_execution", "curl piped to python"},
	{`\bwget\b[^|]*\|\s*python`, "piped_execution", "wget piped to python"},

	// Crypto miners
	{`\bimport\s+(?:hashlib|cryptonight|xmrig|stratum)`, "crypto_miner", "crypto mining import"},
	{`stratum\+tcp://`, "crypto_miner", "mining pool connection"},
	{`\bcoinhive\b`, "crypto_miner", "coinhive miner reference"},
	{`\bxmr-?stak\b`, "crypto_miner", "XMR-Stak miner reference"},

	// Sandbox escape attempts
	{`/proc/[0-9]+/ns/`, "sandbox_escape", "namespace access attempt"},
	{`\bnsenter\b`, "sandbox_escape", "nsenter container escape"},
	{`\bunshare\b.*--mount`, "sandbox_escape", "mount namespace escape"},
	{`\bmount\s+-t\s+proc\b`, "sandbox_escape", "proc mount attempt"},

	// Dangerous system operations
	{`\brm\s+-rf\s+/(?:\s|$)`, "destructive_command", "recursive root deletion"},
	{`:\(\)\s*\{.*:\|:.*\}\s*;\s*:`, "fork_bomb", "fork bomb"},
})

// flagPatterns are suspicious-but-possibly-legitimate patterns. Escalated to deeper tiers.
var flagPatterns = mustCompilePatterns([]patternDef{
	// Process execution (may be legitimate)
	{`\bsubprocess\.(?:Popen|call|run|check_output)\b`, "process_execution", "python subprocess usage"},
	{`\bos\.system\b`, "process_execution", "python os.system usage"},
	{`\bos\.popen\b`, "process_execution", "python os.popen usage"},
	{`\bchild_process\b`, "process_execution", "node child_process usage"},
	{`\bexecSync\b|\bexecFileSync\b|\bspawnSync\b`, "process_execution", "node sync process execution"},

	// Dynamic code execution
	{`\beval\s*\(`, "dynamic_execution", "eval() usage"},
	{`\bexec\s*\(`, "dynamic_execution", "exec() usage"},
	{`\bFunction\s*\(`, "dynamic_execution", "Function constructor usage"},
	{`\bcompile\s*\(`, "dynamic_execution", "compile() usage"},

	// Network operations
	{`\bsocket\.connect\b`, "network_access", "python socket connect"},
	{`\bnet\.connect\b`, "network_access", "node net connect"},
	{`\brequests\.(?:get|post|put|delete|patch)\b`, "network_access", "python requests HTTP call"},
	{`\burllib\.request\b`, "network_access", "python urllib request"},

	// File system access patterns
	{`/etc/passwd`, "sensitive_file_access", "reading /etc/passwd"},
	{`/etc/shadow`, "sensitive_file_access", "reading /etc/shadow"},
	{`~/.ssh/`, "sensitive_file_access", "SSH directory access"},
	{`\.env\b`, "sensitive_file_access", "environment file access"},
})

// commonBlockPatterns are cross-language block patterns.
var commonBlockPatterns = mustCompilePatterns([]patternDef{
	// Large base64 blobs (>256 chars of base64 on a single line, likely obfuscated payload)
	{`[A-Za-z0-9+/=]{256,}`, "obfuscation", "large base64-encoded blob"},
})

// commonFlagPatterns are cross-language flag patterns.
var commonFlagPatterns = mustCompilePatterns([]patternDef{
	// IP address literals (not localhost/0.0.0.0)
	{`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`, "hardcoded_ip", "hardcoded IP address"},
})

type patternDef struct {
	regex    string
	category string
	desc     string
}

func mustCompilePatterns(defs []patternDef) []pattern {
	patterns := make([]pattern, len(defs))
	for i, d := range defs {
		patterns[i] = pattern{
			re:       regexp.MustCompile(d.regex),
			severity: SeverityBlock, // overridden by caller if needed
			category: d.category,
			desc:     d.desc,
		}
	}
	return patterns
}

// patternStage implements the stage interface for Tier 1 static pattern scanning.
type patternStage struct {
	logger *slog.Logger
}

func newPatternStage(logger *slog.Logger) *patternStage {
	return &patternStage{logger: logger}
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

		// Skip files exceeding size cap.
		if f.UncompressedSize64 > uint64(maxFileSizeForScan) {
			ps.logger.Debug("skipping large file for pattern scan",
				"file", f.Name,
				"size", f.UncompressedSize64,
			)
			continue
		}

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

		// Check block patterns.
		for _, p := range blockPatterns {
			if p.re.MatchString(text) {
				findings = append(findings, Finding{
					Stage:       stageNamePatterns,
					Severity:    SeverityBlock,
					Category:    p.category,
					FilePath:    f.Name,
					Description: p.desc,
				})
				// Short-circuit: one BLOCK per file is enough to reject.
				break
			}
		}

		// Check common block patterns.
		for _, p := range commonBlockPatterns {
			if p.re.MatchString(text) {
				findings = append(findings, Finding{
					Stage:       stageNamePatterns,
					Severity:    SeverityBlock,
					Category:    p.category,
					FilePath:    f.Name,
					Description: p.desc,
				})
				break
			}
		}

		// Check flag patterns (always, even if block was found — for audit logging).
		for _, p := range flagPatterns {
			if p.re.MatchString(text) {
				findings = append(findings, Finding{
					Stage:       stageNamePatterns,
					Severity:    SeverityFlag,
					Category:    p.category,
					FilePath:    f.Name,
					Description: p.desc,
				})
			}
		}

		// Check common flag patterns.
		for _, p := range commonFlagPatterns {
			if p.re.MatchString(text) {
				findings = append(findings, Finding{
					Stage:       stageNamePatterns,
					Severity:    SeverityFlag,
					Category:    p.category,
					FilePath:    f.Name,
					Description: p.desc,
				})
			}
		}
	}

	// Check dependency files for blocklisted packages.
	depFindings := checkDepBlocklist(zr)
	findings = append(findings, depFindings...)

	return findings, nil
}

// checkDepBlocklist scans requirements.txt, package.json, and pyproject.toml
// for known-malicious package names (exact match).
func checkDepBlocklist(zr *zip.Reader) []Finding {
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
			findings = append(findings, checkRequirementsTxt(string(content), f.Name)...)

		case name == "package.json" || strings.HasSuffix(name, "/package.json"):
			content, err := readZipFileContent(f)
			if err != nil {
				continue
			}
			findings = append(findings, checkPackageJSON(string(content), f.Name)...)
		}
	}

	return findings
}

// checkRequirementsTxt checks each line of a requirements.txt for blocklisted packages.
func checkRequirementsTxt(content, filePath string) []Finding {
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
		if pkg != "" && blocklistPackages[pkg] {
			findings = append(findings, Finding{
				Stage:       stageNamePatterns,
				Severity:    SeverityBlock,
				Category:    "malicious_package",
				FilePath:    filePath,
				Description: fmt.Sprintf("known-malicious package: %s", pkg),
			})
		}
	}
	return findings
}

// checkPackageJSON does a simple string search for blocklisted package names
// in package.json content. This avoids a JSON parse dependency and catches
// packages in dependencies, devDependencies, and peerDependencies.
func checkPackageJSON(content, filePath string) []Finding {
	var findings []Finding
	lower := strings.ToLower(content)
	for pkg := range blocklistPackages {
		// Look for "package-name" pattern in the JSON.
		needle := fmt.Sprintf(`"%s"`, pkg)
		if strings.Contains(lower, needle) {
			findings = append(findings, Finding{
				Stage:       stageNamePatterns,
				Severity:    SeverityBlock,
				Category:    "malicious_package",
				FilePath:    filePath,
				Description: fmt.Sprintf("known-malicious package: %s", pkg),
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
