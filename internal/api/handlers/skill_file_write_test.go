package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWriteSkillFile_InvalidPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := WriteSkillFile(nil, nil, nil, nil)

	tests := []struct {
		name string
		body map[string]any
	}{
		{"missing path", map[string]any{"content": "x"}},
		{"traversal", map[string]any{"path": "../evil.py", "content": "x"}},
		{"absolute", map[string]any{"path": "/etc/passwd", "content": "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyJSON, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPut, "/v1/skills/my-skill/files", bytes.NewReader(bodyJSON))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "name", Value: "my-skill"}}
			c.Set("tenant_id", "test-tenant")

			handler(c)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d (%s)", w.Code, http.StatusBadRequest, w.Body.String())
			}
		})
	}
}

func TestWriteSkillFile_InvalidName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := WriteSkillFile(nil, nil, nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/v1/skills/x/files", bytes.NewReader([]byte(`{"path":"main.py","content":"x"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "name", Value: "../evil"}}
	c.Set("tenant_id", "test-tenant")

	handler(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRepackageWithFile_ReplaceAndAdd(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	md, _ := zw.Create("SKILL.md")
	_, _ = md.Write([]byte("---\nname: \"demo\"\ndescription: \"d\"\nlang: \"python\"\nversion: \"1.0.0\"\n---\nbody"))
	mp, _ := zw.Create("main.py")
	_, _ = mp.Write([]byte("print('old')"))
	_ = zw.Close()

	// Replace existing file.
	out, desc, lang, err := repackageWithFile(buf.Bytes(), "main.py", "print('new')")
	if err != nil {
		t.Fatalf("repackage: %v", err)
	}
	if desc != "d" || lang != "python" {
		t.Errorf("metadata = %q/%q, want d/python", desc, lang)
	}
	if got := fileFromZip(t, out, "main.py"); got != "print('new')" {
		t.Errorf("main.py = %q, want replaced", got)
	}

	// Add a new file.
	out2, _, _, err := repackageWithFile(buf.Bytes(), "helper.py", "x=1")
	if err != nil {
		t.Fatalf("repackage add: %v", err)
	}
	if got := fileFromZip(t, out2, "helper.py"); got != "x=1" {
		t.Errorf("helper.py = %q, want added", got)
	}
	if got := fileFromZip(t, out2, "main.py"); got != "print('old')" {
		t.Errorf("main.py = %q, want preserved", got)
	}
}

func fileFromZip(t *testing.T, data []byte, name string) string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range r.File {
		if f.Name == name {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			return string(b)
		}
	}
	return ""
}
