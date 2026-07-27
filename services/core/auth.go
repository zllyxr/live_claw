package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrUnauthorized = errors.New("登录状态失效，请重新登录")

type Authenticator struct {
	db    *sql.DB
	redis *redis.Client
}

func NewAuthenticator(db *sql.DB, redisClient *redis.Client) *Authenticator {
	return &Authenticator{db: db, redis: redisClient}
}

func (a *Authenticator) Verify(ctx context.Context, uid int64, token string) error {
	if uid < 1 || token == "" {
		return ErrUnauthorized
	}

	var found int
	err := a.db.QueryRowContext(ctx,
		"SELECT 1 FROM cmf_user_token WHERE user_id=? AND token=? AND expire_time>? LIMIT 1",
		uid, token, time.Now().Unix(),
	).Scan(&found)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("verify database token: %w", err)
	}

	if a.redis == nil {
		return ErrUnauthorized
	}
	raw, err := a.redis.Get(ctx, token).Result()
	if err != nil {
		return ErrUnauthorized
	}
	var payload map[string]any
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return ErrUnauthorized
	}
	redisUID := valueInt64(payload["id"])
	if redisUID == 0 {
		redisUID = valueInt64(payload["uid"])
	}
	if redisUID != uid {
		return ErrUnauthorized
	}
	return nil
}

func valueInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case json.Number:
		out, _ := typed.Int64()
		return out
	case string:
		out, _ := strconv.ParseInt(typed, 10, 64)
		return out
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}
