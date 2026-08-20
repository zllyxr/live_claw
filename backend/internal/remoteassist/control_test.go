package remoteassist

import (
	"context"
	"errors"
	"testing"
)

func TestControlSessionProtectsFrames(t *testing.T) {
	service, err := New(nil, "test-master-key-that-is-long-enough", Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	device := Device{ID: "device-one", Status: 1}
	if err = service.StoreFrame(ctx, device, ScreenFrame{
		JPEG: []byte{0xff, 0xd8, 0xff, 0xd9}, Width: 1, Height: 1, Sequence: 7,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Frame(ctx, device.ID, 9, "invalid"); !errors.Is(err, ErrNotReady) {
		t.Fatalf("frame accepted an invalid session: %v", err)
	}
	token, _, err := service.startControlSession(ctx, device.ID, 9, "authorization-one")
	if err != nil {
		t.Fatal(err)
	}
	frame, err := service.Frame(ctx, device.ID, 9, token)
	if err != nil || frame.Sequence != 7 {
		t.Fatalf("frame=%#v err=%v", frame, err)
	}
	if err = service.EndControlSession(ctx, device.ID, 9, token); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Frame(ctx, device.ID, 9, token); !errors.Is(err, ErrNotReady) {
		t.Fatalf("ended session still reads frames: %v", err)
	}
}

func TestControlPayloadValidation(t *testing.T) {
	valid := []struct {
		command string
		payload map[string]any
	}{
		{"tap", map[string]any{"x": 0.5, "y": 1.0}},
		{"swipe", map[string]any{"x1": 0.1, "y1": 0.2, "x2": 0.9, "y2": 0.8, "duration_ms": float64(300)}},
		{"system_action", map[string]any{"action": "back"}},
		{"text", map[string]any{"text": "测试"}},
		{"clipboard_set", map[string]any{"text": "clipboard"}},
		{"end_session", map[string]any{}},
	}
	for _, item := range valid {
		if _, err := validateControlPayload(item.command, item.payload); err != nil {
			t.Fatalf("%s rejected: %v", item.command, err)
		}
	}
	invalid := []struct {
		command string
		payload map[string]any
	}{
		{"tap", map[string]any{"x": -1.0, "y": 0.2}},
		{"swipe", map[string]any{"x1": 0.1, "y1": 0.2}},
		{"system_action", map[string]any{"action": "power"}},
		{"text", map[string]any{"text": string(make([]rune, 2049))}},
		{"unknown", map[string]any{}},
	}
	for _, item := range invalid {
		if _, err := validateControlPayload(item.command, item.payload); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s accepted invalid payload: %v", item.command, err)
		}
	}
}
