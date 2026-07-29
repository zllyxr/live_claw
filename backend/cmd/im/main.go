package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/im"
	"github.com/zllyxr/live_claw/backend/internal/servicehost"
)

func main() {
	logger := servicehost.Logger("im")
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
	authService := auth.New(dependencies.DB, auth.Options{
		LegacyAuthCode: cfg.LegacyAuthCode, LegacyTablePrefix: cfg.LegacyTablePrefix,
	})
	imService := im.New(dependencies.DB, dependencies.Redis)
	imService.SetMediaBaseURL(cfg.MediaBaseURL)
	imHandler := im.NewHandler(imService, authService, dependencies.Redis)
	dispatcher := im.NewDispatcher(dependencies.DB, dependencies.Redis, logger)
	go func() {
		if dispatchErr := dispatcher.Run(ctx); dispatchErr != nil &&
			!errors.Is(dispatchErr, context.Canceled) {
			logger.Error("IM dispatcher stopped", "error", dispatchErr)
			stop()
		}
	}()
	handler := servicehost.Handler("im", dependencies, logger, imHandler.Register)
	err = servicehost.Serve(
		ctx, "im", servicehost.Address("V2_IM_LISTEN_ADDR", cfg.ListenAddress),
		handler, cfg.ShutdownGrace, logger,
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("im stopped", "error", err)
		os.Exit(1)
	}
}
