package scheduler

import (
	"encoding/json"
	"testing"
)

func TestSportsOddsNormalization(t *testing.T) {
	if got, ok := scaledSportsOdds("1.95"); !ok || got != 1_950_000 {
		t.Fatalf("unexpected scaled odds: %d %v", got, ok)
	}
	if _, ok := scaledSportsOdds("1"); ok {
		t.Fatal("odds at or below one must be rejected")
	}
	code, name := normalizedCorrectScore("2:1")
	if code != "CS_2_1" || name != "2:1" {
		t.Fatalf("unexpected score option: %s %s", code, name)
	}
	if code, _ = normalizedCorrectScore("Any Other Score"); code != "OTHER" {
		t.Fatalf("unexpected other score option: %s", code)
	}
	if code, name = normalizedTotalGoals("Over 2.5"); code != "OVER_2_5" || name != "大于 2.5" {
		t.Fatalf("unexpected total goals option: %s %s", code, name)
	}
	if code, _ = normalizedTotalGoals("Over 2"); code != "" {
		t.Fatalf("whole-number total must not be accepted without push support: %s", code)
	}
	if code, name = normalizedDoubleChance("Home/Draw"); code != "HOME_OR_DRAW" || name != "主胜或平" {
		t.Fatalf("unexpected double chance option: %s %s", code, name)
	}
	if !totalGoalsOptionWins("OVER_2_5", 3) || totalGoalsOptionWins("UNDER_2_5", 3) {
		t.Fatal("total goals result mapping is incorrect")
	}
}

func TestNormalizedSportsStatus(t *testing.T) {
	cases := map[string]string{
		"NS": "NS", "1H": "LIVE", "HT": "HT", "AET": "FT", "PST": "CANCELLED",
	}
	for raw, expected := range cases {
		if got := normalizedSportsStatus(raw); got != expected {
			t.Fatalf("%s: expected %s, got %s", raw, expected, got)
		}
	}
}

func TestSportsMatchIsTerminal(t *testing.T) {
	for _, status := range []string{"FT", "AET", "PEN", "CANC", "CANCELLED", "PST", "ABD", "AWD", "WO"} {
		if !sportsMatchIsTerminal(status) {
			t.Fatalf("expected %s to be terminal", status)
		}
	}
	for _, status := range []string{"NS", "LIVE", "HT"} {
		if sportsMatchIsTerminal(status) {
			t.Fatalf("expected %s to remain mutable", status)
		}
	}
}

func TestQualifiedSportsFixtureIDsRequireUsableOdds(t *testing.T) {
	var items []apiFootballOdds
	if err := json.Unmarshal([]byte(`[
		{"fixture":{"id":11},"bookmakers":[]},
		{"fixture":{"id":22},"bookmakers":[{"bets":[
			{"id":1,"name":"Match Winner","values":[
				{"value":"Home","odd":"2.10"},
				{"value":"Draw","odd":"3.20"},
				{"value":"Away","odd":"3.40"}
			]}
		]}]},
		{"fixture":{"id":33},"bookmakers":[{"bets":[
			{"id":1,"name":"Match Winner","values":[
				{"value":"Home","odd":"1.00"},
				{"value":"Draw","odd":"1.00"},
				{"value":"Away","odd":"1.00"}
			]}
		]}]}
	]`), &items); err != nil {
		t.Fatal(err)
	}
	ids := qualifiedSportsFixtureIDs(items)
	if len(ids) != 1 {
		t.Fatalf("expected exactly one odds-backed fixture, got %d", len(ids))
	}
	if _, ok := ids[22]; !ok {
		t.Fatal("fixture with complete usable odds was rejected")
	}
	for _, rejected := range []int64{11, 33} {
		if _, ok := ids[rejected]; ok {
			t.Fatalf("fixture %d without usable odds was accepted", rejected)
		}
	}
}
