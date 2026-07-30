package payment

import "testing"

func TestDecimalToMinorIsExact(t *testing.T) {
	tests := []struct {
		raw   string
		scale int
		want  int64
		ok    bool
	}{
		{raw: "28.88", scale: 2, want: 2888, ok: true},
		{raw: "28.8800", scale: 2, want: 2888, ok: true},
		{raw: "2.888e1", scale: 2, want: 2888, ok: true},
		{raw: "0.01", scale: 2, want: 1, ok: true},
		{raw: "0.001", scale: 2, ok: false},
		{raw: "-1", scale: 2, ok: false},
		{raw: "92233720368547758.08", scale: 2, ok: false},
	}
	for _, test := range tests {
		got, err := decimalToMinor(test.raw, test.scale)
		if (err == nil) != test.ok {
			t.Fatalf("decimalToMinor(%q,%d) error = %v", test.raw, test.scale, err)
		}
		if err == nil && got != test.want {
			t.Fatalf("decimalToMinor(%q,%d) = %d, want %d", test.raw, test.scale, got, test.want)
		}
	}
}

func TestAmountMatchesMinor(t *testing.T) {
	if !AmountMatchesMinor("28.8800", 2888, 2) {
		t.Fatal("equivalent exact amount did not match")
	}
	if AmountMatchesMinor("28.89", 2888, 2) ||
		AmountMatchesMinor("28.881", 2888, 2) {
		t.Fatal("mismatched or over-precise amount was accepted")
	}
}

func TestFormatMinorCanonical(t *testing.T) {
	tests := map[int64]string{
		0: "0", 1: "0.01", 100: "1", 2888: "28.88", 2880: "28.8",
	}
	for amount, want := range tests {
		got, err := formatMinorCanonical(amount, 2)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("formatMinorCanonical(%d) = %q, want %q", amount, got, want)
		}
	}
}
