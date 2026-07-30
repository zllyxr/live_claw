package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestUpdateUserProfileRequestNormalize(t *testing.T) {
	tests := []struct {
		name      string
		request   updateUserProfileRequest
		wantError bool
	}{
		{
			name: "valid and normalized",
			request: updateUserProfileRequest{
				Username: "  user_123  ", Nickname: "  测试用户  ", CountryCode: " 86 ",
				Mobile: " 13800138000 ", Email: " USER@Example.COM ",
			},
		},
		{
			name: "optional mobile and email",
			request: updateUserProfileRequest{
				Username: "virtual_user", Nickname: "", CountryCode: "86",
			},
		},
		{
			name: "username with whitespace",
			request: updateUserProfileRequest{
				Username: "invalid user", Nickname: "用户", CountryCode: "86",
			},
			wantError: true,
		},
		{
			name: "nickname with control character",
			request: updateUserProfileRequest{
				Username: "valid_user", Nickname: "用户\n名称", CountryCode: "86",
			},
			wantError: true,
		},
		{
			name: "invalid country code",
			request: updateUserProfileRequest{
				Username: "valid_user", Nickname: "用户", CountryCode: "+86",
			},
			wantError: true,
		},
		{
			name: "invalid mobile",
			request: updateUserProfileRequest{
				Username: "valid_user", Nickname: "用户", CountryCode: "86", Mobile: "123x",
			},
			wantError: true,
		},
		{
			name: "invalid email",
			request: updateUserProfileRequest{
				Username: "valid_user", Nickname: "用户", CountryCode: "86", Email: "invalid",
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.normalize()
			if test.wantError {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.name == "valid and normalized" {
				if test.request.Username != "user_123" ||
					test.request.Nickname != "测试用户" ||
					test.request.CountryCode != "86" ||
					test.request.Mobile != "13800138000" ||
					test.request.Email != "user@example.com" {
					t.Fatalf("request was not normalized: %#v", test.request)
				}
			}
		})
	}
}

func TestResetUserPasswordRequestNormalize(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		reason    string
		wantError bool
	}{
		{name: "valid", password: "AdminReset!123", reason: " 用户申请重置 "},
		{name: "valid unicode", password: strings.Repeat("密", 12), reason: "用户申请重置"},
		{name: "too short", password: "short", reason: "用户申请重置", wantError: true},
		{name: "unicode too short", password: strings.Repeat("密", 11), reason: "用户申请重置", wantError: true},
		{name: "too long", password: strings.Repeat("a", 129), reason: "用户申请重置", wantError: true},
		{name: "unicode too long", password: strings.Repeat("密", 129), reason: "用户申请重置", wantError: true},
		{name: "missing reason", password: "AdminReset!123", reason: " ", wantError: true},
		{
			name: "reason too long", password: "AdminReset!123",
			reason: strings.Repeat("a", 501), wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := resetUserPasswordRequest{Password: test.password, Reason: test.reason}
			err := request.normalize()
			if test.wantError {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if request.Reason != "用户申请重置" {
				t.Fatalf("reason was not normalized: %q", request.Reason)
			}
		})
	}
}

func TestValidManagedPasswordCountsUnicodeCharacters(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{name: "twelve ascii characters", password: strings.Repeat("a", 12), want: true},
		{name: "twelve multibyte characters", password: strings.Repeat("密", 12), want: true},
		{name: "one hundred twenty eight four byte characters", password: strings.Repeat("🔐", 128), want: true},
		{name: "eleven multibyte characters", password: strings.Repeat("密", 11), want: false},
		{name: "one hundred twenty nine characters", password: strings.Repeat("🔐", 129), want: false},
		{name: "invalid utf8", password: string([]byte{0xff, 0xfe, 0xfd}), want: false},
		{name: "oversized raw input", password: strings.Repeat("a", 513), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validManagedPassword(test.password); got != test.want {
				t.Fatalf("validManagedPassword() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestResetAdministratorPasswordRequestCountsUnicodeCharacters(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantError bool
	}{
		{name: "valid unicode", password: strings.Repeat("密", 12)},
		{name: "unicode too short", password: strings.Repeat("密", 11), wantError: true},
		{name: "unicode too long", password: strings.Repeat("密", 129), wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := resetAdministratorPasswordRequest{
				Password: test.password,
				Reason:   "安全审计要求",
			}
			err := request.normalize()
			if test.wantError && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantError && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUserManagementIntegration(t *testing.T) {
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
	numericSuffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	numericSuffix = numericSuffix[len(numericSuffix)-10:]
	originalPassword := "OriginalPass!123"
	originalHash, err := adminauth.HashPassword(originalPassword)
	if err != nil {
		t.Fatal(err)
	}
	firstResult, err := db.ExecContext(ctx, `
		INSERT INTO users
			(username,country_code,mobile,email,password_hash,password_algo,nickname,status)
		VALUES(?,?,?,?,?,'argon2id',?,1)`,
		"managed_"+suffix, "86", "15"+numericSuffix,
		"managed-"+suffix+"@example.test", originalHash, "原始昵称",
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := firstResult.LastInsertId()
	secondResult, err := db.ExecContext(ctx, `
		INSERT INTO users
			(username,country_code,mobile,email,password_hash,password_algo,nickname,status)
		VALUES(?,?,?,?,?,'argon2id',?,1)`,
		"conflict_"+suffix, "86", "16"+numericSuffix,
		"conflict-"+suffix+"@example.test", originalHash, "冲突账号",
	)
	if err != nil {
		t.Fatal(err)
	}
	conflictUserID, _ := secondResult.LastInsertId()
	adminID := time.Now().UnixNano() & 0x3fffffffffffffff
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM audit_logs
			WHERE actor_type=1 AND actor_id=? AND action IN ('user.profile.update','user.password.reset')`,
			adminID,
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM user_sessions WHERE user_id IN (?,?)", userID, conflictUserID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id IN (?,?)", userID, conflictUserID)
	})

	tokenSum := sha256.Sum256([]byte("managed-user-session-" + suffix))
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_sessions
			(id,user_id,token_hash,device_id,platform,ip,user_agent,expires_at)
		VALUES(?,?,?,'integration-device','h5','127.0.0.1','integration-test',
		       CURRENT_TIMESTAMP(3)+INTERVAL 1 DAY)`,
		fmt.Sprintf("S%025d", userID%1000000000000000000), userID, hex.EncodeToString(tokenSum[:]),
	)
	if err != nil {
		t.Fatal(err)
	}

	handler := &Handler{db: db}
	adminUser := adminauth.Admin{ID: adminID, Username: "integration-admin"}
	run := func(path string, targetID int64, body map[string]any, endpoint http.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		encoded, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
		request.SetPathValue("id", strconv.FormatInt(targetID, 10))
		recorder := httptest.NewRecorder()
		httpx.RequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint(w, r.WithContext(withAdmin(r, adminUser)))
		})).ServeHTTP(recorder, request)
		return recorder
	}

	updatedProfile := updateUserProfileRequest{
		Username:    "updated_" + suffix,
		Nickname:    "更新后的昵称",
		CountryCode: "852",
		Mobile:      "17" + numericSuffix,
		Email:       "updated-" + suffix + "@example.test",
	}
	profileRecorder := run(
		"/admin/api/users/"+strconv.FormatInt(userID, 10),
		userID,
		map[string]any{
			"username": updatedProfile.Username, "nickname": updatedProfile.Nickname,
			"country_code": updatedProfile.CountryCode, "mobile": updatedProfile.Mobile,
			"email": updatedProfile.Email,
		},
		handler.updateUserProfile,
	)
	if profileRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected profile update response (%d): %s", profileRecorder.Code, profileRecorder.Body.String())
	}
	var persisted editableUserProfile
	var persistedMobile, persistedEmail string
	err = db.QueryRowContext(ctx, `
		SELECT username,nickname,country_code,COALESCE(mobile,''),COALESCE(email,'')
		FROM users WHERE id=?`,
		userID,
	).Scan(
		&persisted.Username, &persisted.Nickname, &persisted.CountryCode,
		&persistedMobile, &persistedEmail,
	)
	if err != nil {
		t.Fatal(err)
	}
	persisted.Mobile = persistedMobile
	persisted.Email = persistedEmail
	if persisted != updatedProfile.profile() {
		t.Fatalf("unexpected persisted profile: %#v", persisted)
	}

	var beforeJSON, afterJSON string
	err = db.QueryRowContext(ctx, `
		SELECT CAST(before_data AS CHAR),CAST(after_data AS CHAR)
		FROM audit_logs
		WHERE actor_type=1 AND actor_id=? AND action='user.profile.update' AND resource_id=?
		ORDER BY id DESC LIMIT 1`,
		adminID, userID,
	).Scan(&beforeJSON, &afterJSON)
	if err != nil {
		t.Fatal(err)
	}
	var beforeAudit, afterAudit editableUserProfile
	if err = json.Unmarshal([]byte(beforeJSON), &beforeAudit); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(afterJSON), &afterAudit); err != nil {
		t.Fatal(err)
	}
	if beforeAudit.Username != "managed_"+suffix || afterAudit != updatedProfile.profile() {
		t.Fatalf("unexpected profile audit: before=%#v after=%#v", beforeAudit, afterAudit)
	}

	conflictRecorder := run(
		"/admin/api/users/"+strconv.FormatInt(userID, 10),
		userID,
		map[string]any{
			"username": "conflict_" + suffix, "nickname": "不会保存",
			"country_code": "86", "mobile": updatedProfile.Mobile, "email": updatedProfile.Email,
		},
		handler.updateUserProfile,
	)
	if conflictRecorder.Code != http.StatusConflict {
		t.Fatalf("expected unique conflict, got (%d): %s", conflictRecorder.Code, conflictRecorder.Body.String())
	}
	var conflictEnvelope httpx.Envelope
	if err = json.Unmarshal(conflictRecorder.Body.Bytes(), &conflictEnvelope); err != nil {
		t.Fatal(err)
	}
	if conflictEnvelope.Message != "该国家/地区代码下的用户账号已存在" {
		t.Fatalf("unexpected conflict message: %q", conflictEnvelope.Message)
	}

	shortPasswordRecorder := run(
		"/admin/api/users/"+strconv.FormatInt(userID, 10)+"/password",
		userID,
		map[string]any{"password": "short", "reason": "测试短密码"},
		handler.resetUserPassword,
	)
	if shortPasswordRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected password validation failure, got (%d): %s", shortPasswordRecorder.Code, shortPasswordRecorder.Body.String())
	}

	nextPassword := "NewManagedPass!456"
	passwordRecorder := run(
		"/admin/api/users/"+strconv.FormatInt(userID, 10)+"/password",
		userID,
		map[string]any{"password": nextPassword, "reason": "管理员应用户申请重置"},
		handler.resetUserPassword,
	)
	if passwordRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected password reset response (%d): %s", passwordRecorder.Code, passwordRecorder.Body.String())
	}
	if strings.Contains(passwordRecorder.Body.String(), nextPassword) ||
		strings.Contains(passwordRecorder.Body.String(), "$argon2id$") {
		t.Fatalf("password response leaked credential material: %s", passwordRecorder.Body.String())
	}

	var persistedHash, persistedAlgorithm string
	var activeSessions int
	err = db.QueryRowContext(ctx, `
		SELECT
			(SELECT password_hash FROM users WHERE id=?),
			(SELECT password_algo FROM users WHERE id=?),
			(SELECT COUNT(*) FROM user_sessions WHERE user_id=? AND revoked_at IS NULL)`,
		userID, userID, userID,
	).Scan(&persistedHash, &persistedAlgorithm, &activeSessions)
	if err != nil {
		t.Fatal(err)
	}
	if persistedAlgorithm != "argon2id" || !adminauth.VerifyPassword(persistedHash, nextPassword) ||
		adminauth.VerifyPassword(persistedHash, originalPassword) || activeSessions != 0 {
		t.Fatalf(
			"unexpected password state: algorithm=%q next_valid=%t old_valid=%t active_sessions=%d",
			persistedAlgorithm, adminauth.VerifyPassword(persistedHash, nextPassword),
			adminauth.VerifyPassword(persistedHash, originalPassword), activeSessions,
		)
	}

	var passwordBeforeAudit, passwordAfterAudit string
	err = db.QueryRowContext(ctx, `
		SELECT CAST(before_data AS CHAR),CAST(after_data AS CHAR)
		FROM audit_logs
		WHERE actor_type=1 AND actor_id=? AND action='user.password.reset' AND resource_id=?
		ORDER BY id DESC LIMIT 1`,
		adminID, userID,
	).Scan(&passwordBeforeAudit, &passwordAfterAudit)
	if err != nil {
		t.Fatal(err)
	}
	auditPayload := passwordBeforeAudit + passwordAfterAudit
	if strings.Contains(auditPayload, nextPassword) || strings.Contains(auditPayload, persistedHash) ||
		strings.Contains(auditPayload, "$argon2id$") {
		t.Fatalf("password audit leaked credential material: %s", auditPayload)
	}
	var passwordAudit map[string]any
	if err = json.Unmarshal([]byte(passwordAfterAudit), &passwordAudit); err != nil {
		t.Fatal(err)
	}
	if passwordAudit["reason"] != "管理员应用户申请重置" ||
		passwordAudit["revoked_sessions"] != float64(1) {
		t.Fatalf("unexpected password audit: %#v", passwordAudit)
	}
}
