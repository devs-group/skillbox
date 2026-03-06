package scanner

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	stageNameLLM = "llm_analysis"

	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"

	// maxContentForLLM is the max bytes of skill content sent to the LLM (100KB).
	maxContentForLLM = 100 * 1024

	// canaryLength is the length of the random canary token.
	canaryLength = 8

	// delimiterLength is the length of the random delimiter suffix.
	delimiterLength = 12
)

// LLMConfig holds configuration for the LLM analysis stage.
type LLMConfig struct {
	APIKey        string
	Model         string
	Timeout       time.Duration
	MaxConcurrent int
	// BaseURL overrides the API endpoint (used in tests).
	BaseURL string
}

// llmResponse is the expected JSON structure from the LLM.
type llmResponse struct {
	Canary     string   `json:"canary"`
	Threat     bool     `json:"threat"`
	Confidence float64  `json:"confidence"`
	Reasoning  string   `json:"reasoning"`
	Categories []string `json:"categories"`
}

// anthropicRequest is the request body for the Anthropic Messages API.
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	System      string             `json:"system"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is a minimal parse of the Anthropic Messages API response.
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// llmStage implements the stage interface for Tier 3 LLM deep analysis.
type llmStage struct {
	config    LLMConfig
	client    *http.Client
	semaphore chan struct{}
	logger    *slog.Logger
}

func newLLMStage(cfg LLMConfig, logger *slog.Logger) *llmStage {
	return &llmStage{
		config:    cfg,
		client:    &http.Client{Timeout: cfg.Timeout},
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
		logger:    logger,
	}
}

func (ls *llmStage) name() string {
	return stageNameLLM
}

// run sends flagged content to the LLM for contextual analysis.
// It only runs when there are unresolved FLAG findings from prior tiers.
func (ls *llmStage) run(ctx context.Context, zr *zip.Reader, priorFlags []Finding) ([]Finding, error) {
	if len(priorFlags) == 0 {
		return nil, nil
	}

	// Acquire semaphore slot.
	select {
	case ls.semaphore <- struct{}{}:
		defer func() { <-ls.semaphore }()
	case <-ctx.Done():
		return nil, fmt.Errorf("%s: %w", stageNameLLM, ctx.Err())
	}

	// Collect content to analyze.
	content := ls.collectContent(zr)
	if content == "" {
		return nil, nil
	}

	// Build prompt with hardening.
	canary := randomAlphanumeric(canaryLength)
	delimiter := fmt.Sprintf("===SCAN_%s===", randomAlphanumeric(delimiterLength))

	systemPrompt := ls.buildSystemPrompt(canary)
	userMessage := ls.buildUserMessage(content, priorFlags, delimiter)

	// Call LLM API.
	responseText, err := ls.callAPI(ctx, systemPrompt, userMessage)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", stageNameLLM, err)
	}

	// Parse and validate response.
	findings, err := ls.parseResponse(responseText, canary, priorFlags)
	if err != nil {
		return nil, fmt.Errorf("%s: response validation failed: %w", stageNameLLM, err)
	}

	return findings, nil
}

// collectContent gathers SKILL.md and entrypoint files, truncated to maxContentForLLM.
func (ls *llmStage) collectContent(zr *zip.Reader) string {
	var parts []string
	totalSize := 0

	// Priority: SKILL.md first, then entrypoint files.
	priorityFiles := []string{"SKILL.md", "main.py", "index.js", "run.sh", "main.go", "app.py", "server.js"}
	fileContents := make(map[string]string)

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimPrefix(f.Name, "./")
		for _, pf := range priorityFiles {
			if name == pf || strings.HasSuffix(name, "/"+pf) {
				data, err := readZipFileContent(f)
				if err != nil || isBinaryContent(data) {
					continue
				}
				fileContents[name] = string(data)
			}
		}
	}

	// Add files in priority order.
	for _, pf := range priorityFiles {
		for name, content := range fileContents {
			if name == pf || strings.HasSuffix(name, "/"+pf) {
				remaining := maxContentForLLM - totalSize
				if remaining <= 0 {
					break
				}
				if len(content) > remaining {
					content = content[:remaining]
				}
				parts = append(parts, fmt.Sprintf("--- file: %s ---\n%s", name, content))
				totalSize += len(content)
			}
		}
	}

	return strings.Join(parts, "\n\n")
}

// buildSystemPrompt creates the system message with analysis instructions.
func (ls *llmStage) buildSystemPrompt(canary string) string {
	return fmt.Sprintf(`You are a security analyst reviewing uploaded code for a skill execution platform.
Your task is to analyze the provided code and flag findings for potential security threats.

IMPORTANT: Your response MUST be a single JSON object with this exact schema:
{
  "canary": "%s",
  "threat": <boolean>,
  "confidence": <float 0.0-1.0>,
  "reasoning": "<brief explanation>",
  "categories": ["<category1>", ...]
}

Rules:
1. The "canary" field MUST contain exactly: %s
2. Set "threat" to true if the code poses a genuine security risk
3. Set "threat" to false if the flagged patterns are benign/legitimate usage
4. Valid categories: "malicious_code", "data_exfiltration", "prompt_injection", "obfuscation", "dependency_risk", "sandbox_escape", "legitimate_usage"
5. Be conservative: if genuinely uncertain, set threat=true
6. Output ONLY the JSON object. No markdown, no explanation outside the JSON.`, canary, canary)
}

// buildUserMessage wraps the scanned content with random delimiters.
func (ls *llmStage) buildUserMessage(content string, flags []Finding, delimiter string) string {
	var flagDescs []string
	for _, f := range flags {
		flagDescs = append(flagDescs, fmt.Sprintf("- [%s] %s in %s: %s", f.Severity, f.Category, f.FilePath, f.Description))
	}

	return fmt.Sprintf(`The following code was uploaded as a "skill" (a runnable code package). Prior automated scanning flagged the patterns listed below. Analyze whether these flags represent genuine security threats or legitimate code patterns.

Prior scan flags:
%s

Skill content (delimited):
%s
%s
%s

Respond with the JSON analysis.`, strings.Join(flagDescs, "\n"), delimiter, content, delimiter)
}

// callAPI makes the HTTP request to the Anthropic Messages API.
func (ls *llmStage) callAPI(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	reqBody := anthropicRequest{
		Model:       ls.config.Model,
		MaxTokens:   1024,
		Temperature: 0,
		System:      systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userMessage},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	apiURL := anthropicAPIURL
	if ls.config.BaseURL != "" {
		apiURL = ls.config.BaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", ls.config.APIKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := ls.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("parse API response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("API returned empty content")
	}

	// Extract text from the first content block.
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("API response has no text content block")
}

// parseResponse validates the LLM response and converts it to findings.
func (ls *llmStage) parseResponse(text, expectedCanary string, priorFlags []Finding) ([]Finding, error) {
	// Strip markdown code fences if present.
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var resp llmResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	// Validate canary token — if missing or altered, the content may have hijacked the prompt.
	if resp.Canary != expectedCanary {
		return nil, fmt.Errorf("canary mismatch: expected %q, got %q — possible prompt hijacking", expectedCanary, resp.Canary)
	}

	// Validate confidence range.
	if resp.Confidence < 0 || resp.Confidence > 1 {
		return nil, fmt.Errorf("confidence out of range [0,1]: %f", resp.Confidence)
	}

	ls.logger.Info("LLM analysis complete",
		"threat", resp.Threat,
		"confidence", resp.Confidence,
		"categories", resp.Categories,
		"reasoning", resp.Reasoning,
	)

	if !resp.Threat {
		// LLM says the flags are benign — return no findings (clears the flags).
		return nil, nil
	}

	// LLM confirms threat — convert to BLOCK finding.
	var findings []Finding
	severity := SeverityFlag
	if resp.Confidence >= 0.7 {
		severity = SeverityBlock
	}

	categories := strings.Join(resp.Categories, ", ")
	if categories == "" {
		categories = "llm_detected_threat"
	}

	findings = append(findings, Finding{
		Stage:       stageNameLLM,
		Severity:    severity,
		Category:    "llm_threat",
		FilePath:    "",
		Description: fmt.Sprintf("LLM analysis (confidence: %.2f, categories: [%s]): %s", resp.Confidence, categories, resp.Reasoning),
	})

	return findings, nil
}

// randomAlphanumeric generates a random string of the given length.
func randomAlphanumeric(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
