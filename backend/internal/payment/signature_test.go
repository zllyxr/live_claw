package payment

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestBEpusdtSignatureOfficialVector(t *testing.T) {
	fields := map[string]any{
		"order_id":     "20220201030210321",
		"amount":       json.Number("42"),
		"notify_url":   "http://example.com/notify",
		"redirect_url": "http://example.com/redirect",
		"signature":    "must-not-participate",
		"empty":        "",
	}
	signature, err := signFields(fields, "epusdt_password_xasddawqe")
	if err != nil {
		t.Fatal(err)
	}
	if signature != "1cd4b52df5587cfb1968b0c0c6e156cd" {
		t.Fatalf("signature = %q", signature)
	}
}

func TestSignBEpusdtRequestDoesNotMutateFields(t *testing.T) {
	fields := map[string]any{
		"order_id":  "missing-trade-id",
		"signature": "stale-signature",
	}
	signature, err := SignBEpusdtRequest(fields, "protocol-check-token")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := signFields(map[string]any{"order_id": "missing-trade-id"}, "protocol-check-token")
	if err != nil {
		t.Fatal(err)
	}
	if signature != expected || fields["signature"] != "stale-signature" {
		t.Fatalf("signature helper mutated fields or returned a wrong signature")
	}
	if _, err = SignBEpusdtRequest(fields, "short"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("short token error = %v", err)
	}
}

func TestBEpusdtSignatureMatchesFloat64JSONFormatting(t *testing.T) {
	tests := []struct {
		number    string
		canonical string
	}{
		{number: "28.0", canonical: "amount=28"},
		{number: "28.8800", canonical: "amount=28.88"},
		{number: "0.00000001", canonical: "amount=1e-08"},
		{number: "9007199254740993", canonical: "amount=9.007199254740992e+15"},
	}
	for _, test := range tests {
		t.Run(test.number, func(t *testing.T) {
			fields := map[string]any{"amount": json.Number(test.number)}
			canonical, err := canonicalFields(fields)
			if err != nil {
				t.Fatal(err)
			}
			if canonical != test.canonical {
				t.Fatalf("canonical = %q, want %q", canonical, test.canonical)
			}
		})
	}
}

func TestDecodeBEpusdtCallbackSignatureAndNumericFields(t *testing.T) {
	const token = "integration-api-token"
	fields := map[string]any{
		"trade_id":             "trade-123",
		"order_id":             "1234567890123456789",
		"amount":               json.Number("28.0"),
		"actual_amount":        "4.2500",
		"token":                "TRx-payment-address",
		"block_transaction_id": "block-123",
		"status":               2,
	}
	signature, err := signFields(fields, token)
	if err != nil {
		t.Fatal(err)
	}
	fields["signature"] = signature
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	callback, err := decodeBEpusdtCallback(raw, token)
	if err != nil {
		t.Fatal(err)
	}
	if callback.OrderID != "1234567890123456789" ||
		callback.Amount != "28" ||
		callback.ActualAmount != "4.25" ||
		callback.Status != 2 {
		t.Fatalf("unexpected callback: %#v", callback)
	}

	fields["signature"] = "00000000000000000000000000000000"
	raw, _ = json.Marshal(fields)
	if _, err = decodeBEpusdtCallback(raw, token); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("invalid signature error = %v", err)
	}
}

func TestDecodeBEpusdtCallbackLimitsAndPaidRequirements(t *testing.T) {
	if _, err := decodeBEpusdtCallback(make([]byte, maxCallbackBody+1), "token"); !errors.Is(err, ErrInvalidCallback) {
		t.Fatalf("oversized callback error = %v", err)
	}
	fields := map[string]any{
		"trade_id": "trade-123", "order_id": "order-123",
		"amount": 28, "actual_amount": "4.25", "status": 2,
	}
	signature, _ := signFields(fields, "token-token")
	fields["signature"] = signature
	raw, _ := json.Marshal(fields)
	if _, err := decodeBEpusdtCallback(raw, "token-token"); !errors.Is(err, ErrInvalidCallback) {
		t.Fatalf("incomplete paid callback error = %v", err)
	}
}

func TestCreateTraceLockLeavesDatabasePoolHeadroom(t *testing.T) {
	// database.Open uses SetMaxOpenConns(60). The lock holds one connection
	// while CreateRecharge needs other pooled connections for its queries.
	if cap(createTraceLockSlots) != createTraceLockConcurrency ||
		createTraceLockConcurrency > 16 {
		t.Fatalf("unsafe create-trace lock concurrency: %d", cap(createTraceLockSlots))
	}
}
