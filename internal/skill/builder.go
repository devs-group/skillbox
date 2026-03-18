package skill

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

const defaultVersion = "1.0.0"

// EntrypointFilename returns the script filename for the given language.
func EntrypointFilename(lang string) string {
	switch lang {
	case LangNode:
		return "main.js"
	case LangBash:
		return "run.sh"
	default:
		return "main.py"
	}
}

// BuildSkillMD generates SKILL.md content from structured fields.
// The instructions (body) are placed immediately after the frontmatter
// so the behavioral summary appears in the first 500 chars visible to LLMs.
func BuildSkillMD(name, description, lang, version, instructions string) string {
	if version == "" {
		version = defaultVersion
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "name: %q\n", name)
	fmt.Fprintf(&sb, "description: %q\n", description)
	if lang != "" {
		fmt.Fprintf(&sb, "lang: %q\n", lang)
	}
	fmt.Fprintf(&sb, "version: %q\n", version)
	sb.WriteString("---\n\n")
	if instructions != "" {
		sb.WriteString(instructions)
		sb.WriteString("\n")
	}
	return sb.String()
}

// PackageSkillZip creates a zip archive containing SKILL.md and the
// entrypoint script. Returns the raw zip bytes.
func PackageSkillZip(skillMDContent, code, lang string) ([]byte, error) {
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("skill code is required")
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	f, err := w.Create("SKILL.md")
	if err != nil {
		return nil, fmt.Errorf("create SKILL.md entry: %w", err)
	}
	if _, err := f.Write([]byte(skillMDContent)); err != nil {
		return nil, fmt.Errorf("write SKILL.md: %w", err)
	}

	ef, err := w.Create(EntrypointFilename(lang))
	if err != nil {
		return nil, fmt.Errorf("create entrypoint entry: %w", err)
	}
	if _, err := ef.Write([]byte(code)); err != nil {
		return nil, fmt.Errorf("write entrypoint: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}
