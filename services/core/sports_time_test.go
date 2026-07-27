package main

import (
	"encoding/json"
	"testing"
)

func TestNormalizeSportsTimestampUsesSeconds(t *testing.T) {
	const seconds int64 = 1_785_054_600
	for _, input := range []int64{seconds, seconds * 1000, seconds * 1_000_000} {
		if got := normalizeSportsTimestamp(input); got != seconds {
			t.Fatalf("normalize %d = %d, want %d", input, got, seconds)
		}
	}
}

func TestSportsTimestampTextUsesBusinessTimezone(t *testing.T) {
	const timestamp int64 = 1_785_054_600
	if got := sportsTimestampText(timestamp, "2006-01-02 15:04:05"); got != "2026-07-26 16:30:00" {
		t.Fatalf("unexpected formatted timestamp %q", got)
	}
}

func TestSportsBetWindowCanReopenFutureMatch(t *testing.T) {
	match := sportsMatch{BetStatus: 2, SettleStatus: 0, Status: "NS", BetCloseTime: 2_000}
	if !sportsBetWindowOpen(match, 1_000) {
		t.Fatal("future pre-match event should be open even when a stale stored status is closed")
	}
	if effectiveSportsBetStatus(match, 1_000) != 1 {
		t.Fatal("effective status should repair stale closed state")
	}
}

func TestOddsScalarAcceptsStringsAndNumbers(t *testing.T) {
	var payload apiFootballOddsResponse
	raw := `{"results":1,"response":[{"fixture":{"id":1},"bookmakers":[{"id":1,"name":"x","bets":[{"id":1,"name":"Match Winner","values":[{"value":1,"odd":1.95},{"value":"Draw","odd":"3.2"}]}]}]}]}`
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode mixed scalar odds: %v", err)
	}
	values := payload.Response[0].Bookmakers[0].Bets[0].Values
	if string(values[0].Value) != "1" || string(values[0].Odd) != "1.95" {
		t.Fatalf("unexpected normalized values: %#v", values[0])
	}
}
