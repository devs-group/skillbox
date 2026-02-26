package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/devs-group/skillbox/internal/skill"
	"github.com/devs-group/skillbox/internal/version"
	skillbox "github.com/devs-group/skillbox/sdks/go"
)

// Global flag values, populated by the root command's persistent flags.
var (
	flagServer string
	flagAPIKey string
	flagTenant string
	flagOutput string
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// --------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------

// newClient creates a Skillbox SDK client from the global flag values.
func newClient() *skillbox.Client {
	var opts []skillbox.Option
	if flagTenant != "" {
		opts = append(opts, skillbox.WithTenant(flagTenant))
	}
	return skillbox.New(flagServer, flagAPIKey, opts...)
}

// contextWithTimeout returns a context with a 5-minute timeout.
func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

// printJSON marshals v to indented JSON and writes it to stdout.
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// packageDir creates a zip archive of dir and writes it to the current
// working directory as {name}-{version}.zip. It returns the path to the
// created zip file and the parsed skill metadata.
func packageDir(dir string) (zipPath string, sk *skill.Skill, err error) {
	skillMD := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return "", nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	sk, err = skill.ParseSkillMD(data)
	if err != nil {
		return "", nil, fmt.Errorf("parse SKILL.md: %w", err)
	}

	zipName := fmt.Sprintf("%s-%s.zip", sk.Name, sk.Version)
	zipPath, err = filepath.Abs(zipName)
	if err != nil {
		return "", nil, err
	}

	f, err := os.Create(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("create zip file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, err
	}

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		if info.IsDir() {
			_, err := zw.Create(rel + "/")
			return err
		}

		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(w, r)
		return err
	})
	if err != nil {
		return "", nil, fmt.Errorf("create zip archive: %w", err)
	}

	return zipPath, sk, nil
}

// --------------------------------------------------------------------
// Root command
// --------------------------------------------------------------------

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "skillbox",
		Short:         "Skillbox CLI — manage and run skills",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVarP(&flagServer, "server", "s", envOrDefault("SKILLBOX_SERVER_URL", "http://localhost:8080"), "Skillbox server URL")
	rootCmd.PersistentFlags().StringVarP(&flagAPIKey, "api-key", "k", os.Getenv("SKILLBOX_API_KEY"), "API key for authentication")
	rootCmd.PersistentFlags().StringVarP(&flagTenant, "tenant", "t", os.Getenv("SKILLBOX_TENANT_ID"), "Tenant ID for multi-tenancy")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "", "Output format: json, table")

	rootCmd.AddCommand(
		newRunCmd(),
		newSkillCmd(),
		newExecCmd(),
		newHealthCmd(),
		newVersionCmd(),
	)

	return rootCmd
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// --------------------------------------------------------------------
// skillbox run
// --------------------------------------------------------------------

func newRunCmd() *cobra.Command {
	var (
		input    string
		ver      string
		download string
		envVars  []string
	)

	cmd := &cobra.Command{
		Use:   "run <skill>",
		Short: "Run a skill synchronously and print the result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newClient()
			ctx, cancel := contextWithTimeout()
			defer cancel()

			req := skillbox.RunRequest{
				Skill:   args[0],
				Version: ver,
			}

			if input != "" {
				req.Input = json.RawMessage(input)
			}

			if len(envVars) > 0 {
				req.Env = make(map[string]string, len(envVars))
				for _, kv := range envVars {
					parts := strings.SplitN(kv, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid --env value %q: must be KEY=VALUE", kv)
					}
					req.Env[parts[0]] = parts[1]
				}
			}

			result, err := client.Run(ctx, req)
			if err != nil {
				return err
			}

			if err := printJSON(result); err != nil {
				return err
			}

			if download != "" && result.HasFiles() {
				fmt.Fprintf(os.Stderr, "Downloading files to %s...\n", download)
				if err := client.DownloadFiles(ctx, result, download); err != nil {
					return fmt.Errorf("download files: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Files downloaded to %s\n", download)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&input, "input", "", "JSON input payload")
	cmd.Flags().StringVar(&ver, "version", "latest", "Skill version to run")
	cmd.Flags().StringVar(&download, "download", "", "Directory to download output files to")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "Environment variables as KEY=VALUE (repeatable)")

	return cmd
}

// --------------------------------------------------------------------
// skillbox skill (parent)
// --------------------------------------------------------------------

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage skills: package, push, list, lint",
	}

	cmd.AddCommand(
		newSkillPackageCmd(),
		newSkillPushCmd(),
		newSkillListCmd(),
		newSkillLintCmd(),
	)

	return cmd
}

// --------------------------------------------------------------------
// skillbox skill package
// --------------------------------------------------------------------

func newSkillPackageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "package <dir>",
		Short: "Package a skill directory into a zip archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]

			// Validate SKILL.md exists.
			skillMD := filepath.Join(dir, "SKILL.md")
			if _, err := os.Stat(skillMD); os.IsNotExist(err) {
				return fmt.Errorf("SKILL.md not found in %s", dir)
			}

			zipPath, sk, err := packageDir(dir)
			if err != nil {
				return err
			}

			fmt.Printf("Packaged %s v%s -> %s\n", sk.Name, sk.Version, zipPath)
			return nil
		},
	}
}

// --------------------------------------------------------------------
// skillbox skill push
// --------------------------------------------------------------------

func newSkillPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push <dir|zip>",
		Short: "Push a skill to the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newClient()
			ctx, cancel := contextWithTimeout()
			defer cancel()

			target := args[0]
			var zipPath string
			var skillName string

			info, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("stat %s: %w", target, err)
			}

			if info.IsDir() {
				// Package the directory first.
				zp, sk, err := packageDir(target)
				if err != nil {
					return err
				}
				zipPath = zp
				skillName = sk.Name
			} else {
				// Treat as a zip file.
				zipPath = target
				// Extract skill name from zip filename (name-version.zip).
				base := strings.TrimSuffix(filepath.Base(target), ".zip")
				if idx := strings.LastIndex(base, "-"); idx > 0 {
					skillName = base[:idx]
				} else {
					skillName = base
				}
			}

			if err := client.RegisterSkill(ctx, zipPath); err != nil {
				return err
			}

			fmt.Printf("Pushed skill %q successfully\n", skillName)
			return nil
		},
	}
}

// --------------------------------------------------------------------
// skillbox skill list
// --------------------------------------------------------------------

func newSkillListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newClient()
			ctx, cancel := contextWithTimeout()
			defer cancel()

			skills, err := client.ListSkills(ctx)
			if err != nil {
				return err
			}

			if flagOutput == "json" {
				return printJSON(skills)
			}

			// Default to table output.
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION")
			for _, s := range skills {
				fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Version, s.Description)
			}
			return w.Flush()
		},
	}
}

// --------------------------------------------------------------------
// skillbox skill lint
// --------------------------------------------------------------------

// defaultAllowedImages is the set of images considered safe by default.
var defaultAllowedImages = map[string]bool{
	"python:3.12-slim": true,
	"python:3.11-slim": true,
	"node:20-slim":     true,
	"node:22-slim":     true,
	"bash:5":           true,
}

func newSkillLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <dir>",
		Short: "Lint a skill directory for common issues",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			skillMD := filepath.Join(dir, "SKILL.md")

			data, err := os.ReadFile(skillMD)
			if err != nil {
				return fmt.Errorf("read SKILL.md: %w", err)
			}

			// Parse the frontmatter (but we do our own validation below).
			sk, parseErr := skill.ParseSkillMD(data)

			hasErrors := false
			check := func(name string, ok bool, msg string) {
				if ok {
					fmt.Printf("  PASS  %s\n", name)
				} else {
					fmt.Printf("  FAIL  %s: %s\n", name, msg)
					hasErrors = true
				}
			}

			fmt.Println("Linting", dir)

			if parseErr != nil {
				// If parsing failed entirely, report what we can.
				check("parse", false, parseErr.Error())
				return fmt.Errorf("lint failed")
			}

			check("name", sk.Name != "", "name is required")
			check("version", sk.Version != "", "version is required")
			check("description", sk.Description != "", "description is required")

			// Check entrypoint existence.
			entrypoints := []string{
				"scripts/main.py",
				"scripts/main.js",
				"scripts/main.sh",
				"scripts/run.py",
			}
			entrypointFound := false
			for _, ep := range entrypoints {
				if _, err := os.Stat(filepath.Join(dir, ep)); err == nil {
					entrypointFound = true
					break
				}
			}
			check("entrypoint", entrypointFound, fmt.Sprintf("no entrypoint found; expected one of: %s", strings.Join(entrypoints, ", ")))

			// Check image against allowlist.
			image := sk.DefaultImage()
			imageOK := image == "" || defaultAllowedImages[image]
			check("image", imageOK, fmt.Sprintf("image %q is not in the default allowlist", image))

			if hasErrors {
				return fmt.Errorf("lint failed")
			}
			fmt.Println("All checks passed")
			return nil
		},
	}
}

// --------------------------------------------------------------------
// skillbox exec (parent)
// --------------------------------------------------------------------

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Manage executions",
	}

	cmd.AddCommand(newExecLogsCmd())
	return cmd
}

// --------------------------------------------------------------------
// skillbox exec logs
// --------------------------------------------------------------------

func newExecLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <execution-id>",
		Short: "Fetch and print logs for an execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newClient()
			ctx, cancel := contextWithTimeout()
			defer cancel()

			logs, err := client.GetExecutionLogs(ctx, args[0])
			if err != nil {
				return err
			}

			fmt.Print(logs)
			return nil
		},
	}
}

// --------------------------------------------------------------------
// skillbox health
// --------------------------------------------------------------------

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check if the Skillbox server is healthy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newClient()
			ctx, cancel := contextWithTimeout()
			defer cancel()

			if err := client.Health(ctx); err != nil {
				return fmt.Errorf("server is unhealthy: %w", err)
			}

			fmt.Println("Server is healthy")
			return nil
		},
	}
}

// --------------------------------------------------------------------
// skillbox version
// --------------------------------------------------------------------

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("skillbox %s\n", version.Version)
			fmt.Printf("  commit:     %s\n", version.Commit)
			fmt.Printf("  built:      %s\n", version.BuildTime)
			return nil
		},
	}
}
