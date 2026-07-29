package game

import "testing"

func TestParseFishingAssignmentBounds(t *testing.T) {
	table, seat, err := parseAssignment("300:4")
	if err != nil || table != 300 || seat != 4 {
		t.Fatalf("valid assignment rejected: table=%d seat=%d err=%v", table, seat, err)
	}
	for _, value := range []string{"0:1", "301:1", "1:0", "1:5", "1", "a:1"} {
		if _, _, err := parseAssignment(value); err == nil {
			t.Fatalf("invalid assignment accepted: %q", value)
		}
	}
}

func TestSecureIndexRange(t *testing.T) {
	for index := 0; index < 1000; index++ {
		value, err := secureIndex(FishingTableCount)
		if err != nil {
			t.Fatal(err)
		}
		if value < 0 || value >= FishingTableCount {
			t.Fatalf("random index out of range: %d", value)
		}
	}
}
