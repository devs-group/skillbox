package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"

	"github.com/devs-group/skillbox/internal/api"
	skillboxgrpc "github.com/devs-group/skillbox/internal/api/grpc"
	"github.com/devs-group/skillbox/internal/artifacts"
	"github.com/devs-group/skillbox/internal/config"
	"github.com/devs-group/skillbox/internal/registry"
	"github.com/devs-group/skillbox/internal/runner"
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
	defer db.Close()

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

	// Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(
		client.WithHost(cfg.DockerHost),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		slog.Error("failed to create docker client", "error", err)
		os.Exit(1)
	}
	defer dockerClient.Close()

	// Clean up orphaned containers from previous runs
	if err := runner.CleanupOrphans(context.Background(), dockerClient); err != nil {
		slog.Warn("orphan cleanup failed", "error", err)
	}

	// Initialize runner
	r := runner.New(cfg, dockerClient, reg, db, collector)

	// Build router
	router := api.NewRouter(cfg, db, r, reg)

	// Create HTTP server
	srv := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Create gRPC server
	grpcSrv := skillboxgrpc.NewServer(r, db, reg)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start HTTP server
	go func() {
		slog.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	// Start gRPC server
	go func() {
		grpcAddr := ":" + cfg.GRPCPort
		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			slog.Error("failed to listen for grpc", "addr", grpcAddr, "error", err)
			os.Exit(1)
		}
		slog.Info("grpc server listening", "addr", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			slog.Error("grpc server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down servers...")

	// Stop gRPC server gracefully
	grpcSrv.Stop()

	// Stop HTTP server gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}
	slog.Info("servers stopped")
}
