package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration loaded from environment variables.
// It follows 12-factor methodology: configuration is strictly separated from code.
type Config struct {
	// Database
	DBDSN string

	// Redis (optional — system degrades gracefully without it)
	RedisURL string

	// S3 / MinIO
	S3Endpoint        string
	S3AccessKey       string
	S3SecretKey       string
	S3BucketSkills    string
	S3BucketExecs     string
	S3UseSSL          bool

	// OpenSandbox
	OpenSandboxURL    string
	OpenSandboxAPIKey string
	SandboxExpiration time.Duration
	ImageAllowlist    []string

	// Execution limits
	DefaultTimeout         time.Duration
	MaxTimeout             time.Duration
	DefaultMemory          int64   // bytes
	MaxMemory              int64   // bytes — hard cap for skill-specified memory
	DefaultCPU             float64 // fractional CPU (e.g. 0.5 = half a core)
	MaxCPU                 float64 // hard cap for skill-specified CPU
	MaxOutputSize          int64   // bytes
	MaxSkillSize           int64   // bytes
	MaxConcurrentExecs     int     // max parallel sandbox executions

	// Sandbox session management
	SandboxSessionTTL   time.Duration // idle TTL for session sandboxes
	SandboxSessionImage string        // default image for session sandboxes
	MaxSessionSandboxes int           // max concurrent session sandboxes per server

	// Ory (Identity & OAuth2)
	KratosPublicURL string
	KratosAdminURL  string
	HydraPublicURL  string
	HydraAdminURL   string

	// GitHub (optional — marketplace disabled when empty)
	GitHubToken string

	// Server
	APIPort string

	// Observability
	LogLevel string
}

// defaultMemoryStr and defaultCPUStr store the raw string values from
// environment variables, set during Load(). Used to pass the original
// format to OpenSandbox without re-reading the environment.
var (
	defaultMemoryStr = "256Mi"
	defaultCPUStr    = "500m"
)

// DefaultMemoryStr returns the default memory limit as a Kubernetes-style
// string suitable for passing to OpenSandbox (e.g. "256Mi").
func (c *Config) DefaultMemoryStr() string {
	return defaultMemoryStr
}

// DefaultCPUStr returns the default CPU limit as a string suitable for
// passing to OpenSandbox (e.g. "500m" for 0.5 cores).
func (c *Config) DefaultCPUStr() string {
	return defaultCPUStr
}

// Load reads configuration from environment variables, validates required
// fields, parses durations and resource limits, and returns a fully
// populated Config. An error is returned if any required variable is
// missing or a value cannot be parsed.
func Load() (*Config, error) {
	var missing []string

	get := func(key string) string {
		return strings.TrimSpace(os.Getenv(key))
	}

	require := func(key string) string {
		v := get(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		DBDSN:             require("SKILLBOX_DB_DSN"),
		RedisURL:          get("SKILLBOX_REDIS_URL"),
		S3Endpoint:        require("SKILLBOX_S3_ENDPOINT"),
		S3AccessKey:       require("SKILLBOX_S3_ACCESS_KEY"),
		S3SecretKey:       require("SKILLBOX_S3_SECRET_KEY"),
		S3BucketSkills:    envOrDefault("SKILLBOX_S3_BUCKET_SKILLS", "skills"),
		S3BucketExecs:     envOrDefault("SKILLBOX_S3_BUCKET_EXECUTIONS", "executions"),
		OpenSandboxURL:    envOrDefault("SKILLBOX_OPENSANDBOX_URL", "http://localhost:8080"),
		OpenSandboxAPIKey: require("SKILLBOX_OPENSANDBOX_API_KEY"),
		KratosPublicURL:   envOrDefault("SKILLBOX_KRATOS_PUBLIC_URL", "http://localhost:4433"),
		KratosAdminURL:    envOrDefault("SKILLBOX_KRATOS_ADMIN_URL", "http://localhost:4434"),
		HydraPublicURL:    envOrDefault("SKILLBOX_HYDRA_PUBLIC_URL", "http://localhost:4444"),
		HydraAdminURL:     envOrDefault("SKILLBOX_HYDRA_ADMIN_URL", "http://localhost:4445"),
		GitHubToken:       get("GITHUB_TOKEN"),
		APIPort:           envOrDefault("SKILLBOX_API_PORT", "8080"),
		LogLevel:          envOrDefault("SKILLBOX_LOG_LEVEL", "info"),
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	var err error

	// S3 SSL
	useSSL, err := parseBool(envOrDefault("SKILLBOX_S3_USE_SSL", "false"))
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_S3_USE_SSL: %w", err)
	}
	cfg.S3UseSSL = useSSL

	// Sandbox expiration
	cfg.SandboxExpiration, err = time.ParseDuration(envOrDefault("SKILLBOX_SANDBOX_EXPIRATION", "5m"))
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_SANDBOX_EXPIRATION: %w", err)
	}

	// Image allowlist
	raw := envOrDefault("SKILLBOX_IMAGE_ALLOWLIST", "ghcr.io/devs-group/skillbox-sandbox:latest,python:3.12,python:3.12-slim,python:3.11-slim,node:20-slim,node:18-slim,bash:5")
	for _, img := range strings.Split(raw, ",") {
		img = strings.TrimSpace(img)
		if img != "" {
			cfg.ImageAllowlist = append(cfg.ImageAllowlist, img)
		}
	}

	// Timeouts
	cfg.DefaultTimeout, err = time.ParseDuration(envOrDefault("SKILLBOX_DEFAULT_TIMEOUT", "120s"))
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_DEFAULT_TIMEOUT: %w", err)
	}
	cfg.MaxTimeout, err = time.ParseDuration(envOrDefault("SKILLBOX_MAX_TIMEOUT", "10m"))
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_MAX_TIMEOUT: %w", err)
	}
	if cfg.DefaultTimeout > cfg.MaxTimeout {
		return nil, fmt.Errorf("SKILLBOX_DEFAULT_TIMEOUT (%s) exceeds SKILLBOX_MAX_TIMEOUT (%s)", cfg.DefaultTimeout, cfg.MaxTimeout)
	}

	// Memory — also store the raw string for passing to OpenSandbox.
	defaultMemoryStr = envOrDefault("SKILLBOX_DEFAULT_MEMORY", "256Mi")
	cfg.DefaultMemory, err = ParseMemory(defaultMemoryStr)
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_DEFAULT_MEMORY: %w", err)
	}

	cfg.MaxMemory, err = ParseMemory(envOrDefault("SKILLBOX_MAX_MEMORY", "1Gi"))
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_MAX_MEMORY: %w", err)
	}
	if cfg.DefaultMemory > cfg.MaxMemory {
		return nil, fmt.Errorf("SKILLBOX_DEFAULT_MEMORY (%d) exceeds SKILLBOX_MAX_MEMORY (%d)", cfg.DefaultMemory, cfg.MaxMemory)
	}

	// CPU — also store the raw string for passing to OpenSandbox.
	defaultCPUStr = envOrDefault("SKILLBOX_DEFAULT_CPU", "0.5")
	cfg.DefaultCPU, err = strconv.ParseFloat(defaultCPUStr, 64)
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_DEFAULT_CPU: %w", err)
	}
	if cfg.DefaultCPU <= 0 {
		return nil, fmt.Errorf("SKILLBOX_DEFAULT_CPU must be positive, got %f", cfg.DefaultCPU)
	}

	cfg.MaxCPU, err = strconv.ParseFloat(envOrDefault("SKILLBOX_MAX_CPU", "4.0"), 64)
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_MAX_CPU: %w", err)
	}
	if cfg.MaxCPU <= 0 {
		return nil, fmt.Errorf("SKILLBOX_MAX_CPU must be positive, got %f", cfg.MaxCPU)
	}
	if cfg.DefaultCPU > cfg.MaxCPU {
		return nil, fmt.Errorf("SKILLBOX_DEFAULT_CPU (%f) exceeds SKILLBOX_MAX_CPU (%f)", cfg.DefaultCPU, cfg.MaxCPU)
	}

	// Max output size
	cfg.MaxOutputSize, err = strconv.ParseInt(envOrDefault("SKILLBOX_MAX_OUTPUT_SIZE", "1048576"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_MAX_OUTPUT_SIZE: %w", err)
	}

	// Max skill size
	cfg.MaxSkillSize, err = strconv.ParseInt(envOrDefault("SKILLBOX_MAX_SKILL_SIZE", "52428800"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_MAX_SKILL_SIZE: %w", err)
	}

	// Max concurrent executions
	maxConcurrent, err := strconv.Atoi(envOrDefault("SKILLBOX_MAX_CONCURRENT_EXECS", "10"))
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_MAX_CONCURRENT_EXECS: %w", err)
	}
	if maxConcurrent <= 0 {
		return nil, fmt.Errorf("SKILLBOX_MAX_CONCURRENT_EXECS must be positive, got %d", maxConcurrent)
	}
	cfg.MaxConcurrentExecs = maxConcurrent

	// Sandbox session TTL
	cfg.SandboxSessionTTL, err = time.ParseDuration(envOrDefault("SKILLBOX_SANDBOX_SESSION_TTL", "30m"))
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_SANDBOX_SESSION_TTL: %w", err)
	}

	// Sandbox session image
	cfg.SandboxSessionImage = envOrDefault("SKILLBOX_SANDBOX_SESSION_IMAGE", "ghcr.io/devs-group/skillbox-sandbox:latest")

	// Max session sandboxes
	maxSessions, err := strconv.Atoi(envOrDefault("SKILLBOX_MAX_SESSION_SANDBOXES", "20"))
	if err != nil {
		return nil, fmt.Errorf("SKILLBOX_MAX_SESSION_SANDBOXES: %w", err)
	}
	if maxSessions <= 0 {
		return nil, fmt.Errorf("SKILLBOX_MAX_SESSION_SANDBOXES must be positive, got %d", maxSessions)
	}
	cfg.MaxSessionSandboxes = maxSessions

	return cfg, nil
}

// envOrDefault returns the value of the environment variable named by key,
// or fallback if the variable is unset or empty.
func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

// parseBool parses a string as a boolean, accepting "true", "1", "yes"
// (case-insensitive) as true and "false", "0", "no" as false.
func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %q", s)
	}
}

// ParseMemory converts a human-readable memory string to bytes.
// Supported suffixes (case-insensitive):
//
//	Ki / K  — kibibytes (1024)
//	Mi / M  — mebibytes (1024^2)
//	Gi / G  — gibibytes (1024^3)
//
// A plain integer is treated as bytes.
func ParseMemory(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty memory string")
	}

	type suffix struct {
		label      string
		multiplier int64
	}

	// Order matters: check longer suffixes first to avoid partial matches.
	suffixes := []suffix{
		{"Gi", 1024 * 1024 * 1024},
		{"gi", 1024 * 1024 * 1024},
		{"GI", 1024 * 1024 * 1024},
		{"G", 1024 * 1024 * 1024},
		{"g", 1024 * 1024 * 1024},
		{"Mi", 1024 * 1024},
		{"mi", 1024 * 1024},
		{"MI", 1024 * 1024},
		{"M", 1024 * 1024},
		{"m", 1024 * 1024},
		{"Ki", 1024},
		{"ki", 1024},
		{"KI", 1024},
		{"K", 1024},
		{"k", 1024},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf.label) {
			numStr := strings.TrimSuffix(s, sf.label)
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid numeric part %q in memory string %q: %w", numStr, s, err)
			}
			if n < 0 {
				return 0, fmt.Errorf("negative memory value: %s", s)
			}
			return n * sf.multiplier, nil
		}
	}

	// No suffix — treat as raw bytes.
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory string %q: %w", s, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("negative memory value: %s", s)
	}
	return n, nil
}
