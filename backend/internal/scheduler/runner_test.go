package scheduler

import (
	"encoding/json"
	"math"
	"testing"
)

func TestWinnerOptionIDs(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"winner_option_ids": []int64{10, 20, 10}})
	result, err := winnerOptionIDs(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected unique winners, got %#v", result)
	}
	if _, exists := result[10]; !exists {
		t.Fatal("winner 10 is missing")
	}
}

func TestScaledPayout(t *testing.T) {
	payout, err := scaledPayout(125, 2_500_000)
	if err != nil {
		t.Fatal(err)
	}
	if payout != 312 {
		t.Fatalf("expected floor payout 312, got %d", payout)
	}
	if _, err = scaledPayout(math.MaxInt64, math.MaxInt64); err == nil {
		t.Fatal("expected overflow error")
	}
}
