package scheduler

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestLegacyFishingHoldIsSettledBeforeGenericExpiryIntegration(t *testing.T) {
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
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	runtimeDB := db
	if runtimeDSN := strings.TrimSpace(os.Getenv("CLAW_TEST_SCHEDULER_MYSQL_DSN")); runtimeDSN != "" {
		runtimeDB, err = database.Open(ctx, runtimeDSN)
		if err != nil {
			t.Fatalf("open restricted scheduler runtime database: %v", err)
		}
		t.Cleanup(func() { _ = runtimeDB.Close() })
	}
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,?,'捕鱼旧托管迁移用户',1)`,
		fmt.Sprintf("legacy-fishing-%d", time.Now().UnixNano()), "integration-test-only",
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	sessionID, _ := idgen.New()
	t.Cleanup(func() {
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanup, "DELETE FROM fishing_checkpoints WHERE session_id=?", sessionID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM game_sessions WHERE id=?", sessionID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM wallet_holds WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM wallet_accounts WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM users WHERE id=?", userID)
	})
	walletService := wallet.New(db)
	if _, err = walletService.Apply(ctx, wallet.ApplyRequest{
		UserID: userID, Amount: 2000, BusinessType: "test_legacy_fishing_credit", BusinessID: "credit",
	}); err != nil {
		t.Fatal(err)
	}
	hold, err := walletService.PlaceHold(ctx, wallet.HoldRequest{
		UserID: userID, Amount: 1000,
		BusinessType: "game_session", BusinessID: sessionID,
		ExpiresAt: time.Now().Add(-time.Minute), Description: "旧捕鱼场次托管",
	})
	if err != nil {
		t.Fatal(err)
	}
	var gameID, venueID int64
	if err = db.QueryRowContext(ctx, `
		SELECT game.id,venue.id
		FROM games game JOIN game_venues venue ON venue.game_id=game.id
		WHERE game.game_code='deepsea_hunter' AND venue.venue_code='novice'`,
	).Scan(&gameID, &venueID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		INSERT INTO game_sessions
			(id,user_id,game_id,venue_id,table_no,seat_no,resume_token_hash,
			 escrow_hold_no,wallet_mode,escrow_balance,event_seq,status,connected_at,expires_at)
		VALUES(?,?,?,?,1,1,REPEAT('a',64),?,0,750,0,4,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3))`,
		sessionID, userID, gameID, venueID, hold.HoldNo,
	); err != nil {
		t.Fatal(err)
	}
	runner := &Runner{db: runtimeDB, wallet: wallet.New(runtimeDB), now: time.Now}
	if err = runner.releaseExpiredHolds(ctx); err != nil {
		t.Fatal(err)
	}
	var holdStatus int
	if err = db.QueryRowContext(ctx, "SELECT status FROM wallet_holds WHERE hold_no=?", hold.HoldNo).Scan(&holdStatus); err != nil {
		t.Fatal(err)
	}
	if holdStatus != 0 {
		t.Fatalf("generic expiry released a legacy fishing hold: status=%d", holdStatus)
	}
	if err = runner.settleLegacyFishingHold(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	if err = runner.settleLegacyFishingHold(ctx, sessionID); err != nil {
		t.Fatalf("legacy settlement was not idempotent: %v", err)
	}
	balance, err := walletService.Balance(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Available != 1750 || balance.Frozen != 0 {
		t.Fatalf("legacy payout was not reconciled exactly: %#v", balance)
	}
	var ledgerCount int
	var deltaAvailable, deltaFrozen int64
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*),COALESCE(SUM(delta_available),0),COALESCE(SUM(delta_frozen),0)
		FROM wallet_ledger_entries
		WHERE user_id=? AND business_type='hold_commit/game_session' AND business_id=?`,
		userID, sessionID,
	).Scan(&ledgerCount, &deltaAvailable, &deltaFrozen); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 || deltaAvailable != 750 || deltaFrozen != -1000 {
		t.Fatalf(
			"unexpected legacy settlement ledger: count=%d available=%d frozen=%d",
			ledgerCount, deltaAvailable, deltaFrozen,
		)
	}
}
