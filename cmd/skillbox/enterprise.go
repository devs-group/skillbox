package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/devs-group/skillbox/internal/cli"
)

// --------------------------------------------------------------------
// skillbox login
// --------------------------------------------------------------------

func newLoginCmd() *cobra.Command {
	var (
		inviteCode string
		hydraURL   string
		clientID   string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Skillbox registry via device authorization",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Start device auth flow.
			dar, err := cli.StartDeviceAuth(hydraURL, clientID)
			if err != nil {
				return err
			}

			// 2. Show user code and open browser.
			fmt.Printf("Your one-time code is: %s\n", dar.UserCode)
			fmt.Printf("Open this URL to authenticate: %s\n", dar.VerificationURI)
			if dar.VerificationURIComplete != "" {
				fmt.Printf("Or open: %s\n", dar.VerificationURIComplete)
			}
			fmt.Println("Waiting for authentication...")

			// 3. Poll for token.
			tok, err := cli.PollForToken(hydraURL, clientID, dar.DeviceCode, dar.Interval)
			if err != nil {
				return err
			}

			// 4. Fetch user info from /v1/users/me.
			userInfo, statusCode, err := fetchUserMe(flagServer, tok.AccessToken)
			if err != nil && statusCode != http.StatusUnauthorized {
				return fmt.Errorf("fetch user info: %w", err)
			}

			// 5. If user not found and invite code provided, redeem invite.
			if statusCode == http.StatusUnauthorized && inviteCode != "" {
				if err := redeemInvite(flagServer, tok.AccessToken, inviteCode); err != nil {
					return fmt.Errorf("redeem invite: %w", err)
				}
				// Re-fetch user info after redeeming.
				userInfo, _, err = fetchUserMe(flagServer, tok.AccessToken)
				if err != nil {
					return fmt.Errorf("fetch user info after invite: %w", err)
				}
			} else if statusCode == http.StatusUnauthorized {
				return fmt.Errorf("user not found — use --invite <code> to redeem an invitation")
			}

			// 6. Save credentials.
			creds := &cli.Credentials{
				AccessToken:  tok.AccessToken,
				RefreshToken: tok.RefreshToken,
				ExpiresAt:    time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second),
				Email:        userInfo.Email,
				TenantID:     userInfo.TenantID,
			}
			if err := cli.SaveCredentials(creds); err != nil {
				return err
			}

			fmt.Printf("Logged in as %s (tenant: %s)\n", creds.Email, creds.TenantID)
			return nil
		},
	}

	cmd.Flags().StringVar(&inviteCode, "invite", "", "Invitation code to redeem")
	cmd.Flags().StringVar(&hydraURL, "hydra-url", envOrDefault("SKILLBOX_HYDRA_URL", "http://localhost:4444"), "Hydra public URL")
	cmd.Flags().StringVar(&clientID, "client-id", "skillbox-cli", "OAuth2 client ID")

	return cmd
}

// --------------------------------------------------------------------
// skillbox logout
// --------------------------------------------------------------------

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.ClearCredentials(); err != nil {
				return err
			}
			fmt.Println("Logged out successfully")
			return nil
		},
	}
}

// --------------------------------------------------------------------
// skillbox add
// --------------------------------------------------------------------

func newAddCmd() *cobra.Command {
	var (
		global  bool
		force   bool
		version string
	)

	cmd := &cobra.Command{
		Use:   "add <skill_name>",
		Short: "Install a skill from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName := args[0]

			// 1. Ensure the user is logged in.
			creds, err := cli.LoadCredentials()
			if err != nil {
				return fmt.Errorf("not logged in: %w — run `skillbox login` first", err)
			}

			// 2. Check if already installed (unless --force).
			if !force && cli.IsInstalled(skillName) {
				return fmt.Errorf("skill %q is already installed — use --force to reinstall", skillName)
			}

			// 3. Fetch skill metadata from marketplace.
			skillMeta, err := fetchSkillMeta(flagServer, creds.AccessToken, skillName)
			if err != nil {
				return fmt.Errorf("fetch skill metadata: %w", err)
			}

			// 4. Check approval status.
			if !skillMeta.Approved {
				if err := requestApproval(flagServer, creds.AccessToken, skillName); err != nil {
					return fmt.Errorf("request approval: %w", err)
				}
				fmt.Printf("Approval requested. Run `skillbox add %s` again after admin approves.\n", skillName)
				return nil
			}

			// 5. Install the skill (placeholder SKILL.md for now).
			installPath := cli.InstallPath(skillName, global)
			installDir := filepath.Dir(installPath)
			if err := os.MkdirAll(installDir, 0700); err != nil {
				return fmt.Errorf("create skill directory: %w", err)
			}

			// Write a placeholder SKILL.md.
			useVersion := version
			if useVersion == "" {
				useVersion = skillMeta.Version
			}
			content := fmt.Sprintf("---\nname: %s\nversion: %s\ndescription: %s\n---\n\n# %s\n\nInstalled from registry.\n",
				skillName, useVersion, skillMeta.Description, skillName)

			if err := os.WriteFile(installPath, []byte(content), 0600); err != nil {
				return fmt.Errorf("write SKILL.md: %w", err)
			}

			// 6. Update lock file.
			scope := "project"
			if global {
				scope = "global"
			}
			if err := cli.AddToLockFile(cli.InstalledSkill{
				Name:        skillName,
				Version:     useVersion,
				Provider:    skillMeta.Provider,
				Scope:       scope,
				Path:        installPath,
				InstalledAt: time.Now().UTC(),
			}); err != nil {
				return fmt.Errorf("update lock file: %w", err)
			}

			fmt.Printf("Installed %s to %s\n", skillName, installPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Install skill globally (~/.claude/skills/)")
	cmd.Flags().BoolVar(&force, "force", false, "Reinstall even if already installed")
	cmd.Flags().StringVar(&version, "version", "", "Skill version to install")

	return cmd
}

// --------------------------------------------------------------------
// skillbox list (installed skills)
// --------------------------------------------------------------------

func newListInstalledCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List locally installed skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			lf, err := cli.LoadLockFile()
			if err != nil {
				return err
			}

			if len(lf.Skills) == 0 {
				fmt.Println("No skills installed.")
				return nil
			}

			if flagOutput == "json" {
				return printJSON(lf.Skills)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tPROVIDER\tSCOPE\tINSTALLED_AT") //nolint:errcheck
			for _, s := range lf.Skills {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", //nolint:errcheck
					s.Name, s.Version, s.Provider, s.Scope, s.InstalledAt.Format(time.RFC3339))
			}
			return w.Flush()
		},
	}
}

// --------------------------------------------------------------------
// skillbox remove
// --------------------------------------------------------------------

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <skill_name>",
		Short: "Remove an installed skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			skillName := args[0]

			lf, err := cli.LoadLockFile()
			if err != nil {
				return err
			}

			// Find the skill in the lock file.
			var found *cli.InstalledSkill
			for i := range lf.Skills {
				if lf.Skills[i].Name == skillName {
					found = &lf.Skills[i]
					break
				}
			}
			if found == nil {
				return fmt.Errorf("skill %q is not installed", skillName)
			}

			// Remove the skill directory.
			skillDir := filepath.Dir(found.Path)
			if err := os.RemoveAll(skillDir); err != nil {
				return fmt.Errorf("remove skill directory: %w", err)
			}

			// Update lock file.
			if err := cli.RemoveFromLockFile(skillName); err != nil {
				return fmt.Errorf("update lock file: %w", err)
			}

			fmt.Printf("Removed %s\n", skillName)
			return nil
		},
	}
}

// --------------------------------------------------------------------
// HTTP helpers for JWT-authenticated API calls
// --------------------------------------------------------------------

type userMeResponse struct {
	Email    string `json:"email"`
	TenantID string `json:"tenant_id"`
}

type skillMetaResponse struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Provider    string `json:"provider"`
	Approved    bool   `json:"approved"`
}

func fetchUserMe(serverURL, token string) (*userMeResponse, int, error) {
	url := strings.TrimRight(serverURL, "/") + "/v1/users/me"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var u userMeResponse
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, resp.StatusCode, err
	}
	return &u, resp.StatusCode, nil
}

func redeemInvite(serverURL, token, inviteCode string) error {
	url := strings.TrimRight(serverURL, "/") + "/v1/invites/redeem"
	payload := fmt.Sprintf(`{"code":%q}`, inviteCode)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func fetchSkillMeta(serverURL, token, skillName string) (*skillMetaResponse, error) {
	url := strings.TrimRight(serverURL, "/") + "/v1/marketplace/skills/" + skillName
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var meta skillMetaResponse
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// --------------------------------------------------------------------
// skillbox search
// --------------------------------------------------------------------

func newSearchCmd() *cobra.Command {
	var page int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search GitHub for skills",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")

			// Call the Skillbox API's GitHub search endpoint.
			u := fmt.Sprintf("%s/v1/github/search?q=%s&page=%d", flagServer, url.QueryEscape(query), page)

			resp, err := http.Get(u) //nolint:gosec
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("search failed (HTTP %d): %s", resp.StatusCode, string(body))
			}

			var result struct {
				Results []struct {
					Name      string `json:"name"`
					RepoOwner string `json:"repo_owner"`
					RepoName  string `json:"repo_name"`
					FilePath  string `json:"file_path"`
					Stars     int    `json:"stars"`
					HTMLURL   string `json:"html_url"`
				} `json:"results"`
				TotalCount int  `json:"total_count"`
				HasMore    bool `json:"has_more"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			if flagOutput == "json" {
				return printJSON(result)
			}

			// Table output.
			if len(result.Results) == 0 {
				fmt.Println("No skills found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tREPO\tSTARS\tURL") //nolint:errcheck
			for _, r := range result.Results {
				fmt.Fprintf(w, "%s\t%s/%s\t%d\t%s\n", r.Name, r.RepoOwner, r.RepoName, r.Stars, r.HTMLURL) //nolint:errcheck
			}
			w.Flush() //nolint:errcheck

			if result.HasMore {
				fmt.Printf("\nShowing page %d of results (%d total). Use --page %d for more.\n", page, result.TotalCount, page+1)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number for search results")
	return cmd
}

func requestApproval(serverURL, token, skillName string) error {
	url := strings.TrimRight(serverURL, "/") + "/v1/approvals"
	payload := fmt.Sprintf(`{"skill_name":%q}`, skillName)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
