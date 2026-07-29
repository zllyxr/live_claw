package apprelease

import "testing"

func TestRolloutIsDeterministic(t *testing.T) {
	first := rolloutAllows(35, "device-123", 211)
	for i := 0; i < 20; i++ {
		if rolloutAllows(35, "device-123", 211) != first {
			t.Fatal("rollout decision changed for the same device and release")
		}
	}
	if rolloutAllows(0, "device-123", 211) {
		t.Fatal("zero-percent rollout was allowed")
	}
	if !rolloutAllows(100, "", 211) {
		t.Fatal("full rollout should not require a device key")
	}
	if rolloutAllows(50, "", 211) {
		t.Fatal("partial rollout without a stable device key was allowed")
	}
}

func TestNormalizePlatform(t *testing.T) {
	if normalizePlatform("iOS") != "ios" || normalizePlatform("unknown") != "android" {
		t.Fatal("platform normalization failed")
	}
}
