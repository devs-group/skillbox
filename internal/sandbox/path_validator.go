package sandbox

import (
	"fmt"
	"path"
	"strings"
)

// PathMode represents the allowed access mode for a sandbox path.
type PathMode int

const (
	PathModeRead PathMode = iota
	PathModeWrite
	PathModeReadWrite
)

// ValidateSandboxPath validates that a path is safe for sandbox operations.
// Rules:
//   - Must be absolute, starting with /sandbox/
//   - No ".." components allowed
//   - /sandbox/session/ → ReadWrite (workspace)
//   - /sandbox/scripts/ → Read only
//   - /sandbox/input/ → Read only
//   - /sandbox/out/ → Write (and read)
func ValidateSandboxPath(p string, mode PathMode) error {
	// Must be absolute.
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("sandbox path must be absolute, got %q", p)
	}

	// Clean the path and reject any ".." traversal.
	cleaned := path.Clean(p)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("sandbox path must not contain '..': %q", p)
	}

	// Must start with /sandbox/.
	if !strings.HasPrefix(cleaned, "/sandbox/") && cleaned != "/sandbox" {
		return fmt.Errorf("sandbox path must start with /sandbox/: %q", p)
	}

	// Determine the zone from the second path component.
	// e.g. /sandbox/session/foo → zone = "session"
	rel := strings.TrimPrefix(cleaned, "/sandbox/")
	zone := rel
	if idx := strings.Index(rel, "/"); idx >= 0 {
		zone = rel[:idx]
	}

	switch zone {
	case "session":
		// ReadWrite — any mode is allowed.
		return nil

	case "scripts", "input":
		// Read-only zones.
		if mode == PathModeWrite || mode == PathModeReadWrite {
			return fmt.Errorf("sandbox path %q is read-only (zone %q)", p, zone)
		}
		return nil

	case "out":
		// Write (and read) allowed.
		return nil

	default:
		return fmt.Errorf("sandbox path %q is outside allowed zones (session, scripts, input, out)", p)
	}
}
