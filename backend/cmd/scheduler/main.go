package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/scheduler"
	"github.com/zllyxr/live_claw/backend/internal/servicehost"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func main() {
	logger := servicehost.Logger("scheduler")
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dependencies, err := servicehost.Open(ctx, cfg, true)
	if err != nil {
		logger.Error("open dependencies", "error", err)
		os.Exit(1)
	}
	defer dependencies.Close()
	runner := scheduler.New(
		dependencies.DB, dependencies.Redis, wallet.New(dependencies.DB), logger,
	)
	runner.ConfigureSportsSync(scheduler.SportsSyncConfig{
		BaseURL: cfg.SportsAPIBaseURL, APIKey: cfg.SportsAPIKey,
		LiveInterval:    cfg.SportsLiveInterval,
		CatalogInterval: cfg.SportsCatalogInterval,
		OddsInterval:    cfg.SportsOddsInterval,
	})
	if cfg.SportsAPIKey == "" {
		logger.Warn("sports upstream sync disabled: V2_SPORTS_API_KEY is not configured")
	}
	logger.Info("scheduler started")
	if err = runner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("scheduler stopped", "error", err)
		os.Exit(1)
	}
	logger.Info("scheduler stopped")
}
