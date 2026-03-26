package scanner

import (
	"archive/zip"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
)

const stageNameDeps = "dependencies"

// depsStage implements the stage interface for Tier 2 dependency deep scanning.
// It checks for:
//   - Typosquatting via Levenshtein distance against popular packages
//   - Homoglyph/mixed-script package names
//   - preinstall/postinstall npm hooks
//   - pyproject.toml dependency parsing
type depsStage struct {
	logger            *slog.Logger
	popularPackages   map[string]bool
	blocklistPackages map[string]bool
}

func newDepsStage(logger *slog.Logger, popularPackages, blocklistPackages map[string]bool) *depsStage {
	return &depsStage{
		logger:            logger,
		popularPackages:   popularPackages,
		blocklistPackages: blocklistPackages,
	}
}

func (ds *depsStage) name() string {
	return stageNameDeps
}

func (ds *depsStage) run(ctx context.Context, zr *zip.Reader, _ []Finding) ([]Finding, error) {
	var findings []Finding

	for _, f := range zr.File {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s: %w", stageNameDeps, ctx.Err())
		}

		if f.FileInfo().IsDir() {
			continue
		}

		name := strings.TrimPrefix(f.Name, "./")
		switch {
		case name == "requirements.txt" || strings.HasSuffix(name, "/requirements.txt"):
			content, err := readZipFileContent(f)
			if err != nil {
				return nil, fmt.Errorf("%s: read %s: %w", stageNameDeps, f.Name, err)
			}
			findings = append(findings, ds.checkRequirementsTyposquat(string(content), f.Name)...)

		case name == "pyproject.toml" || strings.HasSuffix(name, "/pyproject.toml"):
			content, err := readZipFileContent(f)
			if err != nil {
				return nil, fmt.Errorf("%s: read %s: %w", stageNameDeps, f.Name, err)
			}
			findings = append(findings, ds.checkPyprojectToml(string(content), f.Name)...)

		case name == "package.json" || strings.HasSuffix(name, "/package.json"):
			content, err := readZipFileContent(f)
			if err != nil {
				return nil, fmt.Errorf("%s: read %s: %w", stageNameDeps, f.Name, err)
			}
			findings = append(findings, ds.checkPackageJSONHooks(string(content), f.Name)...)
			findings = append(findings, ds.checkPackageJSONTyposquat(string(content), f.Name)...)
		}
	}

	return findings, nil
}

// checkRequirementsTyposquat parses requirements.txt and checks each package
// for typosquatting and homoglyph attacks.
func (ds *depsStage) checkRequirementsTyposquat(content, filePath string) []Finding {
	var findings []Finding
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		pkg := extractPkgName(line)
		if pkg == "" {
			continue
		}

		// Already in blocklist → handled by Tier 1. Skip.
		if ds.blocklistPackages[pkg] {
			continue
		}

		// Already a known popular package → not a typosquat.
		if ds.popularPackages[pkg] {
			continue
		}

		findings = append(findings, ds.checkPackageName(pkg, filePath)...)
	}
	return findings
}

// pyprojectMetadataKeys are keys in [project] and [tool.poetry] sections that
// are NOT dependency names and should be skipped.
var pyprojectMetadataKeys = map[string]bool{
	"name": true, "version": true, "description": true, "readme": true,
	"license": true, "authors": true, "maintainers": true, "keywords": true,
	"classifiers": true, "urls": true, "scripts": true, "gui-scripts": true,
	"entry-points": true, "requires-python": true, "python": true,
	"dependencies": true, "optional-dependencies": true,
}

// checkPyprojectToml does a simple line-by-line extraction of dependency names
// from pyproject.toml. Parses [project.dependencies] and
// [tool.poetry.dependencies] sections without a full TOML parser.
func (ds *depsStage) checkPyprojectToml(content, filePath string) []Finding {
	var findings []Finding
	inDeps := false
	inDepArray := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Detect section headers.
		if strings.HasPrefix(trimmed, "[") {
			lower := strings.ToLower(trimmed)
			inDeps = lower == "[project.dependencies]" ||
				lower == "[tool.poetry.dependencies]"
			inDepArray = false
			continue
		}

		// Inside [project] (not [project.dependencies]), look for the
		// dependencies = [...] inline array.
		// For simplicity, we only handle [project.dependencies] and
		// [tool.poetry.dependencies] sections.

		if !inDeps {
			continue
		}

		// Handle start/end of array.
		if strings.HasPrefix(trimmed, "dependencies") && strings.Contains(trimmed, "[") {
			inDepArray = true
			continue
		}
		if trimmed == "]" {
			inDepArray = false
			continue
		}

		if inDepArray {
			// Array items: "requests>=2.0",
			item := strings.Trim(trimmed, `"' ,`)
			pkg := extractPkgName(item)
			if pkg != "" && !ds.blocklistPackages[pkg] && !ds.popularPackages[pkg] {
				findings = append(findings, ds.checkPackageName(pkg, filePath)...)
			}
			continue
		}

		// For poetry style: requests = "^2.28"
		if idx := strings.Index(trimmed, "="); idx > 0 {
			pkg := strings.TrimSpace(trimmed[:idx])
			pkg = strings.Trim(pkg, `"'`)
			pkg = strings.ToLower(pkg)
			if pyprojectMetadataKeys[pkg] || pkg == "" {
				continue
			}
			if !ds.blocklistPackages[pkg] && !ds.popularPackages[pkg] {
				findings = append(findings, ds.checkPackageName(pkg, filePath)...)
			}
			continue
		}
	}

	return findings
}

// checkPackageJSONHooks checks for preinstall/postinstall scripts in package.json.
// These run before sandbox network-deny, making them dangerous.
func (ds *depsStage) checkPackageJSONHooks(content, filePath string) []Finding {
	var findings []Finding
	lower := strings.ToLower(content)

	hookPatterns := []string{
		`"preinstall"`,
		`"postinstall"`,
		`"preuninstall"`,
		`"postuninstall"`,
	}

	for _, hook := range hookPatterns {
		if strings.Contains(lower, hook) {
			findings = append(findings, Finding{
				Stage:       stageNameDeps,
				Severity:    SeverityBlock,
				Category:    "install_hook",
				FilePath:    filePath,
				Description: fmt.Sprintf("npm lifecycle hook detected: %s", strings.Trim(hook, `"`)),
				Line:        findLineNumberCI(content, hook),
				Remediation: fmt.Sprintf("Remove the %s script from package.json. Lifecycle hooks run before sandbox restrictions are applied, making them a security risk.", strings.Trim(hook, `"`)),
				IssueCode:   "E006",
			})
		}
	}

	return findings
}

// checkPackageJSONTyposquat extracts dependency names from package.json and
// checks for typosquatting. Only parses dependencies/devDependencies/peerDependencies.
func (ds *depsStage) checkPackageJSONTyposquat(content, filePath string) []Finding {
	var findings []Finding

	// Track brace depth to correctly handle nested objects.
	inDeps := false
	braceDepth := 0

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Detect dependency section start.
		if !inDeps {
			if (strings.Contains(trimmed, `"dependencies"`) ||
				strings.Contains(trimmed, `"devDependencies"`) ||
				strings.Contains(trimmed, `"peerDependencies"`)) &&
				strings.Contains(trimmed, "{") {
				inDeps = true
				braceDepth = 1
				continue
			}
			// Handle case where { is on the next line.
			if strings.Contains(trimmed, `"dependencies"`) ||
				strings.Contains(trimmed, `"devDependencies"`) ||
				strings.Contains(trimmed, `"peerDependencies"`) {
				inDeps = true
				braceDepth = 0
				continue
			}
			continue
		}

		// Track brace depth.
		for _, ch := range trimmed {
			if ch == '{' {
				braceDepth++
			}
			if ch == '}' {
				braceDepth--
			}
		}
		if braceDepth <= 0 {
			inDeps = false
			continue
		}

		// Extract package name from "pkg-name": "version"
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		pkg := strings.Trim(strings.TrimSpace(parts[0]), `",`)
		pkg = strings.ToLower(pkg)

		if pkg == "" || ds.blocklistPackages[pkg] || ds.popularPackages[pkg] {
			continue
		}

		findings = append(findings, ds.checkPackageName(pkg, filePath)...)
	}

	return findings
}

// checkPackageName checks a single package name for typosquatting and homoglyphs.
func (ds *depsStage) checkPackageName(pkg, filePath string) []Finding {
	var findings []Finding

	// Check homoglyphs first (mixed script).
	if hasMixedScript(pkg) {
		findings = append(findings, Finding{
			Stage:       stageNameDeps,
			Severity:    SeverityBlock,
			Category:    "homoglyph_package",
			FilePath:    filePath,
			Description: fmt.Sprintf("mixed-script package name (possible homoglyph attack): %s", pkg),
			Remediation: fmt.Sprintf("The package name '%s' contains characters from multiple scripts (e.g., Latin mixed with Cyrillic). This is a homoglyph attack. Use the correct ASCII package name.", pkg),
			IssueCode:   "E006",
		})
		return findings // No need to also check Levenshtein.
	}

	// Check Levenshtein distance against popular packages.
	// Find the closest match (minimum distance) to avoid non-deterministic
	// results from map iteration order.
	bestDist := 3 // Only care about distance 1 or 2.
	bestPopular := ""

	for popular := range ds.popularPackages {
		dist := levenshtein(pkg, popular)
		if dist == 0 {
			return nil // Exact match — not a typosquat.
		}
		if dist < bestDist {
			bestDist = dist
			bestPopular = popular
		}
	}

	switch bestDist {
	case 1:
		findings = append(findings, Finding{
			Stage:       stageNameDeps,
			Severity:    SeverityBlock,
			Category:    "typosquat_package",
			FilePath:    filePath,
			Description: fmt.Sprintf("possible typosquat of %q (distance 1): %s", bestPopular, pkg),
			Remediation: fmt.Sprintf("Did you mean '%s'? The package '%s' is very similar to a popular package and may be a typosquatting attack. Fix the package name.", bestPopular, pkg),
			IssueCode:   "E006",
		})
	case 2:
		findings = append(findings, Finding{
			Stage:       stageNameDeps,
			Severity:    SeverityFlag,
			Category:    "typosquat_package",
			FilePath:    filePath,
			Description: fmt.Sprintf("possible typosquat of %q (distance 2): %s", bestPopular, pkg),
			Remediation: fmt.Sprintf("Verify the package name '%s'. It resembles the popular package '%s'. If this is intentional, no action is needed.", pkg, bestPopular),
		})
	}

	return findings
}

// extractPkgName extracts the package name from a requirements.txt line,
// stripping version specifiers and extras.
func extractPkgName(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	for _, sep := range []string{"==", ">=", "<=", "!=", "~=", ">", "<", "[", ";", "@"} {
		if idx := strings.Index(line, sep); idx >= 0 {
			line = line[:idx]
		}
	}
	return strings.TrimSpace(strings.ToLower(line))
}

// hasMixedScript returns true if a string contains characters from multiple
// Unicode scripts (e.g., Latin + Cyrillic), which indicates a homoglyph attack.
func hasMixedScript(s string) bool {
	hasLatin := false
	hasCyrillic := false
	hasGreek := false

	for _, r := range s {
		if r == '-' || r == '_' || r == '.' || r == '@' || r == '/' {
			continue // Skip separators.
		}
		if unicode.Is(unicode.Latin, r) {
			hasLatin = true
		}
		if unicode.Is(unicode.Cyrillic, r) {
			hasCyrillic = true
		}
		if unicode.Is(unicode.Greek, r) {
			hasGreek = true
		}
	}

	scripts := 0
	if hasLatin {
		scripts++
	}
	if hasCyrillic {
		scripts++
	}
	if hasGreek {
		scripts++
	}
	return scripts > 1
}

// levenshtein computes the Levenshtein edit distance between two strings.
// Uses the standard dynamic programming approach with O(min(m,n)) space.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Ensure a is the shorter string for space optimization.
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}

	prev := make([]int, la+1)
	curr := make([]int, la+1)

	for i := 0; i <= la; i++ {
		prev[i] = i
	}

	for j := 1; j <= lb; j++ {
		curr[0] = j
		for i := 1; i <= la; i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min(
				prev[i]+1,      // deletion
				curr[i-1]+1,    // insertion
				prev[i-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[la]
}
