package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"reconix-cloud/internal/api"
	"reconix-cloud/internal/config"
	"reconix-cloud/internal/jobs"
	"reconix-cloud/internal/policy"
	"reconix-cloud/internal/storage"
)

func main() {
	cfg := config.Load()
	p, err := policy.New(cfg.TargetMode, cfg.AllowedDomains, cfg.AllowedCIDRs)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required for API")
		os.Exit(1)
	}
	db, err := storage.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer db.DB.Close()
	var store storage.Store = db
	var results storage.ResultStore = db
	manager := jobs.NewAPI(store, p)
	server := api.Server{Manager: manager, Store: store, Results: results, APIKey: cfg.APIKey}
	slog.Info("reconix cloud listening", "addr", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.Handler()); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
