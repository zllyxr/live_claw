package apprelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/storage"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestReleaseCheckIntegration(t *testing.T) {
	dsn := os.Getenv("CLAW_TEST_MYSQL_DSN")
	minioEndpoint := os.Getenv("CLAW_TEST_MINIO_ENDPOINT")
	if dsn == "" || minioEndpoint == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN and CLAW_TEST_MINIO_ENDPOINT are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		MinIOEndpoint:       minioEndpoint,
		MinIOPublicEndpoint: minioEndpoint,
		MinIOAccessKey:      os.Getenv("CLAW_TEST_MINIO_ACCESS_KEY"),
		MinIOSecretKey:      os.Getenv("CLAW_TEST_MINIO_SECRET_KEY"),
	}
	storageService, err := storage.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = storageService.EnsureBuckets(ctx); err != nil {
		t.Fatal(err)
	}

	body := []byte("local-wgt-integration-test")
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	versionCode := time.Now().UnixNano() & 0x3fffffff
	objectKey := "integration/" + strconv.FormatInt(versionCode, 10) + ".wgt"
	if err = storageService.PutObject(
		ctx, storage.ReleasesBucket, objectKey, bytes.NewReader(body), int64(len(body)), "application/octet-stream",
	); err != nil {
		t.Fatal(err)
	}
	defer storageService.RemoveObject(context.Background(), storage.ReleasesBucket, objectKey) //nolint:errcheck

	result, err := db.ExecContext(ctx, `
		INSERT INTO media_assets
			(owner_user_id,bucket,object_key,media_type,mime_type,size_bytes,sha256,status)
		VALUES(0,?,?, 'app_release','application/octet-stream',?,?,1)`,
		storage.ReleasesBucket, objectKey, len(body), hash,
	)
	if err != nil {
		t.Fatal(err)
	}
	assetID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), "DELETE FROM media_assets WHERE id=?", assetID) //nolint:errcheck
	releaseResult, err := db.ExecContext(ctx, `
		INSERT INTO app_releases
			(platform,release_type,version_name,version_code,min_native_code,force_update,silent_update,
			 rollout_percent,package_asset_id,package_size,package_sha256,release_notes,status,published_at)
		VALUES('android','wgt','integration',?,0,0,1,100,?,?,?,'local test',1,CURRENT_TIMESTAMP(3))`,
		versionCode, assetID, len(body), hash,
	)
	if err != nil {
		t.Fatal(err)
	}
	releaseID, err := releaseResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), "DELETE FROM app_releases WHERE id=?", releaseID) //nolint:errcheck

	service := New(db, storageService)
	update, err := service.Check(ctx, versionCode-1, 210, "android", "integration-device")
	if err != nil {
		t.Fatal(err)
	}
	if update["has_update"] != "1" || update["silent"] != "1" || update["sha256"] != hash {
		t.Fatalf("unexpected update response: %#v", update)
	}
	downloadURL, _ := update["wgt_url"].(string)
	response, err := http.Get(downloadURL) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	downloaded, err := io.ReadAll(io.LimitReader(response.Body, 1024))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !bytes.Equal(downloaded, body) {
		t.Fatalf("signed download failed: status=%d body=%q", response.StatusCode, downloaded)
	}
}
