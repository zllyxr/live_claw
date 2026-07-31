package game

import "testing"

func TestFishingShotWithoutTargetIsAcceptedMiss(t *testing.T) {
	captured, err := fishingShotCaptured(1_000_000, 0)
	if err != nil {
		t.Fatalf("resolve empty-water shot: %v", err)
	}
	if captured {
		t.Fatal("an empty-water shot must never report a capture")
	}
}
