package main

import "testing"

func TestDecodeBetItemsAcceptsNumericAndStringIntegers(t *testing.T) {
	tests := []string{
		`[{"option_id":123,"amount":10}]`,
		`[{"option_id":"123","amount":"10"}]`,
	}
	for _, raw := range tests {
		items, err := decodeBetItems[lotteryBetInput](raw)
		if err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		if len(items) != 1 || int64(items[0].OptionID) != 123 || int64(items[0].Amount) != 10 {
			t.Fatalf("unexpected decoded items for %s: %#v", raw, items)
		}
	}
}

func TestDecodeBetItemsRejectsMalformedValues(t *testing.T) {
	tests := []string{
		`[{"option_id":"abc","amount":10}]`,
		`[{"option_id":1.5,"amount":10}]`,
		`[{"option_id":1,"amount":10,"unexpected":true}]`,
		`[{"option_id":1,"amount":10}] {}`,
	}
	for _, raw := range tests {
		if _, err := decodeBetItems[lotteryBetInput](raw); err == nil {
			t.Fatalf("expected %s to be rejected", raw)
		}
	}
}
