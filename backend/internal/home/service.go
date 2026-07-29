package home

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Loader interface {
	Banners(context.Context) ([]Banner, error)
	LiveRooms(context.Context) ([]LiveRoom, error)
	SportsMatches(context.Context) ([]SportsMatch, error)
	LotteryGames(context.Context) ([]LotteryGame, error)
	FishingVenues(context.Context) ([]FishingVenue, error)
	Wallet(context.Context, int64) (WalletSummary, error)
}

type Service struct {
	loader Loader
	cache  *redis.Client
	logger *slog.Logger
	now    func() time.Time
}

func New(loader Loader, cache *redis.Client, logger *slog.Logger) *Service {
	return &Service{loader: loader, cache: cache, logger: logger, now: time.Now}
}

func (s *Service) Dashboard(ctx context.Context, user *UserSummary) Dashboard {
	now := s.now()
	result := Dashboard{ServerTime: now.Unix(), User: user}
	var wait sync.WaitGroup
	wait.Add(5)

	go func() {
		defer wait.Done()
		items, stale, err := loadCached(ctx, s.cache, "home:v2:banners", 5*time.Minute, s.loader.Banners)
		result.Banners = makeSection(s.logger, "banners", items, stale, err, now)
	}()
	go func() {
		defer wait.Done()
		items, stale, err := loadCached(ctx, s.cache, "home:v2:live", 15*time.Second, s.loader.LiveRooms)
		result.Live = makeSection(s.logger, "live", items, stale, err, now)
	}()
	go func() {
		defer wait.Done()
		items, stale, err := loadCached(ctx, s.cache, "home:v2:sports", 10*time.Second, s.loader.SportsMatches)
		result.Sports = makeSection(s.logger, "sports", items, stale, err, now)
	}()
	go func() {
		defer wait.Done()
		items, stale, err := loadCached(ctx, s.cache, "home:v2:lottery", 5*time.Second, s.loader.LotteryGames)
		result.Lottery = makeSection(s.logger, "lottery", items, stale, err, now)
	}()
	go func() {
		defer wait.Done()
		items, stale, err := loadCached(ctx, s.cache, "home:v2:fishing", time.Minute, s.loader.FishingVenues)
		result.Fishing = makeSection(s.logger, "fishing", items, stale, err, now)
	}()
	wait.Wait()

	if user != nil && user.ID > 0 {
		wallet, err := s.loader.Wallet(ctx, user.ID)
		if err != nil {
			s.logger.Error("load home wallet", "user_id", user.ID, "error", err)
		} else {
			result.Wallet = &wallet
		}
	}
	return result
}

func makeSection[T any](logger *slog.Logger, name string, items []T, stale bool, err error, now time.Time) Section[T] {
	if items == nil {
		items = make([]T, 0)
	}
	if err != nil {
		logger.Error("load home section", "section", name, "error", err)
		return Section[T]{Status: "degraded", Items: items, Error: "temporarily_unavailable", UpdatedAt: now.Unix()}
	}
	status := "ok"
	if stale {
		status = "stale"
	}
	return Section[T]{Status: status, Items: items, UpdatedAt: now.Unix()}
}

func loadCached[T any](
	ctx context.Context,
	cache *redis.Client,
	key string,
	ttl time.Duration,
	load func(context.Context) ([]T, error),
) ([]T, bool, error) {
	if cache != nil {
		if raw, err := cache.Get(ctx, key).Bytes(); err == nil {
			var items []T
			if json.Unmarshal(raw, &items) == nil {
				return items, false, nil
			}
		}
	}

	items, err := load(ctx)
	if err != nil {
		if cache != nil {
			if raw, cacheErr := cache.Get(ctx, key+":stale").Bytes(); cacheErr == nil {
				var staleItems []T
				if json.Unmarshal(raw, &staleItems) == nil {
					return staleItems, true, nil
				}
			}
		}
		return nil, false, err
	}
	if items == nil {
		items = make([]T, 0)
	}
	if cache != nil {
		if raw, marshalErr := json.Marshal(items); marshalErr == nil {
			pipe := cache.Pipeline()
			pipe.Set(ctx, key, raw, ttl)
			pipe.Set(ctx, key+":stale", raw, 24*time.Hour)
			_, _ = pipe.Exec(ctx)
		}
	}
	return items, false, nil
}
