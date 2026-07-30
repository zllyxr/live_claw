package sports

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

type asyncSportsBetResult struct {
	order map[string]any
	err   error
}

func TestPlaceBetRevalidatesAfterWalletHold(t *testing.T) {
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
	publicMatchID := sportsRaceID("m", suffix)
	traceID := "sports-race-" + suffix
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'integration-test-only','体育下注竞态联调用户',1)`,
		"sports_race_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	var matchID, marketID, optionID int64
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_bet_items
			WHERE market_id=? OR option_id=? OR order_id IN (
				SELECT id FROM sports_bet_orders
				WHERE user_id=? AND client_trace_id=?
			)`,
			marketID, optionID, userID, traceID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_bet_orders WHERE user_id=? AND client_trace_id=?`,
			userID, traceID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_settlement_runs WHERE match_id=?`,
			matchID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_market_options WHERE id=?`,
			optionID,
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM sports_markets WHERE id=?", marketID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM sports_matches WHERE id=?", matchID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_holds WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", userID)
	})

	matchResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_matches
			(public_match_id,source,source_match_id,competition,competition_type,
			 home_name,away_name,kickoff_at,bet_close_at,match_status,bet_status,
			 settle_status,min_bet,max_bet)
		VALUES(?,'manual_admin',?,'竞态联赛','football','主队','客队',
		       CURRENT_TIMESTAMP(3)+INTERVAL 2 HOUR,
		       CURRENT_TIMESTAMP(3)+INTERVAL 90 MINUTE,'NS',1,0,1,10000)`,
		publicMatchID, "race-"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	matchID, _ = matchResult.LastInsertId()
	marketResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_markets
			(match_id,market_code,name,settlement_rule,status,sort_order)
		VALUES(?,'race_1x2','竞态胜平负','full_time',1,0)`,
		matchID,
	)
	if err != nil {
		t.Fatal(err)
	}
	marketID, _ = marketResult.LastInsertId()
	optionResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_market_options
			(market_id,option_code,name,odds_scaled,result,status)
		VALUES(?,'race_home','主胜',1800000,0,1)`,
		marketID,
	)
	if err != nil {
		t.Fatal(err)
	}
	optionID, _ = optionResult.LastInsertId()

	walletService := wallet.New(db)
	const startingBalance int64 = 1000
	if _, err = walletService.Apply(ctx, wallet.ApplyRequest{
		UserID: userID, Amount: startingBalance,
		BusinessType: "test_sports_race_credit", BusinessID: "credit-" + suffix,
		Description: "sports bet race integration test",
	}); err != nil {
		t.Fatal(err)
	}

	lockTx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}
	var lockConnectionID int64
	if err = lockTx.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&lockConnectionID); err != nil {
		_ = lockTx.Rollback()
		t.Fatal(err)
	}
	var accountID int64
	if err = lockTx.QueryRowContext(ctx, `
		SELECT id FROM wallet_accounts
		WHERE user_id=? AND currency='COIN'
		FOR UPDATE`,
		userID,
	).Scan(&accountID); err != nil {
		_ = lockTx.Rollback()
		t.Fatal(err)
	}

	type placeBetResult struct {
		order map[string]any
		err   error
	}
	placeCtx, placeCancel := context.WithTimeout(ctx, 20*time.Second)
	resultCh := make(chan placeBetResult, 1)
	service := New(db, walletService)
	go func() {
		order, placeErr := service.PlaceBet(placeCtx, BetRequest{
			UserID: userID, MatchID: publicMatchID, ClientTraceID: traceID,
			ItemsJSON: fmt.Sprintf(`[{"option_id":%d,"amount":100}]`, optionID),
		})
		resultCh <- placeBetResult{order: order, err: placeErr}
	}()
	placeFinished := false
	t.Cleanup(func() {
		placeCancel()
		_ = lockTx.Rollback()
		if placeFinished {
			return
		}
		select {
		case <-resultCh:
		case <-time.After(5 * time.Second):
		}
	})

	barrierCtx, barrierCancel := context.WithTimeout(ctx, 8*time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
barrier:
	for {
		select {
		case early := <-resultCh:
			placeFinished = true
			barrierCancel()
			t.Fatalf(
				"PlaceBet returned before reaching the wallet lock barrier: order=%#v err=%v",
				early.order, early.err,
			)
		case <-ticker.C:
			waiting, waitErr := walletAccountLockHasWaiter(
				barrierCtx, db, lockConnectionID, userID,
			)
			if waitErr != nil {
				barrierCancel()
				t.Fatalf("inspect wallet lock wait: %v", waitErr)
			}
			if waiting {
				break barrier
			}
		case <-barrierCtx.Done():
			barrierCancel()
			t.Fatal("timed out waiting for PlaceBet to block on the wallet account")
		}
	}
	barrierCancel()

	stateCtx, stateCancel := context.WithTimeout(ctx, 5*time.Second)
	result, err := db.ExecContext(stateCtx, `
		UPDATE sports_matches
		SET bet_status=0,settle_status=1
		WHERE id=? AND bet_status=1 AND settle_status=0`,
		matchID,
	)
	stateCancel()
	if err != nil {
		t.Fatal(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("expected to close one match, updated %d", affected)
	}
	if err = lockTx.Commit(); err != nil {
		t.Fatal(err)
	}

	var placed placeBetResult
	select {
	case placed = <-resultCh:
		placeFinished = true
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for PlaceBet to revalidate and release its hold")
	}
	placeCancel()
	if placed.order != nil {
		t.Fatalf("unexpected sports order: %#v", placed.order)
	}
	var sportsErr *Error
	if !errors.As(placed.err, &sportsErr) || sportsErr.Code != 1005 {
		t.Fatalf("expected closed-match error 1005, got %T: %v", placed.err, placed.err)
	}

	var orderCount, itemCount int
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sports_bet_orders
		WHERE user_id=? AND client_trace_id=?`,
		userID, traceID,
	).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sports_bet_items
		WHERE market_id=? OR option_id=?`,
		marketID, optionID,
	).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if orderCount != 0 || itemCount != 0 {
		t.Fatalf("rejected bet left order rows: orders=%d items=%d", orderCount, itemCount)
	}

	var holdStatus int
	var holdAmount int64
	var releasedAt sql.NullTime
	if err = db.QueryRowContext(ctx, `
		SELECT status,amount,released_at
		FROM wallet_holds
		WHERE user_id=? AND business_type='sports_bet' AND business_id=?`,
		userID, traceID,
	).Scan(&holdStatus, &holdAmount, &releasedAt); err != nil {
		t.Fatal(err)
	}
	if holdStatus != 2 || holdAmount != 100 || !releasedAt.Valid {
		t.Fatalf(
			"wallet hold was not released: status=%d amount=%d released_at=%v",
			holdStatus, holdAmount, releasedAt.Valid,
		)
	}
	balance, err := walletService.Balance(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Available != startingBalance || balance.Frozen != 0 {
		t.Fatalf(
			"rejected bet changed wallet balance: available=%d frozen=%d",
			balance.Available, balance.Frozen,
		)
	}
}

func TestPlaceBetRevalidatesAdminLimitChangesAfterWalletHold(t *testing.T) {
	tests := []struct {
		name       string
		minBet     int64
		maxBet     int64
		wantCode   int
		fixtureTag string
	}{
		{
			name: "minimum raised", minBet: 101, maxBet: 10_000,
			wantCode: 1009, fixtureTag: "limit_min",
		},
		{
			name: "maximum lowered", minBet: 1, maxBet: 99,
			wantCode: 1010, fixtureTag: "limit_max",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSportsBetFixture(t, test.fixtureTag)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			lockTx, err := fixture.db.BeginTx(
				ctx,
				&sql.TxOptions{Isolation: sql.LevelReadCommitted},
			)
			if err != nil {
				t.Fatal(err)
			}
			defer lockTx.Rollback() //nolint:errcheck
			var lockConnectionID int64
			if err = lockTx.QueryRowContext(
				ctx,
				"SELECT CONNECTION_ID()",
			).Scan(&lockConnectionID); err != nil {
				t.Fatal(err)
			}
			var accountID int64
			if err = lockTx.QueryRowContext(ctx, `
				SELECT id FROM wallet_accounts
				WHERE user_id=? AND currency='COIN'
				FOR UPDATE`,
				fixture.userID,
			).Scan(&accountID); err != nil {
				t.Fatal(err)
			}

			resultCh := make(chan asyncSportsBetResult, 1)
			placeCtx, placeCancel := context.WithTimeout(ctx, 20*time.Second)
			defer placeCancel()
			go func() {
				order, placeErr := fixture.service.PlaceBet(placeCtx, fixture.request(100))
				resultCh <- asyncSportsBetResult{order: order, err: placeErr}
			}()
			placeFinished := false
			t.Cleanup(func() {
				placeCancel()
				_ = lockTx.Rollback()
				if placeFinished {
					return
				}
				select {
				case <-resultCh:
				case <-time.After(5 * time.Second):
				}
			})

			barrierCtx, barrierCancel := context.WithTimeout(ctx, 8*time.Second)
			early, barrierErr := waitForSportsWalletAccountWaiter(
				barrierCtx,
				fixture.db,
				lockConnectionID,
				fixture.userID,
				resultCh,
			)
			barrierCancel()
			if early != nil {
				placeFinished = true
				t.Fatalf(
					"PlaceBet returned before wallet barrier: order=%#v err=%v",
					early.order,
					early.err,
				)
			}
			if barrierErr != nil {
				t.Fatal(barrierErr)
			}

			updateResult, err := fixture.db.ExecContext(ctx, `
				UPDATE sports_matches
				SET min_bet=?,max_bet=?
				WHERE id=?`,
				test.minBet, test.maxBet, fixture.matchID,
			)
			if err != nil {
				t.Fatal(err)
			}
			if updated, rowsErr := updateResult.RowsAffected(); rowsErr != nil {
				t.Fatal(rowsErr)
			} else if updated != 1 {
				t.Fatalf("administrator limit update affected %d matches", updated)
			}
			if err = lockTx.Commit(); err != nil {
				t.Fatal(err)
			}

			var placed asyncSportsBetResult
			select {
			case placed = <-resultCh:
				placeFinished = true
			case <-time.After(10 * time.Second):
				t.Fatal("timed out waiting for limit revalidation")
			}
			if placed.order != nil {
				t.Fatalf("limit change still created an order: %#v", placed.order)
			}
			var sportsErr *Error
			if !errors.As(placed.err, &sportsErr) || sportsErr.Code != test.wantCode {
				t.Fatalf(
					"limit change error = %T %v, want sports code %d",
					placed.err,
					placed.err,
					test.wantCode,
				)
			}

			var orderCount, holdStatus int
			var releasedAt sql.NullTime
			if err = fixture.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM sports_bet_orders
				WHERE user_id=? AND client_trace_id=?`,
				fixture.userID, fixture.traceID,
			).Scan(&orderCount); err != nil {
				t.Fatal(err)
			}
			if err = fixture.db.QueryRowContext(ctx, `
				SELECT status,released_at FROM wallet_holds
				WHERE user_id=? AND business_type='sports_bet' AND business_id=?`,
				fixture.userID, fixture.traceID,
			).Scan(&holdStatus, &releasedAt); err != nil {
				t.Fatal(err)
			}
			if orderCount != 0 || holdStatus != 2 || !releasedAt.Valid {
				t.Fatalf(
					"limit rejection state: orders=%d hold_status=%d released=%v",
					orderCount,
					holdStatus,
					releasedAt.Valid,
				)
			}
			balance, err := fixture.wallet.Balance(ctx, fixture.userID)
			if err != nil {
				t.Fatal(err)
			}
			if balance.Available != fixture.startingBalance || balance.Frozen != 0 {
				t.Fatalf("limit rejection changed wallet balance: %#v", balance)
			}
		})
	}
}

func TestPlaceBetRejectsRetryBackedByReleasedHold(t *testing.T) {
	fixture := newSportsBetFixture(t, "released")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	hold, err := fixture.wallet.PlaceHold(ctx, wallet.HoldRequest{
		UserID: fixture.userID, Amount: 100,
		BusinessType: "sports_bet", BusinessID: fixture.traceID,
		ExpiresAt: time.Now().Add(2 * time.Hour), Description: "released hold retry setup",
		GameCode: "sports", RoundNo: fixture.publicMatchID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.wallet.ReleaseHold(
		ctx, hold.HoldNo, "released hold retry setup", nil,
	); err != nil {
		t.Fatal(err)
	}

	order, err := fixture.service.PlaceBet(ctx, fixture.request(100))
	if order != nil {
		t.Fatalf("released hold retry created an order: %#v", order)
	}
	var sportsErr *Error
	if !errors.As(err, &sportsErr) || sportsErr.Code != 1001 {
		t.Fatalf("expected invalid trace error 1001, got %T: %v", err, err)
	}

	var orderCount, holdCount, holdStatus int
	if err = fixture.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sports_bet_orders
		WHERE user_id=? AND client_trace_id=?`,
		fixture.userID, fixture.traceID,
	).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if err = fixture.db.QueryRowContext(ctx, `
		SELECT COUNT(*),COALESCE(MAX(status),-1)
		FROM wallet_holds
		WHERE user_id=? AND business_type='sports_bet' AND business_id=?`,
		fixture.userID, fixture.traceID,
	).Scan(&holdCount, &holdStatus); err != nil {
		t.Fatal(err)
	}
	if orderCount != 0 || holdCount != 1 || holdStatus != 2 {
		t.Fatalf(
			"released hold retry state: orders=%d holds=%d hold_status=%d",
			orderCount, holdCount, holdStatus,
		)
	}
	balance, err := fixture.wallet.Balance(ctx, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Available != fixture.startingBalance || balance.Frozen != 0 {
		t.Fatalf(
			"released hold retry changed balance: available=%d frozen=%d",
			balance.Available, balance.Frozen,
		)
	}
}

func TestPlaceBetConcurrentDuplicateTraceUsesOneActiveHold(t *testing.T) {
	fixture := newSportsBetFixture(t, "duplicate")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const callers = 12
	start := make(chan struct{})
	type result struct {
		order map[string]any
		err   error
	}
	results := make(chan result, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-start
			order, err := fixture.service.PlaceBet(ctx, fixture.request(100))
			results <- result{order: order, err: err}
		}()
	}
	ready.Wait()
	close(start)

	var orderNo string
	for range callers {
		placed := <-results
		if placed.err != nil {
			t.Fatalf("duplicate trace PlaceBet failed: %v", placed.err)
		}
		currentOrderNo, ok := placed.order["order_no"].(string)
		if !ok || currentOrderNo == "" {
			t.Fatalf("PlaceBet returned invalid order: %#v", placed.order)
		}
		if orderNo == "" {
			orderNo = currentOrderNo
		} else if currentOrderNo != orderNo {
			t.Fatalf("duplicate trace returned different orders: %q and %q", orderNo, currentOrderNo)
		}
	}

	var orderCount, itemCount, holdCount, holdStatus int
	var holdAmount int64
	if err := fixture.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sports_bet_orders
		WHERE user_id=? AND client_trace_id=?`,
		fixture.userID, fixture.traceID,
	).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sports_bet_items item
		JOIN sports_bet_orders bet_order ON bet_order.id=item.order_id
		WHERE bet_order.user_id=? AND bet_order.client_trace_id=?`,
		fixture.userID, fixture.traceID,
	).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.QueryRowContext(ctx, `
		SELECT COUNT(*),COALESCE(MAX(status),-1),COALESCE(MAX(amount),0)
		FROM wallet_holds
		WHERE user_id=? AND business_type='sports_bet' AND business_id=?`,
		fixture.userID, fixture.traceID,
	).Scan(&holdCount, &holdStatus, &holdAmount); err != nil {
		t.Fatal(err)
	}
	if orderCount != 1 || itemCount != 1 ||
		holdCount != 1 || holdStatus != 0 || holdAmount != 100 {
		t.Fatalf(
			"duplicate trace state: orders=%d items=%d holds=%d status=%d amount=%d",
			orderCount, itemCount, holdCount, holdStatus, holdAmount,
		)
	}
	balance, err := fixture.wallet.Balance(ctx, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Available != fixture.startingBalance-100 || balance.Frozen != 100 {
		t.Fatalf(
			"duplicate trace charged incorrectly: available=%d frozen=%d",
			balance.Available, balance.Frozen,
		)
	}
}

type sportsBetFixture struct {
	db              *sql.DB
	service         *Service
	wallet          *wallet.Service
	userID          int64
	matchID         int64
	marketID        int64
	optionID        int64
	publicMatchID   string
	traceID         string
	startingBalance int64
}

func newSportsBetFixture(t *testing.T, label string) *sportsBetFixture {
	t.Helper()
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
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	publicMatchID := sportsRaceID(label, suffix)
	traceID := "sports-" + label + "-" + suffix
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'integration-test-only','体育资金一致性联调用户',1)`,
		"sports_"+label+"_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()

	fixture := &sportsBetFixture{
		db: db, userID: userID, publicMatchID: publicMatchID, traceID: traceID,
		startingBalance: 1000,
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE item FROM sports_bet_items item
			JOIN sports_bet_orders bet_order ON bet_order.id=item.order_id
			WHERE bet_order.user_id=? AND bet_order.client_trace_id=?`,
			fixture.userID, fixture.traceID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_bet_orders WHERE user_id=? AND client_trace_id=?`,
			fixture.userID, fixture.traceID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_settlement_runs WHERE match_id=?`,
			fixture.matchID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_market_options WHERE id=?`,
			fixture.optionID,
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM sports_markets WHERE id=?", fixture.marketID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM sports_matches WHERE id=?", fixture.matchID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id=?", fixture.userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_holds WHERE user_id=?", fixture.userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id=?", fixture.userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", fixture.userID)
	})

	matchResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_matches
			(public_match_id,source,source_match_id,competition,competition_type,
			 home_name,away_name,kickoff_at,bet_close_at,match_status,bet_status,
			 settle_status,min_bet,max_bet)
		VALUES(?,'manual_admin',?,'资金一致性联赛','football','主队','客队',
		       CURRENT_TIMESTAMP(3)+INTERVAL 2 HOUR,
		       CURRENT_TIMESTAMP(3)+INTERVAL 90 MINUTE,'NS',1,0,1,10000)`,
		publicMatchID, label+"-"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.matchID, _ = matchResult.LastInsertId()
	marketResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_markets
			(match_id,market_code,name,settlement_rule,status,sort_order)
		VALUES(?,'funds_1x2','资金胜平负','full_time',1,0)`,
		fixture.matchID,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.marketID, _ = marketResult.LastInsertId()
	optionResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_market_options
			(market_id,option_code,name,odds_scaled,result,status)
		VALUES(?,'funds_home','主胜',1800000,0,1)`,
		fixture.marketID,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.optionID, _ = optionResult.LastInsertId()

	fixture.wallet = wallet.New(db)
	if _, err = fixture.wallet.Apply(ctx, wallet.ApplyRequest{
		UserID: fixture.userID, Amount: fixture.startingBalance,
		BusinessType: "test_sports_funds_credit", BusinessID: "credit-" + suffix,
		Description: "sports bet funds integration test",
	}); err != nil {
		t.Fatal(err)
	}
	fixture.service = New(db, fixture.wallet)
	return fixture
}

func (f *sportsBetFixture) request(amount int64) BetRequest {
	return BetRequest{
		UserID: f.userID, MatchID: f.publicMatchID, ClientTraceID: f.traceID,
		ItemsJSON: fmt.Sprintf(`[{"option_id":%d,"amount":%d}]`, f.optionID, amount),
	}
}

func walletAccountLockHasWaiter(
	ctx context.Context,
	db *sql.DB,
	blockerConnectionID int64,
	userID int64,
) (bool, error) {
	var count int
	// MySQL exposes a user's own sessions here without PROCESS or
	// performance_schema grants. Go driver prepared statements appear as
	// COMMAND='Execute', and the unique user id keeps this barrier isolated
	// from other packages using the same validation database.
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.PROCESSLIST
		WHERE ID<>CONNECTION_ID() AND ID<>?
		  AND DB=DATABASE() AND COMMAND IN ('Query','Execute')
		  AND INFO IS NOT NULL
		  AND LOWER(INFO) LIKE '%wallet_accounts%'
		  AND INFO LIKE ?`,
		blockerConnectionID, "%"+strconv.FormatInt(userID, 10)+"%",
	).Scan(&count)
	return count > 0, err
}

func waitForSportsWalletAccountWaiter(
	ctx context.Context,
	db *sql.DB,
	blockerConnectionID int64,
	userID int64,
	resultCh <-chan asyncSportsBetResult,
) (*asyncSportsBetResult, error) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case early := <-resultCh:
			return &early, nil
		case <-ticker.C:
			waiting, err := walletAccountLockHasWaiter(
				ctx,
				db,
				blockerConnectionID,
				userID,
			)
			if err != nil {
				return nil, fmt.Errorf("inspect wallet lock wait: %w", err)
			}
			if waiting {
				return nil, nil
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for PlaceBet wallet barrier: %w", ctx.Err())
		}
	}
}

func sportsRaceID(prefix, suffix string) string {
	value := strings.ToUpper(prefix + suffix)
	if len(value) > 26 {
		value = value[len(value)-26:]
	}
	return value + strings.Repeat("0", 26-len(value))
}
