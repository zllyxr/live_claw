package game

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/database"
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
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddress, DB: 15})
	defer redisClient.Close()
	if err = redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	defer redisClient.FlushDB(context.Background()) //nolint:errcheck

	userID := time.Now().UnixNano() & 0x3fffffffffffffff
	walletService := wallet.New(db)
	if _, err = walletService.Apply(ctx, wallet.ApplyRequest{
		UserID: userID, Amount: 2000, BusinessType: "test_game_credit", BusinessID: "credit",
	}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.ExecContext(context.Background(), "DELETE FROM fishing_checkpoints WHERE session_id IN (SELECT id FROM game_sessions WHERE user_id=?)", userID) //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM game_sessions WHERE user_id=?", userID)                                                          //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)                                                  //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM wallet_holds WHERE user_id=?", userID)                                                           //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM wallet_accounts WHERE user_id=?", userID)                                                        //nolint:errcheck
	}()

	service := NewService(db, NewMatchmaker(redisClient), walletService)
	launch, err := service.EnterFishing(ctx, userID, "novice")
	if err != nil {
		t.Fatal(err)
	}
	if launch.Multiplier != 1 || launch.Table < 1 || launch.Table > 300 ||
		launch.Seat < 1 || launch.Seat > 4 || launch.EscrowAmount != 1000 {
		t.Fatalf("invalid fishing launch: %#v", launch)
	}
	balance, err := walletService.Balance(ctx, userID)
	if err != nil || balance.Available != 1000 || balance.Frozen != 1000 {
		t.Fatalf("escrow was not frozen: %#v %v", balance, err)
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
	if miss.Captured || miss.Reward != 0 || miss.Bet != 1 || miss.Balance != 999 {
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
	if _, err = db.ExecContext(ctx, `
		INSERT INTO fishing_checkpoints
			(session_id,event_seq,escrow_balance,total_cost,total_reward,state_payload,state_hash)
		VALUES(?,2,1250,500,750,JSON_OBJECT('test',true),REPEAT('a',64))`,
		launch.SessionID,
	); err != nil {
		t.Fatal(err)
	}
	settlement, err := service.LeaveFishing(ctx, userID, launch.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if settlement.Available != 2250 || settlement.Frozen != 0 ||
		settlement.GameCode != "deepsea_hunter" || settlement.VenueCode != "novice" ||
		settlement.TableNo != launch.Table || settlement.RoundNo != launch.SessionID {
		t.Fatalf("invalid fishing settlement: %#v", settlement)
	}
	repeated, err := service.LeaveFishing(ctx, userID, launch.SessionID)
	if err != nil || repeated.EntryNo != settlement.EntryNo {
		t.Fatalf("fishing settlement was not idempotent: %#v %v", repeated, err)
	}
}
