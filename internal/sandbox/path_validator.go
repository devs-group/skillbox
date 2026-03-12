package sandbox

import (
	"fmt"
	"strings"
)

// ValidateSandboxPath validates that a path is safe for sandbox operations.
// All paths must be absolute, start with /sandbox/session/, and contain no "..".
func ValidateSandboxPath(p string) error {
	if p == "" || !strings.HasPrefix(p, "/sandbox/session") || strings.Contains(p, "..") {
		return fmt.Errorf("invalid sandbox path: must start with /sandbox/session and not contain '..': %q", p)
	}
	return nil
}
