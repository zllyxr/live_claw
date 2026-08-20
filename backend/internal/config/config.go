package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress                  string
	MySQLDSN                       string
	RedisAddress                   string
	RedisPassword                  string
	RedisDB                        int
	MinIOEndpoint                  string
	MinIOPublicEndpoint            string
	MinIORegion                    string
	MinIOAccessKey                 string
	MinIOSecretKey                 string
	MinIOUseTLS                    bool
	MinIOPublicUseTLS              bool
	PublicURL                      string
	MediaBaseURL                   string
	Environment                    string
	DataEncryptionKey              string
	RemoteAssistanceEnabled        bool
	RemoteAssistanceAllowedUserIDs []int64
	RustDeskIDServer               string
	RustDeskRelayServer            string
	RustDeskAPIServer              string
	RustDeskPublicKey              string
	SportsAPIBaseURL               string
	SportsAPIKey                   string
	SportsLiveInterval             time.Duration
	SportsCatalogInterval          time.Duration
	SportsOddsInterval             time.Duration
	LegacyAuthCode                 string
	LegacyTablePrefix              string
	ShutdownGrace                  time.Duration
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
	remoteAssistanceEnabled, err := strconv.ParseBool(env("V2_REMOTE_ASSISTANCE_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("V2_REMOTE_ASSISTANCE_ENABLED must be true or false")
	}
	remoteAllowedUserIDs, err := positiveIDListEnv("V2_REMOTE_ASSISTANCE_ALLOWED_USER_IDS")
	if err != nil {
		return Config{}, err
	}
	environment := env("V2_ENV", "development")
	minioAccessKey := env("V2_MINIO_ACCESS_KEY", "clawlocal")
	minioSecretKey := env("V2_MINIO_SECRET_KEY", "claw-local-minio-password")
	dataEncryptionKey := env("V2_DATA_ENCRYPTION_KEY", "claw-local-data-encryption-key")
	if environment == "production" {
		// Credentials are injected only into services that use them. Consumers
		// such as paymentconfig.NewCipher and storage.New reject empty values;
		// unrelated services can start without receiving these secrets.
		minioAccessKey = strings.TrimSpace(os.Getenv("V2_MINIO_ACCESS_KEY"))
		minioSecretKey = strings.TrimSpace(os.Getenv("V2_MINIO_SECRET_KEY"))
		dataEncryptionKey = strings.TrimSpace(os.Getenv("V2_DATA_ENCRYPTION_KEY"))
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
		ListenAddress:                  env("V2_LISTEN_ADDR", ":8080"),
		MySQLDSN:                       strings.TrimSpace(os.Getenv("V2_MYSQL_DSN")),
		RedisAddress:                   env("V2_REDIS_ADDR", "redis-v2:6379"),
		RedisPassword:                  os.Getenv("V2_REDIS_PASSWORD"),
		RedisDB:                        redisDB,
		MinIOEndpoint:                  env("V2_MINIO_ENDPOINT", "minio:9000"),
		MinIOPublicEndpoint:            env("V2_MINIO_PUBLIC_ENDPOINT", "127.0.0.1:29000"),
		MinIORegion:                    env("V2_MINIO_REGION", "us-east-1"),
		MinIOAccessKey:                 minioAccessKey,
		MinIOSecretKey:                 minioSecretKey,
		MinIOUseTLS:                    minioTLS,
		MinIOPublicUseTLS:              minioPublicTLS,
		PublicURL:                      strings.TrimRight(env("V2_PUBLIC_URL", "http://127.0.0.1:28080"), "/"),
		MediaBaseURL:                   strings.TrimRight(env("V2_MEDIA_BASE_URL", "/media"), "/"),
		Environment:                    environment,
		DataEncryptionKey:              dataEncryptionKey,
		RemoteAssistanceEnabled:        remoteAssistanceEnabled,
		RemoteAssistanceAllowedUserIDs: remoteAllowedUserIDs,
		RustDeskIDServer:               env("V2_RUSTDESK_ID_SERVER", "rd.tmpai2.com"),
		RustDeskRelayServer:            env("V2_RUSTDESK_RELAY_SERVER", "rd.tmpai2.com"),
		RustDeskAPIServer:              strings.TrimRight(env("V2_RUSTDESK_API_SERVER", "https://rd-admin.tmpai2.com"), "/"),
		RustDeskPublicKey:              strings.TrimSpace(os.Getenv("V2_RUSTDESK_PUBLIC_KEY")),
		SportsAPIBaseURL:               strings.TrimRight(env("V2_SPORTS_API_BASE_URL", "https://v3.football.api-sports.io"), "/"),
		SportsAPIKey:                   strings.TrimSpace(os.Getenv("V2_SPORTS_API_KEY")),
		SportsLiveInterval:             sportsLiveInterval,
		SportsCatalogInterval:          sportsCatalogInterval,
		SportsOddsInterval:             sportsOddsInterval,
		LegacyAuthCode:                 os.Getenv("V2_LEGACY_AUTHCODE"),
		LegacyTablePrefix:              env("V2_LEGACY_TABLE_PREFIX", "cmf_"),
		ShutdownGrace:                  time.Duration(graceSeconds) * time.Second,
	}
	if cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("V2_MYSQL_DSN is required")
	}
	return cfg, nil
}

func positiveIDListEnv(key string) ([]int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}
	result := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, item := range strings.Split(value, ",") {
		parsed, err := strconv.ParseInt(strings.TrimSpace(item), 10, 64)
		if err != nil || parsed < 1 {
			return nil, fmt.Errorf("%s must be a comma-separated list of positive user IDs", key)
		}
		if _, exists := seen[parsed]; exists {
			continue
		}
		seen[parsed] = struct{}{}
		result = append(result, parsed)
	}
	return result, nil
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
