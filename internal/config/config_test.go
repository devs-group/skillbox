package config

import (
	"strings"
	"testing"
	"time"
)

// setRequiredEnv sets the minimum required environment variables for Load()
// to succeed, using t.Setenv so they are automatically cleaned up.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SKILLBOX_DB_DSN", "postgres://user:pass@localhost:5432/testdb?sslmode=disable")
	t.Setenv("SKILLBOX_S3_ENDPOINT", "localhost:9000")
	t.Setenv("SKILLBOX_S3_ACCESS_KEY", "minioadmin")
	t.Setenv("SKILLBOX_S3_SECRET_KEY", "minioadmin")
}

func TestLoad_AllRequired(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DBDSN != "postgres://user:pass@localhost:5432/testdb?sslmode=disable" {
		t.Errorf("DBDSN = %q, want %q", cfg.DBDSN, "postgres://user:pass@localhost:5432/testdb?sslmode=disable")
	}
	if cfg.S3Endpoint != "localhost:9000" {
		t.Errorf("S3Endpoint = %q, want %q", cfg.S3Endpoint, "localhost:9000")
	}
	if cfg.S3AccessKey != "minioadmin" {
		t.Errorf("S3AccessKey = %q, want %q", cfg.S3AccessKey, "minioadmin")
	}
	if cfg.S3SecretKey != "minioadmin" {
		t.Errorf("S3SecretKey = %q, want %q", cfg.S3SecretKey, "minioadmin")
	}
}

func TestLoad_MissingDBDSN(t *testing.T) {
	// Set S3 vars but omit DB DSN.
	t.Setenv("SKILLBOX_S3_ENDPOINT", "localhost:9000")
	t.Setenv("SKILLBOX_S3_ACCESS_KEY", "minioadmin")
	t.Setenv("SKILLBOX_S3_SECRET_KEY", "minioadmin")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when SKILLBOX_DB_DSN is missing, got nil")
	}
	if !strings.Contains(err.Error(), "SKILLBOX_DB_DSN") {
		t.Errorf("error = %q, want it to mention SKILLBOX_DB_DSN", err.Error())
	}
}

func TestLoad_MissingS3(t *testing.T) {
	tests := []struct {
		name   string
		omitKey string
	}{
		{"missing S3 endpoint", "SKILLBOX_S3_ENDPOINT"},
		{"missing S3 access key", "SKILLBOX_S3_ACCESS_KEY"},
		{"missing S3 secret key", "SKILLBOX_S3_SECRET_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set all required env vars.
			t.Setenv("SKILLBOX_DB_DSN", "postgres://localhost/test")
			t.Setenv("SKILLBOX_S3_ENDPOINT", "localhost:9000")
			t.Setenv("SKILLBOX_S3_ACCESS_KEY", "minioadmin")
			t.Setenv("SKILLBOX_S3_SECRET_KEY", "minioadmin")

			// Unset the one we want to test as missing.
			t.Setenv(tt.omitKey, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error when %s is missing, got nil", tt.omitKey)
			}
			if !strings.Contains(err.Error(), tt.omitKey) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tt.omitKey)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIPort != "8080" {
		t.Errorf("APIPort = %q, want %q", cfg.APIPort, "8080")
	}
	if cfg.GRPCPort != "9090" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9090")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.S3BucketSkills != "skills" {
		t.Errorf("S3BucketSkills = %q, want %q", cfg.S3BucketSkills, "skills")
	}
	if cfg.S3BucketExecs != "executions" {
		t.Errorf("S3BucketExecs = %q, want %q", cfg.S3BucketExecs, "executions")
	}
	if cfg.DockerHost != "tcp://localhost:2375" {
		t.Errorf("DockerHost = %q, want %q", cfg.DockerHost, "tcp://localhost:2375")
	}
	if cfg.S3UseSSL != false {
		t.Errorf("S3UseSSL = %v, want false", cfg.S3UseSSL)
	}
	if cfg.DefaultTimeout != 120*time.Second {
		t.Errorf("DefaultTimeout = %v, want %v", cfg.DefaultTimeout, 120*time.Second)
	}
	if cfg.MaxTimeout != 10*time.Minute {
		t.Errorf("MaxTimeout = %v, want %v", cfg.MaxTimeout, 10*time.Minute)
	}
	if cfg.DefaultMemory != 256*1024*1024 {
		t.Errorf("DefaultMemory = %d, want %d", cfg.DefaultMemory, 256*1024*1024)
	}
	if cfg.DefaultCPU != 0.5 {
		t.Errorf("DefaultCPU = %f, want %f", cfg.DefaultCPU, 0.5)
	}
	if cfg.MaxOutputSize != 1048576 {
		t.Errorf("MaxOutputSize = %d, want %d", cfg.MaxOutputSize, 1048576)
	}
	if cfg.MaxSkillSize != 52428800 {
		t.Errorf("MaxSkillSize = %d, want %d", cfg.MaxSkillSize, 52428800)
	}

	// Default image allowlist.
	expectedImages := []string{"python:3.12-slim", "python:3.11-slim", "node:20-slim", "node:18-slim", "bash:5"}
	if len(cfg.ImageAllowlist) != len(expectedImages) {
		t.Fatalf("ImageAllowlist length = %d, want %d", len(cfg.ImageAllowlist), len(expectedImages))
	}
	for i, img := range expectedImages {
		if cfg.ImageAllowlist[i] != img {
			t.Errorf("ImageAllowlist[%d] = %q, want %q", i, cfg.ImageAllowlist[i], img)
		}
	}
}

func TestLoad_CustomValues(t *testing.T) {
	setRequiredEnv(t)

	t.Setenv("SKILLBOX_API_PORT", "3000")
	t.Setenv("SKILLBOX_GRPC_PORT", "50051")
	t.Setenv("SKILLBOX_LOG_LEVEL", "debug")
	t.Setenv("SKILLBOX_S3_BUCKET_SKILLS", "my-skills")
	t.Setenv("SKILLBOX_S3_BUCKET_EXECUTIONS", "my-execs")
	t.Setenv("SKILLBOX_DOCKER_HOST", "unix:///var/run/docker.sock")
	t.Setenv("SKILLBOX_S3_USE_SSL", "true")
	t.Setenv("SKILLBOX_IMAGE_ALLOWLIST", "alpine:3.19,ubuntu:22.04")
	t.Setenv("SKILLBOX_DEFAULT_TIMEOUT", "30s")
	t.Setenv("SKILLBOX_MAX_TIMEOUT", "5m")
	t.Setenv("SKILLBOX_DEFAULT_MEMORY", "1Gi")
	t.Setenv("SKILLBOX_DEFAULT_CPU", "2.0")
	t.Setenv("SKILLBOX_MAX_OUTPUT_SIZE", "2097152")
	t.Setenv("SKILLBOX_MAX_SKILL_SIZE", "104857600")
	t.Setenv("SKILLBOX_REDIS_URL", "redis://localhost:6379")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIPort != "3000" {
		t.Errorf("APIPort = %q, want %q", cfg.APIPort, "3000")
	}
	if cfg.GRPCPort != "50051" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "50051")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.S3BucketSkills != "my-skills" {
		t.Errorf("S3BucketSkills = %q, want %q", cfg.S3BucketSkills, "my-skills")
	}
	if cfg.S3BucketExecs != "my-execs" {
		t.Errorf("S3BucketExecs = %q, want %q", cfg.S3BucketExecs, "my-execs")
	}
	if cfg.DockerHost != "unix:///var/run/docker.sock" {
		t.Errorf("DockerHost = %q, want %q", cfg.DockerHost, "unix:///var/run/docker.sock")
	}
	if cfg.S3UseSSL != true {
		t.Errorf("S3UseSSL = %v, want true", cfg.S3UseSSL)
	}
	if cfg.RedisURL != "redis://localhost:6379" {
		t.Errorf("RedisURL = %q, want %q", cfg.RedisURL, "redis://localhost:6379")
	}

	expectedImages := []string{"alpine:3.19", "ubuntu:22.04"}
	if len(cfg.ImageAllowlist) != len(expectedImages) {
		t.Fatalf("ImageAllowlist length = %d, want %d", len(cfg.ImageAllowlist), len(expectedImages))
	}
	for i, img := range expectedImages {
		if cfg.ImageAllowlist[i] != img {
			t.Errorf("ImageAllowlist[%d] = %q, want %q", i, cfg.ImageAllowlist[i], img)
		}
	}

	if cfg.DefaultTimeout != 30*time.Second {
		t.Errorf("DefaultTimeout = %v, want %v", cfg.DefaultTimeout, 30*time.Second)
	}
	if cfg.MaxTimeout != 5*time.Minute {
		t.Errorf("MaxTimeout = %v, want %v", cfg.MaxTimeout, 5*time.Minute)
	}
	if cfg.DefaultMemory != 1*1024*1024*1024 {
		t.Errorf("DefaultMemory = %d, want %d", cfg.DefaultMemory, 1*1024*1024*1024)
	}
	if cfg.DefaultCPU != 2.0 {
		t.Errorf("DefaultCPU = %f, want %f", cfg.DefaultCPU, 2.0)
	}
	if cfg.MaxOutputSize != 2097152 {
		t.Errorf("MaxOutputSize = %d, want %d", cfg.MaxOutputSize, 2097152)
	}
	if cfg.MaxSkillSize != 104857600 {
		t.Errorf("MaxSkillSize = %d, want %d", cfg.MaxSkillSize, 104857600)
	}
}
