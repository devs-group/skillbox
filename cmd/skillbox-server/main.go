package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devs-group/skillbox/internal/api"
	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/runner"
	"github.com/devs-group/skillbox/internal/sandbox"
	"github.com/devs-group/skillbox/internal/scanner"
	"github.com/devs-group/skillbox/internal/store"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Set up structured logging
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	slog.Info("starting skillbox server",
		"version", "dev",
		"port", cfg.APIPort,
	)

	// Initialize database
	db, err := store.New(cfg.DBDSN)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close() //nolint:errcheck

	// Initialize skill registry (MinIO)
	reg, err := registry.New(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3BucketSkills, cfg.S3UseSSL)
	if err != nil {
		slog.Error("failed to initialize skill registry", "error", err)
		os.Exit(1)
	}

	// Initialize artifact collector (MinIO)
	collector, err := artifacts.New(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3BucketExecs, cfg.S3UseSSL)
	if err != nil {
		slog.Error("failed to initialize artifact collector", "error", err)
		os.Exit(1)
	}

	// Initialize OpenSandbox client
	sbClient := sandbox.New(cfg.OpenSandboxURL, cfg.OpenSandboxAPIKey, nil)

	// Clean up orphaned sandboxes from previous runs
	if err := runner.CleanupOrphans(context.Background(), sbClient); err != nil {
		slog.Warn("orphan cleanup failed", "error", err)
	}

	// Initialize session manager for sandbox shell API
	sessMgr := sandbox.NewSessionManager(sbClient, db, collector, cfg)

	// Initialize security scanner
	var sc scanner.Scanner
	if cfg.ScannerEnabled {
		var llmCfg *scanner.LLMConfig
		if cfg.ScannerLLMEnabled {
			llmCfg = &scanner.LLMConfig{
				APIKey:        cfg.ScannerLLMAPIKey,
				Model:         cfg.ScannerLLMModel,
				Timeout:       cfg.ScannerLLMTimeout,
				MaxConcurrent: cfg.ScannerLLMMaxConcurrent,
			}
		}
		sc = scanner.New(cfg.ScannerTimeout, slog.Default(), llmCfg)
		slog.Info("security scanner enabled", "timeout", cfg.ScannerTimeout, "llm_enabled", cfg.ScannerLLMEnabled)
	} else {
		sc = &scanner.NoopScanner{}
		slog.Warn("security scanner is DISABLED — uploads are not scanned")
	}

	// Initialize runner
	r := runner.New(cfg, sbClient, reg, db, collector)

	// Build router
	router := api.NewRouter(cfg, db, r, reg, sc, sessMgr, collector)

	// Create HTTP server
	srv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start background session sandbox cleanup goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sessMgr.Cleanup(context.Background(), cfg.SandboxSessionTTL)
			}
		}
	}()

	// Start HTTP server
	go func() {
		slog.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down servers...")

	// Stop HTTP server gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Sync and destroy all managed session sandboxes
	sessMgr.Shutdown(shutdownCtx)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}
	slog.Info("servers stopped")
}
