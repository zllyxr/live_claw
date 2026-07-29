package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/config"
)

func TestPresignedURLsDoNotContactPublicEndpoint(t *testing.T) {
	service, err := New(config.Config{
		MinIOEndpoint:       "127.0.0.1:1",
		MinIOPublicEndpoint: "127.0.0.1:1",
		MinIOAccessKey:      "test-access",
		MinIOSecretKey:      "test-secret-value",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	putURL, err := service.PresignedPut(
		ctx,
		ReleasesBucket,
		"app/2026/07/update.wgt",
		"application/octet-stream",
		20*time.Minute,
	)
	if err != nil {
		t.Fatalf("presign upload contacted the unreachable public endpoint: %v", err)
	}
	if putURL == "" || !strings.Contains(putURL, "X-Amz-Signature=") {
		t.Fatalf("unexpected presigned upload URL %q", putURL)
	}

	getURL, err := service.PresignedGet(
		ctx,
		ReleasesBucket,
		"app/2026/07/update.wgt",
		20*time.Minute,
	)
	if err != nil {
		t.Fatalf("presign download contacted the unreachable public endpoint: %v", err)
	}
	if getURL == "" || !strings.Contains(getURL, "X-Amz-Signature=") {
		t.Fatalf("unexpected presigned download URL %q", getURL)
	}
}
