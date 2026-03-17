package scanner

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// pattern defines a compiled regex with its category and description.
type pattern struct {
	re       *regexp.Regexp
	category string
	desc     string
}

//go:embed default_patterns.yaml
var defaultPatternsYAML []byte

// PatternFile is the YAML schema for pattern definition files.
type PatternFile struct {
	Version             int            `yaml:"version"`
	BlockPatterns       []PatternEntry `yaml:"block_patterns"`
	FlagPatterns        []PatternEntry `yaml:"flag_patterns"`
	CommonBlockPatterns []PatternEntry `yaml:"common_block_patterns"`
	CommonFlagPatterns  []PatternEntry `yaml:"common_flag_patterns"`
	BlocklistPackages   []string       `yaml:"blocklist_packages"`
	PopularPackages     []string       `yaml:"popular_packages"`
}

// PatternEntry defines a single regex pattern in the YAML file.
type PatternEntry struct {
	Regex       string `yaml:"regex"`
	Category    string `yaml:"category"`
	Description string `yaml:"description"`
}

// loadedPatterns holds the merged and compiled result of all pattern sources.
type loadedPatterns struct {
	blockPatterns       []pattern
	flagPatterns        []pattern
	commonBlockPatterns []pattern
	commonFlagPatterns  []pattern
	blocklistPackages   map[string]bool
	popularPackages     map[string]bool
}

// loadPatterns loads and merges all pattern sources:
// 1. Embedded defaults (always loaded)
// 2. Custom YAML file (optional, merged on top)
// 3. OSSF feed directory (optional, adds to blocklist)
//
// Patterns are loaded once at startup. Restart the server to pick up changes.
func loadPatterns(customFile, ossfFeedDir string, logger *slog.Logger) (*loadedPatterns, error) {
	// Parse embedded defaults.
	base, err := parsePatternFile(defaultPatternsYAML)
	if err != nil {
		return nil, fmt.Errorf("parse embedded defaults: %w", err)
	}

	// Merge custom file if provided.
	if customFile != "" {
		data, err := os.ReadFile(customFile)
		if err != nil {
			return nil, fmt.Errorf("read custom patterns file %s: %w", customFile, err)
		}
		overlay, err := parsePatternFile(data)
		if err != nil {
			return nil, fmt.Errorf("parse custom patterns file %s: %w", customFile, err)
		}
		base = mergePatternFiles(base, overlay)
		logger.Info("loaded custom scanner patterns", "file", customFile,
			"block_patterns", len(overlay.BlockPatterns),
			"flag_patterns", len(overlay.FlagPatterns),
			"blocklist_packages", len(overlay.BlocklistPackages),
			"popular_packages", len(overlay.PopularPackages),
		)
	}

	// Load OSSF feed if provided.
	if ossfFeedDir != "" {
		pkgs, err := loadOSSFFeed(ossfFeedDir, logger)
		if err != nil {
			return nil, fmt.Errorf("load OSSF feed from %s: %w", ossfFeedDir, err)
		}
		base.BlocklistPackages = append(base.BlocklistPackages, pkgs...)
		logger.Info("loaded OSSF malicious packages feed", "dir", ossfFeedDir, "packages", len(pkgs))
	}

	return compilePatternFile(base)
}

// ParsePatternData parses pattern data from either YAML or JSON format.
// This is the public entry point for user-uploaded custom patterns.
// JSON is a subset of YAML, so the YAML parser handles both.
func ParsePatternData(data []byte) (*PatternFile, error) {
	return parsePatternFile(data)
}

// parsePatternFile parses YAML data into a PatternFile.
func parsePatternFile(data []byte) (*PatternFile, error) {
	var pf PatternFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("unmarshal YAML: %w", err)
	}
	if pf.Version != 1 {
		return nil, fmt.Errorf("unsupported pattern file version: %d (expected 1)", pf.Version)
	}
	return &pf, nil
}

// mergePatternFiles appends overlay entries onto base. Overlay never replaces base.
func mergePatternFiles(base, overlay *PatternFile) *PatternFile {
	return &PatternFile{
		Version:             base.Version,
		BlockPatterns:       append(base.BlockPatterns, overlay.BlockPatterns...),
		FlagPatterns:        append(base.FlagPatterns, overlay.FlagPatterns...),
		CommonBlockPatterns: append(base.CommonBlockPatterns, overlay.CommonBlockPatterns...),
		CommonFlagPatterns:  append(base.CommonFlagPatterns, overlay.CommonFlagPatterns...),
		BlocklistPackages:   append(base.BlocklistPackages, overlay.BlocklistPackages...),
		PopularPackages:     append(base.PopularPackages, overlay.PopularPackages...),
	}
}

// compilePatternFile converts a PatternFile into compiled regex patterns and lookup maps.
func compilePatternFile(pf *PatternFile) (*loadedPatterns, error) {
	compile := func(entries []PatternEntry) ([]pattern, error) {
		patterns := make([]pattern, 0, len(entries))
		for _, e := range entries {
			re, err := regexp.Compile(e.Regex)
			if err != nil {
				return nil, fmt.Errorf("compile regex %q: %w", e.Regex, err)
			}
			patterns = append(patterns, pattern{
				re:       re,
				category: e.Category,
				desc:     e.Description,
			})
		}
		return patterns, nil
	}

	bp, err := compile(pf.BlockPatterns)
	if err != nil {
		return nil, fmt.Errorf("block_patterns: %w", err)
	}
	fp, err := compile(pf.FlagPatterns)
	if err != nil {
		return nil, fmt.Errorf("flag_patterns: %w", err)
	}
	cbp, err := compile(pf.CommonBlockPatterns)
	if err != nil {
		return nil, fmt.Errorf("common_block_patterns: %w", err)
	}
	cfp, err := compile(pf.CommonFlagPatterns)
	if err != nil {
		return nil, fmt.Errorf("common_flag_patterns: %w", err)
	}

	blocklist := make(map[string]bool, len(pf.BlocklistPackages))
	for _, pkg := range pf.BlocklistPackages {
		blocklist[strings.ToLower(strings.TrimSpace(pkg))] = true
	}

	popular := make(map[string]bool, len(pf.PopularPackages))
	for _, pkg := range pf.PopularPackages {
		popular[strings.ToLower(strings.TrimSpace(pkg))] = true
	}

	return &loadedPatterns{
		blockPatterns:       bp,
		flagPatterns:        fp,
		commonBlockPatterns: cbp,
		commonFlagPatterns:  cfp,
		blocklistPackages:   blocklist,
		popularPackages:     popular,
	}, nil
}

// osvRecord is the minimal structure of an OSV JSON record from the OSSF feed.
type osvRecord struct {
	Affected []struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
	} `json:"affected"`
}

// loadOSSFFeed reads OSV JSON files from a directory and extracts package names.
func loadOSSFFeed(dir string, logger *slog.Logger) ([]string, error) {
	var packages []string
	seen := make(map[string]bool)

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("skipping unreadable OSSF file", "path", path, "error", err)
			return nil
		}

		var record osvRecord
		if err := json.Unmarshal(data, &record); err != nil {
			logger.Warn("skipping malformed OSSF file", "path", path, "error", err)
			return nil
		}

		for _, a := range record.Affected {
			name := strings.ToLower(strings.TrimSpace(a.Package.Name))
			if name != "" && !seen[name] {
				seen[name] = true
				packages = append(packages, name)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk OSSF feed dir: %w", err)
	}

	return packages, nil
}
