package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CredentialPath returns ~/.config/skillbox/credentials.json.
func CredentialPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillbox", "credentials.json")
}

// Credentials holds the JWT tokens and associated metadata for the current session.
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email"`
	TenantID     string    `json:"tenant_id"`
}

// LoadCredentials reads credentials from CredentialPath. It returns an error if
// the file is missing or the access token has expired.
func LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(CredentialPath())
	if err != nil {
		return nil, fmt.Errorf("no credentials found: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("corrupt credentials file: %w", err)
	}

	if time.Now().After(creds.ExpiresAt) {
		return nil, fmt.Errorf("credentials expired at %s", creds.ExpiresAt.Format(time.RFC3339))
	}

	return &creds, nil
}

// SaveCredentials writes the credentials to CredentialPath with 0600 permissions.
// Parent directories are created with 0700.
func SaveCredentials(creds *Credentials) error {
	p := CredentialPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

// ClearCredentials deletes the credentials file.
func ClearCredentials() error {
	if err := os.Remove(CredentialPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsLoggedIn returns true if credentials exist and the token has not expired.
func IsLoggedIn() bool {
	_, err := LoadCredentials()
	return err == nil
}
