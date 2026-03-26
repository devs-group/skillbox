package scanner

import (
	"archive/zip"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

const stageNameSecurity = "security_analysis"

// securityStage implements the stage interface for Tier 2 security analysis.
// It detects:
//   - Hardcoded secrets (API keys, tokens, private keys)
//   - Suspicious URLs (executable downloads, URL shorteners, untrusted hosts)
//   - Credential exposure (skills requiring agents to output secrets)
//   - Financial execution capabilities
//   - Runtime external dependencies (rug pull risk)
//   - System service modification
type securityStage struct {
	logger *slog.Logger
}

func newSecurityStage(logger *slog.Logger) *securityStage {
	return &securityStage{logger: logger}
}

func (ss *securityStage) name() string {
	return stageNameSecurity
}

func (ss *securityStage) run(ctx context.Context, zr *zip.Reader, _ []Finding) ([]Finding, error) {
	var findings []Finding

	for _, f := range zr.File {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s: %w", stageNameSecurity, ctx.Err())
		}

		if f.FileInfo().IsDir() {
			continue
		}

		if f.UncompressedSize64 > uint64(maxFileSizeForScan) {
			continue
		}

		content, err := readZipFileContent(f)
		if err != nil {
			return nil, fmt.Errorf("%s: read %s: %w", stageNameSecurity, f.Name, err)
		}

		if isBinaryContent(content) {
			continue
		}

		text := string(content)
		name := strings.TrimPrefix(f.Name, "./")
		isSkillMD := name == "SKILL.md" || strings.HasSuffix(name, "/SKILL.md")

		findings = append(findings, checkHardcodedSecrets(text, f.Name)...)
		findings = append(findings, checkSuspiciousURLs(text, f.Name)...)
		findings = append(findings, checkRuntimeExternalDeps(text, f.Name, isSkillMD)...)
		findings = append(findings, checkSystemServiceMod(text, f.Name, isSkillMD)...)

		if isSkillMD {
			findings = append(findings, checkCredentialExposure(text, f.Name)...)
			findings = append(findings, checkFinancialExecution(text, f.Name)...)
		}
	}

	return findings, nil
}

// --- W008: Hardcoded Secrets ---

var secretPatterns = []struct {
	re          *regexp.Regexp
	desc        string
	remediation string
}{
	// AWS
	{regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9])AKIA[0-9A-Z]{16}(?:[^a-zA-Z0-9]|$)`), "AWS access key ID detected", "Remove the AWS access key and use environment variables or a secrets manager instead."},
	// Generic API key assignments
	{regexp.MustCompile(`(?i)(?:api[_-]?key|apikey)\s*[:=]\s*['"][a-zA-Z0-9_\-]{20,}['"]`), "API key assignment detected", "Remove the hardcoded API key. Use environment variables (e.g., os.environ['API_KEY']) or a .env file excluded from the upload."},
	// Generic secret/token assignments
	{regexp.MustCompile(`(?i)(?:secret|token|password|passwd|pwd)\s*[:=]\s*['"][a-zA-Z0-9_\-/+=]{8,}['"]`), "hardcoded secret or token assignment", "Remove the hardcoded secret. Use environment variables or a secrets manager."},
	// GitHub personal access tokens
	{regexp.MustCompile(`(?:^|[^a-zA-Z0-9])ghp_[a-zA-Z0-9]{36}(?:[^a-zA-Z0-9]|$)`), "GitHub personal access token detected", "Revoke this token immediately at github.com/settings/tokens and use environment variables instead."},
	{regexp.MustCompile(`(?:^|[^a-zA-Z0-9])gho_[a-zA-Z0-9]{36}(?:[^a-zA-Z0-9]|$)`), "GitHub OAuth token detected", "Revoke this token and use environment variables."},
	{regexp.MustCompile(`(?:^|[^a-zA-Z0-9])ghs_[a-zA-Z0-9]{36}(?:[^a-zA-Z0-9]|$)`), "GitHub app installation token detected", "Remove this token; it should never be in source code."},
	// Slack tokens
	{regexp.MustCompile(`xox[bpors]-[a-zA-Z0-9-]{10,}`), "Slack token detected", "Revoke the Slack token and use environment variables."},
	// Stripe keys
	{regexp.MustCompile(`(?:^|[^a-zA-Z0-9])(?:sk|pk)_(?:live|test)_[a-zA-Z0-9]{20,}`), "Stripe API key detected", "Rotate this Stripe key at dashboard.stripe.com and use environment variables."},
	// Private keys (PEM format)
	{regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`), "private key embedded in file", "Remove the private key from source code. Load it from a file path specified via environment variable."},
	// OpenAI keys
	{regexp.MustCompile(`(?:^|[^a-zA-Z0-9])sk-[a-zA-Z0-9]{32,}`), "OpenAI API key detected", "Remove the key and set OPENAI_API_KEY as an environment variable."},
	// Google API keys
	{regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`), "Google API key detected", "Restrict or revoke this key in the Google Cloud Console and use environment variables."},
	// Anthropic keys
	{regexp.MustCompile(`(?:^|[^a-zA-Z0-9])sk-ant-[a-zA-Z0-9_-]{20,}`), "Anthropic API key detected", "Remove the key and set ANTHROPIC_API_KEY as an environment variable."},
	// Generic bearer tokens in strings
	{regexp.MustCompile(`(?i)['"]Bearer\s+[a-zA-Z0-9_\-.]{20,}['"]`), "hardcoded Bearer token", "Remove the Bearer token and load it from an environment variable at runtime."},
	// Database connection strings with passwords
	{regexp.MustCompile(`(?i)(?:postgres|mysql|mongodb|redis)://[^:]+:[^@\s]{8,}@`), "database connection string with embedded password", "Use environment variables for database credentials (e.g., DATABASE_URL)."},
}

func checkHardcodedSecrets(text, filePath string) []Finding {
	var findings []Finding
	for _, sp := range secretPatterns {
		if sp.re.MatchString(text) {
			loc := sp.re.FindStringIndex(text)
			line := 0
			snippet := ""
			if loc != nil {
				line = strings.Count(text[:loc[0]], "\n") + 1
				snippet = snippetAtLine(text, line, 120)
			}
			findings = append(findings, Finding{
				Stage:       stageNameSecurity,
				Severity:    SeverityBlock,
				Category:    "hardcoded_secret",
				FilePath:    filePath,
				Description: sp.desc,
				Line:        line,
				MatchText:   snippet,
				Remediation: sp.remediation,
				IssueCode:   "W008",
			})
			// Report first secret found per file — scanning more may leak secret values in output.
			return findings
		}
	}
	return findings
}

// --- E005: Suspicious URLs ---

var suspiciousURLPatterns = []struct {
	re          *regexp.Regexp
	desc        string
	severity    Severity
	remediation string
}{
	// Direct executable downloads
	{regexp.MustCompile(`(?i)https?://[^\s'"]+\.(?:exe|msi|bat|cmd|ps1|sh|bin|elf|dmg|pkg|deb|rpm|appimage)(?:\s|['"]|$)`), "URL pointing to executable download", SeverityBlock, "Remove direct executable URLs. Use package managers (pip, npm) to install dependencies instead."},
	// URL shorteners (hide true destination)
	{regexp.MustCompile(`(?i)https?://(?:bit\.ly|tinyurl\.com|t\.co|goo\.gl|is\.gd|buff\.ly|ow\.ly|rebrand\.ly|shorturl\.at|cutt\.ly|tiny\.cc)/[^\s'"]+`), "URL shortener detected (destination cannot be verified)", SeverityBlock, "Replace shortened URLs with their full destination URLs so they can be verified."},
	// Raw GitHub user content (can change without notice)
	{regexp.MustCompile(`(?i)https?://raw\.githubusercontent\.com/[^\s'"]+`), "raw GitHub content URL (content can change without notice)", SeverityFlag, "Pin to a specific commit SHA instead of a branch name, or vendor the file locally."},
	// Pastebin / temp file hosting
	{regexp.MustCompile(`(?i)https?://(?:pastebin\.com|paste\.ee|hastebin\.com|ghostbin\.com|rentry\.co|dpaste\.org|termbin\.com)/[^\s'"]+`), "paste/temp hosting URL (content is ephemeral and unverifiable)", SeverityBlock, "Include the content directly in the skill rather than referencing external paste services."},
	// Personal file hosting
	{regexp.MustCompile(`(?i)https?://(?:transfer\.sh|file\.io|0x0\.st|catbox\.moe|litterbox\.catbox\.moe|uguu\.se|tmp\.ninja)/[^\s'"]+`), "temporary file hosting URL", SeverityBlock, "Host files in a permanent, verifiable location (e.g., a versioned repository) or vendor them locally."},
	// IP-based URLs (non-localhost)
	{regexp.MustCompile(`https?://(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(?::\d+)?/`), "URL using raw IP address", SeverityFlag, "Use a proper domain name instead of a raw IP address for external resources."},
}

func checkSuspiciousURLs(text, filePath string) []Finding {
	var findings []Finding
	for _, sp := range suspiciousURLPatterns {
		if sp.re.MatchString(text) {
			loc := sp.re.FindStringIndex(text)
			line := 0
			snippet := ""
			if loc != nil {
				line = strings.Count(text[:loc[0]], "\n") + 1
				snippet = snippetAtLine(text, line, 120)
			}
			findings = append(findings, Finding{
				Stage:       stageNameSecurity,
				Severity:    sp.severity,
				Category:    "suspicious_url",
				FilePath:    filePath,
				Description: sp.desc,
				Line:        line,
				MatchText:   snippet,
				Remediation: sp.remediation,
				IssueCode:   "E005",
			})
		}
	}
	return findings
}

// --- W007: Credential Exposure in Output ---

var credentialExposurePatterns = []struct {
	re          *regexp.Regexp
	desc        string
	remediation string
}{
	{regexp.MustCompile(`(?i)(?:output|print|display|show|return|echo|log)\b.*\b(?:api[_-]?key|secret|token|password|credentials?)\b`), "skill instructs agent to output credentials", "Never instruct the agent to output, print, or display secret values. Use masked references or environment variable names instead."},
	{regexp.MustCompile(`(?i)(?:include|embed|insert|add)\b.*\b(?:api[_-]?key|secret|token|password)\b.*\b(?:in|to|into)\b.*\b(?:output|response|message|reply)\b`), "skill instructs embedding secrets in output", "Avoid embedding secrets in agent responses. Reference environment variable names (e.g., $API_KEY) rather than actual values."},
	{regexp.MustCompile(`(?i)(?:send|post|put|submit)\b.*\b(?:api[_-]?key|secret|token|password|credentials?)\b.*\b(?:to|via|through)\b`), "skill instructs sending credentials externally", "Never instruct the agent to send credentials to external services. Use server-side secret injection."},
}

func checkCredentialExposure(text, filePath string) []Finding {
	var findings []Finding
	for _, p := range credentialExposurePatterns {
		if p.re.MatchString(text) {
			loc := p.re.FindStringIndex(text)
			line := 0
			snippet := ""
			if loc != nil {
				line = strings.Count(text[:loc[0]], "\n") + 1
				snippet = snippetAtLine(text, line, 120)
			}
			findings = append(findings, Finding{
				Stage:       stageNameSecurity,
				Severity:    SeverityFlag,
				Category:    "credential_exposure",
				FilePath:    filePath,
				Description: p.desc,
				Line:        line,
				MatchText:   snippet,
				Remediation: p.remediation,
				IssueCode:   "W007",
			})
		}
	}
	return findings
}

// --- W009: Financial Execution ---

var financialPatterns = []struct {
	re          *regexp.Regexp
	desc        string
	remediation string
}{
	{regexp.MustCompile(`(?i)\b(?:stripe|paypal|braintree|adyen|square)\.(?:charges?|payments?|transactions?|transfers?)\b`), "direct financial API integration detected", "Financial operations should require explicit user confirmation for each transaction. Add a human-in-the-loop approval step."},
	{regexp.MustCompile(`(?i)\b(?:send|transfer|withdraw|deposit)\b.*\b(?:payment|money|funds|bitcoin|ethereum|crypto|btc|eth|usdt)\b`), "financial transaction instruction detected", "Skills that handle money transfers must implement explicit user approval for every transaction."},
	{regexp.MustCompile(`(?i)\b(?:web3|ethers|solana|anchor)\b.*\b(?:send|transfer|sign|approve)(?:Transaction|Transfer)?\b`), "blockchain transaction capability detected", "Blockchain transactions are irreversible. Require explicit user confirmation and display transaction details before signing."},
	{regexp.MustCompile(`(?i)\b(?:place|execute|submit)\b.*\b(?:order|trade|buy|sell)\b.*\b(?:stock|share|option|future|forex)\b`), "financial trading instruction detected", "Trading operations should require explicit user confirmation and display order details before execution."},
}

func checkFinancialExecution(text, filePath string) []Finding {
	var findings []Finding
	for _, p := range financialPatterns {
		if p.re.MatchString(text) {
			loc := p.re.FindStringIndex(text)
			line := 0
			snippet := ""
			if loc != nil {
				line = strings.Count(text[:loc[0]], "\n") + 1
				snippet = snippetAtLine(text, line, 120)
			}
			findings = append(findings, Finding{
				Stage:       stageNameSecurity,
				Severity:    SeverityFlag,
				Category:    "financial_execution",
				FilePath:    filePath,
				Description: p.desc,
				Line:        line,
				MatchText:   snippet,
				Remediation: p.remediation,
				IssueCode:   "W009",
			})
		}
	}
	return findings
}

// --- W012: Runtime External Dependencies / Rug Pull ---

var runtimeDepPatterns = []struct {
	re          *regexp.Regexp
	desc        string
	severity    Severity
	remediation string
}{
	// Skills that fetch instructions at runtime
	{regexp.MustCompile(`(?i)(?:fetch|download|load|import|source)\b.*\b(?:instructions?|config|prompt|rules?|script)\b.*\b(?:from|at|via)\b.*\bhttps?://`), "skill fetches instructions from external URL at runtime", SeverityBlock, "Vendor all instructions locally. External instruction fetching enables remote behavior changes without skill updates (rug pull risk)."},
	{regexp.MustCompile(`(?i)(?:curl|wget|fetch|requests\.get|axios\.get|http\.get)\b.*\b(?:\.md|\.txt|\.yaml|\.yml|\.json|\.toml)\b`), "skill downloads configuration/instruction files at runtime", SeverityFlag, "Include configuration files directly in the skill package rather than downloading them at runtime."},
	// Auto-update mechanisms
	{regexp.MustCompile(`(?i)(?:auto[_-]?update|self[_-]?update|update[_-]?check)\b`), "auto-update mechanism detected", SeverityFlag, "Auto-update mechanisms bypass version pinning and security review. Users should update skills explicitly."},
	// Dynamic code loading from URLs
	{regexp.MustCompile(`(?i)(?:eval|exec|execfile|importlib|__import__)\s*\(.*(?:requests|urllib|fetch|http|download)`), "dynamic code loading from remote source", SeverityBlock, "Never load and execute code from remote sources at runtime. Vendor all dependencies locally."},
}

func checkRuntimeExternalDeps(text, filePath string, isSkillMD bool) []Finding {
	var findings []Finding
	for _, p := range runtimeDepPatterns {
		if p.re.MatchString(text) {
			loc := p.re.FindStringIndex(text)
			line := 0
			snippet := ""
			if loc != nil {
				line = strings.Count(text[:loc[0]], "\n") + 1
				snippet = snippetAtLine(text, line, 120)
			}
			sev := p.severity
			// In SKILL.md, runtime fetching of instructions is more dangerous.
			if isSkillMD && sev == SeverityFlag {
				sev = SeverityBlock
			}
			findings = append(findings, Finding{
				Stage:       stageNameSecurity,
				Severity:    sev,
				Category:    "runtime_dependency",
				FilePath:    filePath,
				Description: p.desc,
				Line:        line,
				MatchText:   snippet,
				Remediation: p.remediation,
				IssueCode:   "W012",
			})
		}
	}
	return findings
}

// --- W013: System Service Modification ---

var systemServicePatterns = []struct {
	re          *regexp.Regexp
	desc        string
	severity    Severity
	remediation string
}{
	{regexp.MustCompile(`(?i)\bsudo\b`), "sudo usage detected", SeverityFlag, "Avoid requiring elevated privileges. If necessary, document exactly which commands need sudo and why."},
	{regexp.MustCompile(`(?i)\bsystemctl\s+(?:enable|disable|start|stop|restart|mask)\b`), "systemd service modification", SeverityBlock, "Skills should not modify system services. Remove systemctl commands or document why they are necessary."},
	{regexp.MustCompile(`(?i)\blaunchctl\s+(?:load|unload|enable|disable)\b`), "macOS launch daemon modification", SeverityBlock, "Skills should not modify launch daemons. Remove launchctl commands."},
	{regexp.MustCompile(`(?i)(?:/etc/(?:crontab|cron\.d/|init\.d/|systemd/)|\.(?:bashrc|zshrc|profile|bash_profile))\b`), "system configuration file modification", SeverityFlag, "Avoid modifying system configuration files. Use user-level configuration when possible."},
	{regexp.MustCompile(`(?i)\bchmod\s+[+0-7]*[sS]`), "setuid/setgid permission change", SeverityBlock, "Setting setuid/setgid bits is a privilege escalation risk. Remove this operation."},
	{regexp.MustCompile(`(?i)\bchown\s+root\b`), "changing file ownership to root", SeverityBlock, "Changing ownership to root is a privilege escalation risk. Remove this operation."},
	{regexp.MustCompile(`(?i)\b(?:iptables|ufw|firewall-cmd|nft)\b.*(?:\s-[AIDRadr]\s|\b(?:add|insert|append|delete)\b)`), "firewall rule modification", SeverityBlock, "Skills should not modify firewall rules. Remove firewall modification commands."},
	{regexp.MustCompile(`(?i)\b(?:reg\s+add|regedit|New-ItemProperty)\b.*\b(?:HKLM|HKEY_LOCAL_MACHINE)\b`), "Windows registry modification (system-level)", SeverityBlock, "Skills should not modify the system registry. Remove registry modification commands."},
}

func checkSystemServiceMod(text, filePath string, isSkillMD bool) []Finding {
	var findings []Finding
	for _, p := range systemServicePatterns {
		if p.re.MatchString(text) {
			loc := p.re.FindStringIndex(text)
			line := 0
			snippet := ""
			if loc != nil {
				line = strings.Count(text[:loc[0]], "\n") + 1
				snippet = snippetAtLine(text, line, 120)
			}
			findings = append(findings, Finding{
				Stage:       stageNameSecurity,
				Severity:    p.severity,
				Category:    "system_modification",
				FilePath:    filePath,
				Description: p.desc,
				Line:        line,
				MatchText:   snippet,
				Remediation: p.remediation,
				IssueCode:   "W013",
			})
		}
	}
	return findings
}
