package skill

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Supported language runtimes.
const (
	LangPython = "python"
	LangNode   = "node"
	LangBash   = "bash"
)

// validLangs is the set of accepted values for the Lang field.
var validLangs = map[string]bool{
	LangPython: true,
	LangNode:   true,
	LangBash:   true,
}

// nameRe validates skill names: alphanumeric, hyphens, underscores, and dots.
// Must start with an alphanumeric character. No path separators or traversal.
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

// versionRe validates semantic-version-like strings: MAJOR.MINOR.PATCH
// with optional pre-release suffix (e.g. 1.0.0, 2.3.1-beta).
var versionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$`)

// Resources describes the CPU and memory constraints for a skill execution.
type Resources struct {
	CPU    string `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`
}

// frontmatter mirrors the YAML structure inside the SKILL.md header.
type frontmatter struct {
	Name        string    `yaml:"name"`
	Version     string    `yaml:"version"`
	Description string    `yaml:"description"`
	Lang        string    `yaml:"lang"`
	Image       string    `yaml:"image,omitempty"`
	Timeout     string    `yaml:"timeout,omitempty"`
	Resources   Resources `yaml:"resources,omitempty"`
	Mode        string    `yaml:"mode,omitempty"`
}

// Skill is the fully parsed and validated representation of a SKILL.md file.
type Skill struct {
	Name         string
	Version      string
	Description  string
	Lang         string // python | node | bash
	Image        string // Docker image; empty means use DefaultImage()
	Timeout      time.Duration
	Resources    Resources
	Instructions string // body text after the frontmatter
	Mode         string // "executable" (default) or "cognitive"
}

// ParseSkillMD extracts the YAML frontmatter (between two "---" lines)
// and the body from a SKILL.md file. It returns the parsed Skill with
// Validate() already called on it, or an error.
func ParseSkillMD(data []byte) (*Skill, error) {
	fm, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var f frontmatter
	if err := yaml.Unmarshal(fm, &f); err != nil {
		return nil, fmt.Errorf("parse frontmatter YAML: %w", err)
	}

	version := f.Version
	if version == "" {
		version = "0.0.0"
	}

	mode := f.Mode
	if mode == "" {
		mode = "executable"
	}

	s := &Skill{
		Name:         f.Name,
		Version:      version,
		Description:  f.Description,
		Lang:         f.Lang,
		Image:        f.Image,
		Resources:    f.Resources,
		Instructions: strings.TrimSpace(body),
		Mode:         mode,
	}

	// Parse timeout if provided.
	if f.Timeout != "" {
		d, err := time.ParseDuration(f.Timeout)
		if err != nil {
			return nil, fmt.Errorf("parse timeout %q: %w", f.Timeout, err)
		}
		s.Timeout = d
	}

	if err := s.Validate(); err != nil {
		return nil, err
	}

	return s, nil
}

// Validate checks that the skill's required fields are present and that
// enum and format constraints are satisfied.
func (s *Skill) Validate() error {
	var errs []string

	if s.Name == "" {
		errs = append(errs, "name is required")
	} else if !nameRe.MatchString(s.Name) {
		errs = append(errs, fmt.Sprintf("name %q contains invalid characters (use alphanumeric, hyphens, underscores, dots; must start with alphanumeric)", s.Name))
	}
	if s.Version != "" && !versionRe.MatchString(s.Version) {
		errs = append(errs, fmt.Sprintf("version %q must be semver (MAJOR.MINOR.PATCH)", s.Version))
	}
	if s.Description == "" {
		errs = append(errs, "description is required")
	}
	if s.Lang != "" && !validLangs[s.Lang] {
		errs = append(errs, fmt.Sprintf("lang %q is not supported (use python, node, or bash)", s.Lang))
	}
	if s.Mode != "" && s.Mode != "executable" && s.Mode != "cognitive" {
		errs = append(errs, fmt.Sprintf("mode %q is not supported (use executable or cognitive)", s.Mode))
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid skill: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SkillMetadata is the subset of Skill returned in list/get API responses.
type SkillMetadata struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	Lang         string    `json:"lang"`
	Image        string    `json:"image,omitempty"`
	Instructions string    `json:"instructions,omitempty"`
	Timeout      string    `json:"timeout,omitempty"`
	Resources    Resources `json:"resources,omitempty"`
	Mode         string    `json:"mode"`
}

// SkillSummary is the compact representation returned by list endpoints.
// It includes description so agents can decide which skill to use.
type SkillSummary struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Lang        string `json:"lang"`
	Mode        string `json:"mode"`
}

// ValidateName checks that a skill name contains only safe characters.
// It rejects path traversal sequences, slashes, and other unsafe characters.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("name %q contains invalid characters", name)
	}
	return nil
}

// ValidateVersion checks that a version string is valid semver or "latest".
func ValidateVersion(version string) error {
	if version == "" || version == "latest" {
		return nil
	}
	if !versionRe.MatchString(version) {
		return fmt.Errorf("version %q must be semver (MAJOR.MINOR.PATCH)", version)
	}
	return nil
}

// InferLangFromEntrypoint maps a file extension to a language runtime.
// Returns an empty string if the extension is not recognized.
func InferLangFromEntrypoint(entrypoint string) string {
	switch filepath.Ext(entrypoint) {
	case ".py":
		return LangPython
	case ".js":
		return LangNode
	case ".sh":
		return LangBash
	default:
		return ""
	}
}

// DefaultImage returns the canonical Docker image for the skill's language.
// If a custom Image is already set on the Skill it is returned as-is.
func (s *Skill) DefaultImage() string {
	if s.Image != "" {
		return s.Image
	}
	switch s.Lang {
	case LangPython:
		return "python:3.12-slim"
	case LangNode:
		return "node:20-slim"
	case LangBash:
		return "bash:5"
	default:
		return "bash:5"
	}
}
