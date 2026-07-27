package main

import (
	"errors"
	"strings"
	"testing"
)

func TestGenerateOpenCodeRespectsConfig(t *testing.T) {
	config := drawConfig{Count: 10, Min: 1, Max: 10, Unique: true, Pad: 2}
	for i := 0; i < 50; i++ {
		code, err := generateOpenCode(config)
		if err != nil {
			t.Fatalf("generate draw: %v", err)
		}
		if !validateDraw(code, config) {
			t.Fatalf("invalid draw %q", code)
		}
		for _, item := range strings.Split(code, ",") {
			if len(item) != 2 {
				t.Fatalf("draw item %q is not zero-padded", item)
			}
		}
	}
}

func TestLotteryRules(t *testing.T) {
	tests := []struct {
		name, draw, play, option, rule string
		want                           bool
	}{
		{"sum big", "9,8,7,6,5", "SUM_SIZE", "BIG", "sum_size:0:9", true},
		{"sum even", "1,2,3", "SUM_ODD_EVEN", "EVEN", "sum_odd_even", true},
		{"position", "3,6,9", "POSITION_2", "6", "position_2", true},
		{"dragon", "9,2,1", "DRAGON_TIGER", "DRAGON", "dragon_tiger:1:3", true},
		{"k3 triple", "4,4,4", "TRIPLE", "ANY", "k3_triple_any", true},
		{"lhc color", "1,2,3,4,5,6,49", "SPECIAL_COLOR", "GREEN", "lhc_special_color", true},
		{"wrong exact sum", "1,2,3", "EXACT", "7", "exact_sum", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isLotteryWin(test.draw, test.play, test.option, test.rule); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestFixedPointOdds(t *testing.T) {
	tests := map[string]int64{"1": 10000, "1.95": 19500, "9.5000": 95000, "42.32199": 423219}
	for input, want := range tests {
		got, err := parseOddsScaled(input)
		if err != nil {
			t.Fatalf("parse %q: %v", input, err)
		}
		if got != want {
			t.Fatalf("parse %q = %d, want %d", input, got, want)
		}
	}
}

func TestSportsRules(t *testing.T) {
	tests := []struct {
		home, away     int
		market, option string
		want           bool
	}{
		{2, 1, "MATCH_RESULT", "HOME_WIN", true},
		{2, 2, "MATCH_RESULT", "DRAW", true},
		{1, 3, "TOTAL_GOALS", "TG_4", true},
		{8, 0, "CORRECT_SCORE", "OTHER", true},
		{2, 1, "CORRECT_SCORE", "CS_2_0", false},
	}
	for _, test := range tests {
		if got := isSportsWin(test.home, test.away, test.market, test.option); got != test.want {
			t.Fatalf("%s/%s got %v, want %v", test.market, test.option, got, test.want)
		}
		if !validSportsOption(test.market, test.option) {
			t.Fatalf("expected valid option %s/%s", test.market, test.option)
		}
	}
}

func TestNormalizeCollectedMarkets(t *testing.T) {
	var item apiFootballOdds
	item.Fixture.ID = 42
	item.Bookmakers = []apiFootballBookmaker{{
		ID: 1, Name: "bookmaker", Bets: []apiFootballBet{{
			ID: 1, Name: "Match Winner", Values: []apiFootballOddValue{
				{Value: "Home", Odd: "1.95"}, {Value: "Draw", Odd: "3.20"}, {Value: "Away", Odd: "4.10"},
			},
		}},
	}}
	markets := normalizeCollectedMarkets(item)
	if len(markets) != 1 || markets[0].Code != "MATCH_RESULT" || len(markets[0].Options) != 3 {
		t.Fatalf("unexpected normalized markets: %#v", markets)
	}
}

func TestSportsQuotaError(t *testing.T) {
	if !isSportsQuotaError(errors.New("You have reached the request limit for the day")) {
		t.Fatal("daily quota error was not detected")
	}
	if isSportsQuotaError(errors.New("upstream timeout")) {
		t.Fatal("transient error must not be treated as a daily quota error")
	}
}
