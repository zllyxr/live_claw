package lottery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

type asyncLotteryBetResult struct {
	order map[string]any
	err   error
}

func TestFormatIssueExposesBettableCompatibilityFields(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	issue := formatIssue(
		11,
		22,
		"202701150001",
		now.Unix()-30,
		now.Unix()+25,
		now.Unix()+30,
		1,
		nil,
		now,
	)

	for key, expected := range map[string]any{
		"status":         "1",
		"can_bet":        "1",
		"seal_countdown": "25",
		"bet_countdown":  "25",
		"countdown":      "25",
		"open_countdown": "30",
	} {
		if actual := issue[key]; actual != expected {
			t.Fatalf("%s = %#v, want %#v", key, actual, expected)
		}
	}
}

func TestFormatIssueMarksClosedIssueAsNotBettable(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	issue := formatIssue(
		11,
		22,
		"202701150001",
		now.Unix()-60,
		now.Unix()-1,
		now.Unix(),
		2,
		nil,
		now,
	)

	if actual := issue["can_bet"]; actual != "0" {
		t.Fatalf("can_bet = %#v, want %q", actual, "0")
	}
	if actual := issue["seal_countdown"]; actual != "0" {
		t.Fatalf("seal_countdown = %#v, want %q", actual, "0")
	}
}

func TestValidateRevalidatedBetState(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	expected := []betOption{
		{ID: 11, PlayID: 21, OddsScaled: 2_000_000},
		{ID: 12, PlayID: 21, OddsScaled: 3_500_000},
	}
	validPlays := func() map[int64]int {
		return map[int64]int{21: 1}
	}
	validOptions := func() map[int64]lockedBetOption {
		return map[int64]lockedBetOption{
			11: {PlayID: 21, Status: 1, OddsScaled: 2_000_000},
			12: {PlayID: 21, Status: 1, OddsScaled: 3_500_000},
		}
	}
	errorCode := func(t *testing.T, err error) int {
		t.Helper()
		var lotteryErr *Error
		if !errors.As(err, &lotteryErr) {
			t.Fatalf("error = %v, want *Error", err)
		}
		return lotteryErr.Code
	}

	if err := validateRevalidatedBetState(
		1, 1, 1_000, 100,
		1, now.Add(time.Second), now, expected, validPlays(), validOptions(),
	); err != nil {
		t.Fatalf("valid locked state rejected: %v", err)
	}

	tests := []struct {
		name        string
		gameStatus  int
		issueStatus int
		saleClose   time.Time
		plays       map[int64]int
		options     map[int64]lockedBetOption
		minBet      int64
		maxBet      int64
		totalBet    int64
		wantCode    int
	}{
		{
			name: "game disabled", gameStatus: 0, issueStatus: 1,
			saleClose: now.Add(time.Second), plays: validPlays(), options: validOptions(),
			wantCode: 1003,
		},
		{
			name: "issue closed", gameStatus: 1, issueStatus: 2,
			saleClose: now.Add(time.Second), plays: validPlays(), options: validOptions(),
			wantCode: 1005,
		},
		{
			name: "sale deadline reached", gameStatus: 1, issueStatus: 1,
			saleClose: now, plays: validPlays(), options: validOptions(),
			wantCode: 1005,
		},
		{
			name: "play disabled", gameStatus: 1, issueStatus: 1,
			saleClose: now.Add(time.Second), plays: map[int64]int{21: 0}, options: validOptions(),
			wantCode: 1007,
		},
		{
			name: "option missing", gameStatus: 1, issueStatus: 1,
			saleClose: now.Add(time.Second), plays: validPlays(),
			options: map[int64]lockedBetOption{
				11: {PlayID: 21, Status: 1, OddsScaled: 2_000_000},
			},
			wantCode: 1007,
		},
		{
			name: "option moved to another play", gameStatus: 1, issueStatus: 1,
			saleClose: now.Add(time.Second), plays: validPlays(),
			options: map[int64]lockedBetOption{
				11: {PlayID: 22, Status: 1, OddsScaled: 2_000_000},
				12: {PlayID: 21, Status: 1, OddsScaled: 3_500_000},
			},
			wantCode: 1007,
		},
		{
			name: "option disabled", gameStatus: 1, issueStatus: 1,
			saleClose: now.Add(time.Second), plays: validPlays(),
			options: map[int64]lockedBetOption{
				11: {PlayID: 21, Status: 0, OddsScaled: 2_000_000},
				12: {PlayID: 21, Status: 1, OddsScaled: 3_500_000},
			},
			wantCode: 1007,
		},
		{
			name: "odds changed", gameStatus: 1, issueStatus: 1,
			saleClose: now.Add(time.Second), plays: validPlays(),
			options: map[int64]lockedBetOption{
				11: {PlayID: 21, Status: 1, OddsScaled: 2_000_001},
				12: {PlayID: 21, Status: 1, OddsScaled: 3_500_000},
			},
			wantCode: 1008,
		},
		{
			name: "minimum raised", gameStatus: 1, issueStatus: 1,
			saleClose: now.Add(time.Second), plays: validPlays(), options: validOptions(),
			minBet: 101, maxBet: 1_000, totalBet: 100, wantCode: 1009,
		},
		{
			name: "maximum lowered", gameStatus: 1, issueStatus: 1,
			saleClose: now.Add(time.Second), plays: validPlays(), options: validOptions(),
			minBet: 1, maxBet: 99, totalBet: 100, wantCode: 1010,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			minBet, maxBet, totalBet := test.minBet, test.maxBet, test.totalBet
			if minBet == 0 {
				minBet = 1
			}
			if maxBet == 0 {
				maxBet = 1_000
			}
			if totalBet == 0 {
				totalBet = 100
			}
			err := validateRevalidatedBetState(
				test.gameStatus,
				minBet,
				maxBet,
				totalBet,
				test.issueStatus,
				test.saleClose,
				now,
				expected,
				test.plays,
				test.options,
			)
			if actual := errorCode(t, err); actual != test.wantCode {
				t.Fatalf("error code = %d, want %d", actual, test.wantCode)
			}
		})
	}
}

func TestPlaceBetTraceLockNameIsBoundedAndScoped(t *testing.T) {
	first := placeBetTraceLockName(41, "trace-a")
	if len(first) > 64 {
		t.Fatalf("lock name length = %d, MySQL allows at most 64", len(first))
	}
	if strings.Contains(first, "trace-a") {
		t.Fatalf("lock name exposes the client trace: %q", first)
	}
	if repeated := placeBetTraceLockName(41, "trace-a"); repeated != first {
		t.Fatalf("lock name is not deterministic: %q != %q", repeated, first)
	}
	if otherUser := placeBetTraceLockName(42, "trace-a"); otherUser == first {
		t.Fatal("different users share a trace lock")
	}
	if otherTrace := placeBetTraceLockName(41, "trace-b"); otherTrace == first {
		t.Fatal("different traces share a trace lock")
	}
}

func TestPlaceBetRevalidationAndTraceIdempotencyIntegration(t *testing.T) {
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
	username := "lottery_race_" + suffix
	categoryKey := "lr_" + suffix
	gameCode := "lr_" + suffix
	issueNo := "issue_" + suffix
	expiredTrace := "expired_" + suffix
	minLimitTrace := "limit_min_" + suffix
	maxLimitTrace := "limit_max_" + suffix
	idempotentTrace := "idem_" + suffix

	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'integration-test-only','彩票并发联调用户',1)`,
		username,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	var categoryID, gameID, playID, optionID, issueID int64
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM lottery_bet_items
			WHERE order_id IN (SELECT id FROM lottery_bet_orders WHERE user_id=?)`,
			userID,
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM lottery_bet_orders WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM lottery_issues WHERE id=?", issueID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM lottery_options WHERE id=?", optionID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM lottery_plays WHERE id=?", playID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM lottery_games WHERE id=?", gameID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM lottery_categories WHERE id=?", categoryID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_holds WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", userID)
	})

	categoryResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_categories(category_key,name,status,sort_order)
		VALUES(?,'彩票并发联调分类',1,0)`,
		categoryKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	categoryID, _ = categoryResult.LastInsertId()
	gameResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_games
			(category_id,game_code,name,issue_interval_seconds,sale_close_seconds,
			 min_bet,max_bet,status,config)
		VALUES(?,?,'彩票并发联调',300,10,1,10000,1,JSON_OBJECT())`,
		categoryID, gameCode,
	)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ = gameResult.LastInsertId()
	playResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_plays
			(game_id,play_code,name,settlement_rule,status,config)
		VALUES(?,'single','单项','winner_option_ids',1,JSON_OBJECT())`,
		gameID,
	)
	if err != nil {
		t.Fatal(err)
	}
	playID, _ = playResult.LastInsertId()
	optionResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_options(play_id,option_code,name,odds_scaled,status)
		VALUES(?,'winner','中奖项',2000000,1)`,
		playID,
	)
	if err != nil {
		t.Fatal(err)
	}
	optionID, _ = optionResult.LastInsertId()
	now := time.Now().Truncate(time.Millisecond)
	saleClose := now.Add(10 * time.Minute)
	issueResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_issues
			(game_id,issue_no,sale_open_at,sale_close_at,draw_at,status)
		VALUES(?,?,?,?,?,1)`,
		gameID, issueNo, now.Add(-time.Minute), saleClose, saleClose.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID, _ = issueResult.LastInsertId()

	walletService := wallet.New(db)
	if _, err = walletService.Apply(ctx, wallet.ApplyRequest{
		UserID: userID, Amount: 1_000, BusinessType: "lottery_race_credit",
		BusinessID: suffix, Description: "彩票并发联调入金",
	}); err != nil {
		t.Fatal(err)
	}
	service := New(db, walletService, "")
	request := BetRequest{
		UserID: userID, GameID: gameID, IssueID: issueID,
		ItemsJSON: fmt.Sprintf(`[{"option_id":%d,"amount":100}]`, optionID),
	}

	t.Run("deadline is revalidated and orphan hold is released", func(t *testing.T) {
		var clockCalls atomic.Int32
		service.now = func() time.Time {
			if clockCalls.Add(1) == 1 {
				return now
			}
			return saleClose.Add(time.Millisecond)
		}
		expiredRequest := request
		expiredRequest.ClientTraceID = expiredTrace
		if _, placeErr := service.PlaceBet(ctx, expiredRequest); placeErr == nil {
			t.Fatal("bet succeeded after the sale deadline changed between transactions")
		} else {
			var lotteryErr *Error
			if !errors.As(placeErr, &lotteryErr) || lotteryErr.Code != 1005 {
				t.Fatalf("error = %v, want lottery code 1005", placeErr)
			}
		}
		var orderCount, holdStatus int
		if err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM lottery_bet_orders
			WHERE user_id=? AND client_trace_id=?`,
			userID, expiredTrace,
		).Scan(&orderCount); err != nil {
			t.Fatal(err)
		}
		if orderCount != 0 {
			t.Fatalf("expired trace persisted %d orders", orderCount)
		}
		if err = db.QueryRowContext(ctx, `
			SELECT status FROM wallet_holds
			WHERE user_id=? AND business_type='lottery_bet' AND business_id=?`,
			userID, expiredTrace,
		).Scan(&holdStatus); err != nil {
			t.Fatal(err)
		}
		if holdStatus != 2 {
			t.Fatalf("orphan hold status = %d, want released status 2", holdStatus)
		}
		balance, balanceErr := walletService.Balance(ctx, userID)
		if balanceErr != nil {
			t.Fatal(balanceErr)
		}
		if balance.Available != 1_000 || balance.Frozen != 0 {
			t.Fatalf("balance after revalidation failure = %#v", balance)
		}
	})

	t.Run("administrator limit changes are revalidated after wallet hold", func(t *testing.T) {
		service.now = time.Now
		tests := []struct {
			name     string
			traceID  string
			minBet   int64
			maxBet   int64
			wantCode int
		}{
			{
				name: "minimum raised", traceID: minLimitTrace,
				minBet: 101, maxBet: 10_000, wantCode: 1009,
			},
			{
				name: "maximum lowered", traceID: maxLimitTrace,
				minBet: 1, maxBet: 99, wantCode: 1010,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				if _, resetErr := db.ExecContext(ctx, `
					UPDATE lottery_games SET min_bet=1,max_bet=10000 WHERE id=?`,
					gameID,
				); resetErr != nil {
					t.Fatal(resetErr)
				}
				t.Cleanup(func() {
					cleanupCtx, cleanupCancel := context.WithTimeout(
						context.Background(),
						5*time.Second,
					)
					defer cleanupCancel()
					_, _ = db.ExecContext(cleanupCtx, `
						UPDATE lottery_games SET min_bet=1,max_bet=10000 WHERE id=?`,
						gameID,
					)
				})

				lockTx, lockErr := db.BeginTx(
					ctx,
					&sql.TxOptions{Isolation: sql.LevelReadCommitted},
				)
				if lockErr != nil {
					t.Fatal(lockErr)
				}
				defer lockTx.Rollback() //nolint:errcheck
				var lockConnectionID int64
				if lockErr = lockTx.QueryRowContext(
					ctx,
					"SELECT CONNECTION_ID()",
				).Scan(&lockConnectionID); lockErr != nil {
					t.Fatal(lockErr)
				}
				var accountID int64
				if lockErr = lockTx.QueryRowContext(ctx, `
					SELECT id FROM wallet_accounts
					WHERE user_id=? AND currency='COIN'
					FOR UPDATE`,
					userID,
				).Scan(&accountID); lockErr != nil {
					t.Fatal(lockErr)
				}

				limitRequest := request
				limitRequest.ClientTraceID = test.traceID
				resultCh := make(chan asyncLotteryBetResult, 1)
				placeCtx, placeCancel := context.WithTimeout(ctx, 20*time.Second)
				defer placeCancel()
				go func() {
					order, placeErr := service.PlaceBet(placeCtx, limitRequest)
					resultCh <- asyncLotteryBetResult{order: order, err: placeErr}
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
				early, barrierErr := waitForLotteryWalletAccountWaiter(
					barrierCtx,
					db,
					lockConnectionID,
					userID,
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

				updateResult, updateErr := db.ExecContext(ctx, `
					UPDATE lottery_games SET min_bet=?,max_bet=? WHERE id=?`,
					test.minBet, test.maxBet, gameID,
				)
				if updateErr != nil {
					t.Fatal(updateErr)
				}
				if updated, rowsErr := updateResult.RowsAffected(); rowsErr != nil {
					t.Fatal(rowsErr)
				} else if updated != 1 {
					t.Fatalf("administrator limit update affected %d games", updated)
				}
				if lockErr = lockTx.Commit(); lockErr != nil {
					t.Fatal(lockErr)
				}

				var placed asyncLotteryBetResult
				select {
				case placed = <-resultCh:
					placeFinished = true
				case <-time.After(10 * time.Second):
					t.Fatal("timed out waiting for lottery limit revalidation")
				}
				if placed.order != nil {
					t.Fatalf("limit change still created an order: %#v", placed.order)
				}
				var lotteryErr *Error
				if !errors.As(placed.err, &lotteryErr) || lotteryErr.Code != test.wantCode {
					t.Fatalf(
						"limit change error = %T %v, want lottery code %d",
						placed.err,
						placed.err,
						test.wantCode,
					)
				}

				var orderCount, holdStatus int
				var releasedAt sql.NullTime
				if queryErr := db.QueryRowContext(ctx, `
					SELECT COUNT(*) FROM lottery_bet_orders
					WHERE user_id=? AND client_trace_id=?`,
					userID, test.traceID,
				).Scan(&orderCount); queryErr != nil {
					t.Fatal(queryErr)
				}
				if queryErr := db.QueryRowContext(ctx, `
					SELECT status,released_at FROM wallet_holds
					WHERE user_id=? AND business_type='lottery_bet' AND business_id=?`,
					userID, test.traceID,
				).Scan(&holdStatus, &releasedAt); queryErr != nil {
					t.Fatal(queryErr)
				}
				if orderCount != 0 || holdStatus != 2 || !releasedAt.Valid {
					t.Fatalf(
						"limit rejection state: orders=%d hold_status=%d released=%v",
						orderCount,
						holdStatus,
						releasedAt.Valid,
					)
				}
				balance, balanceErr := walletService.Balance(ctx, userID)
				if balanceErr != nil {
					t.Fatal(balanceErr)
				}
				if balance.Available != 1_000 || balance.Frozen != 0 {
					t.Fatalf("limit rejection changed wallet balance: %#v", balance)
				}
			})
		}
	})

	t.Run("concurrent duplicate trace keeps the accepted hold", func(t *testing.T) {
		service.now = time.Now
		idempotentRequest := request
		idempotentRequest.ClientTraceID = idempotentTrace
		const workers = 8
		start := make(chan struct{})
		results := make(chan map[string]any, workers)
		errs := make(chan error, workers)
		var group sync.WaitGroup
		for range workers {
			group.Add(1)
			go func() {
				defer group.Done()
				<-start
				result, placeErr := service.PlaceBet(ctx, idempotentRequest)
				results <- result
				errs <- placeErr
			}()
		}
		close(start)
		group.Wait()
		close(results)
		close(errs)
		for placeErr := range errs {
			if placeErr != nil {
				t.Fatalf("concurrent PlaceBet failed: %v", placeErr)
			}
		}
		var orderNo string
		for result := range results {
			current := fmt.Sprint(result["order_no"])
			if current == "" {
				t.Fatalf("missing order number in result: %#v", result)
			}
			if orderNo == "" {
				orderNo = current
			} else if current != orderNo {
				t.Fatalf("duplicate trace returned orders %q and %q", orderNo, current)
			}
		}

		var orderCount, holdCount, holdStatus int
		if err = db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM lottery_bet_orders
			WHERE user_id=? AND client_trace_id=?`,
			userID, idempotentTrace,
		).Scan(&orderCount); err != nil {
			t.Fatal(err)
		}
		if err = db.QueryRowContext(ctx, `
			SELECT COUNT(*),COALESCE(MAX(status),-1)
			FROM wallet_holds
			WHERE user_id=? AND business_type='lottery_bet' AND business_id=?`,
			userID, idempotentTrace,
		).Scan(&holdCount, &holdStatus); err != nil {
			t.Fatal(err)
		}
		if orderCount != 1 || holdCount != 1 || holdStatus != 0 {
			t.Fatalf(
				"orders=%d holds=%d hold_status=%d, want 1/1/active",
				orderCount, holdCount, holdStatus,
			)
		}
		balance, balanceErr := walletService.Balance(ctx, userID)
		if balanceErr != nil {
			t.Fatal(balanceErr)
		}
		if balance.Available != 900 || balance.Frozen != 100 {
			t.Fatalf("balance after duplicate trace = %#v", balance)
		}
	})
}

func waitForLotteryWalletAccountWaiter(
	ctx context.Context,
	db *sql.DB,
	blockerConnectionID int64,
	userID int64,
	resultCh <-chan asyncLotteryBetResult,
) (*asyncLotteryBetResult, error) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case early := <-resultCh:
			return &early, nil
		case <-ticker.C:
			waiting, err := lotteryWalletAccountLockHasWaiter(
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

func lotteryWalletAccountLockHasWaiter(
	ctx context.Context,
	db *sql.DB,
	blockerConnectionID int64,
	userID int64,
) (bool, error) {
	var count int
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
