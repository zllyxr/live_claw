package lottery

import (
	"testing"
	"time"
)

func TestFormatIssueExposesBettableCompatibilityFields(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	issue := formatIssue(
		11,
		22,
		"202701150001",
		now.Unix()-30,
		now.Unix()+25,
		now.Unix()+30,
		1,
		nil,
		now,
	)

	for key, expected := range map[string]any{
		"status":         "1",
		"can_bet":        "1",
		"seal_countdown": "25",
		"bet_countdown":  "25",
		"countdown":      "25",
		"open_countdown": "30",
	} {
		if actual := issue[key]; actual != expected {
			t.Fatalf("%s = %#v, want %#v", key, actual, expected)
		}
	}
}

func TestFormatIssueMarksClosedIssueAsNotBettable(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	issue := formatIssue(
		11,
		22,
		"202701150001",
		now.Unix()-60,
		now.Unix()-1,
		now.Unix(),
		2,
		nil,
		now,
	)

	if actual := issue["can_bet"]; actual != "0" {
		t.Fatalf("can_bet = %#v, want %q", actual, "0")
	}
	if actual := issue["seal_countdown"]; actual != "0" {
		t.Fatalf("seal_countdown = %#v, want %q", actual, "0")
	}
}
