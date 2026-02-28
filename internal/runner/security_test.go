package runner

import (
	"strings"
	"testing"
)

func TestValidateImage_Allowed(t *testing.T) {
	allowlist := []string{"python:3.12-slim", "node:20-slim", "alpine:3"}

	tests := []string{"python:3.12-slim", "node:20-slim", "alpine:3"}
	for _, img := range tests {
		t.Run(img, func(t *testing.T) {
			if err := ValidateImage(img, allowlist); err != nil {
				t.Errorf("expected image %q to be allowed, got error: %v", img, err)
			}
		})
	}
}

func TestValidateImage_NotAllowed(t *testing.T) {
	allowlist := []string{"python:3.12-slim", "node:20-slim"}

	tests := []string{"alpine:latest", "ubuntu:22.04", "rust:1.75"}
	for _, img := range tests {
		t.Run(img, func(t *testing.T) {
			err := ValidateImage(img, allowlist)
			if err == nil {
				t.Errorf("expected image %q to be rejected, got nil", img)
			}
			if !strings.Contains(err.Error(), "not in the allowlist") {
				t.Errorf("error = %q, want it to mention 'not in the allowlist'", err.Error())
			}
		})
	}
}

func TestValidateImage_ExactMatch(t *testing.T) {
	allowlist := []string{"python:3.12-slim"}

	// Exact match should pass.
	if err := ValidateImage("python:3.12-slim", allowlist); err != nil {
		t.Errorf("expected exact match to pass, got error: %v", err)
	}

	// Partial match (missing -slim) should fail.
	if err := ValidateImage("python:3.12", allowlist); err == nil {
		t.Error("expected partial match 'python:3.12' to fail, got nil")
	}

	// Superset match (extra suffix) should fail.
	if err := ValidateImage("python:3.12-slim-custom", allowlist); err == nil {
		t.Error("expected superset match 'python:3.12-slim-custom' to fail, got nil")
	}
}

func TestValidateImage_EmptyImage(t *testing.T) {
	err := ValidateImage("", []string{"python:3.12-slim"})
	if err == nil {
		t.Fatal("expected error for empty image name, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error = %q, want it to mention 'required'", err.Error())
	}
}

func TestValidateImage_EmptyAllowlist(t *testing.T) {
	err := ValidateImage("python:3.12-slim", nil)
	if err == nil {
		t.Fatal("expected error for empty allowlist, got nil")
	}
	if !strings.Contains(err.Error(), "allowlist is empty") {
		t.Errorf("error = %q, want it to mention 'allowlist is empty'", err.Error())
	}
}

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"256Mi", 256 * 1024 * 1024},
		{"1Gi", 1024 * 1024 * 1024},
		{"512Ki", 512 * 1024},
		{"1024", 1024},
		{"1", 1},
		{"2Gi", 2 * 1024 * 1024 * 1024},
		{"100Mi", 100 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseMemoryLimit(tt.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseMemoryLimit(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseMemoryLimit_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"non-numeric", "abc"},
		{"negative bytes", "-100"},
		{"zero bytes", "0"},
		{"zero with suffix", "0Mi"},
		{"negative with suffix", "-1Gi"},
		{"invalid suffix", "256Xi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMemoryLimit(tt.input)
			if err == nil {
				t.Errorf("ParseMemoryLimit(%q) should return error, got nil", tt.input)
			}
		})
	}
}

func TestParseCPULimit(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"0.5", 50000},
		{"1", 100000},
		{"1.0", 100000},
		{"2", 200000},
		{"2.5", 250000},
		{"0.25", 25000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseCPULimit(tt.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseCPULimit(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseCPULimit_MinimumQuota(t *testing.T) {
	// Very small CPU value should be clamped to minimum quota of 1000.
	got, err := ParseCPULimit("0.001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1000 {
		t.Errorf("ParseCPULimit(\"0.001\") = %d, want 1000 (minimum quota)", got)
	}
}

func TestParseCPULimit_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"non-numeric", "abc"},
		{"negative", "-1"},
		{"zero", "0"},
		{"negative decimal", "-0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCPULimit(tt.input)
			if err == nil {
				t.Errorf("ParseCPULimit(%q) should return error, got nil", tt.input)
			}
		})
	}
}
