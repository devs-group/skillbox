package runner

import "errors"

// ErrNotImplemented is returned when the runner has not been implemented yet.
var ErrNotImplemented = errors.New("runner: not implemented")

// ErrSkillNotFound is returned when the requested skill cannot be found in the registry.
var ErrSkillNotFound = errors.New("runner: skill not found")

// ErrImageNotAllowed is returned when the skill's Docker image is not in the allowlist.
var ErrImageNotAllowed = errors.New("runner: image not in allowlist")

// ErrTimeout is returned when execution exceeds the configured timeout.
var ErrTimeout = errors.New("runner: execution timed out")

// ErrSkillNotAvailable is returned when a skill exists but is not in 'available' status.
var ErrSkillNotAvailable = errors.New("runner: skill not available")
