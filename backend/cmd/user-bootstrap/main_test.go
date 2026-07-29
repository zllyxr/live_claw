package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/invite"
)

func TestLoadAndPrepareAppUser(t *testing.T) {
	values := map[string]string{
		"V2_APP_USER_USERNAME":     " 13800138000 ",
		"V2_APP_USER_PASSWORD":     "Bootstrap-2026!",
		"V2_APP_USER_NICKNAME":     "",
		"V2_APP_USER_EMAIL":        " Bootstrap.User@Example.COM ",
		"V2_APP_USER_COUNTRY_CODE": "",
	}
	input, err := loadAppUserInput(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if input.Username != "13800138000" ||
		input.Nickname != "13800138000" ||
		input.Email != "bootstrap.user@example.com" ||
		input.CountryCode != "86" {
		t.Fatalf("unexpected normalized input: %#v", input)
	}
	prepared, err := prepareAppUser(input)
	if err != nil {
		t.Fatal(err)
	}
	if !adminauth.VerifyPassword(prepared.PasswordHash, values["V2_APP_USER_PASSWORD"]) {
		t.Fatal("prepared password hash cannot verify the input password")
	}
}

func TestLoadAppUserInputRejectsInvalidValues(t *testing.T) {
	valid := map[string]string{
		"V2_APP_USER_USERNAME":     "13800138000",
		"V2_APP_USER_PASSWORD":     "Bootstrap-2026!",
		"V2_APP_USER_NICKNAME":     "Bootstrap User",
		"V2_APP_USER_EMAIL":        "bootstrap@example.com",
		"V2_APP_USER_COUNTRY_CODE": "86",
	}
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "missing username", key: "V2_APP_USER_USERNAME", value: ""},
		{name: "username is not mobile digits", key: "V2_APP_USER_USERNAME", value: "bootstrap_user"},
		{name: "invalid email", key: "V2_APP_USER_EMAIL", value: "not-an-email"},
		{name: "invalid country", key: "V2_APP_USER_COUNTRY_CODE", value: "+86"},
		{name: "nickname control", key: "V2_APP_USER_NICKNAME", value: "name\nvalue"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := make(map[string]string, len(valid))
			for key, value := range valid {
				values[key] = value
			}
			values[test.key] = test.value
			if _, err := loadAppUserInput(func(key string) string { return values[key] }); err == nil {
				t.Fatal("expected invalid input to be rejected")
			}
		})
	}
}

func TestPrepareAppUserRejectsWeakPassword(t *testing.T) {
	input := appUserInput{
		Username: "13800138000", Password: "abcdefghijkl",
		Nickname: "Bootstrap User", Email: "bootstrap@example.com", CountryCode: "86",
	}
	if _, err := prepareAppUser(input); err == nil {
		t.Fatal("expected weak password to be rejected")
	}
}

func TestCreateAppUserIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
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

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	mobile := strconv.FormatInt(time.Now().UnixNano()%9_000_000_000+10_000_000_000, 10)
	input, err := prepareAppUser(appUserInput{
		Username:    mobile,
		Password:    "Bootstrap-2026!",
		Nickname:    "生产初始化用户",
		Email:       "bootstrap_" + suffix + "@example.test",
		CountryCode: "86",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := createAppUser(ctx, db, input)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupAppUserTest(db, result.ID)

	var username, country, persistedMobile, email, nickname, hash, algorithm, teamCode string
	var status, isVirtual int
	var teamID int64
	err = db.QueryRowContext(ctx, `
		SELECT user.username,user.country_code,user.mobile,user.email,user.nickname,
		       user.password_hash,user.password_algo,user.status,user.is_virtual,
		       user.team_id,team.code
		FROM users user
		JOIN teams team ON team.id=user.team_id
		WHERE user.id=?`,
		result.ID,
	).Scan(
		&username, &country, &persistedMobile, &email, &nickname, &hash, &algorithm,
		&status, &isVirtual, &teamID, &teamCode,
	)
	if err != nil {
		t.Fatal(err)
	}
	if username != input.Username || country != input.CountryCode ||
		persistedMobile != input.Username || email != input.Email || nickname != input.Nickname ||
		algorithm != "argon2id" || status != 1 || isVirtual != 0 ||
		teamID < 1 || teamCode != "sys" ||
		!adminauth.VerifyPassword(hash, input.Password) {
		t.Fatalf(
			"unexpected persisted user: username=%q country=%q email=%q nickname=%q algorithm=%q status=%d virtual=%d team=%d/%q",
			username, country, email, nickname, algorithm, status, isVirtual, teamID, teamCode,
		)
	}
	if !invite.ValidCode(result.InviteCode) ||
		!strings.HasPrefix(result.InviteCode, "sys-") {
		t.Fatalf("invalid system invitation code %q", result.InviteCode)
	}

	var walletCount, membershipCount, inviteCount int
	err = db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM wallet_accounts
		   WHERE user_id=? AND currency='COIN' AND available=0 AND frozen=0 AND status=1),
		  (SELECT COUNT(*) FROM team_members
		   WHERE user_id=? AND team_id=? AND inviter_user_id=0 AND status=1),
		  (SELECT COUNT(*) FROM invite_codes
		   WHERE user_id=? AND team_id=? AND full_code=? AND status=1)`,
		result.ID,
		result.ID, teamID,
		result.ID, teamID, result.InviteCode,
	).Scan(&walletCount, &membershipCount, &inviteCount)
	if err != nil {
		t.Fatal(err)
	}
	if walletCount != 1 || membershipCount != 1 || inviteCount != 1 {
		t.Fatalf(
			"bootstrap relations are incomplete: wallet=%d membership=%d invite=%d",
			walletCount, membershipCount, inviteCount,
		)
	}

	duplicate := input
	duplicate.Nickname = "不应覆盖"
	duplicate.Password = "Different-2026!"
	duplicate.PasswordHash, err = adminauth.HashPassword(duplicate.Password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = createAppUser(ctx, db, duplicate); !errors.Is(err, errAppUserExists) {
		t.Fatalf("duplicate bootstrap error = %v, want %v", err, errAppUserExists)
	}
	var persistedNickname, persistedHash string
	if err = db.QueryRowContext(ctx,
		"SELECT nickname,password_hash FROM users WHERE id=?",
		result.ID,
	).Scan(&persistedNickname, &persistedHash); err != nil {
		t.Fatal(err)
	}
	if persistedNickname != input.Nickname ||
		!adminauth.VerifyPassword(persistedHash, input.Password) ||
		adminauth.VerifyPassword(persistedHash, duplicate.Password) {
		t.Fatal("duplicate bootstrap overwrote the existing account")
	}
}

func cleanupAppUserTest(db *sql.DB, userID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, query := range []string{
		"DELETE FROM invite_codes WHERE user_id=?",
		"DELETE FROM team_members WHERE user_id=?",
		"DELETE FROM wallet_accounts WHERE user_id=?",
		"DELETE FROM users WHERE id=?",
	} {
		if _, err := db.ExecContext(ctx, query, userID); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup user bootstrap test: %v\n", err)
		}
	}
}
