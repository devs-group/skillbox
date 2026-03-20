package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// InstalledSkill describes a skill that has been installed locally.
type InstalledSkill struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Provider    string    `json:"provider"`
	Scope       string    `json:"scope"` // "project" or "global"
	Path        string    `json:"path"`
	InstalledAt time.Time `json:"installed_at"`
}

// LockFile holds the set of locally installed skills.
type LockFile struct {
	Skills []InstalledSkill `json:"skills"`
}

// LockFilePath returns ~/.config/skillbox/skill-lock.json.
func LockFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillbox", "skill-lock.json")
}

// LoadLockFile reads the lock file. If the file does not exist an empty
// LockFile is returned with no error.
func LoadLockFile() (*LockFile, error) {
	data, err := os.ReadFile(LockFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &LockFile{}, nil
		}
		return nil, fmt.Errorf("read lock file: %w", err)
	}

	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("decode lock file: %w", err)
	}
	return &lf, nil
}

// SaveLockFile writes the lock file with 0600 permissions, creating parent
// directories (0700) as needed.
func SaveLockFile(lf *LockFile) error {
	p := LockFilePath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}
	return nil
}

// InstallPath returns the target directory for a skill. Project-scoped skills
// are stored under .claude/skills/<name>/SKILL.md relative to the current
// directory; global skills under ~/.claude/skills/<name>/SKILL.md.
func InstallPath(skillName string, global bool) string {
	if global {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".claude", "skills", skillName, "SKILL.md")
	}
	return filepath.Join(".claude", "skills", skillName, "SKILL.md")
}

// AddToLockFile adds or updates a skill entry in the lock file.
func AddToLockFile(skill InstalledSkill) error {
	lf, err := LoadLockFile()
	if err != nil {
		return err
	}

	found := false
	for i, s := range lf.Skills {
		if s.Name == skill.Name {
			lf.Skills[i] = skill
			found = true
			break
		}
	}
	if !found {
		lf.Skills = append(lf.Skills, skill)
	}

	return SaveLockFile(lf)
}

// RemoveFromLockFile removes a skill by name from the lock file.
func RemoveFromLockFile(skillName string) error {
	lf, err := LoadLockFile()
	if err != nil {
		return err
	}

	filtered := lf.Skills[:0]
	for _, s := range lf.Skills {
		if s.Name != skillName {
			filtered = append(filtered, s)
		}
	}
	lf.Skills = filtered

	return SaveLockFile(lf)
}

// IsInstalled checks whether a skill with the given name is present in the
// lock file.
func IsInstalled(skillName string) bool {
	lf, err := LoadLockFile()
	if err != nil {
		return false
	}
	for _, s := range lf.Skills {
		if s.Name == skillName {
			return true
		}
	}
	return false
}
