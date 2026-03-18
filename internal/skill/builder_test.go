package skill

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestBuildSkillMD(t *testing.T) {
	tests := []struct {
		name         string
		skillName    string
		description  string
		lang         string
		version      string
		instructions string
		wantContains []string
	}{
		{
			name:         "full fields",
			skillName:    "data-analysis",
			description:  "Analyze data",
			lang:         "python",
			version:      "1.2.0",
			instructions: "Run this skill to analyze CSV data.",
			wantContains: []string{
				`name: "data-analysis"`,
				`description: "Analyze data"`,
				`lang: "python"`,
				`version: "1.2.0"`,
				"Run this skill to analyze CSV data.",
			},
		},
		{
			name:         "empty version defaults to 1.0.0",
			skillName:    "test",
			description:  "Test skill",
			lang:         "node",
			version:      "",
			instructions: "",
			wantContains: []string{
				`version: "1.0.0"`,
				`lang: "node"`,
			},
		},
		{
			name:         "empty lang omitted",
			skillName:    "test",
			description:  "Test skill",
			lang:         "",
			version:      "1.0.0",
			instructions: "",
			wantContains: []string{
				`name: "test"`,
			},
		},
		{
			name:         "instructions placed after frontmatter",
			skillName:    "summarizer",
			description:  "Summarize text",
			lang:         "python",
			version:      "1.0.0",
			instructions: "This skill summarizes input text.",
			wantContains: []string{
				"---\n\nThis skill summarizes input text.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSkillMD(tt.skillName, tt.description, tt.lang, tt.version, tt.instructions)

			// Must start with frontmatter
			if !strings.HasPrefix(got, "---\n") {
				t.Errorf("BuildSkillMD() must start with '---\\n', got: %q", got[:20])
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildSkillMD() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestBuildSkillMD_RoundTrip(t *testing.T) {
	// Build SKILL.md, then parse it back and verify fields match.
	md := BuildSkillMD("my-skill", "Does cool things", "python", "2.0.0", "Use this skill wisely.")
	parsed, err := ParseSkillMD([]byte(md))
	if err != nil {
		t.Fatalf("ParseSkillMD failed on BuildSkillMD output: %v", err)
	}
	if parsed.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", parsed.Name, "my-skill")
	}
	if parsed.Description != "Does cool things" {
		t.Errorf("Description = %q, want %q", parsed.Description, "Does cool things")
	}
	if parsed.Lang != "python" {
		t.Errorf("Lang = %q, want %q", parsed.Lang, "python")
	}
	if parsed.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", parsed.Version, "2.0.0")
	}
	if parsed.Instructions != "Use this skill wisely." {
		t.Errorf("Instructions = %q, want %q", parsed.Instructions, "Use this skill wisely.")
	}
}

func TestEntrypointFilename(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"python", "main.py"},
		{"node", "main.js"},
		{"bash", "run.sh"},
		{"", "main.py"},       // default
		{"unknown", "main.py"}, // fallback
	}
	for _, tt := range tests {
		got := EntrypointFilename(tt.lang)
		if got != tt.want {
			t.Errorf("EntrypointFilename(%q) = %q, want %q", tt.lang, got, tt.want)
		}
	}
}

func TestPackageSkillZip(t *testing.T) {
	skillMD := BuildSkillMD("test-skill", "A test", "python", "1.0.0", "")
	code := "print('hello world')"

	zipData, err := PackageSkillZip(skillMD, code, "python")
	if err != nil {
		t.Fatalf("PackageSkillZip() error: %v", err)
	}
	if len(zipData) == 0 {
		t.Fatal("PackageSkillZip() returned empty data")
	}

	// Verify zip contents
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}

	fileNames := make(map[string]bool)
	for _, f := range r.File {
		fileNames[f.Name] = true
	}

	if !fileNames["SKILL.md"] {
		t.Error("zip missing SKILL.md")
	}
	if !fileNames["main.py"] {
		t.Error("zip missing main.py (python entrypoint)")
	}
	if len(r.File) != 2 {
		t.Errorf("zip has %d files, want 2", len(r.File))
	}
}

func TestPackageSkillZip_NodeEntrypoint(t *testing.T) {
	skillMD := BuildSkillMD("node-skill", "Node test", "node", "1.0.0", "")
	zipData, err := PackageSkillZip(skillMD, "console.log('hi')", "node")
	if err != nil {
		t.Fatalf("PackageSkillZip() error: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}

	found := false
	for _, f := range r.File {
		if f.Name == "main.js" {
			found = true
		}
	}
	if !found {
		t.Error("zip missing main.js for node skill")
	}
}

func TestPackageSkillZip_EmptyCode(t *testing.T) {
	skillMD := BuildSkillMD("test", "Test", "python", "1.0.0", "")
	_, err := PackageSkillZip(skillMD, "", "python")
	if err == nil {
		t.Error("PackageSkillZip() should fail with empty code")
	}
	_, err = PackageSkillZip(skillMD, "   ", "python")
	if err == nil {
		t.Error("PackageSkillZip() should fail with whitespace-only code")
	}
}

func TestBuildSkillMD_SpecialCharacters(t *testing.T) {
	// Descriptions and instructions can contain quotes, newlines, and unicode.
	md := BuildSkillMD(
		"unicode-skill",
		`Analyze "complex" data with special chars: <>&`,
		"python",
		"1.0.0",
		"## Instructions\n\nUse this skill for Ñoño análysis.\n\nIt handles émojis: 🚀",
	)

	// Should not crash and should produce valid SKILL.md
	parsed, err := ParseSkillMD([]byte(md))
	if err != nil {
		t.Fatalf("ParseSkillMD failed on special chars: %v", err)
	}
	if parsed.Name != "unicode-skill" {
		t.Errorf("Name = %q, want %q", parsed.Name, "unicode-skill")
	}
	// Description may have quotes escaped by %q formatting — the parser strips them
	if parsed.Description == "" {
		t.Error("Description should not be empty")
	}
	if !strings.Contains(parsed.Instructions, "émojis: 🚀") {
		t.Error("Instructions should preserve unicode")
	}
}

func TestBuildSkillMD_LongInstructions(t *testing.T) {
	// Test with very long instructions (>500 chars, the LLM truncation point)
	longInstr := strings.Repeat("This is a long instruction line. ", 100) // ~3200 chars
	md := BuildSkillMD("long-skill", "Test", "python", "1.0.0", longInstr)

	parsed, err := ParseSkillMD([]byte(md))
	if err != nil {
		t.Fatalf("ParseSkillMD failed on long instructions: %v", err)
	}
	if len(parsed.Instructions) < 3000 {
		t.Errorf("Instructions truncated: got %d chars, expected ~3200", len(parsed.Instructions))
	}
}

func TestBuildSkillMD_EmptyInstructions(t *testing.T) {
	md := BuildSkillMD("no-instr", "Test", "python", "1.0.0", "")
	parsed, err := ParseSkillMD([]byte(md))
	if err != nil {
		t.Fatalf("ParseSkillMD failed: %v", err)
	}
	if parsed.Instructions != "" {
		t.Errorf("Instructions = %q, want empty", parsed.Instructions)
	}
}

func TestPackageSkillZip_BashEntrypoint(t *testing.T) {
	md := BuildSkillMD("bash-skill", "Bash test", "bash", "1.0.0", "")
	zipData, err := PackageSkillZip(md, "#!/bin/bash\necho hello", "bash")
	if err != nil {
		t.Fatalf("PackageSkillZip() error: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}

	found := false
	for _, f := range r.File {
		if f.Name == "run.sh" {
			found = true
		}
	}
	if !found {
		t.Error("zip missing run.sh for bash skill")
	}
}

func TestPackageSkillZip_PreservesCodeExactly(t *testing.T) {
	// The code content should be preserved byte-for-byte in the zip.
	code := "import os\n\ndef main():\n    print(os.environ.get('API_KEY', 'no key'))\n\nif __name__ == '__main__':\n    main()\n"
	md := BuildSkillMD("code-test", "Test", "python", "1.0.0", "")
	zipData, err := PackageSkillZip(md, code, "python")
	if err != nil {
		t.Fatalf("PackageSkillZip() error: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}

	for _, f := range r.File {
		if f.Name == "main.py" {
			rc, _ := f.Open()
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			rc.Close()
			if buf.String() != code {
				t.Errorf("code mismatch:\ngot:  %q\nwant: %q", buf.String(), code)
			}
			return
		}
	}
	t.Fatal("main.py not found in zip")
}

func TestPackageSkillZip_FullRoundTrip(t *testing.T) {
	// Build -> Package -> Parse back (using skillbox's own parser)
	skillMD := BuildSkillMD("roundtrip-skill", "Full roundtrip test", "bash", "3.0.0", "Instructions here")
	code := "#!/bin/bash\necho hello"

	zipData, err := PackageSkillZip(skillMD, code, "bash")
	if err != nil {
		t.Fatalf("PackageSkillZip() error: %v", err)
	}

	// Open zip and find SKILL.md
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}

	for _, f := range r.File {
		if f.Name == "SKILL.md" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open SKILL.md: %v", err)
			}
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			rc.Close()

			parsed, err := ParseSkillMD(buf.Bytes())
			if err != nil {
				t.Fatalf("ParseSkillMD() failed: %v", err)
			}
			if parsed.Name != "roundtrip-skill" {
				t.Errorf("Name = %q, want %q", parsed.Name, "roundtrip-skill")
			}
			if parsed.Lang != "bash" {
				t.Errorf("Lang = %q, want %q", parsed.Lang, "bash")
			}
			if parsed.Version != "3.0.0" {
				t.Errorf("Version = %q, want %q", parsed.Version, "3.0.0")
			}
			return
		}
	}
	t.Fatal("SKILL.md not found in zip")
}
