package scanner

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// maxDecompressedSize is the total decompressed size cap (50MB).
	maxDecompressedSize uint64 = 50 << 20

	// maxEntryCount is the maximum number of files allowed in a ZIP.
	maxEntryCount = 500

	// maxCompressionRatio rejects files with suspiciously high compression ratios.
	maxCompressionRatio uint64 = 100
)

// nestedArchiveExts are extensions that indicate a nested archive.
var nestedArchiveExts = map[string]bool{
	".zip": true,
	".tar": true,
	".gz":  true,
	".tgz": true,
	".bz2": true,
	".xz":  true,
	".7z":  true,
	".rar": true,
}

// CheckZIPSafety performs pre-scan safety checks on a ZIP archive to prevent
// resource exhaustion attacks (zip bombs). It checks:
//
//  1. Total decompressed size cap (50MB)
//  2. Entry count limit (500 files)
//  3. Compression ratio per file (reject if any file > 100:1)
//  4. Nested archives (reject if ZIP contains .zip, .tar, .gz, .7z, etc.)
//
// Returns nil if all checks pass, or a descriptive error.
func CheckZIPSafety(zr *zip.Reader) error {
	if len(zr.File) > maxEntryCount {
		return fmt.Errorf("zip contains %d entries, exceeds limit of %d", len(zr.File), maxEntryCount)
	}

	var totalDecompressed uint64
	for _, f := range zr.File {
		// Reject path traversal entries.
		if strings.Contains(f.Name, "..") {
			return fmt.Errorf("zip contains path traversal entry: %s", f.Name)
		}

		// Reject symlinks.
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip contains symlink entry: %s", f.Name)
		}

		// Check nested archives.
		ext := strings.ToLower(filepath.Ext(f.Name))
		if nestedArchiveExts[ext] {
			return fmt.Errorf("zip contains nested archive: %s", f.Name)
		}

		// Skip directories.
		if f.FileInfo().IsDir() {
			continue
		}

		uncompressed := f.UncompressedSize64
		totalDecompressed += uncompressed

		if totalDecompressed > maxDecompressedSize {
			return fmt.Errorf("total decompressed size exceeds %d bytes", maxDecompressedSize)
		}

		// Check compression ratio.
		compressed := f.CompressedSize64
		if compressed > 0 && uncompressed/compressed > maxCompressionRatio {
			return fmt.Errorf("file %s has compression ratio %d:1, exceeds limit of %d:1",
				f.Name, uncompressed/compressed, maxCompressionRatio)
		}
	}

	return nil
}
