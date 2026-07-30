package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/apprelease"
	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/home"
	"github.com/zllyxr/live_claw/backend/internal/im"
	"github.com/zllyxr/live_claw/backend/internal/invite"
	"github.com/zllyxr/live_claw/backend/internal/live"
	"github.com/zllyxr/live_claw/backend/internal/lottery"
	"github.com/zllyxr/live_claw/backend/internal/payment"
	"github.com/zllyxr/live_claw/backend/internal/server"
	"github.com/zllyxr/live_claw/backend/internal/servicehost"
	"github.com/zllyxr/live_claw/backend/internal/sports"
	"github.com/zllyxr/live_claw/backend/internal/storage"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func main() {
	logger := servicehost.Logger("api")
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

	authService := auth.New(dependencies.DB, auth.Options{
		LegacyAuthCode: cfg.LegacyAuthCode, LegacyTablePrefix: cfg.LegacyTablePrefix,
	})
	walletService := wallet.New(dependencies.DB)
	paymentService, err := payment.New(
		dependencies.DB, walletService, cfg.DataEncryptionKey, cfg.PublicURL,
		payment.Options{},
	)
	if err != nil {
		logger.Error("initialize payment service", "error", err)
		os.Exit(1)
	}
	imService := im.New(dependencies.DB, dependencies.Redis)
	imService.SetMediaBaseURL(cfg.MediaBaseURL)
	apiServer := server.New(
		dependencies.DB, dependencies.Redis, authService,
		home.New(home.NewSQLLoader(dependencies.DB, cfg.MediaBaseURL), dependencies.Redis, logger),
		apprelease.New(dependencies.DB, storageService),
		lottery.New(dependencies.DB, walletService, cfg.MediaBaseURL),
		sports.New(dependencies.DB, walletService),
		live.New(dependencies.DB, dependencies.Redis),
		invite.New(dependencies.DB),
		storageService, walletService, paymentService, imService, cfg.MediaBaseURL, cfg.PublicURL,
		cfg.Environment, cfg.DataEncryptionKey, logger,
	)
	err = servicehost.Serve(
		ctx, "api", servicehost.Address("V2_API_LISTEN_ADDR", cfg.ListenAddress),
		apiServer.Handler(), cfg.ShutdownGrace, logger,
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
