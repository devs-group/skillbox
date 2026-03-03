package skill

import (
	"strings"
	"testing"
	"time"
)

func TestParseValidSkill(t *testing.T) {
	input := []byte(`---
name: data-analyzer
version: 1.2.0
description: Analyzes CSV data and produces summary statistics
lang: python
image: python:3.11-slim
timeout: 60s
resources:
  cpu: "1.0"
  memory: 512Mi
---
You are a data analysis assistant.
Parse the provided CSV and return summary statistics as JSON.
`)

	s, err := ParseSkillMD(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Name != "data-analyzer" {
		t.Errorf("Name = %q, want %q", s.Name, "data-analyzer")
	}
	if s.Version != "1.2.0" {
		t.Errorf("Version = %q, want %q", s.Version, "1.2.0")
	}
	if s.Description != "Analyzes CSV data and produces summary statistics" {
		t.Errorf("Description = %q, want %q", s.Description, "Analyzes CSV data and produces summary statistics")
	}
	if s.Lang != "python" {
		t.Errorf("Lang = %q, want %q", s.Lang, "python")
	}
	if s.Image != "python:3.11-slim" {
		t.Errorf("Image = %q, want %q", s.Image, "python:3.11-slim")
	}
	if s.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", s.Timeout, 60*time.Second)
	}
	if s.Resources.CPU != "1.0" {
		t.Errorf("Resources.CPU = %q, want %q", s.Resources.CPU, "1.0")
	}
	if s.Resources.Memory != "512Mi" {
		t.Errorf("Resources.Memory = %q, want %q", s.Resources.Memory, "512Mi")
	}
	if !strings.Contains(s.Instructions, "data analysis assistant") {
		t.Errorf("Instructions should contain body text, got %q", s.Instructions)
	}
	if !strings.Contains(s.Instructions, "summary statistics as JSON") {
		t.Errorf("Instructions should contain full body, got %q", s.Instructions)
	}

	// DefaultImage should return the custom image since one was set.
	if img := s.DefaultImage(); img != "python:3.11-slim" {
		t.Errorf("DefaultImage() = %q, want %q", img, "python:3.11-slim")
	}
}

func TestParseMinimalSkill(t *testing.T) {
	input := []byte(`---
name: hello
version: 0.1.0
description: A minimal skill
lang: bash
---
Say hello.
`)

	s, err := ParseSkillMD(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Name != "hello" {
		t.Errorf("Name = %q, want %q", s.Name, "hello")
	}
	if s.Version != "0.1.0" {
		t.Errorf("Version = %q, want %q", s.Version, "0.1.0")
	}
	if s.Description != "A minimal skill" {
		t.Errorf("Description = %q, want %q", s.Description, "A minimal skill")
	}
	if s.Lang != "bash" {
		t.Errorf("Lang = %q, want %q", s.Lang, "bash")
	}
	if s.Image != "" {
		t.Errorf("Image should be empty for minimal skill, got %q", s.Image)
	}
	if s.Timeout != 0 {
		t.Errorf("Timeout should be zero for minimal skill, got %v", s.Timeout)
	}
	if s.Instructions != "Say hello." {
		t.Errorf("Instructions = %q, want %q", s.Instructions, "Say hello.")
	}
}

func TestParseMissingRequiredField(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "missing name",
			input: `---
version: 1.0.0
description: Some skill
lang: python
---
Body text.
`,
			wantErr: "name is required",
		},
		{
			name: "missing description",
			input: `---
name: my-skill
version: 1.0.0
lang: python
---
Body text.
`,
			wantErr: "description is required",
		},
		{
			name: "invalid version format",
			input: `---
name: my-skill
version: v1.0
description: Some skill
lang: python
---
Body text.
`,
			wantErr: "must be semver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSkillMD([]byte(tt.input))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseInvalidLang(t *testing.T) {
	input := []byte(`---
name: bad-lang
version: 1.0.0
description: A skill with unsupported lang
lang: rust
---
Do something.
`)

	_, err := ParseSkillMD(input)
	if err == nil {
		t.Fatal("expected error for invalid lang, got nil")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %q, want it to contain 'not supported'", err.Error())
	}
	if !strings.Contains(err.Error(), "rust") {
		t.Errorf("error = %q, want it to mention 'rust'", err.Error())
	}
}

func TestDefaultImage(t *testing.T) {
	tests := []struct {
		lang      string
		image     string
		wantImage string
	}{
		{lang: "python", image: "", wantImage: "python:3.12-slim"},
		{lang: "node", image: "", wantImage: "node:20-slim"},
		{lang: "bash", image: "", wantImage: "bash:5"},
		{lang: "python", image: "python:3.11-slim", wantImage: "python:3.11-slim"},
		{lang: "node", image: "custom-node:latest", wantImage: "custom-node:latest"},
		{lang: "", image: "", wantImage: "bash:5"},
	}

	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.image, func(t *testing.T) {
			s := &Skill{
				Name:        "test",
				Version:     "1.0.0",
				Description: "test",
				Lang:        tt.lang,
				Image:       tt.image,
			}
			got := s.DefaultImage()
			if got != tt.wantImage {
				t.Errorf("DefaultImage() = %q, want %q", got, tt.wantImage)
			}
		})
	}
}

func TestParseFrontmatterEdgeCases(t *testing.T) {
	t.Run("no opening delimiter", func(t *testing.T) {
		_, err := ParseSkillMD([]byte("name: test\n"))
		if err == nil {
			t.Fatal("expected error for missing opening delimiter")
		}
	})

	t.Run("no closing delimiter", func(t *testing.T) {
		_, err := ParseSkillMD([]byte("---\nname: test\n"))
		if err == nil {
			t.Fatal("expected error for missing closing delimiter")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := ParseSkillMD([]byte(""))
		if err == nil {
			t.Fatal("expected error for empty input")
		}
	})
}

func TestValidateName(t *testing.T) {
	valid := []string{"hello", "data-analyzer", "my_skill", "skill.v2", "A123", "a"}
	for _, name := range valid {
		t.Run("valid_"+name, func(t *testing.T) {
			if err := ValidateName(name); err != nil {
				t.Errorf("ValidateName(%q) = %v, want nil", name, err)
			}
		})
	}

	invalid := []string{
		"",
		"../traversal",
		"skill/name",
		"skill\\name",
		".hidden",
		"-starts-with-dash",
		"_starts-with-underscore",
		"has spaces",
		"has\x00null",
		strings.Repeat("a", 129), // exceeds 128 char limit
	}
	for _, name := range invalid {
		label := name
		if len(label) > 20 {
			label = label[:20] + "..."
		}
		t.Run("invalid_"+label, func(t *testing.T) {
			if err := ValidateName(name); err == nil {
				t.Errorf("ValidateName(%q) = nil, want error", name)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	valid := []string{"", "latest", "1.0.0", "1.0.0-beta"}
	for _, v := range valid {
		t.Run("valid_"+v, func(t *testing.T) {
			if err := ValidateVersion(v); err != nil {
				t.Errorf("ValidateVersion(%q) = %v, want nil", v, err)
			}
		})
	}

	invalid := []string{"../traversal", "v1.0.0", "1.0", "abc"}
	for _, v := range invalid {
		t.Run("invalid_"+v, func(t *testing.T) {
			if err := ValidateVersion(v); err == nil {
				t.Errorf("ValidateVersion(%q) = nil, want error", v)
			}
		})
	}
}

func TestParseSkillNameTraversal(t *testing.T) {
	// Skill names containing path traversal characters should be rejected
	// during parsing since Validate() is called.
	input := []byte(`---
name: "../../etc/passwd"
version: 1.0.0
description: Malicious skill
lang: python
---
body
`)
	_, err := ParseSkillMD(input)
	if err == nil {
		t.Fatal("expected error for skill name with path traversal")
	}
	if !strings.Contains(err.Error(), "invalid characters") {
		t.Errorf("error = %q, want it to mention invalid characters", err.Error())
	}
}

func TestParseVersionFormats(t *testing.T) {
	validVersions := []string{"0.0.1", "1.0.0", "10.20.30", "1.0.0-beta", "2.0.0-rc.1"}
	for _, v := range validVersions {
		t.Run("valid_"+v, func(t *testing.T) {
			input := []byte("---\nname: test\nversion: " + v + "\ndescription: test\nlang: python\n---\nbody\n")
			_, err := ParseSkillMD(input)
			if err != nil {
				t.Errorf("version %q should be valid, got error: %v", v, err)
			}
		})
	}

	invalidVersions := []string{"1.0", "1", "v1.0.0", "1.0.0.0", "abc"}
	for _, v := range invalidVersions {
		t.Run("invalid_"+v, func(t *testing.T) {
			input := []byte("---\nname: test\nversion: " + v + "\ndescription: test\nlang: python\n---\nbody\n")
			_, err := ParseSkillMD(input)
			if err == nil {
				t.Errorf("version %q should be invalid, got no error", v)
			}
		})
	}
}

func TestParseSkillWithoutVersion(t *testing.T) {
	input := []byte(`---
name: my-skill
description: A skill without version
lang: python
---
Body text.
`)
	s, err := ParseSkillMD(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Version != "0.0.0" {
		t.Errorf("Version = %q, want %q", s.Version, "0.0.0")
	}
}

func TestParseSkillWithoutLang(t *testing.T) {
	input := []byte(`---
name: my-skill
description: A skill without lang
---
Body text.
`)
	s, err := ParseSkillMD(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Lang != "" {
		t.Errorf("Lang = %q, want empty string", s.Lang)
	}
}

func TestInferLangFromEntrypoint(t *testing.T) {
	tests := []struct {
		entrypoint string
		want       string
	}{
		{"main.py", "python"},
		{"scripts/run.py", "python"},
		{"main.js", "node"},
		{"main.sh", "bash"},
		{"unknown", ""},
		{"main.rb", ""},
	}

	for _, tt := range tests {
		t.Run(tt.entrypoint, func(t *testing.T) {
			got := InferLangFromEntrypoint(tt.entrypoint)
			if got != tt.want {
				t.Errorf("InferLangFromEntrypoint(%q) = %q, want %q", tt.entrypoint, got, tt.want)
			}
		})
	}
}
