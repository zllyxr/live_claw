package remoteassist

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestRemoteAssistanceLifecycleIntegration(t *testing.T) {
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

	suffix := fmt.Sprintf("%x", time.Now().UnixNano())
	userResult, err := db.ExecContext(ctx, `INSERT INTO users
		(username,country_code,mobile,email,password_hash,password_algo,nickname,status)
		VALUES(?,'86',?,?,'integration-test-only','argon2id','远程协助测试',1)`,
		"remote_it_"+suffix, "18"+suffix[len(suffix)-9:], "remote-"+suffix+"@example.test")
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	adminResult, err := db.ExecContext(ctx, `INSERT INTO admin_users(username,password_hash,display_name,status) VALUES(?,'integration-test-only','远程协助管理员',1)`, "remote_admin_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	adminID, _ := adminResult.LastInsertId()
	var deviceID string
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if deviceID != "" {
			_, _ = db.ExecContext(cleanup, `DELETE FROM remote_sessions WHERE remote_device_id=?`, deviceID)
			_, _ = db.ExecContext(cleanup, `DELETE FROM remote_credential_requests WHERE remote_device_id=?`, deviceID)
			_, _ = db.ExecContext(cleanup, `DELETE FROM remote_commands WHERE remote_device_id=?`, deviceID)
			_, _ = db.ExecContext(cleanup, `DELETE FROM remote_devices WHERE id=?`, deviceID)
		}
		_, _ = db.ExecContext(cleanup, `DELETE FROM admin_users WHERE id=?`, adminID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM users WHERE id=?`, userID)
	})

	service, err := New(db, "remote-integration-master-key", Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := service.Enroll(ctx, userID, EnrollRequest{
		InstallID: "android-integration-" + suffix, DeviceName: "Pixel Integration",
		AndroidVersion: "16", AndroidSDK: 36, AppVersion: "8.2.0", AppNativeCode: 216,
		PluginVersion: "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	deviceID = enrollment.DeviceID
	if enrollment.DeviceToken == "" {
		t.Fatal("enrollment did not return the one-time token")
	}
	var storedHash sql.NullString
	if err = db.QueryRowContext(ctx, `SELECT device_token_hash FROM remote_devices WHERE id=?`, deviceID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if !storedHash.Valid || storedHash.String == enrollment.DeviceToken || len(storedHash.String) != 64 {
		t.Fatal("device token was not stored as a SHA-256 hash")
	}

	device, err := service.AuthenticateDevice(ctx, enrollment.DeviceToken)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := service.Heartbeat(ctx, device, Heartbeat{
		DeviceCode: "123456789", ServiceStatus: "running",
		PermissionStatus: map[string]any{"media_projection": true},
		Capabilities:     map[string]any{"screen": true}, AndroidSDK: 36,
	})
	if err != nil || len(commands) != 0 {
		t.Fatalf("initial heartbeat: commands=%#v err=%v", commands, err)
	}
	var appVersion string
	var appNativeCode int
	if err = db.QueryRowContext(ctx, `SELECT app_version,app_native_code FROM remote_devices WHERE id=?`, deviceID).Scan(&appVersion, &appNativeCode); err != nil {
		t.Fatal(err)
	}
	if appVersion != "8.2.0" || appNativeCode != 216 {
		t.Fatalf("heartbeat erased enrollment metadata: version=%q code=%d", appVersion, appNativeCode)
	}
	credential, err := service.CreateCredential(ctx, deviceID, adminID)
	if err != nil {
		t.Fatal(err)
	}
	commands, err = service.Heartbeat(ctx, device, Heartbeat{DeviceCode: "123456789", ServiceStatus: "running"})
	if err != nil || len(commands) != 1 {
		t.Fatalf("credential heartbeat: commands=%#v err=%v", commands, err)
	}
	if commands[0].Type != "authorize_session" {
		t.Fatalf("unexpected authorization command %q", commands[0].Type)
	}
	var commandPayload map[string]any
	if err = json.Unmarshal(commands[0].Payload, &commandPayload); err != nil {
		t.Fatal(err)
	}
	if _, leaked := commandPayload["password"]; leaked || commandPayload["credential_request_id"] != credential.ID {
		t.Fatal("authorization payload is invalid")
	}
	var commandCipher, credentialCipher []byte
	if err = db.QueryRowContext(ctx, `SELECT command.payload_ciphertext,credential.authorization_ciphertext FROM remote_commands command JOIN remote_credential_requests credential ON credential.command_id=command.id WHERE credential.id=?`, credential.ID).Scan(&commandCipher, &credentialCipher); err != nil {
		t.Fatal(err)
	}
	if len(commandCipher) < 32 || len(credentialCipher) < 32 {
		t.Fatal("authorization records were not encrypted")
	}
	if err = service.AckCommand(ctx, device, commands[0].ID, Ack{Status: "applied"}); err != nil {
		t.Fatal(err)
	}
	status, err := service.CredentialStatus(ctx, credential.ID, adminID)
	if err != nil || status.Status != "ready" {
		t.Fatalf("credential status=%#v err=%v", status, err)
	}
	revealed, err := service.RevealCredential(ctx, credential.ID, adminID)
	if err != nil {
		t.Fatal(err)
	}
	if revealed.ControlToken == "" || !revealed.ExpiresAt.After(time.Now()) {
		t.Fatal("control authorization was not issued")
	}
	jpeg := []byte{0xff, 0xd8, 0xff, 0xd9}
	if err = service.StoreFrame(ctx, device, ScreenFrame{JPEG: jpeg, Width: 1, Height: 1, Sequence: 1}); err != nil {
		t.Fatal(err)
	}
	frame, err := service.Frame(ctx, deviceID, adminID, revealed.ControlToken)
	if err != nil || frame.Sequence != 1 {
		t.Fatalf("read frame=%#v err=%v", frame, err)
	}
	if err = service.QueueControlCommand(ctx, deviceID, adminID, revealed.ControlToken, "tap", map[string]any{"x": 0.5, "y": 0.25}); err != nil {
		t.Fatal(err)
	}
	commands, err = service.Heartbeat(ctx, device, Heartbeat{DeviceCode: "123456789", ServiceStatus: "running"})
	if err != nil || len(commands) != 1 || commands[0].Type != "tap" {
		t.Fatalf("control heartbeat: commands=%#v err=%v", commands, err)
	}
	if err = service.AckCommand(ctx, device, commands[0].ID, Ack{Status: "applied"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RevealCredential(ctx, credential.ID, adminID); !errors.Is(err, ErrAlreadyRevealed) {
		t.Fatalf("second reveal returned %v", err)
	}
	if _, err = service.CreateCredential(ctx, deviceID, adminID); err != nil {
		t.Fatal(err)
	}
	if err = service.RevokeDevice(ctx, deviceID); err != nil {
		t.Fatal(err)
	}
	commands, err = service.Heartbeat(ctx, Device{ID: device.ID, Status: 2}, Heartbeat{DeviceCode: "123456789", ServiceStatus: "running"})
	if err != nil || len(commands) != 1 || commands[0].Type != "stop" {
		t.Fatalf("revoking device received non-stop commands: commands=%#v err=%v", commands, err)
	}
	if err = service.AckCommand(ctx, Device{ID: device.ID, Status: 2}, commands[0].ID, Ack{Status: "applied", Result: map[string]any{"code": ""}}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AuthenticateDevice(ctx, enrollment.DeviceToken); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked device token remained valid: %v", err)
	}
}
