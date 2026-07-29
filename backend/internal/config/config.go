package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress         string
	MySQLDSN              string
	RedisAddress          string
	RedisPassword         string
	RedisDB               int
	MinIOEndpoint         string
	MinIOPublicEndpoint   string
	MinIORegion           string
	MinIOAccessKey        string
	MinIOSecretKey        string
	MinIOUseTLS           bool
	MinIOPublicUseTLS     bool
	PublicURL             string
	MediaBaseURL          string
	Environment           string
	DataEncryptionKey     string
	SportsAPIBaseURL      string
	SportsAPIKey          string
	SportsLiveInterval    time.Duration
	SportsCatalogInterval time.Duration
	SportsOddsInterval    time.Duration
	LegacyAuthCode        string
	LegacyTablePrefix     string
	ShutdownGrace         time.Duration
}

func Load() (Config, error) {
	redisDB, err := strconv.Atoi(env("V2_REDIS_DB", "0"))
	if err != nil || redisDB < 0 {
		return Config{}, fmt.Errorf("V2_REDIS_DB must be a non-negative integer")
	}
	graceSeconds, err := strconv.Atoi(env("V2_SHUTDOWN_GRACE_SECONDS", "15"))
	if err != nil || graceSeconds < 1 || graceSeconds > 120 {
		return Config{}, fmt.Errorf("V2_SHUTDOWN_GRACE_SECONDS must be between 1 and 120")
	}

	minioTLS, err := strconv.ParseBool(env("V2_MINIO_USE_TLS", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("V2_MINIO_USE_TLS must be true or false")
	}
	minioPublicTLS, err := strconv.ParseBool(env("V2_MINIO_PUBLIC_USE_TLS", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("V2_MINIO_PUBLIC_USE_TLS must be true or false")
	}
	environment := env("V2_ENV", "development")
	minioAccessKey := env("V2_MINIO_ACCESS_KEY", "clawlocal")
	minioSecretKey := env("V2_MINIO_SECRET_KEY", "claw-local-minio-password")
	if environment == "production" &&
		(os.Getenv("V2_MINIO_ACCESS_KEY") == "" || os.Getenv("V2_MINIO_SECRET_KEY") == "") {
		return Config{}, fmt.Errorf("V2_MINIO_ACCESS_KEY and V2_MINIO_SECRET_KEY are required in production")
	}
	dataEncryptionKey := env("V2_DATA_ENCRYPTION_KEY", "claw-local-data-encryption-key")
	if environment == "production" && strings.TrimSpace(os.Getenv("V2_DATA_ENCRYPTION_KEY")) == "" {
		return Config{}, fmt.Errorf("V2_DATA_ENCRYPTION_KEY is required in production")
	}
	sportsLiveInterval, err := durationEnv("V2_SPORTS_LIVE_INTERVAL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	sportsCatalogInterval, err := durationEnv("V2_SPORTS_CATALOG_INTERVAL", 6*time.Hour)
	if err != nil {
		return Config{}, err
	}
	sportsOddsInterval, err := durationEnv("V2_SPORTS_ODDS_INTERVAL", 3*time.Hour)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddress:         env("V2_LISTEN_ADDR", ":8080"),
		MySQLDSN:              strings.TrimSpace(os.Getenv("V2_MYSQL_DSN")),
		RedisAddress:          env("V2_REDIS_ADDR", "redis-v2:6379"),
		RedisPassword:         os.Getenv("V2_REDIS_PASSWORD"),
		RedisDB:               redisDB,
		MinIOEndpoint:         env("V2_MINIO_ENDPOINT", "minio:9000"),
		MinIOPublicEndpoint:   env("V2_MINIO_PUBLIC_ENDPOINT", "127.0.0.1:29000"),
		MinIORegion:           env("V2_MINIO_REGION", "us-east-1"),
		MinIOAccessKey:        minioAccessKey,
		MinIOSecretKey:        minioSecretKey,
		MinIOUseTLS:           minioTLS,
		MinIOPublicUseTLS:     minioPublicTLS,
		PublicURL:             strings.TrimRight(env("V2_PUBLIC_URL", "http://127.0.0.1:28080"), "/"),
		MediaBaseURL:          strings.TrimRight(env("V2_MEDIA_BASE_URL", "/media"), "/"),
		Environment:           environment,
		DataEncryptionKey:     dataEncryptionKey,
		SportsAPIBaseURL:      strings.TrimRight(env("V2_SPORTS_API_BASE_URL", "https://v3.football.api-sports.io"), "/"),
		SportsAPIKey:          strings.TrimSpace(os.Getenv("V2_SPORTS_API_KEY")),
		SportsLiveInterval:    sportsLiveInterval,
		SportsCatalogInterval: sportsCatalogInterval,
		SportsOddsInterval:    sportsOddsInterval,
		LegacyAuthCode:        os.Getenv("V2_LEGACY_AUTHCODE"),
		LegacyTablePrefix:     env("V2_LEGACY_TABLE_PREFIX", "cmf_"),
		ShutdownGrace:         time.Duration(graceSeconds) * time.Second,
	}
	if cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("V2_MYSQL_DSN is required")
	}
	return cfg, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < time.Minute {
		return 0, fmt.Errorf("%s must be a duration of at least one minute", key)
	}
	return parsed, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
