package gameserver

import (
	"math"
	"testing"
	"time"
)

func TestFishingFireLimiterAllowsNetworkBurstWithoutLosingRateControl(t *testing.T) {
	var limiter fishingFireLimiter
	now := time.Unix(1_000, 0)
	if !limiter.allow(now) || !limiter.allow(now) {
		t.Fatal("initial two-shot burst should be accepted")
	}
	if limiter.allow(now) {
		t.Fatal("third simultaneous shot should be rate limited")
	}
	if !limiter.allow(now.Add(112 * time.Millisecond)) {
		t.Fatal("token should refill at the configured sustained rate")
	}
}

func TestFishingRayExitPointReachesArenaBoundary(t *testing.T) {
	tests := []struct {
		name  string
		seat  int
		angle float64
		wantX float64
		wantY float64
	}{
		{name: "bottom left upward", seat: 0, angle: -math.Pi / 2, wantX: 430, wantY: 0},
		{name: "bottom right upward", seat: 1, angle: -math.Pi / 2, wantX: 850, wantY: 0},
		{name: "top left downward", seat: 2, angle: math.Pi / 2, wantX: 430, wantY: 720},
		{name: "top right downward", seat: 3, angle: math.Pi / 2, wantX: 850, wantY: 720},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			x, y := fishingRayExitPoint(test.seat, test.angle)
			if math.Abs(x-test.wantX) > 0.001 || math.Abs(y-test.wantY) > 0.001 {
				t.Fatalf("exit=(%.3f, %.3f), want=(%.3f, %.3f)", x, y, test.wantX, test.wantY)
			}
		})
	}
}

func TestNearestFishingTargetCanBeAbsentWithoutInvalidatingShot(t *testing.T) {
	fishes := []fishingFish{{ID: "off-axis", X: 1200, Y: 690, Multiplier: 2}}
	if target := nearestFishingTarget(0, -math.Pi/2, fishes); target != nil {
		t.Fatalf("expected no target, received %#v", target)
	}
	x, y := fishingRayExitPoint(0, -math.Pi/2)
	if math.Abs(x-430) > 0.001 || math.Abs(y) > 0.001 {
		t.Fatalf("empty-water shot did not resolve to arena edge: %.3f, %.3f", x, y)
	}
}
