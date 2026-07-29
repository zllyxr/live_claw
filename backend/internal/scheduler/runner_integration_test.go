package scheduler

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/lottery"
	"github.com/zllyxr/live_claw/backend/internal/sports"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func TestBetHoldAndLotterySettlementIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	rawID, err := idgen.New()
	if err != nil {
		t.Fatal(err)
	}
	suffix := strings.ToLower(rawID[len(rawID)-8:])
	username := "worker_" + suffix
	gameCode := "lt_" + suffix
	categoryKey := "cat_" + suffix
	issueNo := "issue_" + suffix
	traceID := "lottery_" + suffix

	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'integration-test-only','结算联调用户',1)`,
		username,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	var categoryID, gameID, playID, optionID, issueID int64
	var sportsMatchID, sportsMarketID, sportsOptionID int64
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		statements := []struct {
			query string
			args  []any
		}{
			{"DELETE FROM sports_bet_items WHERE order_id IN (SELECT id FROM sports_bet_orders WHERE user_id=?)", []any{userID}},
			{"DELETE FROM sports_bet_orders WHERE user_id=?", []any{userID}},
			{"DELETE FROM sports_settlement_runs WHERE match_id=?", []any{sportsMatchID}},
			{"DELETE FROM sports_market_options WHERE market_id=?", []any{sportsMarketID}},
			{"DELETE FROM sports_markets WHERE match_id=?", []any{sportsMatchID}},
			{"DELETE FROM sports_matches WHERE id=?", []any{sportsMatchID}},
			{"DELETE FROM lottery_bet_items WHERE order_id IN (SELECT id FROM lottery_bet_orders WHERE user_id=?)", []any{userID}},
			{"DELETE FROM lottery_bet_orders WHERE user_id=?", []any{userID}},
			{"DELETE FROM lottery_settlement_runs WHERE issue_id=?", []any{issueID}},
			{"DELETE FROM lottery_draw_audits WHERE issue_id=?", []any{issueID}},
			{"DELETE FROM lottery_issues WHERE game_id=?", []any{gameID}},
			{"DELETE FROM lottery_options WHERE play_id=?", []any{playID}},
			{"DELETE FROM lottery_plays WHERE game_id=?", []any{gameID}},
			{"DELETE FROM lottery_games WHERE id=?", []any{gameID}},
			{"DELETE FROM lottery_categories WHERE id=?", []any{categoryID}},
			{"DELETE FROM wallet_ledger_entries WHERE user_id=?", []any{userID}},
			{"DELETE FROM wallet_holds WHERE user_id=?", []any{userID}},
			{"DELETE FROM wallet_accounts WHERE user_id=?", []any{userID}},
			{"DELETE FROM users WHERE id=?", []any{userID}},
		}
		for _, statement := range statements {
			_, _ = db.ExecContext(cleanupCtx, statement.query, statement.args...)
		}
	})

	walletService := wallet.New(db)
	if _, err = walletService.Apply(ctx, wallet.ApplyRequest{
		UserID: userID, Amount: 1_000, BusinessType: "integration_credit",
		BusinessID: suffix, Description: "本地结算联调入金",
	}); err != nil {
		t.Fatal(err)
	}

	categoryResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_categories(category_key,name,status,sort_order)
		VALUES(?,'联调彩种',1,0)`,
		categoryKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	categoryID, _ = categoryResult.LastInsertId()
	gameResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_games
			(category_id,game_code,name,issue_interval_seconds,sale_close_seconds,min_bet,max_bet,status,config)
		VALUES(?,?,'联调彩票',300,10,1,10000,1,JSON_OBJECT())`,
		categoryID, gameCode,
	)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ = gameResult.LastInsertId()
	playResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_plays(game_id,play_code,name,settlement_rule,status,config)
		VALUES(?,'single','独赢','winner_option_ids',1,JSON_OBJECT())`,
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
	issueResult, err := db.ExecContext(ctx, `
		INSERT INTO lottery_issues
			(game_id,issue_no,sale_open_at,sale_close_at,draw_at,status)
		VALUES(?,?,CURRENT_TIMESTAMP(3)-INTERVAL 1 MINUTE,
		       CURRENT_TIMESTAMP(3)+INTERVAL 10 MINUTE,
		       CURRENT_TIMESTAMP(3)+INTERVAL 11 MINUTE,1)`,
		gameID, issueNo,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID, _ = issueResult.LastInsertId()

	lotteryService := lottery.New(db, walletService, "")
	order, err := lotteryService.PlaceBet(ctx, lottery.BetRequest{
		UserID: userID, GameID: gameID, IssueID: issueID, ClientTraceID: traceID,
		ItemsJSON: `[{"option_id":` + strconv.FormatInt(optionID, 10) + `,"amount":100}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if order["status"] != "0" {
		t.Fatalf("expected accepted lottery order, got %#v", order)
	}
	assertBalance(t, ctx, walletService, userID, 900, 100)

	if _, err = db.ExecContext(ctx, `
		UPDATE lottery_issues
		SET status=3,draw_result=JSON_OBJECT('winner_option_ids',JSON_ARRAY(?)),
		    result_source='integration_test'
		WHERE id=?`,
		optionID, issueID,
	); err != nil {
		t.Fatal(err)
	}
	runner := New(db, nil, walletService, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err = runner.settleLotteryIssue(ctx, issueID); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, ctx, walletService, userID, 1_100, 0)
	var orderStatus int
	var payout int64
	if err = db.QueryRowContext(ctx, `
		SELECT status,total_payout FROM lottery_bet_orders
		WHERE user_id=? AND client_trace_id=?`,
		userID, traceID,
	).Scan(&orderStatus, &payout); err != nil {
		t.Fatal(err)
	}
	if orderStatus != 1 || payout != 200 {
		t.Fatalf("expected won order with payout 200, got status=%d payout=%d", orderStatus, payout)
	}
	var gameLedgerCount int
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM wallet_ledger_entries
		WHERE user_id=? AND game_code=? AND round_no=?`,
		userID, gameCode, issueNo,
	).Scan(&gameLedgerCount); err != nil {
		t.Fatal(err)
	}
	if gameLedgerCount != 2 {
		t.Fatalf("expected hold and settlement ledger rows, got %d", gameLedgerCount)
	}

	publicMatchID, err := idgen.New()
	if err != nil {
		t.Fatal(err)
	}
	matchResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_matches
			(public_match_id,source,source_match_id,competition,competition_type,
			 home_name,away_name,kickoff_at,bet_close_at,match_status,bet_status,min_bet,max_bet)
		VALUES(?,'manual_admin',?,'联调联赛','football','主队','客队',
		       CURRENT_TIMESTAMP(3)+INTERVAL 1 HOUR,
		       CURRENT_TIMESTAMP(3)+INTERVAL 50 MINUTE,'NS',1,1,10000)`,
		publicMatchID, suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	sportsMatchID, _ = matchResult.LastInsertId()
	marketResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_markets(match_id,market_code,name,settlement_rule,status)
		VALUES(?,'1x2','胜平负','result_option',1)`,
		sportsMatchID,
	)
	if err != nil {
		t.Fatal(err)
	}
	sportsMarketID, _ = marketResult.LastInsertId()
	sportsOptionResult, err := db.ExecContext(ctx, `
		INSERT INTO sports_market_options(market_id,option_code,name,odds_scaled,status)
		VALUES(?,'home','主胜',1800000,1)`,
		sportsMarketID,
	)
	if err != nil {
		t.Fatal(err)
	}
	sportsOptionID, _ = sportsOptionResult.LastInsertId()

	sportsService := sports.New(db, walletService)
	sportsOrder, err := sportsService.PlaceBet(ctx, sports.BetRequest{
		UserID: userID, MatchID: publicMatchID, ClientTraceID: "sports_" + suffix,
		ItemsJSON: `[{"option_id":` + strconv.FormatInt(sportsOptionID, 10) + `,"amount":50}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sportsOrder["status"] != "0" {
		t.Fatalf("expected accepted sports order, got %#v", sportsOrder)
	}
	assertBalance(t, ctx, walletService, userID, 1_050, 50)

	if _, err = db.ExecContext(ctx, `
		UPDATE sports_market_options SET result=1 WHERE id=?`,
		sportsOptionID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE sports_matches
		SET match_status='FT',bet_status=0,settle_status=1
		WHERE id=?`,
		sportsMatchID,
	); err != nil {
		t.Fatal(err)
	}
	if err = runner.settleSportsMatch(ctx, sportsMatchID); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, ctx, walletService, userID, 1_140, 0)

	if err = db.QueryRowContext(ctx, `
		SELECT status,total_payout FROM sports_bet_orders
		WHERE user_id=? AND client_trace_id=?`,
		userID, "sports_"+suffix,
	).Scan(&orderStatus, &payout); err != nil {
		t.Fatal(err)
	}
	if orderStatus != 1 || payout != 90 {
		t.Fatalf("expected won sports order with payout 90, got status=%d payout=%d", orderStatus, payout)
	}
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM wallet_ledger_entries
		WHERE user_id=? AND game_code='sports' AND round_no=?`,
		userID, publicMatchID,
	).Scan(&gameLedgerCount); err != nil {
		t.Fatal(err)
	}
	if gameLedgerCount != 2 {
		t.Fatalf("expected sports hold and settlement ledger rows, got %d", gameLedgerCount)
	}
	var settleRunStatus int
	var settleRunPayout int64
	if err = db.QueryRowContext(ctx, `
		SELECT status,total_payout FROM sports_settlement_runs
		WHERE match_id=?`,
		sportsMatchID,
	).Scan(&settleRunStatus, &settleRunPayout); err != nil {
		t.Fatal(err)
	}
	if settleRunStatus != 3 || settleRunPayout != 90 {
		t.Fatalf(
			"expected completed sports settlement run with payout 90, got status=%d payout=%d",
			settleRunStatus, settleRunPayout,
		)
	}
}

func assertBalance(
	t *testing.T,
	ctx context.Context,
	service *wallet.Service,
	userID int64,
	available int64,
	frozen int64,
) {
	t.Helper()
	balance, err := service.Balance(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Available != available || balance.Frozen != frozen {
		t.Fatalf(
			"expected balance available=%d frozen=%d, got available=%d frozen=%d",
			available, frozen, balance.Available, balance.Frozen,
		)
	}
}
