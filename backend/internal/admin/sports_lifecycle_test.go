package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestSportsLifecycleGuardsIntegration(t *testing.T) {
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
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'integration-test-only','体育状态机联调用户',1)`,
		"sports_lifecycle_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	adminID := time.Now().UnixNano() & 0x3fffffffffffffff
	var (
		pendingMatchID, pendingMarketID, pendingOptionID int64
		settledMatchID, settledMarketID, settledOptionID int64
		readyMatchID, readyMarketID, readyOptionID       int64
		pendingOrderID, readyOrderID                     int64
	)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM audit_logs
			WHERE actor_type=1 AND actor_id=?
			  AND resource_type IN ('sports_market','sports_option','sports_match')`,
			adminID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_settlement_runs WHERE match_id IN (?,?,?)`,
			pendingMatchID, settledMatchID, readyMatchID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_bet_items WHERE order_id IN (?,?)`,
			pendingOrderID, readyOrderID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_bet_orders WHERE id IN (?,?)`,
			pendingOrderID, readyOrderID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_market_options WHERE id IN (?,?,?)`,
			pendingOptionID, settledOptionID, readyOptionID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_markets WHERE id IN (?,?,?)`,
			pendingMarketID, settledMarketID, readyMarketID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM sports_matches WHERE id IN (?,?,?)`,
			pendingMatchID, settledMatchID, readyMatchID,
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", userID)
	})

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	insertMatch := func(prefix string, matchStatus string, settleStatus int) int64 {
		t.Helper()
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO sports_matches
				(public_match_id,source,source_match_id,competition,competition_type,
				 home_name,away_name,kickoff_at,bet_close_at,match_status,bet_status,settle_status)
			VALUES(?,'manual_admin',?,'状态机联赛','football','主队','客队',
			       CURRENT_TIMESTAMP(3)-INTERVAL 2 HOUR,
			       CURRENT_TIMESTAMP(3)-INTERVAL 1 HOUR,?,0,?)`,
			sportsLifecycleID(prefix, suffix), prefix+"-"+suffix, matchStatus, settleStatus,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		id, _ := result.LastInsertId()
		return id
	}
	insertMarket := func(matchID int64, code string, status int) int64 {
		t.Helper()
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO sports_markets
				(match_id,market_code,name,settlement_rule,status,sort_order)
			VALUES(?,?,?,'full_time',?,0)`,
			matchID, code, "状态机盘口 "+code, status,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		id, _ := result.LastInsertId()
		return id
	}
	insertOption := func(marketID int64, code string, resultValue, status int) int64 {
		t.Helper()
		insertResult, insertErr := tx.ExecContext(ctx, `
			INSERT INTO sports_market_options
				(market_id,option_code,name,odds_scaled,result,status)
			VALUES(?,?,?,2000000,?,?)`,
			marketID, code, "状态机选项 "+code, resultValue, status,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		id, _ := insertResult.LastInsertId()
		return id
	}
	insertPendingBet := func(prefix string, matchID, marketID, optionID int64) int64 {
		t.Helper()
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO sports_bet_orders
				(order_no,user_id,match_id,hold_no,total_bet,total_payout,
				 status,client_trace_id)
			VALUES(?,?,?, ?,100,0,0,?)`,
			sportsLifecycleID(prefix, suffix), userID, matchID,
			sportsLifecycleID(prefix+"h", suffix), "sports-lifecycle-"+prefix+"-"+suffix,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		orderID, _ := result.LastInsertId()
		if _, insertErr = tx.ExecContext(ctx, `
			INSERT INTO sports_bet_items
				(order_id,market_id,option_id,bet_amount,odds_scaled,payout_amount,result)
			VALUES(?,?,?,100,2000000,0,0)`,
			orderID, marketID, optionID,
		); insertErr != nil {
			t.Fatal(insertErr)
		}
		return orderID
	}

	pendingMatchID = insertMatch("a", "NS", 0)
	pendingMarketID = insertMarket(pendingMatchID, "guard_pending", 1)
	pendingOptionID = insertOption(pendingMarketID, "pending_home", 0, 1)
	pendingOrderID = insertPendingBet("d", pendingMatchID, pendingMarketID, pendingOptionID)

	settledMatchID = insertMatch("b", "FT", 1)
	settledMarketID = insertMarket(settledMatchID, "guard_settled", 1)
	settledOptionID = insertOption(settledMarketID, "settled_home", 1, 1)

	readyMatchID = insertMatch("c", "FT", 0)
	readyMarketID = insertMarket(readyMatchID, "guard_ready", 0)
	readyOptionID = insertOption(readyMarketID, "ready_home", 0, 0)
	readyOrderID = insertPendingBet("e", readyMatchID, readyMarketID, readyOptionID)
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{db: db}
	adminUser := adminauth.Admin{ID: adminID, Username: "integration-admin"}
	runJSON := func(
		path string,
		targetID int64,
		body map[string]any,
		endpoint http.HandlerFunc,
	) *httptest.ResponseRecorder {
		t.Helper()
		encoded, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
		request.SetPathValue("id", strconv.FormatInt(targetID, 10))
		recorder := httptest.NewRecorder()
		httpx.RequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint(w, r.WithContext(withAdmin(r, adminUser)))
		})).ServeHTTP(recorder, request)
		return recorder
	}

	t.Run("market with pending bet cannot be disabled", func(t *testing.T) {
		recorder := runJSON(
			"/admin/api/sports/markets/"+strconv.FormatInt(pendingMarketID, 10),
			pendingMarketID,
			map[string]any{
				"market_code": "guard_pending", "name": "状态机盘口 guard_pending",
				"settlement_rule": "full_time", "status": 0, "sort_order": 0,
			},
			handler.updateSportsMarket,
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("expected HTTP 409, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var status int
		if queryErr := db.QueryRowContext(ctx, `
			SELECT status FROM sports_markets WHERE id=?`,
			pendingMarketID,
		).Scan(&status); queryErr != nil {
			t.Fatal(queryErr)
		}
		if status != 1 {
			t.Fatalf("pending-bet market status changed to %d", status)
		}
	})

	t.Run("option cannot change after settlement is submitted", func(t *testing.T) {
		recorder := runJSON(
			"/admin/api/sports/options/"+strconv.FormatInt(settledOptionID, 10),
			settledOptionID,
			map[string]any{"odds_scaled": 2500000, "result": 2, "status": 0},
			handler.updateSportsOption,
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("expected HTTP 409, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var oddsScaled int64
		var resultValue, status int
		if queryErr := db.QueryRowContext(ctx, `
			SELECT odds_scaled,result,status FROM sports_market_options WHERE id=?`,
			settledOptionID,
		).Scan(&oddsScaled, &resultValue, &status); queryErr != nil {
			t.Fatal(queryErr)
		}
		if oddsScaled != 2000000 || resultValue != 1 || status != 1 {
			t.Fatalf(
				"settled option changed: odds=%d result=%d status=%d",
				oddsScaled, resultValue, status,
			)
		}
	})

	t.Run("disabled bet option still blocks settlement readiness", func(t *testing.T) {
		recorder := runJSON(
			"/admin/api/sports/matches/"+strconv.FormatInt(readyMatchID, 10)+"/settle",
			readyMatchID,
			map[string]any{},
			handler.markSportsSettlementReady,
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("expected HTTP 409, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var settleStatus int
		if queryErr := db.QueryRowContext(ctx, `
			SELECT settle_status FROM sports_matches WHERE id=?`,
			readyMatchID,
		).Scan(&settleStatus); queryErr != nil {
			t.Fatal(queryErr)
		}
		if settleStatus != 0 {
			t.Fatalf("match unexpectedly advanced to settle_status=%d", settleStatus)
		}
	})
}

func sportsLifecycleID(prefix, suffix string) string {
	value := strings.ToUpper(prefix + suffix)
	if len(value) > 26 {
		value = value[len(value)-26:]
	}
	return value + strings.Repeat("0", 26-len(value))
}
