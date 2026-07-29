package invite

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestInviteTeamBindingIntegration(t *testing.T) {
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
	service := New(db)
	teamID, teamCode, err := service.CreateTeam(ctx, "邀请码集成测试团队", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	inviterID := time.Now().UnixNano() & 0x3fffffffffffffff
	inviteeID := inviterID + 1
	var systemTeamID int64
	if err = db.QueryRowContext(ctx, "SELECT id FROM teams WHERE code='sys'").Scan(&systemTeamID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		INSERT INTO users(id,username,password_hash,team_id,status)
		VALUES(?,?,'test',?,1),(?,?,'test',?,1)`,
		inviterID, "invite_owner_"+teamCode, teamID,
		inviteeID, "invite_member_"+teamCode, systemTeamID,
	); err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.ExecContext(context.Background(), "DELETE FROM invite_relations WHERE invitee_user_id=?", inviteeID)              //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM invite_code_aliases WHERE user_id IN (?,?)", inviterID, inviteeID) //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM invite_codes WHERE user_id IN (?,?)", inviterID, inviteeID)        //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM team_members WHERE user_id IN (?,?)", inviterID, inviteeID)        //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM users WHERE id IN (?,?)", inviterID, inviteeID)                    //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM teams WHERE id=?", teamID)                                         //nolint:errcheck
	}()
	inviterCode, err := service.AssignUserCode(ctx, inviterID, teamID, teamCode)
	if err != nil {
		t.Fatal(err)
	}
	oldInviteeCode, err := service.EnsureUserCode(ctx, inviteeID)
	if err != nil {
		t.Fatal(err)
	}
	compactUpper := strings.ToUpper(strings.ReplaceAll(inviterCode.FullCode, "-", ""))
	bound, err := service.Bind(ctx, inviteeID, compactUpper, "integration")
	if err != nil {
		t.Fatal(err)
	}
	if bound.InviterUserID != inviterID || bound.TeamID != teamID ||
		!strings.HasPrefix(bound.UserCode.FullCode, teamCode+"-") ||
		!ValidCode(bound.UserCode.FullCode) {
		t.Fatalf("unexpected invite binding: %#v", bound)
	}
	var aliasUserID int64
	if err = db.QueryRowContext(ctx, `
		SELECT user_id FROM invite_code_aliases WHERE alias_code=?`,
		oldInviteeCode.FullCode,
	).Scan(&aliasUserID); err != nil || aliasUserID != inviteeID {
		t.Fatalf("old code alias was not preserved: user=%d err=%v", aliasUserID, err)
	}
	repeated, err := service.Bind(ctx, inviteeID, inviterCode.FullCode, "integration")
	if err != nil || !repeated.AlreadyBound || repeated.UserCode.FullCode != bound.UserCode.FullCode {
		t.Fatalf("repeat binding was not idempotent: %#v %v", repeated, err)
	}
}
