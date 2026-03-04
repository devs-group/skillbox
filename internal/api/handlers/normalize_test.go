package handlers

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readZipFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = string(data)
	}
	return files
}

func TestNormalizeSkillZip_AlreadyClean(t *testing.T) {
	original := makeZip(t, map[string]string{
		"SKILL.md": "---\nname: test\n---",
		"main.py":  "print('hi')",
	})

	result, err := normalizeSkillZip(original)
	if err != nil {
		t.Fatal(err)
	}

	// Should return original bytes unchanged.
	if !bytes.Equal(result, original) {
		t.Error("expected original bytes to be returned for a clean zip")
	}
}

func TestNormalizeSkillZip_StripsWrapperDir(t *testing.T) {
	data := makeZip(t, map[string]string{
		"my-skill/SKILL.md": "---\nname: test\n---",
		"my-skill/main.py":  "print('hi')",
	})

	result, err := normalizeSkillZip(data)
	if err != nil {
		t.Fatal(err)
	}

	files := readZipFiles(t, result)
	if _, ok := files["SKILL.md"]; !ok {
		t.Error("expected SKILL.md at root after normalization")
	}
	if _, ok := files["main.py"]; !ok {
		t.Error("expected main.py at root after normalization")
	}
	if _, ok := files["my-skill/SKILL.md"]; ok {
		t.Error("expected wrapper dir to be stripped")
	}
}

func TestNormalizeSkillZip_StripsWrapperWithSubdirs(t *testing.T) {
	data := makeZip(t, map[string]string{
		"slack-gif-creator 2/SKILL.md":         "---\nname: test\n---",
		"slack-gif-creator 2/core/__init__.py":  "",
		"slack-gif-creator 2/core/gif.py":       "# gif code",
		"slack-gif-creator 2/requirements.txt": "pillow",
	})

	result, err := normalizeSkillZip(data)
	if err != nil {
		t.Fatal(err)
	}

	files := readZipFiles(t, result)
	expected := []string{"SKILL.md", "core/__init__.py", "core/gif.py", "requirements.txt"}
	for _, name := range expected {
		if _, ok := files[name]; !ok {
			t.Errorf("expected %q in normalized zip", name)
		}
	}
}

func TestNormalizeSkillZip_RemovesMacOSJunk(t *testing.T) {
	data := makeZip(t, map[string]string{
		"__MACOSX/._SKILL.md":    "junk",
		"__MACOSX/core/._gif.py": "junk",
		".DS_Store":              "junk",
		"SKILL.md":               "---\nname: test\n---",
		"main.py":                "print('hi')",
	})

	result, err := normalizeSkillZip(data)
	if err != nil {
		t.Fatal(err)
	}

	files := readZipFiles(t, result)
	if _, ok := files["__MACOSX/._SKILL.md"]; ok {
		t.Error("expected __MACOSX files to be removed")
	}
	if _, ok := files[".DS_Store"]; ok {
		t.Error("expected .DS_Store to be removed")
	}
	if _, ok := files["SKILL.md"]; !ok {
		t.Error("expected SKILL.md to remain")
	}
	if _, ok := files["main.py"]; !ok {
		t.Error("expected main.py to remain")
	}
}

func TestNormalizeSkillZip_WrapperPlusMacOSJunk(t *testing.T) {
	data := makeZip(t, map[string]string{
		"__MACOSX/my-skill/._SKILL.md": "junk",
		"my-skill/.DS_Store":           "junk",
		"my-skill/SKILL.md":            "---\nname: test\n---",
		"my-skill/main.py":             "print('hi')",
	})

	result, err := normalizeSkillZip(data)
	if err != nil {
		t.Fatal(err)
	}

	files := readZipFiles(t, result)
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
	if _, ok := files["SKILL.md"]; !ok {
		t.Error("expected SKILL.md at root")
	}
	if _, ok := files["main.py"]; !ok {
		t.Error("expected main.py at root")
	}
}

func TestCommonPrefix_NoPrefix(t *testing.T) {
	result := commonPrefix([]string{"SKILL.md", "main.py"})
	if result != "" {
		t.Errorf("expected empty prefix, got %q", result)
	}
}

func TestCommonPrefix_SharedPrefix(t *testing.T) {
	result := commonPrefix([]string{"dir/SKILL.md", "dir/main.py", "dir/lib/util.py"})
	if result != "dir/" {
		t.Errorf("expected %q, got %q", "dir/", result)
	}
}

func TestCommonPrefix_MixedPrefixes(t *testing.T) {
	result := commonPrefix([]string{"dir1/file.py", "dir2/file.py"})
	if result != "" {
		t.Errorf("expected empty prefix for mixed prefixes, got %q", result)
	}
}

func TestJunkFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"__MACOSX/._foo", true},
		{"__MACOSX/dir/._bar", true},
		{".DS_Store", true},
		{"dir/.DS_Store", true},
		{"Thumbs.db", true},
		{"SKILL.md", false},
		{"main.py", false},
		{"core/gif.py", false},
	}

	for _, tt := range tests {
		if got := junkFile(tt.name); got != tt.want {
			t.Errorf("junkFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
