package registry

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZipEntry_NormalizesZeroMode(t *testing.T) {
	// Build a zip where the file has mode 0 (simulates Python zipfile.writestr default).
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	info := &zip.FileHeader{Name: "main.py"}
	info.SetMode(0) // explicitly set mode to 0
	f, err := w.CreateHeader(info)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("print('hello')")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Read the zip and extract.
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	for _, entry := range r.File {
		if err := extractZipEntry(tmpDir, entry); err != nil {
			t.Fatalf("extractZipEntry: %v", err)
		}
	}

	// Verify the extracted file is readable (mode != 0).
	info2, err := os.Stat(filepath.Join(tmpDir, "main.py"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mode := info2.Mode().Perm()
	if mode == 0 {
		t.Errorf("extracted file has mode 0, expected it to be normalized to 0644")
	}
	if mode&0o400 == 0 {
		t.Errorf("extracted file is not readable (mode %o)", mode)
	}
}

func TestExtractZipEntry_PreservesValidMode(t *testing.T) {
	// Build a zip where the file has a proper mode set.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	info := &zip.FileHeader{Name: "script.sh"}
	info.SetMode(0o755)
	f, err := w.CreateHeader(info)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("#!/bin/sh\necho hi")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	for _, entry := range r.File {
		if err := extractZipEntry(tmpDir, entry); err != nil {
			t.Fatalf("extractZipEntry: %v", err)
		}
	}

	info2, err := os.Stat(filepath.Join(tmpDir, "script.sh"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mode := info2.Mode().Perm()
	if mode&0o755 != 0o755 {
		t.Errorf("expected mode to preserve 0755, got %o", mode)
	}
}
