package runner

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ValidateImage checks that the requested Docker image is present in the
// allowlist. If the allowlist is empty, all images are rejected. The
// comparison is case-sensitive and requires an exact match.
func ValidateImage(image string, allowlist []string) error {
	if image == "" {
		return fmt.Errorf("image name is required")
	}
	if len(allowlist) == 0 {
		return fmt.Errorf("image allowlist is empty; no images are permitted")
	}

	for _, allowed := range allowlist {
		if image == allowed {
			return nil
		}
	}

	return fmt.Errorf(
		"image %q is not in the allowlist (allowed: %s)",
		image,
		strings.Join(allowlist, ", "),
	)
}

// ParseMemoryLimit converts a Kubernetes-style memory string to bytes.
// Supported suffixes:
//
//	Ki  — kibibytes (1024)
//	Mi  — mebibytes (1024^2)
//	Gi  — gibibytes (1024^3)
//
// A plain integer is treated as bytes.
func ParseMemoryLimit(limit string) (int64, error) {
	limit = strings.TrimSpace(limit)
	if limit == "" {
		return 0, fmt.Errorf("empty memory limit")
	}

	type suffixDef struct {
		suffix     string
		multiplier int64
	}

	// Check longer suffixes first to avoid partial matches.
	suffixes := []suffixDef{
		{"Gi", 1 << 30},
		{"Mi", 1 << 20},
		{"Ki", 1 << 10},
	}

	for _, s := range suffixes {
		if strings.HasSuffix(limit, s.suffix) {
			numStr := strings.TrimSuffix(limit, s.suffix)
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid numeric part %q in memory limit %q: %w", numStr, limit, err)
			}
			if n <= 0 {
				return 0, fmt.Errorf("memory limit must be positive, got %q", limit)
			}
			return n * s.multiplier, nil
		}
	}

	// No suffix — treat as raw bytes.
	n, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory limit %q: %w", limit, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("memory limit must be positive, got %q", limit)
	}
	return n, nil
}

// ParseCPULimit converts a fractional CPU string (e.g. "0.5", "1", "2")
// into a Docker CPUQuota value in microseconds per 100ms period. Docker
// uses CPUPeriod (default 100000 microseconds = 100ms) and CPUQuota to
// enforce CPU limits:
//
//	0.5 CPU  ->  CPUQuota = 50000   (50ms of 100ms period)
//	1 CPU    -> CPUQuota = 100000  (100ms of 100ms period)
//	2 CPUs   -> CPUQuota = 200000  (200ms of 100ms period)
func ParseCPULimit(limit string) (int64, error) {
	limit = strings.TrimSpace(limit)
	if limit == "" {
		return 0, fmt.Errorf("empty CPU limit")
	}

	cpu, err := strconv.ParseFloat(limit, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU limit %q: %w", limit, err)
	}
	if cpu <= 0 {
		return 0, fmt.Errorf("CPU limit must be positive, got %q", limit)
	}

	// CPUPeriod is 100000 microseconds (100ms).
	const cpuPeriod = 100000
	quota := int64(math.Round(cpu * cpuPeriod))
	if quota < 1000 {
		// Docker requires a minimum quota of 1000 microseconds.
		quota = 1000
	}

	return quota, nil
}
