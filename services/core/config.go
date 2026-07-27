package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr             string
	MySQLDSN               string
	RedisAddr              string
	RedisPassword          string
	LotteryTick            time.Duration
	SportsCollectInterval  time.Duration
	SportsCatalogInterval  time.Duration
	SportsOddsInterval     time.Duration
	SportsAPIBaseURL       string
	SportsAPIKey           string
	OpenIMAPIURL           string
	OpenIMGatewayURL       string
	OpenIMSecret           string
	OpenIMAdminUserID      string
	OpenIMPublicAPIAddress string
	OpenIMPublicWSAddress  string
	HotUpdateDir           string
	HotUpdateBaseURL       string
	MiniGameSecret         string
	MiniGameTableCount     int
}

func loadConfig() Config {
	return Config{
		ListenAddr:             env("CORE_LISTEN_ADDR", ":8080"),
		MySQLDSN:               env("CORE_MYSQL_DSN", "claw:claw_dev_pwd@tcp(db:3306)/claw_live?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai&multiStatements=true"),
		RedisAddr:              env("CORE_REDIS_ADDR", "redis:6379"),
		RedisPassword:          env("CORE_REDIS_PASSWORD", ""),
		LotteryTick:            envDuration("CORE_LOTTERY_TICK", 500*time.Millisecond),
		SportsCollectInterval:  envDuration("CORE_SPORTS_COLLECT_INTERVAL", 30*time.Second),
		SportsCatalogInterval:  envDuration("CORE_SPORTS_CATALOG_INTERVAL", 15*time.Minute),
		SportsOddsInterval:     envDuration("CORE_SPORTS_ODDS_INTERVAL", 30*time.Minute),
		SportsAPIBaseURL:       strings.TrimRight(env("SPORTS_API_BASE_URL", "https://v3.football.api-sports.io"), "/"),
		SportsAPIKey:           strings.TrimSpace(os.Getenv("SPORTS_API_KEY")),
		OpenIMAPIURL:           strings.TrimRight(env("OPENIM_API_URL", "http://openim-server:10002"), "/"),
		OpenIMGatewayURL:       strings.TrimRight(env("OPENIM_GATEWAY_URL", "http://openim-server:10001"), "/"),
		OpenIMSecret:           env("OPENIM_SECRET", "openIM123"),
		OpenIMAdminUserID:      env("OPENIM_ADMIN_USER_ID", "imAdmin"),
		OpenIMPublicAPIAddress: env("OPENIM_PUBLIC_API_ADDRESS", "http://127.0.0.1:10002"),
		OpenIMPublicWSAddress:  env("OPENIM_PUBLIC_WS_ADDRESS", "ws://127.0.0.1:10001"),
		HotUpdateDir:           env("CORE_HOTUPDATE_DIR", "/data/wgt"),
		HotUpdateBaseURL:       strings.TrimRight(env("CORE_HOTUPDATE_BASE_URL", "http://127.0.0.1:18080/core-api"), "/"),
		MiniGameSecret:         env("CORE_MINIGAME_SECRET", "claw_minigame_dev_secret"),
		MiniGameTableCount:     envInt("CORE_MINIGAME_TABLE_COUNT", 1000),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
		return parsed
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
