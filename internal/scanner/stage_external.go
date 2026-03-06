package scanner

import (
	"archive/zip"
	"context"
	"fmt"
	"log/slog"
)

const stageNameExternal = "external_scanner"

// ExternalScanner is the interface for pluggable external scanning engines
// like ClamAV or YARA. Each implementation scans individual file contents
// extracted from the skill ZIP and returns findings or an infrastructure error.
//
// Error contract follows the same pattern as Scanner:
//   - (findings, nil): scan completed — may contain BLOCK/FLAG findings
//   - (nil, error): infrastructure failure — caller fails closed
type ExternalScanner interface {
	// ScanFile inspects a single file's content and returns security findings.
	// filePath is the relative path within the ZIP for reporting.
	ScanFile(ctx context.Context, filePath string, data []byte) ([]Finding, error)

	// Name returns the scanner engine name for logging (e.g., "clamav", "yara").
	Name() string
}

// externalStage wraps an ExternalScanner into the internal stage interface
// so it can be added to the tier2 pipeline.
type externalStage struct {
	scanner ExternalScanner
	logger  *slog.Logger
}

func newExternalStage(ext ExternalScanner, logger *slog.Logger) *externalStage {
	return &externalStage{scanner: ext, logger: logger}
}

func (es *externalStage) name() string {
	return stageNameExternal
}

// run executes the external scanner against every file in the ZIP.
// The priorFlags parameter is ignored — external scanners always run
// when present in the pipeline.
func (es *externalStage) run(ctx context.Context, zr *zip.Reader, _ []Finding) ([]Finding, error) {
	var findings []Finding

	for _, f := range zr.File {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s: %w", stageNameExternal, ctx.Err())
		}
		if f.FileInfo().IsDir() {
			continue
		}

		data, err := readZipFileContent(f)
		if err != nil {
			continue // Skip unreadable files (same as other stages).
		}

		fileFindings, err := es.scanner.ScanFile(ctx, f.Name, data)
		if err != nil {
			// External scanner unavailable → fail closed.
			return nil, fmt.Errorf("%s (%s): %w", stageNameExternal, es.scanner.Name(), err)
		}
		findings = append(findings, fileFindings...)
	}

	if len(findings) > 0 {
		es.logger.Info("external scanner findings",
			"scanner", es.scanner.Name(),
			"findings_count", len(findings),
		)
	}

	return findings, nil
}
