package scanner

import "errors"

// ErrBlocked is returned when a skill is rejected by the security scanner.
var ErrBlocked = errors.New("scanner: skill blocked by security scan")

// ErrScanFailed is returned when the scanner encounters an infrastructure
// failure (e.g. regex compilation, file read error). Callers should treat
// this as a reason to reject the upload (fail closed).
var ErrScanFailed = errors.New("scanner: scan failed")
