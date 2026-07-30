package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestNativeSilentReleaseCannotPublishResumeOrRemainInvalid(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	versionBase := time.Now().UnixNano() & 0x3fffffffffffffff
	releaseIDs := make([]int64, 0, 2)
	for offset, status := range []int{0, 2} {
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO app_releases
				(platform,release_type,version_name,version_code,min_native_code,
				 force_update,silent_update,rollout_percent,package_asset_id,
				 package_size,package_sha256,release_notes,status)
			VALUES('android','native',?,?,0,0,1,100,1,1,?,'invalid legacy row',?)`,
			"invalid-native-"+strconv.Itoa(offset), versionBase+int64(offset),
			strings.Repeat("b", 64), status,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		releaseID, _ := result.LastInsertId()
		releaseIDs = append(releaseIDs, releaseID)
	}
	adminID := time.Now().UnixNano() & 0x3fffffffffffffff
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM audit_logs
			WHERE actor_type=1 AND actor_id=? AND resource_type='app_release'
			  AND resource_id IN (?,?)`,
			adminID, releaseIDs[0], releaseIDs[1],
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM app_releases WHERE id IN (?,?)",
			releaseIDs[0], releaseIDs[1])
	})

	handler := &Handler{db: db}
	admin := adminauth.Admin{ID: adminID, Username: "integration-admin"}
	run := func(path, id string, body []byte, endpoint http.HandlerFunc) int {
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		request.SetPathValue("id", id)
		recorder := httptest.NewRecorder()
		httpx.RequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint(w, r.WithContext(withAdmin(r, admin)))
		})).ServeHTTP(recorder, request)
		return recorder.Code
	}

	draftID := strconv.FormatInt(releaseIDs[0], 10)
	if status := run(
		"/admin/api/app/releases/"+draftID+"/publish",
		draftID,
		nil,
		handler.publishAppRelease,
	); status != http.StatusConflict {
		t.Fatalf("publish invalid native silent release returned HTTP %d", status)
	}
	if status := run(
		"/admin/api/app/releases/"+draftID,
		draftID,
		[]byte(`{"version_name":"still-invalid"}`),
		handler.updateAppRelease,
	); status != http.StatusBadRequest {
		t.Fatalf("update preserving invalid native silent release returned HTTP %d", status)
	}

	pausedID := strconv.FormatInt(releaseIDs[1], 10)
	if status := run(
		"/admin/api/app/releases/"+pausedID+"/resume",
		pausedID,
		nil,
		handler.resumeAppRelease,
	); status != http.StatusConflict {
		t.Fatalf("resume invalid native silent release returned HTTP %d", status)
	}
	for index, expected := range []int{0, 2} {
		var actual int
		if err = db.QueryRowContext(ctx,
			"SELECT status FROM app_releases WHERE id=?", releaseIDs[index],
		).Scan(&actual); err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Fatalf("release %d status changed from %d to %d", releaseIDs[index], expected, actual)
		}
	}
}

func TestConcurrentAppReleasePublishKeepsSingleActive(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	if len(suffix) > 10 {
		suffix = suffix[len(suffix)-10:]
	}
	platform := "t" + suffix
	releaseType := "w" + suffix
	releaseIDs := make([]int64, 0, 2)
	for versionCode := int64(1); versionCode <= 2; versionCode++ {
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO app_releases
				(platform,release_type,version_name,version_code,min_native_code,
				 force_update,silent_update,rollout_percent,package_asset_id,
				 package_size,package_sha256,release_notes,status)
			VALUES(?,?,?, ?,0,0,0,100,1,1,?,'integration concurrency test',0)`,
			platform, releaseType, "test-"+strconv.FormatInt(versionCode, 10),
			versionCode, strings.Repeat("a", 64),
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		releaseID, _ := result.LastInsertId()
		releaseIDs = append(releaseIDs, releaseID)
	}
	adminID := time.Now().UnixNano() & 0x3fffffffffffffff
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM audit_logs
			WHERE actor_type=1 AND actor_id=? AND resource_type='app_release'
			  AND resource_id IN (?,?)`,
			adminID, releaseIDs[0], releaseIDs[1],
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM app_releases WHERE id IN (?,?)",
			releaseIDs[0], releaseIDs[1])
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM app_release_lifecycle_locks
			WHERE platform=? AND release_type=?`,
			platform, releaseType,
		)
	})

	handler := &Handler{db: db}
	start := make(chan struct{})
	statuses := make(chan int, len(releaseIDs))
	var wait sync.WaitGroup
	for _, releaseID := range releaseIDs {
		wait.Add(1)
		go func(targetID int64) {
			defer wait.Done()
			<-start
			request := httptest.NewRequest(
				http.MethodPost,
				"/admin/api/app/releases/"+strconv.FormatInt(targetID, 10)+"/publish",
				nil,
			)
			request.SetPathValue("id", strconv.FormatInt(targetID, 10))
			recorder := httptest.NewRecorder()
			httpx.RequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler.publishAppRelease(
					w,
					r.WithContext(withAdmin(r, adminauth.Admin{
						ID: adminID, Username: "integration-admin",
					})),
				)
			})).ServeHTTP(recorder, request)
			statuses <- recorder.Code
		}(releaseID)
	}
	close(start)
	wait.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("concurrent publish returned HTTP %d", status)
		}
	}

	var activeCount int
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM app_releases
		WHERE platform=? AND release_type=? AND status=1`,
		platform, releaseType,
	).Scan(&activeCount); err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active release, got %d", activeCount)
	}
}
