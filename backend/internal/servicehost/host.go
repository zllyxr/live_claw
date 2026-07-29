package servicehost

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

type Dependencies struct {
	DB    *sql.DB
	Redis *redis.Client
}

func Logger(service string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).
		With("service", service)
}

func Open(ctx context.Context, cfg config.Config, withRedis bool) (Dependencies, error) {
	db, err := database.Open(ctx, cfg.MySQLDSN)
	if err != nil {
		return Dependencies{}, err
	}
	dependencies := Dependencies{DB: db}
	if !withRedis {
		return dependencies, nil
	}
	dependencies.Redis = redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddress, Password: cfg.RedisPassword, DB: cfg.RedisDB,
		DialTimeout: 3 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second,
		MaintNotificationsConfig: &maintnotifications.Config{Mode: maintnotifications.ModeDisabled},
	})
	if err = dependencies.Redis.Ping(ctx).Err(); err != nil {
		dependencies.Close()
		return Dependencies{}, err
	}
	return dependencies, nil
}

func (d Dependencies) Close() {
	if d.Redis != nil {
		_ = d.Redis.Close()
	}
	if d.DB != nil {
		_ = d.DB.Close()
	}
}

func Address(environmentKey, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(environmentKey)); value != "" {
		return value
	}
	return fallback
}

func Handler(
	service string,
	dependencies Dependencies,
	logger *slog.Logger,
	register func(*http.ServeMux),
) http.Handler {
	mux := http.NewServeMux()
	health := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status := http.StatusOK
		databaseStatus := "up"
		redisStatus := "disabled"
		if dependencies.DB == nil || dependencies.DB.PingContext(ctx) != nil {
			status = http.StatusServiceUnavailable
			databaseStatus = "down"
		}
		if dependencies.Redis != nil {
			redisStatus = "up"
			if dependencies.Redis.Ping(ctx).Err() != nil {
				status = http.StatusServiceUnavailable
				redisStatus = "down"
			}
		}
		httpx.JSON(w, status, map[string]any{
			"status":   map[bool]string{true: "ok", false: "unhealthy"}[status == http.StatusOK],
			"service":  service,
			"database": databaseStatus,
			"redis":    redisStatus,
		})
	}
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /readyz", health)
	register(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "接口不存在")
	})

	var handler http.Handler = mux
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		handlerRequest := r
		mux.ServeHTTP(w, handlerRequest)
		logger.Info("http request",
			"request_id", httpx.RequestID(handlerRequest.Context()),
			"method", handlerRequest.Method,
			"path", handlerRequest.URL.Path,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
	handler = httpx.Recover(logger, handler)
	handler = httpx.RequestContext(handler)
	return handler
}

func Serve(
	ctx context.Context,
	service string,
	address string,
	handler http.Handler,
	shutdownGrace time.Duration,
	logger *slog.Logger,
) error {
	httpServer := &http.Server{
		Addr: address, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      65 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	result := make(chan error, 1)
	go func() {
		logger.Info("service listening", "address", address)
		err := httpServer.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		result <- err
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return httpServer.Shutdown(shutdownContext)
	}
}
