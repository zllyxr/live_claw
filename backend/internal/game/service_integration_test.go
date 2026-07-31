package game

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestFishingSessionIntegration(t *testing.T) {
	dsn := os.Getenv("CLAW_TEST_MYSQL_DSN")
	redisAddress := os.Getenv("CLAW_TEST_REDIS_ADDR")
	if dsn == "" || redisAddress == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN and CLAW_TEST_REDIS_ADDR are not configured")
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
	runtimeDB := db
	if runtimeDSN := os.Getenv("CLAW_TEST_GAME_MYSQL_DSN"); runtimeDSN != "" {
		runtimeDB, err = database.Open(ctx, runtimeDSN)
		if err != nil {
			t.Fatalf("open restricted game runtime database: %v", err)
		}
		defer runtimeDB.Close()
	}
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddress, DB: 15})
	defer redisClient.Close()
	if err = redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	defer redisClient.FlushDB(context.Background()) //nolint:errcheck

	userInsert, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,?,'捕鱼测试用户',1)`,
		fmt.Sprintf("fishing-test-%d", time.Now().UnixNano()), "integration-test-only",
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := userInsert.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.ExecContext(context.Background(), "DELETE FROM fishing_checkpoints WHERE session_id IN (SELECT id FROM game_sessions WHERE user_id=?)", userID) //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM game_sessions WHERE user_id=?", userID)                                                          //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)                                                  //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM wallet_holds WHERE user_id=?", userID)                                                           //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM wallet_accounts WHERE user_id=?", userID)                                                        //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM users WHERE id=?", userID)                                                                       //nolint:errcheck
	}()
	walletService := wallet.New(runtimeDB)
	if _, err = walletService.Apply(ctx, wallet.ApplyRequest{
		UserID: userID, Amount: 2000, BusinessType: "test_game_credit", BusinessID: "credit",
	}); err != nil {
		t.Fatal(err)
	}

	service := NewService(runtimeDB, NewMatchmaker(redisClient), walletService)
	fixedNow := time.Now().Truncate(time.Millisecond)
	service.now = func() time.Time { return fixedNow }
	launch, err := service.EnterFishing(ctx, userID, "novice")
	if err != nil {
		t.Fatal(err)
	}
	if launch.Multiplier != 1 || launch.Table < 1 || launch.Table > 300 ||
		launch.Seat < 1 || launch.Seat > 4 || launch.EscrowAmount != 0 || launch.WalletBalance != 2000 {
		t.Fatalf("invalid fishing launch: %#v", launch)
	}
	balance, err := walletService.Balance(ctx, userID)
	if err != nil || balance.Available != 2000 || balance.Frozen != 0 {
		t.Fatalf("entering fishing changed the unified wallet: %#v %v", balance, err)
	}
	resumed, err := service.EnterFishing(ctx, userID, "novice")
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Resumed || resumed.SessionID != launch.SessionID ||
		resumed.Table != launch.Table || resumed.Seat != launch.Seat ||
		resumed.ResumeToken == launch.ResumeToken {
		t.Fatalf("session was not safely resumed: %#v", resumed)
	}
	miss, err := service.FireFishing(ctx, launch.SessionID, userID, "empty-water-shot", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if miss.Captured || miss.Reward != 0 || miss.Bet != 1 || miss.Balance != 1999 {
		t.Fatalf("empty-water shot was not charged as a normal miss: %#v", miss)
	}
	var checkpointCost, checkpointReward int64
	if err = db.QueryRowContext(ctx, `
		SELECT total_cost,total_reward
		FROM fishing_checkpoints
		WHERE session_id=? AND client_command_id=?`,
		launch.SessionID, "empty-water-shot",
	).Scan(&checkpointCost, &checkpointReward); err != nil {
		t.Fatal(err)
	}
	if checkpointCost != 1 || checkpointReward != 0 {
		t.Fatalf("invalid empty-water checkpoint totals: cost=%d reward=%d", checkpointCost, checkpointReward)
	}
	replayedMiss, err := service.FireFishing(ctx, launch.SessionID, userID, "empty-water-shot", 1, 0)
	if err != nil || !replayedMiss.Replayed || replayedMiss.Balance != miss.Balance {
		t.Fatalf("empty-water shot was not idempotent: %#v %v", replayedMiss, err)
	}
	if _, err = walletService.Apply(ctx, wallet.ApplyRequest{
		UserID: userID, Amount: 100, BusinessType: "test_game_credit", BusinessID: "live-topup",
	}); err != nil {
		t.Fatal(err)
	}
	refreshedReplay, err := service.FireFishing(ctx, launch.SessionID, userID, "empty-water-shot", 1, 0)
	if err != nil || !refreshedReplay.Replayed || refreshedReplay.Balance != 2099 {
		t.Fatalf("replayed shot did not return the latest unified wallet: %#v %v", refreshedReplay, err)
	}
	type concurrentFireResult struct {
		result FishingFireResult
		err    error
	}
	concurrentResults := make(chan concurrentFireResult, 2)
	for range 2 {
		go func() {
			result, fireErr := service.FireFishing(
				ctx, launch.SessionID, userID, "concurrent-unified-shot", 1, 0,
			)
			concurrentResults <- concurrentFireResult{result: result, err: fireErr}
		}()
	}
	replayed := 0
	for range 2 {
		outcome := <-concurrentResults
		if outcome.err != nil || outcome.result.Balance != 2098 {
			t.Fatalf("concurrent idempotent shot failed: %#v %v", outcome.result, outcome.err)
		}
		if outcome.result.Replayed {
			replayed++
		}
	}
	if replayed != 1 {
		t.Fatalf("concurrent command was not applied exactly once: replayed=%d", replayed)
	}
	service.MarkFishingDisconnected(ctx, launch.SessionID, userID)
	recoveredShot, err := service.FireFishing(
		ctx, launch.SessionID, userID, "disconnected-session-shot", 1, 0,
	)
	if err != nil || recoveredShot.Balance != 2097 || recoveredShot.Captured {
		t.Fatalf("authenticated disconnected fishing session did not self-heal: %#v %v", recoveredShot, err)
	}
	var recoveredStatus int
	var recoveredDisconnectedAt sql.NullTime
	if err = db.QueryRowContext(ctx, `
		SELECT status,disconnected_at FROM game_sessions WHERE id=? AND user_id=?`,
		launch.SessionID, userID,
	).Scan(&recoveredStatus, &recoveredDisconnectedAt); err != nil {
		t.Fatal(err)
	}
	if recoveredStatus != 1 || recoveredDisconnectedAt.Valid {
		t.Fatalf(
			"fishing session did not return active after a live shot: status=%d disconnected=%v",
			recoveredStatus, recoveredDisconnectedAt,
		)
	}
	projectile, err := service.FireFishing(
		ctx, launch.SessionID, userID, "physical-projectile-shot", 1, 0,
	)
	if err != nil || projectile.Balance != 2096 || projectile.Captured {
		t.Fatalf("physical projectile launch was not charged exactly once: %#v %v", projectile, err)
	}
	hit, err := service.ResolveFishingHit(
		ctx, launch.SessionID, userID, "physical-projectile-shot", 2,
	)
	if err != nil || hit.Bet != 1 || hit.Multiplier != 2 ||
		hit.Balance != projectile.Balance+hit.Reward ||
		(hit.Captured && hit.Reward != 2) || (!hit.Captured && hit.Reward != 0) {
		t.Fatalf("physical projectile hit was not settled correctly: %#v %v", hit, err)
	}
	replayedHit, err := service.ResolveFishingHit(
		ctx, launch.SessionID, userID, "physical-projectile-shot", 2,
	)
	if err != nil || !replayedHit.Replayed || replayedHit.Balance != hit.Balance ||
		replayedHit.Reward != hit.Reward {
		t.Fatalf("physical projectile hit was not idempotent: %#v %v", replayedHit, err)
	}
	service.MarkFishingDisconnected(ctx, launch.SessionID, userID)
	if _, err = service.AuthenticateFishingSession(
		ctx, launch.SessionID, resumed.ResumeToken,
	); err != nil {
		t.Fatalf("disconnected fishing session did not resume: %v", err)
	}
	if err = service.MarkFishingConnected(ctx, launch.SessionID, userID); err != nil {
		t.Fatalf("idempotent fishing connection refresh failed: %v", err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE game_sessions SET expires_at=? WHERE id=? AND user_id=?`,
		time.Now().Add(-time.Minute), launch.SessionID, userID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = service.FireFishing(
		ctx, launch.SessionID, userID, "expired-session-shot", 1, 0,
	); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expired fishing session accepted a new shot: %v", err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE game_sessions SET expires_at=? WHERE id=? AND user_id=?`,
		time.Now().Add(30*time.Minute), launch.SessionID, userID,
	); err != nil {
		t.Fatal(err)
	}
	beforeLeave, err := walletService.Balance(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	settlement, err := service.LeaveFishing(ctx, userID, launch.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if settlement.Available != beforeLeave.Available || settlement.Frozen != 0 {
		t.Fatalf("leaving fishing changed the unified wallet: %#v before=%#v", settlement, beforeLeave)
	}
	repeated, err := service.LeaveFishing(ctx, userID, launch.SessionID)
	if err != nil || repeated.Available != settlement.Available || repeated.Frozen != settlement.Frozen {
		t.Fatalf("fishing leave was not idempotent: %#v %v", repeated, err)
	}
	if _, err = service.FireFishing(
		ctx, launch.SessionID, userID, "settled-session-shot", 1, 0,
	); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("settled fishing session accepted a new shot: %v", err)
	}
	var shotEntries int
	var shotDelta int64
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*),COALESCE(SUM(delta_available),0)
		FROM wallet_ledger_entries WHERE user_id=? AND business_type='fishing_shot'`,
		userID,
	).Scan(&shotEntries, &shotDelta); err != nil {
		t.Fatal(err)
	}
	if shotEntries != 4 || shotDelta != -4 {
		t.Fatalf("unexpected authoritative shot ledger: entries=%d delta=%d", shotEntries, shotDelta)
	}
	var rewardEntries int
	var rewardDelta int64
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*),COALESCE(SUM(delta_available),0)
		FROM wallet_ledger_entries WHERE user_id=? AND business_type='fishing_reward'`,
		userID,
	).Scan(&rewardEntries, &rewardDelta); err != nil {
		t.Fatal(err)
	}
	if (hit.Captured && (rewardEntries != 1 || rewardDelta != hit.Reward)) ||
		(!hit.Captured && (rewardEntries != 0 || rewardDelta != 0)) {
		t.Fatalf("unexpected fishing reward ledger: entries=%d delta=%d hit=%#v", rewardEntries, rewardDelta, hit)
	}
	legacySessionID, _ := idgen.New()
	legacyHoldNo, _ := idgen.New()
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
		VALUES(?,?,?,?,2,1,REPEAT('b',64),?,0,2,0,2,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3)+INTERVAL 30 MINUTE)`,
		legacySessionID, userID, gameID, venueID, legacyHoldNo,
	); err != nil {
		t.Fatal(err)
	}
	directLaunch, err := service.EnterFishing(ctx, userID, "novice")
	if err != nil {
		t.Fatal(err)
	}
	latestBalance, err := walletService.Balance(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if directLaunch.SessionID == legacySessionID || directLaunch.WalletBalance != latestBalance.Available {
		t.Fatalf("legacy session was resumed instead of unified wallet session: %#v", directLaunch)
	}
	var legacyStatus, directWalletMode int
	if err = db.QueryRowContext(ctx, "SELECT status FROM game_sessions WHERE id=?", legacySessionID).Scan(&legacyStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, "SELECT wallet_mode FROM game_sessions WHERE id=?", directLaunch.SessionID).Scan(&directWalletMode); err != nil {
		t.Fatal(err)
	}
	if legacyStatus != 4 || directWalletMode != 1 {
		t.Fatalf("legacy status=%d direct wallet mode=%d", legacyStatus, directWalletMode)
	}
}
