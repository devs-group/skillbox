package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateSandboxPath validates that a path is safe for sandbox operations.
// Cleans the path first to normalize traversal sequences, then verifies
// it starts with /sandbox/session/ and contains no "..".
func ValidateSandboxPath(p string) error {
	if p == "" {
		return fmt.Errorf("invalid sandbox path: path is empty")
	}
	cleaned := filepath.Clean(p)
	if !strings.HasPrefix(cleaned, "/sandbox/session") || strings.Contains(cleaned, "..") {
		return fmt.Errorf("invalid sandbox path: must start with /sandbox/session and not contain '..': %q", p)
	}
	return nil
}
