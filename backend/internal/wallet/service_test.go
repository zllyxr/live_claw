package wallet

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestValidateBusiness(t *testing.T) {
	if err := validateBusiness(1, "game_bet", "round-1"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		userID       int64
		businessType string
		businessID   string
	}{
		{0, "game_bet", "round-1"},
		{1, "", "round-1"},
		{1, " game_bet", "round-1"},
		{1, "game_bet", ""},
	} {
		if err := validateBusiness(test.userID, test.businessType, test.businessID); err == nil {
			t.Fatalf("accepted invalid business identity: %#v", test)
		}
	}
}

func TestWalletIntegration(t *testing.T) {
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
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	userID := time.Now().UnixNano() & 0x3fffffffffffffff
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_holds WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id=?", userID)
	})
	service := New(db)
	credit := ApplyRequest{
		UserID: userID, Amount: 1000, BusinessType: "test_credit",
		BusinessID: "credit-1", Description: "integration test",
	}
	first, err := service.Apply(ctx, credit)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := service.Apply(ctx, credit)
	if err != nil {
		t.Fatal(err)
	}
	if first.EntryNo != repeated.EntryNo || repeated.Available != 1000 {
		t.Fatalf("credit was not idempotent: %#v %#v", first, repeated)
	}

	hold, err := service.PlaceHold(ctx, HoldRequest{
		UserID: userID, Amount: 400, BusinessType: "game_session",
		BusinessID: "session-1", ExpiresAt: time.Now().Add(time.Hour),
		GameCode: "deepsea_hunter", VenueCode: "expert", TableNo: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	repeatedHold, err := service.PlaceHold(ctx, HoldRequest{
		UserID: userID, Amount: 400, BusinessType: "game_session",
		BusinessID: "session-1", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil || repeatedHold.HoldNo != hold.HoldNo {
		t.Fatalf("hold was not idempotent: %#v %v", repeatedHold, err)
	}

	settlement, err := service.CommitHold(ctx, CommitRequest{
		HoldNo: hold.HoldNo, Payout: 700, GameCode: "deepsea_hunter",
		VenueCode: "expert", TableNo: 12, RoundNo: "fish-session-1",
		Description: "fishing session settlement",
	})
	if err != nil {
		t.Fatal(err)
	}
	if settlement.Available != 1300 || settlement.Frozen != 0 ||
		settlement.DeltaAvailable != 700 || settlement.DeltaFrozen != -400 {
		t.Fatalf("unexpected settlement: %#v", settlement)
	}
	repeatedSettlement, err := service.CommitHold(ctx, CommitRequest{
		HoldNo: hold.HoldNo, Payout: 700, GameCode: "deepsea_hunter",
		VenueCode: "expert", TableNo: 12, RoundNo: "fish-session-1",
	})
	if err != nil || repeatedSettlement.EntryNo != settlement.EntryNo {
		t.Fatalf("settlement was not idempotent: %#v %v", repeatedSettlement, err)
	}

	releaseHold, err := service.PlaceHold(ctx, HoldRequest{
		UserID: userID, Amount: 200, BusinessType: "withdraw",
		BusinessID: "withdraw-1", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	released, err := service.ReleaseHold(ctx, releaseHold.HoldNo, "cancelled", nil)
	if err != nil {
		t.Fatal(err)
	}
	if released.Available != 1300 || released.Frozen != 0 {
		t.Fatalf("unexpected released balance: %#v", released)
	}
	if _, err = service.PlaceHold(ctx, HoldRequest{
		UserID: userID, Amount: 2000, BusinessType: "test_large",
		BusinessID: "too-large", ExpiresAt: time.Now().Add(time.Hour),
	}); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected insufficient funds, got %v", err)
	}
}
