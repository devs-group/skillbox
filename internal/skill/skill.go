package skill

import (
	"fmt"
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

	s := &Skill{
		Name:         f.Name,
		Version:      f.Version,
		Description:  f.Description,
		Lang:         f.Lang,
		Image:        f.Image,
		Resources:    f.Resources,
		Instructions: strings.TrimSpace(body),
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
	}
	if s.Version == "" {
		errs = append(errs, "version is required")
	} else if !versionRe.MatchString(s.Version) {
		errs = append(errs, fmt.Sprintf("version %q must be semver (MAJOR.MINOR.PATCH)", s.Version))
	}
	if s.Description == "" {
		errs = append(errs, "description is required")
	}
	if s.Lang == "" {
		errs = append(errs, "lang is required")
	} else if !validLangs[s.Lang] {
		errs = append(errs, fmt.Sprintf("lang %q is not supported (use python, node, or bash)", s.Lang))
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
}

// SkillSummary is the compact representation returned by list endpoints.
// It includes description so agents can decide which skill to use.
type SkillSummary struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Lang        string `json:"lang"`
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
		return ""
	}
}
