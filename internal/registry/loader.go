package registry

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/devs-group/skillbox/internal/skill"
)

// LoadedSkill contains a fully validated skill ready for execution. It
// includes the parsed SKILL.md, the path to the extracted files on disk,
// the entrypoint script, and metadata about dependency files.
type LoadedSkill struct {
	// Skill holds the parsed SKILL.md metadata and instructions.
	Skill *skill.Skill

	// Dir is the absolute path to the temporary directory containing the
	// extracted skill files. The caller is responsible for removing this
	// directory after use.
	Dir string

	// Entrypoint is the relative path (from Dir) to the main script
	// that should be executed (e.g. "main.py", "main.js", "main.sh").
	Entrypoint string

	// HasRequirements is true when a requirements.txt file is present
	// in the extracted archive, indicating Python dependencies.
	HasRequirements bool

	// HasPackageJSON is true when a package.json file is present in
	// the extracted archive, indicating Node.js dependencies.
	HasPackageJSON bool
}

// knownEntrypoints lists the accepted entrypoint filenames in priority order.
var knownEntrypoints = []string{
	"main.py",
	"run.py",
	"main.js",
	"main.sh",
}

// LoadSkill downloads a skill archive from the registry, extracts it to a
// temporary directory, validates its contents, and returns a LoadedSkill.
//
// The caller MUST remove LoadedSkill.Dir when done (e.g. via os.RemoveAll).
//
// Validation includes:
//   - SKILL.md must exist and parse successfully
//   - A recognized entrypoint script must exist
//   - Zip entries are checked for path traversal (zip slip) attacks
func LoadSkill(ctx context.Context, reg *Registry, tenantID, skillName, version string) (*LoadedSkill, error) {
	// Download the zip from the registry.
	rc, err := reg.Download(ctx, tenantID, skillName, version)
	if err != nil {
		return nil, fmt.Errorf("downloading skill %s/%s@%s: %w", tenantID, skillName, version, err)
	}
	defer rc.Close()

	// Read the entire zip into memory so we can use zip.NewReader.
	zipBytes, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("reading skill archive: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("opening skill archive: %w", err)
	}

	// Create a temporary directory to extract into.
	tmpDir, err := os.MkdirTemp("", "skillbox-skill-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}

	// If anything fails after this point, clean up the temp directory.
	success := false
	defer func() {
		if !success {
			os.RemoveAll(tmpDir)
		}
	}()

	// Extract all files, guarding against zip slip.
	for _, f := range zipReader.File {
		if err := extractZipEntry(tmpDir, f); err != nil {
			return nil, fmt.Errorf("extracting %q: %w", f.Name, err)
		}
	}

	// Validate that SKILL.md exists and parse it.
	skillMDPath := filepath.Join(tmpDir, "SKILL.md")
	skillMDData, err := os.ReadFile(skillMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("SKILL.md not found in archive")
		}
		return nil, fmt.Errorf("reading SKILL.md: %w", err)
	}

	parsedSkill, err := skill.ParseSkillMD(skillMDData)
	if err != nil {
		return nil, fmt.Errorf("parsing SKILL.md: %w", err)
	}

	// Find a recognized entrypoint script.
	entrypoint, err := findEntrypoint(tmpDir)
	if err != nil {
		return nil, err
	}

	// Check for dependency files.
	hasReqs := fileExists(filepath.Join(tmpDir, "requirements.txt"))
	hasPkg := fileExists(filepath.Join(tmpDir, "package.json"))

	success = true
	return &LoadedSkill{
		Skill:           parsedSkill,
		Dir:             tmpDir,
		Entrypoint:      entrypoint,
		HasRequirements: hasReqs,
		HasPackageJSON:  hasPkg,
	}, nil
}

// extractZipEntry extracts a single zip entry to the target directory,
// creating intermediate directories as needed. It rejects any entry whose
// path contains ".." components or resolves outside the target directory
// (zip slip protection).
func extractZipEntry(targetDir string, f *zip.File) error {
	name := filepath.FromSlash(f.Name)

	// Reject entries with path traversal components.
	if strings.Contains(f.Name, "..") {
		return fmt.Errorf("illegal path %q: contains '..' (potential zip slip attack)", f.Name)
	}

	destPath := filepath.Join(targetDir, name)
	cleanDest := filepath.Clean(destPath)
	cleanTarget := filepath.Clean(targetDir) + string(filepath.Separator)

	// Final check: the resolved path must be under the target directory.
	if !strings.HasPrefix(cleanDest, cleanTarget) && cleanDest != filepath.Clean(targetDir) {
		return fmt.Errorf("illegal path %q: resolves outside target directory (zip slip attack)", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(destPath, 0o755)
	}

	// Ensure the parent directory exists.
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", f.Name, err)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening zip entry: %w", err)
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// findEntrypoint searches the extracted skill directory for a recognized
// entrypoint script. It checks both the root and a "scripts/" subdirectory.
func findEntrypoint(dir string) (string, error) {
	// Check root directory first.
	for _, name := range knownEntrypoints {
		if fileExists(filepath.Join(dir, name)) {
			return name, nil
		}
	}

	// Check scripts/ subdirectory.
	for _, name := range knownEntrypoints {
		candidate := filepath.Join("scripts", name)
		if fileExists(filepath.Join(dir, candidate)) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(
		"no recognized entrypoint found in archive (expected one of: %s)",
		strings.Join(knownEntrypoints, ", "),
	)
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
