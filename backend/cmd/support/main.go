package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/servicehost"
	"github.com/zllyxr/live_claw/backend/internal/support"
	"github.com/zllyxr/live_claw/backend/internal/supportconsole"
)

func main() {
	logger := servicehost.Logger("support")
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
	authService := auth.New(dependencies.DB, auth.Options{
		LegacyAuthCode: cfg.LegacyAuthCode, LegacyTablePrefix: cfg.LegacyTablePrefix,
	})
	supportService := support.New(dependencies.DB)
	supportHandler := support.NewHandler(supportService, authService)
	consoleService := supportconsole.NewService(dependencies.DB, supportService)
	consoleHandler, err := supportconsole.NewHandler(
		adminauth.New(dependencies.DB), consoleService, cfg.Environment,
	)
	if err != nil {
		logger.Error("create support console", "error", err)
		os.Exit(1)
	}
	handler := servicehost.Handler("support", dependencies, logger, func(mux *http.ServeMux) {
		supportHandler.Register(mux)
		consoleHandler.Register(mux)
	})
	err = servicehost.Serve(
		ctx, "support", servicehost.Address("V2_SUPPORT_LISTEN_ADDR", cfg.ListenAddress),
		handler, cfg.ShutdownGrace, logger,
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("support stopped", "error", err)
		os.Exit(1)
	}
}
