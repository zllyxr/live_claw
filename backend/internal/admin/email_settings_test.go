package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/mailer"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestEmailSettingsManagementIntegration(t *testing.T) {
	dsn := os.Getenv("CLAW_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) sql.Result {
		r, e := db.ExecContext(ctx, q, args...)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	var existing int
	if err = db.QueryRow(`SELECT COUNT(*) FROM system_settings WHERE setting_key=?`, mailer.SettingKey).Scan(&existing); err != nil || existing != 0 {
		t.Fatal("requires an empty SMTP fixture")
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM system_settings WHERE setting_key=?`, mailer.SettingKey) })
	key := "email-settings-integration-master-key"
	adminDB := db
	if adminDSN := os.Getenv("CLAW_TEST_ADMIN_MYSQL_DSN"); adminDSN != "" {
		adminDB, err = database.Open(ctx, adminDSN)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = adminDB.Close() })
	}
	store, err := mailer.NewStore(adminDB, key, mailer.Config{})
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{db: adminDB, auth: adminauth.New(adminDB), emailSettings: store, static: http.NotFoundHandler()}
	mux := http.NewServeMux()
	h.Register(mux)
	roleKey := "email_test_" + time.Now().Format("150405.000000000")
	role, _ := exec(`INSERT INTO admin_roles(role_key,name,status) VALUES(?,'Email settings test',1)`, roleKey).LastInsertId()
	actor, _ := exec(`INSERT INTO admin_users(username,password_hash,status) VALUES(?,'unused-fixture-password',1)`, roleKey).LastInsertId()
	exec(`INSERT INTO admin_user_roles(admin_user_id,role_id) VALUES(?,?)`, actor, role)
	exec(`INSERT INTO admin_role_permissions(role_id,permission_id) SELECT ?,id FROM admin_permissions WHERE permission_key='system.read'`, role)
	token, csrf := "email-fixture-session-token", "email-fixture-csrf-token"
	hash := func(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
	exec(`INSERT INTO admin_sessions(id,admin_user_id,portal,token_hash,csrf_hash,expires_at) VALUES('email-settings-test',?,'admin',?,?,DATE_ADD(NOW(),INTERVAL 1 HOUR))`, actor, hash(token), hash(csrf))
	t.Cleanup(func() {
		db.Exec(`DELETE FROM audit_logs WHERE actor_id=?`, actor)
		db.Exec(`DELETE FROM admin_sessions WHERE admin_user_id=?`, actor)
		db.Exec(`DELETE FROM admin_user_roles WHERE admin_user_id=?`, actor)
		db.Exec(`DELETE FROM admin_role_permissions WHERE role_id=?`, role)
		db.Exec(`DELETE FROM admin_roles WHERE id=?`, role)
		db.Exec(`DELETE FROM admin_users WHERE id=?`, actor)
	})
	call := func(method, path string, body any, authenticated, csrfOK bool, agent string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		if authenticated {
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		}
		if csrfOK {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		r.Header.Set("User-Agent", agent)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	path := "/admin/api/system/email"
	if call("GET", path, nil, false, false, "").Code != 401 {
		t.Fatal("anonymous settings access allowed")
	}
	if call("GET", path, nil, true, false, "").Code != 200 {
		t.Fatal("read-only settings access denied")
	}
	payload := map[string]any{"enabled": true, "host": "smtp.example.com", "port": "587", "tls_mode": "starttls", "username": "sender@example.com", "from": "sender@example.com", "password": "test-smtp-secret-300", "version": 0}
	if call("POST", path, payload, true, true, "").Code != 403 {
		t.Fatal("read-only operator changed SMTP")
	}
	exec(`INSERT INTO admin_role_permissions(role_id,permission_id) SELECT ?,id FROM admin_permissions WHERE permission_key='system.write'`, role)
	if call("POST", path, payload, true, false, "").Code != 403 {
		t.Fatal("SMTP mutation without CSRF allowed")
	}
	result := call("POST", path, payload, true, true, "")
	if result.Code != 200 {
		t.Fatalf("save status=%d body=%s", result.Code, result.Body.String())
	}
	if call("POST", path, payload, true, true, "").Code != 409 {
		t.Fatal("stale write accepted")
	}
	get := call("GET", path, nil, true, false, "")
	if get.Code != 200 || strings.Contains(get.Body.String(), "test-smtp-secret-300") || strings.Contains(get.Body.String(), `"password":`) {
		t.Fatal("settings read leaked secret")
	}
	var stored, audit string
	if err = db.QueryRow(`SELECT CAST(setting_value AS CHAR) FROM system_settings WHERE setting_key=?`, mailer.SettingKey).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT CAST(after_data AS CHAR) FROM audit_logs WHERE actor_id=? AND action='system.email.update' ORDER BY id DESC LIMIT 1`, actor).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "test-smtp-secret-300") || strings.Contains(audit, "test-smtp-secret-300") {
		t.Fatal("secret leaked into storage or audit")
	}
	// A distinct API Store sees each committed admin edit immediately using SELECT-only grants.
	apiDB := db
	if apiDSN := os.Getenv("CLAW_TEST_API_MYSQL_DSN"); apiDSN != "" {
		apiDB, err = database.Open(ctx, apiDSN)
		if err != nil {
			t.Fatal(err)
		}
		defer apiDB.Close()
	}
	reader, err := mailer.NewStore(apiDB, key, mailer.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sender, e := reader.Sender(ctx); e != nil || sender == nil {
		t.Fatal("API failed to load enabled settings")
	}
	payload["password"] = ""
	payload["enabled"] = false
	payload["version"] = 1
	if call("POST", path, payload, true, true, "").Code != 200 {
		t.Fatal("disable failed")
	}
	if sender, e := reader.Sender(ctx); e != nil || sender != nil {
		t.Fatal("disabled override not observed")
	}
	if call("POST", path+"/test", map[string]string{"to": "receiver@example.com"}, true, true, "").Code != 409 {
		t.Fatal("disabled SMTP attempted test delivery")
	}
	payload["enabled"] = true
	payload["version"] = 2
	if call("POST", path, payload, true, true, "").Code != 200 {
		t.Fatal("retained password could not re-enable")
	}
	snapshot, err := reader.Read(ctx)
	if err != nil || snapshot.Password != "test-smtp-secret-300" || snapshot.Version != 3 {
		t.Fatal("password retention/version failed")
	}
	payload["version"] = 3
	payload["host"] = "smtp.updated.example.com"
	if call("POST", path, payload, true, true, strings.Repeat("x", 700)).Code != 500 {
		t.Fatal("audit failure fixture did not fail")
	}
	snapshot, _ = reader.Read(ctx)
	if snapshot.Version != 3 || snapshot.Host != "smtp.example.com" {
		t.Fatal("setting committed without audit")
	}
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); statuses <- call("POST", path, payload, true, true, "").Code }()
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for code := range statuses {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent writes: %v", counts)
	}
	if call("POST", "/admin/api/system/settings/mail.smtp", map[string]any{"value": map[string]string{"password": "plaintext"}, "version": 4}, true, true, "").Code != 400 {
		t.Fatal("generic editor bypassed SMTP encryption")
	}
	if strings.Contains(call("GET", "/admin/api/system/settings", nil, true, false, "").Body.String(), mailer.SettingKey) {
		t.Fatal("generic editor still lists SMTP secret")
	}
	if call("POST", path+"/test", map[string]string{"to": "bad-address"}, true, true, "").Code != 400 {
		t.Fatal("invalid test recipient accepted")
	}
}
