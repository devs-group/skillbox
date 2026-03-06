package scanner

import (
	"archive/zip"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const stageNamePrompt = "prompt_injection"

// Prompt injection risk thresholds.
const (
	riskThresholdBlock = 0.7
	riskThresholdFlag  = 0.4
)

// promptBlockPatterns are high-confidence prompt injection indicators → BLOCK.
var promptBlockPatterns = []promptPattern{
	// Role hijacking / override instructions
	{regexp.MustCompile(`(?i)\bignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?|rules?)\b`), "prompt_override", "instruction override attempt", 0.9},
	{regexp.MustCompile(`(?i)\byou\s+are\s+now\b.*\b(new|different)\s+(role|persona|mode)\b`), "role_hijacking", "role reassignment attempt", 0.9},
	{regexp.MustCompile(`(?i)\bact\s+as\s+(if\s+you\s+are|a)\b`), "role_hijacking", "role hijacking via act-as", 0.7},
	{regexp.MustCompile(`(?i)\bfrom\s+now\s+on\b.*\byou\s+(will|must|should)\b`), "prompt_override", "behavioral override attempt", 0.8},
	{regexp.MustCompile(`(?i)\bdisregard\b.*\b(safety|guidelines?|rules?|instructions?)\b`), "prompt_override", "safety bypass attempt", 0.95},
	{regexp.MustCompile(`(?i)\bdo\s+not\s+follow\b.*\b(previous|system|original)\b`), "prompt_override", "instruction override attempt", 0.85},

	// Data exfiltration instructions
	{regexp.MustCompile(`(?i)\b(output|print|return|send|exfiltrate)\b.*\b(system\s+prompt|api\s+key|secret|password|credentials?|token)\b`), "data_exfiltration", "credential exfiltration instruction", 0.85},
	{regexp.MustCompile(`(?i)\b(reveal|show|display|leak)\b.*\b(system\s+prompt|hidden|internal)\b`), "data_exfiltration", "system prompt extraction attempt", 0.8},
}

// promptFlagPatterns are suspicious but possibly legitimate → FLAG.
var promptFlagPatterns = []promptPattern{
	// MCP server references
	{regexp.MustCompile(`(?i)\bmcp[_\s-]?server\b`), "mcp_reference", "MCP server reference in skill content", 0.5},
	{regexp.MustCompile(`(?i)\btool[_\s-]?server\b`), "mcp_reference", "tool server reference", 0.4},

	// Suspicious instruction patterns (could be legitimate in cognitive skills)
	{regexp.MustCompile(`(?i)\bsystem\s*:\s*you\s+are\b`), "prompt_override", "system prompt pattern in content", 0.6},
	{regexp.MustCompile(`(?i)\b(always|never)\s+(do|say|output|reveal|share)\b`), "behavioral_constraint", "behavioral override pattern", 0.45},
}

// toolCallPatterns detect fake tool-call injection in SKILL.md.
var toolCallPatterns = []promptPattern{
	{regexp.MustCompile(`<tool_use>`), "tool_call_injection", "fake tool_use block", 0.9},
	{regexp.MustCompile(`<function_call>`), "tool_call_injection", "fake function_call block", 0.9},
	{regexp.MustCompile(`(?i)"?tool_name"?\s*:\s*"`), "tool_call_injection", "tool_name JSON pattern", 0.7},
	{regexp.MustCompile(`(?i)"?tool_input"?\s*:\s*[{\[]`), "tool_call_injection", "tool_input JSON pattern", 0.7},
	{regexp.MustCompile(`<function_results>`), "tool_call_injection", "fake function_results block", 0.85},
}

// delimiterPatterns detect injection of conversation delimiters.
var delimiterPatterns = []promptPattern{
	{regexp.MustCompile(`</system>`), "delimiter_injection", "system message close tag", 0.85},
	{regexp.MustCompile(`<system>`), "delimiter_injection", "system message open tag", 0.8},
	{regexp.MustCompile(`(?m)^Human:\s`), "delimiter_injection", "Human: turn delimiter", 0.75},
	{regexp.MustCompile(`(?m)^Assistant:\s`), "delimiter_injection", "Assistant: turn delimiter", 0.75},
	{regexp.MustCompile(`\[INST\]`), "delimiter_injection", "Llama instruction delimiter", 0.7},
	{regexp.MustCompile(`\[/INST\]`), "delimiter_injection", "Llama instruction close delimiter", 0.7},
	{regexp.MustCompile(`<\|im_start\|>`), "delimiter_injection", "ChatML start delimiter", 0.7},
	{regexp.MustCompile(`<\|im_end\|>`), "delimiter_injection", "ChatML end delimiter", 0.7},
}

type promptPattern struct {
	re    *regexp.Regexp
	cat   string
	desc  string
	score float64
}

// promptStage implements the stage interface for Tier 2 prompt injection scanning.
type promptStage struct {
	logger *slog.Logger
}

func newPromptStage(logger *slog.Logger) *promptStage {
	return &promptStage{logger: logger}
}

func (ps *promptStage) name() string {
	return stageNamePrompt
}

func (ps *promptStage) run(ctx context.Context, zr *zip.Reader, _ []Finding) ([]Finding, error) {
	var findings []Finding

	for _, f := range zr.File {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s: %w", stageNamePrompt, ctx.Err())
		}

		if f.FileInfo().IsDir() {
			continue
		}

		if f.UncompressedSize64 > uint64(maxFileSizeForScan) {
			continue
		}

		content, err := readZipFileContent(f)
		if err != nil {
			return nil, fmt.Errorf("%s: read %s: %w", stageNamePrompt, f.Name, err)
		}

		if isBinaryContent(content) {
			continue
		}

		// NFC normalize before all pattern matching.
		text := norm.NFC.String(string(content))
		name := strings.TrimPrefix(f.Name, "./")
		isSkillMD := name == "SKILL.md" || strings.HasSuffix(name, "/SKILL.md")

		// Check invisible Unicode characters in all files.
		if invisFindings := checkInvisibleUnicode(text, f.Name); len(invisFindings) > 0 {
			findings = append(findings, invisFindings...)
		}

		// Prompt injection patterns are most relevant in SKILL.md (agent instructions),
		// but we also check code files for embedded injection strings.
		findings = append(findings, ps.checkPromptPatterns(text, f.Name, isSkillMD)...)

		// Tool-call and delimiter injection — primarily in SKILL.md but check all.
		if isSkillMD {
			findings = append(findings, ps.checkToolCallInjection(text, f.Name)...)
			findings = append(findings, ps.checkDelimiterInjection(text, f.Name)...)
		}
	}

	return findings, nil
}

// checkPromptPatterns runs prompt injection patterns against text content.
// Uses max-of-components scoring: highest score determines severity.
func (ps *promptStage) checkPromptPatterns(text, filePath string, isSkillMD bool) []Finding {
	var findings []Finding
	maxScore := 0.0
	var maxMatch promptPattern

	// Check block patterns.
	for _, p := range promptBlockPatterns {
		if p.re.MatchString(text) {
			if p.score > maxScore {
				maxScore = p.score
				maxMatch = p
			}
		}
	}

	// Check flag patterns.
	for _, p := range promptFlagPatterns {
		if p.re.MatchString(text) {
			if p.score > maxScore {
				maxScore = p.score
				maxMatch = p
			}
		}
	}

	if maxScore == 0 {
		return nil
	}

	// In SKILL.md, scores are used as-is.
	// In code files, halve the score (these patterns are less suspicious in code).
	effectiveScore := maxScore
	if !isSkillMD {
		effectiveScore *= 0.5
	}

	severity := SeverityFlag
	if effectiveScore >= riskThresholdBlock {
		severity = SeverityBlock
	} else if effectiveScore < riskThresholdFlag {
		return nil // Below threshold — ignore.
	}

	findings = append(findings, Finding{
		Stage:       stageNamePrompt,
		Severity:    severity,
		Category:    maxMatch.cat,
		FilePath:    filePath,
		Description: fmt.Sprintf("%s (score: %.2f)", maxMatch.desc, effectiveScore),
	})

	return findings
}

// checkToolCallInjection detects fake tool-call patterns in content.
func (ps *promptStage) checkToolCallInjection(text, filePath string) []Finding {
	var findings []Finding
	maxScore := 0.0
	var maxMatch promptPattern

	for _, p := range toolCallPatterns {
		if p.re.MatchString(text) {
			if p.score > maxScore {
				maxScore = p.score
				maxMatch = p
			}
		}
	}

	if maxScore == 0 {
		return nil
	}

	severity := SeverityFlag
	if maxScore >= riskThresholdBlock {
		severity = SeverityBlock
	}

	findings = append(findings, Finding{
		Stage:       stageNamePrompt,
		Severity:    severity,
		Category:    maxMatch.cat,
		FilePath:    filePath,
		Description: fmt.Sprintf("%s (score: %.2f)", maxMatch.desc, maxScore),
	})

	return findings
}

// checkDelimiterInjection detects conversation delimiter injection.
func (ps *promptStage) checkDelimiterInjection(text, filePath string) []Finding {
	var findings []Finding
	maxScore := 0.0
	var maxMatch promptPattern

	for _, p := range delimiterPatterns {
		if p.re.MatchString(text) {
			if p.score > maxScore {
				maxScore = p.score
				maxMatch = p
			}
		}
	}

	if maxScore == 0 {
		return nil
	}

	severity := SeverityFlag
	if maxScore >= riskThresholdBlock {
		severity = SeverityBlock
	}

	findings = append(findings, Finding{
		Stage:       stageNamePrompt,
		Severity:    severity,
		Category:    maxMatch.cat,
		FilePath:    filePath,
		Description: fmt.Sprintf("%s (score: %.2f)", maxMatch.desc, maxScore),
	})

	return findings
}

// checkInvisibleUnicode detects invisible or suspicious Unicode characters.
func checkInvisibleUnicode(text, filePath string) []Finding {
	var findings []Finding
	found := false

	for _, r := range text {
		if isInvisibleOrSuspicious(r) {
			found = true
			break
		}
	}

	if found {
		findings = append(findings, Finding{
			Stage:       stageNamePrompt,
			Severity:    SeverityFlag,
			Category:    "invisible_unicode",
			FilePath:    filePath,
			Description: "file contains invisible or suspicious Unicode characters (zero-width, RTL override, private use area)",
		})
	}

	return findings
}

// isInvisibleOrSuspicious returns true for Unicode characters commonly used
// in homoglyph attacks or to hide content.
func isInvisibleOrSuspicious(r rune) bool {
	// Zero-width characters.
	if r == '\u200B' || // zero-width space
		r == '\u200C' || // zero-width non-joiner
		r == '\u200D' || // zero-width joiner
		r == '\uFEFF' || // zero-width no-break space (BOM)
		r == '\u00AD' { // soft hyphen
		return true
	}

	// Bidirectional text control characters.
	if r == '\u200E' || // left-to-right mark
		r == '\u200F' || // right-to-left mark
		r == '\u202A' || // left-to-right embedding
		r == '\u202B' || // right-to-left embedding
		r == '\u202C' || // pop directional formatting
		r == '\u202D' || // left-to-right override
		r == '\u202E' || // right-to-left override
		r == '\u2066' || // left-to-right isolate
		r == '\u2067' || // right-to-left isolate
		r == '\u2068' || // first strong isolate
		r == '\u2069' { // pop directional isolate
		return true
	}

	// Private use area.
	if unicode.Is(unicode.Co, r) {
		return true
	}

	// Tag characters (U+E0000-U+E007F) — used in Unicode steganography.
	if r >= 0xE0000 && r <= 0xE007F {
		return true
	}

	return false
}
