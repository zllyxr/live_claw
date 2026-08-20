package config

import (
	"reflect"
	"testing"
)

func TestPositiveIDListEnv(t *testing.T) {
	t.Setenv("TEST_REMOTE_USER_IDS", "42, 7,42")
	value, err := positiveIDListEnv("TEST_REMOTE_USER_IDS")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, []int64{42, 7}) {
		t.Fatalf("unexpected IDs: %#v", value)
	}
	t.Setenv("TEST_REMOTE_USER_IDS", "7,nope")
	if _, err = positiveIDListEnv("TEST_REMOTE_USER_IDS"); err == nil {
		t.Fatal("invalid user ID list was accepted")
	}
}
