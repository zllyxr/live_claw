package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/game"
	"github.com/zllyxr/live_claw/backend/internal/gameserver"
	"github.com/zllyxr/live_claw/backend/internal/servicehost"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func main() {
	logger := servicehost.Logger("game")
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
	gameService := game.NewService(
		dependencies.DB, game.NewMatchmaker(dependencies.Redis), wallet.New(dependencies.DB),
	)
	gameServer := gameserver.New(authService, gameService, logger)
	handler := servicehost.Handler("game", dependencies, logger, gameServer.Register)
	err = servicehost.Serve(
		ctx, "game", servicehost.Address("V2_GAME_LISTEN_ADDR", cfg.ListenAddress),
		handler, cfg.ShutdownGrace, logger,
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("game stopped", "error", err)
		os.Exit(1)
	}
}
