package scanner

import "strings"

// findLineNumber returns the 1-based line number of the first occurrence of
// needle in text. Returns 0 if not found.
func findLineNumber(text, needle string) int {
	idx := strings.Index(text, needle)
	if idx < 0 {
		return 0
	}
	return strings.Count(text[:idx], "\n") + 1
}

// findLineNumberCI returns the 1-based line number of the first case-insensitive
// occurrence of needle in text. Returns 0 if not found.
func findLineNumberCI(text, needle string) int {
	idx := strings.Index(strings.ToLower(text), strings.ToLower(needle))
	if idx < 0 {
		return 0
	}
	return strings.Count(text[:idx], "\n") + 1
}

// snippetAtLine extracts the trimmed content of the given 1-based line number,
// capped at maxLen characters.
func snippetAtLine(text string, line, maxLen int) string {
	if line <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if line > len(lines) {
		return ""
	}
	s := strings.TrimSpace(lines[line-1])
	if len(s) > maxLen {
		s = s[:maxLen] + "..."
	}
	return s
}

// regexMatchLine finds the 1-based line number where a compiled regex first
// matches in the given text.
func regexMatchLine(text string, p pattern) (line int, matchSnippet string) {
	loc := p.re.FindStringIndex(text)
	if loc == nil {
		return 0, ""
	}
	line = strings.Count(text[:loc[0]], "\n") + 1
	return line, snippetAtLine(text, line, 120)
}
