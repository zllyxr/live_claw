package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/admin"
	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/live"
	"github.com/zllyxr/live_claw/backend/internal/servicehost"
	"github.com/zllyxr/live_claw/backend/internal/storage"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func main() {
	logger := servicehost.Logger("admin")
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dependencies, err := servicehost.Open(ctx, cfg, false)
	if err != nil {
		logger.Error("open dependencies", "error", err)
		os.Exit(1)
	}
	defer dependencies.Close()
	storageService, err := storage.New(cfg)
	if err != nil {
		logger.Error("initialize minio", "error", err)
		os.Exit(1)
	}
	storageContext, cancelStorage := context.WithTimeout(ctx, 10*time.Second)
	err = storageService.EnsureBuckets(storageContext)
	cancelStorage()
	if err != nil {
		logger.Error("ensure minio buckets", "error", err)
		os.Exit(1)
	}
	adminHandler, err := admin.New(
		dependencies.DB, adminauth.New(dependencies.DB), storageService,
		wallet.New(dependencies.DB), live.New(dependencies.DB, dependencies.Redis),
		cfg.MediaBaseURL, cfg.Environment,
	)
	if err != nil {
		logger.Error("initialize admin", "error", err)
		os.Exit(1)
	}
	handler := servicehost.Handler("admin", dependencies, logger, adminHandler.Register)
	err = servicehost.Serve(
		ctx, "admin", servicehost.Address("V2_ADMIN_LISTEN_ADDR", cfg.ListenAddress),
		handler, cfg.ShutdownGrace, logger,
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("admin stopped", "error", err)
		os.Exit(1)
	}
}
