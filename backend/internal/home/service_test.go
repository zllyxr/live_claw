package home

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeLoader struct {
	sportsErr error
}

func (fakeLoader) Banners(context.Context) ([]Banner, error) {
	return []Banner{{ID: 1, Title: "活动"}}, nil
}

func (fakeLoader) LiveRooms(context.Context) ([]LiveRoom, error) {
	return []LiveRoom{{ID: 2, Provider: "douyin"}}, nil
}

func (loader fakeLoader) SportsMatches(context.Context) ([]SportsMatch, error) {
	return nil, loader.sportsErr
}

func (fakeLoader) LotteryGames(context.Context) ([]LotteryGame, error) {
	return []LotteryGame{{ID: 3, GameCode: "demo"}}, nil
}

func (fakeLoader) FishingVenues(context.Context) ([]FishingVenue, error) {
	return []FishingVenue{
		{VenueCode: "novice", Multiplier: 1, TableCount: 300, SeatsPerTable: 4},
		{VenueCode: "expert", Multiplier: 5, TableCount: 300, SeatsPerTable: 4},
		{VenueCode: "master", Multiplier: 10, TableCount: 300, SeatsPerTable: 4},
	}, nil
}

func (fakeLoader) Wallet(context.Context, int64) (WalletSummary, error) {
	return WalletSummary{Coin: 900, Frozen: 100}, nil
}

func TestDashboardSectionFailureIsIsolated(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := New(fakeLoader{sportsErr: errors.New("sports unavailable")}, nil, logger)
	service.now = func() time.Time { return time.Unix(1234, 0) }

	result := service.Dashboard(context.Background(), &UserSummary{ID: 10, Nickname: "用户"})

	if result.ServerTime != 1234 || result.User == nil || result.Wallet == nil {
		t.Fatalf("missing authenticated dashboard data: %#v", result)
	}
	if result.Sports.Status != "degraded" || result.Sports.Error != "temporarily_unavailable" {
		t.Fatalf("sports failure not isolated: %#v", result.Sports)
	}
	if result.Live.Status != "ok" || len(result.Live.Items) != 1 {
		t.Fatalf("live section should remain available: %#v", result.Live)
	}
	if result.Lottery.Status != "ok" || result.Fishing.Status != "ok" || result.Banners.Status != "ok" {
		t.Fatal("an unrelated section was degraded")
	}
}

func TestDashboardGuestDoesNotLoadWallet(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := New(fakeLoader{}, nil, logger)
	result := service.Dashboard(context.Background(), nil)
	if result.User != nil || result.Wallet != nil {
		t.Fatalf("guest response leaked user data: %#v", result)
	}
}

func TestLiveRoomUIDIsEncodedWithoutJavaScriptPrecisionLoss(t *testing.T) {
	const uid int64 = 1785252579710207004
	payload, err := json.Marshal(LiveRoom{UID: uid})
	if err != nil {
		t.Fatalf("marshal live room: %v", err)
	}
	if !strings.Contains(string(payload), `"uid":"1785252579710207004"`) {
		t.Fatalf("live uid must be encoded as a string, got %s", payload)
	}
}
