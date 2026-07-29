package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/storage"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestAcceptedUploadType(t *testing.T) {
	tests := []struct {
		name         string
		detected     string
		declared     string
		originalName string
		wantMIME     string
		wantExt      string
	}{
		{
			name:     "detected image wins",
			detected: "image/png; charset=binary", declared: "application/octet-stream",
			originalName: "avatar.bin", wantMIME: "image/png", wantExt: ".png",
		},
		{
			name:     "octet stream uses allowed declared type",
			detected: "application/octet-stream", declared: "video/mp4",
			originalName: "clip.bin", wantMIME: "video/mp4", wantExt: ".mp4",
		},
		{
			name:     "octet stream falls back to extension",
			detected: "application/octet-stream", declared: "",
			originalName: "voice.MP3", wantMIME: "audio/mpeg", wantExt: ".mp3",
		},
		{
			name:     "executable is rejected despite image extension",
			detected: "application/x-msdownload", declared: "image/png",
			originalName: "payload.png",
		},
		{
			name:     "unknown octet stream is rejected",
			detected: "application/octet-stream", declared: "application/octet-stream",
			originalName: "payload.exe",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mimeType, extension := acceptedUploadType(
				test.detected, test.declared, test.originalName,
			)
			if mimeType != test.wantMIME || extension != test.wantExt {
				t.Fatalf(
					"expected mime=%q extension=%q, got mime=%q extension=%q",
					test.wantMIME, test.wantExt, mimeType, extension,
				)
			}
		})
	}
}

func TestMediaType(t *testing.T) {
	tests := map[string]string{
		"image/webp":      "image",
		"video/mp4":       "video",
		"audio/mpeg":      "audio",
		"application/pdf": "file",
	}
	for contentType, want := range tests {
		if got := mediaType(contentType); got != want {
			t.Fatalf("mediaType(%q)=%q, want %q", contentType, got, want)
		}
	}
}

func TestCompatUploadIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	minioEndpoint := strings.TrimSpace(os.Getenv("CLAW_TEST_MINIO_ENDPOINT"))
	if dsn == "" || minioEndpoint == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN and CLAW_TEST_MINIO_ENDPOINT are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	storageService, err := storage.New(config.Config{
		MinIOEndpoint:       minioEndpoint,
		MinIOPublicEndpoint: minioEndpoint,
		MinIOAccessKey:      envOrDefault("CLAW_TEST_MINIO_ACCESS_KEY", "clawlocal"),
		MinIOSecretKey:      envOrDefault("CLAW_TEST_MINIO_SECRET_KEY", "claw-local-minio-password"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = storageService.EnsureBuckets(ctx); err != nil {
		t.Fatal(err)
	}

	uniqueID, err := idgen.New()
	if err != nil {
		t.Fatal(err)
	}
	username := "upload_" + strings.ToLower(uniqueID[len(uniqueID)-8:])
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'integration-test-only','上传联调用户',1)`,
		username,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	sessionID, _ := idgen.New()
	token := "upload-token-" + strings.ToLower(uniqueID)
	tokenHash := sha256.Sum256([]byte(token))
	if _, err = db.ExecContext(ctx, `
		INSERT INTO user_sessions(id,user_id,token_hash,expires_at)
		VALUES(?,?,?,CURRENT_TIMESTAMP(3)+INTERVAL 1 HOUR)`,
		sessionID, userID, hex.EncodeToString(tokenHash[:]),
	); err != nil {
		t.Fatal(err)
	}
	var assetID int64
	var objectKey string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if objectKey != "" {
			_ = storageService.RemoveObject(cleanupCtx, storage.PublicBucket, objectKey)
		}
		if assetID > 0 {
			_, _ = db.ExecContext(cleanupCtx, "DELETE FROM media_assets WHERE id=?", assetID)
		}
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM user_sessions WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", userID)
	})

	pngData, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl0uXcAAAAASUVORK5CYII=",
	)
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err = writer.WriteField("uid", strconv.FormatInt(userID, 10)); err != nil {
		t.Fatal(err)
	}
	if err = writer.WriteField("token", token); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(pngData); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}

	server := &Server{
		db: db, auth: auth.New(db), storage: storageService,
		mediaBaseURL: "http://" + minioEndpoint + "/" + storage.PublicBucket,
	}
	request := httptest.NewRequest(http.MethodPost, "/appapi/?service=Upload.uploadFile", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.compatUpload(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("upload returned HTTP %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			Code int `json:"code"`
			Info []struct {
				ID       int64  `json:"id"`
				URL      string `json:"url"`
				MIMEType string `json:"mime_type"`
				Size     int    `json:"size"`
			} `json:"info"`
		} `json:"data"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Code != 0 || len(payload.Data.Info) != 1 {
		t.Fatalf("unexpected upload response: %s", response.Body.String())
	}
	assetID = payload.Data.Info[0].ID
	if payload.Data.Info[0].MIMEType != "image/png" ||
		payload.Data.Info[0].Size != len(pngData) ||
		!strings.HasPrefix(payload.Data.Info[0].URL, server.mediaBaseURL+"/users/") {
		t.Fatalf("unexpected upload metadata: %#v", payload.Data.Info[0])
	}
	if err = db.QueryRowContext(ctx, `
		SELECT object_key FROM media_assets
		WHERE id=? AND owner_user_id=? AND bucket=? AND status=1`,
		assetID, userID, storage.PublicBucket,
	).Scan(&objectKey); err != nil {
		t.Fatal(err)
	}
	info, err := storageService.InspectObject(ctx, storage.PublicBucket, objectKey)
	if err != nil {
		t.Fatal(err)
	}
	expectedHash := sha256.Sum256(pngData)
	if info.Size != int64(len(pngData)) ||
		info.ContentType != "image/png" ||
		info.SHA256 != hex.EncodeToString(expectedHash[:]) {
		t.Fatalf("unexpected stored object: %#v", info)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
