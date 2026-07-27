package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		logger.Error("open mysql", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(80)
	db.SetMaxIdleConns(20)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer startupCancel()
	if err := db.PingContext(startupCtx); err != nil {
		logger.Error("mysql unavailable", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
	defer redisClient.Close()

	auth := NewAuthenticator(db, redisClient)
	lottery := NewLotteryService(db, logger)
	sports := NewSportsService(db, cfg, logger)
	openIM := NewOpenIMClient(cfg, logger)
	hotUpdate := NewHotUpdateService(cfg.HotUpdateDir, cfg.HotUpdateBaseURL, logger)
	miniGame := NewMiniGameService(db, cfg.MiniGameSecret, cfg.MiniGameTableCount, logger)
	gameWallet := NewMiniGameWalletService(db, cfg.MiniGameSecret, logger)
	if err := gameWallet.EnsureSchema(startupCtx); err != nil {
		logger.Error("ensure minigame wallet schema", "error", err)
		os.Exit(1)
	}
	api := NewAPIServer(db, auth, lottery, sports, openIM, hotUpdate, miniGame, gameWallet, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go lottery.Run(ctx, cfg.LotteryTick)
	go sports.Run(ctx, cfg.SportsCollectInterval)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		logger.Info("core service started", "listen", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
