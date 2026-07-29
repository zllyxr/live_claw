package auth

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestLegacyLoginUpgradeIntegration(t *testing.T) {
	dsn := os.Getenv("CLAW_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
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

	userID := time.Now().UnixNano() & 0x3fffffffffffffff
	login := "legacy_" + time.Now().Format("150405.000000")
	password := "abc123"
	authCode := "local-migration-secret"
	first := md5.Sum([]byte(authCode + password))           //nolint:gosec
	second := md5.Sum([]byte(hex.EncodeToString(first[:]))) //nolint:gosec
	legacyHash := "###" + hex.EncodeToString(second[:])
	if _, err = db.ExecContext(ctx, `
		INSERT INTO users
			(id,username,country_code,mobile,password_hash,password_algo,nickname,status)
		VALUES(?,?,'86',?,?, 'legacy_cmf','迁移用户',1)`,
		userID, login, login, legacyHash,
	); err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.ExecContext(context.Background(), "DELETE FROM user_sessions WHERE user_id=?", userID) //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM users WHERE id=?", userID)              //nolint:errcheck
	}()

	service := New(db, Options{LegacyAuthCode: authCode, LegacyTablePrefix: "cmf_"})
	session, err := service.Login(ctx, "86", login, password, "test-device", "android", "127.0.0.1", "go-test")
	if err != nil {
		t.Fatal(err)
	}
	if session.User.ID != userID || session.Token == "" {
		t.Fatalf("invalid session: %#v", session)
	}
	if _, err = service.Authenticate(ctx, userID, session.Token); err != nil {
		t.Fatalf("new session did not authenticate: %v", err)
	}
	var upgradedHash, upgradedAlgorithm string
	if err = db.QueryRowContext(ctx, `
		SELECT password_hash,password_algo FROM users WHERE id=?`,
		userID,
	).Scan(&upgradedHash, &upgradedAlgorithm); err != nil {
		t.Fatal(err)
	}
	if upgradedAlgorithm != "argon2id" || !adminauth.VerifyPassword(upgradedHash, password) {
		t.Fatal("legacy password was not upgraded to Argon2id")
	}
}
