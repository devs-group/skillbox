package skill

import (
	"bytes"
	"fmt"
)

// splitFrontmatter splits a SKILL.md file into the raw YAML frontmatter
// bytes and the remaining body string. The file must begin with a "---"
// line and contain a second "---" line that closes the frontmatter block.
func splitFrontmatter(data []byte) (yamlBlock []byte, body string, err error) {
	const delimiter = "---"

	trimmed := bytes.TrimLeft(data, " \t\r\n")

	if !bytes.HasPrefix(trimmed, []byte(delimiter)) {
		return nil, "", fmt.Errorf("SKILL.md must begin with '---' frontmatter delimiter")
	}

	// Skip past the opening delimiter line.
	rest := trimmed[len(delimiter):]
	// Consume the newline after the opening ---.
	if idx := bytes.IndexByte(rest, '\n'); idx >= 0 {
		rest = rest[idx+1:]
	} else {
		return nil, "", fmt.Errorf("SKILL.md frontmatter has no content after opening delimiter")
	}

	// Find the closing delimiter. We look for "\n---" to avoid matching
	// dashes inside YAML values.
	idx := bytes.Index(rest, []byte("\n"+delimiter))
	if idx < 0 {
		return nil, "", fmt.Errorf("SKILL.md is missing closing '---' frontmatter delimiter")
	}

	yamlBlock = rest[:idx]

	// Skip past the closing delimiter line.
	after := rest[idx+1+len(delimiter):]
	// Consume the newline after the closing ---.
	if nIdx := bytes.IndexByte(after, '\n'); nIdx >= 0 {
		body = string(after[nIdx+1:])
	} else {
		body = string(after)
	}

	return yamlBlock, body, nil
}
