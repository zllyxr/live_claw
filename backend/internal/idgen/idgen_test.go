package idgen

import (
	"bytes"
	"testing"
	"time"
)

func TestNewAtIsFixedLengthAndTimeSortable(t *testing.T) {
	first, err := NewAt(time.UnixMilli(1000), bytes.NewReader(make([]byte, 10)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewAt(time.UnixMilli(1001), bytes.NewReader(make([]byte, 10)))
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 26 || len(second) != 26 {
		t.Fatalf("unexpected lengths %d and %d", len(first), len(second))
	}
	if first >= second {
		t.Fatalf("ids are not time-sortable: %q >= %q", first, second)
	}
}
