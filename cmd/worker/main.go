package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"reconix-cloud/internal/config"
	"reconix-cloud/internal/jobs"
	"reconix-cloud/internal/policy"
	"reconix-cloud/internal/runner"
	"reconix-cloud/internal/storage"
	"syscall"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required for worker")
		os.Exit(1)
	}
	p, err := policy.New(cfg.TargetMode, cfg.AllowedDomains, cfg.AllowedCIDRs)
	if err != nil {
		slog.Error("invalid target policy", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := storage.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.DB.Close()
	worker := jobs.Worker{Store: db, Policy: p, Runner: runner.Subprocess{Binary: cfg.ReconixBinary, Config: cfg.ReconixConfig, Workdir: cfg.ReconixWorkdir, MaxOutput: cfg.MaxOutput}, MaxConcurrent: cfg.MaxConcurrent, MaxDuration: cfg.MaxDuration, PollInterval: cfg.PollInterval, StaleAfter: cfg.StaleAfter}
	slog.Info("reconix cloud worker started", "concurrency", cfg.MaxConcurrent)
	if err := worker.Run(ctx); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
	slog.Info("reconix cloud worker stopped")
}
