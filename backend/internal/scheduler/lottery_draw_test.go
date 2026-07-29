package scheduler

import (
	"reflect"
	"testing"
	"time"
)

func TestNextDrawAt(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 7, 28, 12, 34, 12, 0, location)
	if got := nextDrawAt(now, 60); !got.Equal(
		time.Date(2026, 7, 28, 12, 35, 0, 0, location),
	) {
		t.Fatalf("unexpected minute draw time: %s", got)
	}
	if got := nextDrawAt(now, 86_400); !got.Equal(
		time.Date(2026, 7, 29, 0, 0, 0, 0, location),
	) {
		t.Fatalf("unexpected daily draw time: %s", got)
	}
}

func TestSecureDrawNumbers(t *testing.T) {
	config := lotteryDrawConfig{
		Mode: "local_auto", Count: 10, Minimum: 1, Maximum: 10, Unique: 1,
	}
	numbers, err := secureDrawNumbers(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(numbers) != 10 {
		t.Fatalf("expected 10 numbers, got %d", len(numbers))
	}
	seen := make(map[int]bool, 10)
	for _, value := range numbers {
		if value < 1 || value > 10 || seen[value] {
			t.Fatalf("invalid unique draw: %#v", numbers)
		}
		seen[value] = true
	}
}

func TestSecureDrawNumbersUsesPositionCandidates(t *testing.T) {
	config := lotteryDrawConfig{
		Mode: "local_auto", Count: 3, Minimum: 0, Maximum: 9,
	}
	for index := 0; index < 20; index++ {
		numbers, err := secureDrawNumbersWithCandidates(config, map[int][]int{
			1: {1, 2},
			2: {8, 9},
		})
		if err != nil {
			t.Fatal(err)
		}
		if numbers[0] < 1 || numbers[0] > 2 || numbers[1] < 8 || numbers[1] > 9 {
			t.Fatalf("position candidates were not respected: %#v", numbers)
		}
	}
}

func TestLotteryWinnerCodes(t *testing.T) {
	config := lotteryDrawConfig{
		Mode: "local_auto", Count: 5, Minimum: 0, Maximum: 9,
	}
	numbers := []int{9, 7, 2, 1, 0}
	cases := map[string]string{
		"position_1":   "9",
		"position_2":   "7",
		"exact_sum":    "19",
		"sum_odd_even": "ODD",
		"sum_size:0:9": "SMALL",
	}
	for rule, expected := range cases {
		got, err := lotteryWinnerCode(rule, config, numbers)
		if err != nil {
			t.Fatalf("%s: %v", rule, err)
		}
		if got != expected {
			t.Fatalf("%s: expected %s, got %s", rule, expected, got)
		}
	}

	pk10 := lotteryDrawConfig{
		Mode: "local_auto", Count: 10, Minimum: 1, Maximum: 10, Unique: 1,
	}
	pk10Numbers := []int{10, 9, 1, 2, 3, 4, 5, 6, 7, 8}
	if sum := lotteryAggregateSum(pk10, pk10Numbers); sum != 19 {
		t.Fatalf("PK10 aggregate should use the first two numbers, got %d", sum)
	}
}

func TestFormatOpenCode(t *testing.T) {
	if got := formatOpenCode([]int{3, 1, 10}, 2); got != "03,01,10" {
		t.Fatalf("unexpected open code: %s", got)
	}
	if !reflect.DeepEqual(
		[]bool{optionCodesEqual("03", "3"), optionCodesEqual("BIG", "big")},
		[]bool{true, true},
	) {
		t.Fatal("option-code normalization failed")
	}
}
