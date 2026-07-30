package admin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestFinanceManagementAtomicityIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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
	var sqlMode string
	if err = db.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&sqlMode); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToUpper(sqlMode), "STRICT") {
		t.Fatalf("rollback checks require MySQL strict mode, got %q", sqlMode)
	}

	service := wallet.New(db)
	handler := &Handler{db: db, wallet: service}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'finance-atomicity-test-only','资金事务联调用户',1)`,
		"finance_atomicity_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	actorBase := (time.Now().UnixNano() & 0x1fffffffffffffff) + 100
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM audit_logs WHERE actor_id BETWEEN ? AND ?", actorBase, actorBase+20)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM recharge_orders WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM withdraw_orders WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_adjustments WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_holds WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", userID)
	})

	t.Run("adjustment approve versus reject serializes on the request row", func(t *testing.T) {
		const rounds = 12
		const amount = int64(7)
		requester := actorBase
		approver := adminauth.Admin{ID: actorBase + 1, Username: "atomic-approver"}
		rejecter := adminauth.Admin{ID: actorBase + 2, Username: "atomic-rejecter"}
		applied := int64(0)

		for index := 0; index < rounds; index++ {
			adjustmentNo, newErr := idgen.New()
			if newErr != nil {
				t.Fatal(newErr)
			}
			result, insertErr := db.ExecContext(ctx, `
				INSERT INTO wallet_adjustments
					(adjustment_no,user_id,amount,reason,status,requested_by)
				VALUES(?,?,?,'并发审核测试',0,?)`,
				adjustmentNo, userID, amount, requester,
			)
			if insertErr != nil {
				t.Fatal(insertErr)
			}
			adjustmentID, _ := result.LastInsertId()
			pathID := strconv.FormatInt(adjustmentID, 10)

			start := make(chan struct{})
			statuses := make(chan int, 2)
			var waitGroup sync.WaitGroup
			waitGroup.Add(2)
			go func() {
				defer waitGroup.Done()
				<-start
				recorder := invokeFinanceAdminHandler(
					handler.approveWalletAdjustment, approver, http.MethodPost,
					"/admin/api/wallet/adjustments/"+pathID+"/approve", pathID, nil, "",
				)
				statuses <- recorder.Code
			}()
			go func() {
				defer waitGroup.Done()
				<-start
				recorder := invokeFinanceAdminHandler(
					handler.rejectWalletAdjustment, rejecter, http.MethodPost,
					"/admin/api/wallet/adjustments/"+pathID+"/reject", pathID,
					map[string]any{"reason": "并发驳回"}, "",
				)
				statuses <- recorder.Code
			}()
			close(start)
			waitGroup.Wait()
			close(statuses)

			successes, conflicts := 0, 0
			for responseStatus := range statuses {
				switch responseStatus {
				case http.StatusOK:
					successes++
				case http.StatusConflict:
					conflicts++
				default:
					t.Fatalf("unexpected concurrent review response: %d", responseStatus)
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Fatalf("expected one success and one conflict, got success=%d conflict=%d", successes, conflicts)
			}

			var adjustmentStatus, ledgerCount, auditCount int64
			if err = db.QueryRowContext(ctx, `
				SELECT adjustment.status,
				       (SELECT COUNT(*) FROM wallet_ledger_entries
				        WHERE user_id=? AND business_type='admin_adjustment' AND business_id=?),
				       (SELECT COUNT(*) FROM audit_logs
				        WHERE resource_type='wallet_adjustment' AND resource_id=CAST(? AS CHAR)
				          AND action IN ('wallet.adjustment.approve','wallet.adjustment.reject'))
				FROM wallet_adjustments adjustment WHERE adjustment.id=?`,
				userID, adjustmentNo, adjustmentID, adjustmentID,
			).Scan(&adjustmentStatus, &ledgerCount, &auditCount); err != nil {
				t.Fatal(err)
			}
			if auditCount != 1 {
				t.Fatalf("adjustment %d has %d terminal audit rows", adjustmentID, auditCount)
			}
			switch adjustmentStatus {
			case 2:
				if ledgerCount != 0 {
					t.Fatalf("rejected adjustment %d wrote %d ledger rows", adjustmentID, ledgerCount)
				}
			case 3:
				if ledgerCount != 1 {
					t.Fatalf("applied adjustment %d wrote %d ledger rows", adjustmentID, ledgerCount)
				}
				applied++
			default:
				t.Fatalf("adjustment %d ended in status %d", adjustmentID, adjustmentStatus)
			}
		}

		var available int64
		if err = db.QueryRowContext(ctx, `
			SELECT COALESCE((
				SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'
			),0)`,
			userID,
		).Scan(&available); err != nil {
			t.Fatal(err)
		}
		if available != applied*amount {
			t.Fatalf("available=%d, want %d after %d applied adjustments", available, applied*amount, applied)
		}
	})

	t.Run("adjustment audit failure rolls back ledger and status", func(t *testing.T) {
		var balanceBefore int64
		if err = db.QueryRowContext(ctx, `
			SELECT COALESCE((
				SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'
			),0)`,
			userID,
		).Scan(&balanceBefore); err != nil {
			t.Fatal(err)
		}
		adjustmentNo, newErr := idgen.New()
		if newErr != nil {
			t.Fatal(newErr)
		}
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO wallet_adjustments
				(adjustment_no,user_id,amount,reason,status,requested_by)
			VALUES(?,?,11,'审计回滚测试',0,?)`,
			adjustmentNo, userID, actorBase,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		adjustmentID, _ := result.LastInsertId()
		pathID := strconv.FormatInt(adjustmentID, 10)
		failed := invokeFinanceAdminHandler(
			handler.approveWalletAdjustment,
			adminauth.Admin{ID: actorBase + 1, Username: "atomic-approver"},
			http.MethodPost, "/admin/api/wallet/adjustments/"+pathID+"/approve",
			pathID, nil, strings.Repeat("a", 27),
		)
		if failed.Code != http.StatusInternalServerError {
			t.Fatalf("expected audit failure, got %d: %s", failed.Code, failed.Body.String())
		}
		var status, ledgerCount, auditCount, balanceAfter int64
		if err = db.QueryRowContext(ctx, `
			SELECT adjustment.status,
			       (SELECT COUNT(*) FROM wallet_ledger_entries
			        WHERE user_id=? AND business_type='admin_adjustment' AND business_id=?),
			       (SELECT COUNT(*) FROM audit_logs
			        WHERE resource_type='wallet_adjustment' AND resource_id=CAST(? AS CHAR)
			          AND action='wallet.adjustment.approve'),
			       COALESCE((
			        SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'
			       ),0)
			FROM wallet_adjustments adjustment WHERE adjustment.id=?`,
			userID, adjustmentNo, adjustmentID, userID, adjustmentID,
		).Scan(&status, &ledgerCount, &auditCount, &balanceAfter); err != nil {
			t.Fatal(err)
		}
		if status != 0 || ledgerCount != 0 || auditCount != 0 || balanceAfter != balanceBefore {
			t.Fatalf(
				"adjustment rollback status=%d ledger=%d audit=%d balance=%d, want 0 0 0 %d",
				status, ledgerCount, auditCount, balanceAfter, balanceBefore,
			)
		}
	})

	t.Run("manual recharge rolls back and preserves provider idempotency", func(t *testing.T) {
		orderNo, newErr := idgen.New()
		if newErr != nil {
			t.Fatal(newErr)
		}
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO recharge_orders
				(order_no,user_id,product_id,channel_id,fiat_currency,currency_scale,
				 amount_minor,coin_amount,bonus_coin,status)
			VALUES(?,?,0,0,'CNY',2,1000,80,20,0)`,
			orderNo, userID,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		orderID, _ := result.LastInsertId()
		pathID := strconv.FormatInt(orderID, 10)
		adminUser := adminauth.Admin{ID: actorBase + 3, Username: "recharge-reviewer"}
		body := map[string]any{"provider_order_no": "provider-" + suffix, "reason": "人工到账"}

		failed := invokeFinanceAdminHandler(
			handler.markRechargePaid, adminUser, http.MethodPost,
			"/admin/api/wallet/recharges/"+pathID+"/mark-paid", pathID, body,
			strings.Repeat("r", 27),
		)
		if failed.Code != http.StatusInternalServerError {
			t.Fatalf("expected audit failure to roll back recharge, got %d: %s", failed.Code, failed.Body.String())
		}
		assertRechargeState(t, ctx, db, userID, orderID, orderNo, 0, "", 0, 0)

		succeeded := invokeFinanceAdminHandler(
			handler.markRechargePaid, adminUser, http.MethodPost,
			"/admin/api/wallet/recharges/"+pathID+"/mark-paid", pathID, body, "",
		)
		if succeeded.Code != http.StatusOK {
			t.Fatalf("manual recharge failed: %d: %s", succeeded.Code, succeeded.Body.String())
		}
		assertRechargeState(t, ctx, db, userID, orderID, orderNo, 2, "provider-"+suffix, 100, 1)

		conflictBody := map[string]any{"provider_order_no": "different-" + suffix, "reason": "重复确认"}
		conflict := invokeFinanceAdminHandler(
			handler.markRechargePaid, adminUser, http.MethodPost,
			"/admin/api/wallet/recharges/"+pathID+"/mark-paid", pathID, conflictBody, "",
		)
		if conflict.Code != http.StatusConflict {
			t.Fatalf("expected provider conflict, got %d: %s", conflict.Code, conflict.Body.String())
		}
		assertRechargeState(t, ctx, db, userID, orderID, orderNo, 2, "provider-"+suffix, 100, 1)
	})

	t.Run("withdraw release and commit roll back with their order state", func(t *testing.T) {
		adminUser := adminauth.Admin{ID: actorBase + 4, Username: "withdraw-reviewer"}
		seedNo, newErr := idgen.New()
		if newErr != nil {
			t.Fatal(newErr)
		}
		seedEntry, applyErr := service.Apply(ctx, wallet.ApplyRequest{
			UserID: userID, Amount: 100, BusinessType: "finance_atomicity_seed",
			BusinessID: seedNo, Description: "提现事务测试本金",
		})
		if applyErr != nil {
			t.Fatal(applyErr)
		}
		startingAvailable := seedEntry.Available

		rejectOrderID, rejectOrderNo, rejectHoldNo := createAtomicWithdrawal(
			t, ctx, db, service, userID, 40, suffix+"-reject",
		)
		rejectPathID := strconv.FormatInt(rejectOrderID, 10)
		approve := invokeFinanceAdminHandler(
			handler.reviewWithdrawal, adminUser, http.MethodPost,
			"/admin/api/wallet/withdrawals/"+rejectPathID+"/review", rejectPathID,
			map[string]any{"action": "approve"}, "",
		)
		if approve.Code != http.StatusOK {
			t.Fatalf("withdraw approve failed: %d: %s", approve.Code, approve.Body.String())
		}

		failedReject := invokeFinanceAdminHandler(
			handler.reviewWithdrawal, adminUser, http.MethodPost,
			"/admin/api/wallet/withdrawals/"+rejectPathID+"/review", rejectPathID,
			map[string]any{"action": "reject", "reason": "账户异常"}, strings.Repeat("w", 27),
		)
		if failedReject.Code != http.StatusInternalServerError {
			t.Fatalf("expected reject audit failure, got %d: %s", failedReject.Code, failedReject.Body.String())
		}
		assertWithdrawalState(
			t, ctx, db, userID, rejectOrderID, rejectOrderNo, rejectHoldNo,
			1, 0, startingAvailable-40, 40, "hold_release/withdraw", 0, "",
		)

		rejected := invokeFinanceAdminHandler(
			handler.reviewWithdrawal, adminUser, http.MethodPost,
			"/admin/api/wallet/withdrawals/"+rejectPathID+"/review", rejectPathID,
			map[string]any{"action": "reject", "reason": "账户异常"}, "",
		)
		if rejected.Code != http.StatusOK {
			t.Fatalf("withdraw reject failed: %d: %s", rejected.Code, rejected.Body.String())
		}
		assertWithdrawalState(
			t, ctx, db, userID, rejectOrderID, rejectOrderNo, rejectHoldNo,
			4, 2, startingAvailable, 0, "hold_release/withdraw", 1, "",
		)

		paidOrderID, paidOrderNo, paidHoldNo := createAtomicWithdrawal(
			t, ctx, db, service, userID, 50, suffix+"-paid",
		)
		paidPathID := strconv.FormatInt(paidOrderID, 10)
		for _, action := range []string{"approve", "paying"} {
			recorder := invokeFinanceAdminHandler(
				handler.reviewWithdrawal, adminUser, http.MethodPost,
				"/admin/api/wallet/withdrawals/"+paidPathID+"/review", paidPathID,
				map[string]any{"action": action}, "",
			)
			if recorder.Code != http.StatusOK {
				t.Fatalf("withdraw %s failed: %d: %s", action, recorder.Code, recorder.Body.String())
			}
		}

		providerOrderNo := "withdraw-provider-" + suffix
		failedPaid := invokeFinanceAdminHandler(
			handler.reviewWithdrawal, adminUser, http.MethodPost,
			"/admin/api/wallet/withdrawals/"+paidPathID+"/review", paidPathID,
			map[string]any{"action": "paid", "provider_order_no": providerOrderNo},
			strings.Repeat("p", 27),
		)
		if failedPaid.Code != http.StatusInternalServerError {
			t.Fatalf("expected paid audit failure, got %d: %s", failedPaid.Code, failedPaid.Body.String())
		}
		assertWithdrawalState(
			t, ctx, db, userID, paidOrderID, paidOrderNo, paidHoldNo,
			2, 0, startingAvailable-50, 50, "hold_commit/withdraw", 0, "",
		)

		paid := invokeFinanceAdminHandler(
			handler.reviewWithdrawal, adminUser, http.MethodPost,
			"/admin/api/wallet/withdrawals/"+paidPathID+"/review", paidPathID,
			map[string]any{"action": "paid", "provider_order_no": providerOrderNo}, "",
		)
		if paid.Code != http.StatusOK {
			t.Fatalf("withdraw paid failed: %d: %s", paid.Code, paid.Body.String())
		}
		assertWithdrawalState(
			t, ctx, db, userID, paidOrderID, paidOrderNo, paidHoldNo,
			3, 1, startingAvailable-50, 0, "hold_commit/withdraw", 1, providerOrderNo,
		)

		conflict := invokeFinanceAdminHandler(
			handler.reviewWithdrawal, adminUser, http.MethodPost,
			"/admin/api/wallet/withdrawals/"+paidPathID+"/review", paidPathID,
			map[string]any{"action": "paid", "provider_order_no": "different-" + suffix}, "",
		)
		if conflict.Code != http.StatusConflict {
			t.Fatalf("expected paid provider conflict, got %d: %s", conflict.Code, conflict.Body.String())
		}
		assertWithdrawalState(
			t, ctx, db, userID, paidOrderID, paidOrderNo, paidHoldNo,
			3, 1, startingAvailable-50, 0, "hold_commit/withdraw", 1, providerOrderNo,
		)
	})
}

func invokeFinanceAdminHandler(
	endpoint http.HandlerFunc,
	adminUser adminauth.Admin,
	method, target, pathID string,
	body any,
	requestID string,
) *httptest.ResponseRecorder {
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, _ := json.Marshal(body)
		requestBody = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, target, requestBody)
	request.SetPathValue("id", pathID)
	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
		request.Header.Set("User-Agent", strings.Repeat("finance-audit-rollback-", 26))
	}
	recorder := httptest.NewRecorder()
	httpx.RequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint(w, r.WithContext(withAdmin(r, adminUser)))
	})).ServeHTTP(recorder, request)
	return recorder
}

func assertRechargeState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	userID, orderID int64,
	orderNo string,
	wantStatus int,
	wantProvider string,
	wantRechargeBalance, wantLedgerCount int64,
) {
	t.Helper()
	var status int
	var provider sql.NullString
	if err := db.QueryRowContext(ctx, `
		SELECT status,provider_order_no FROM recharge_orders WHERE id=?`,
		orderID,
	).Scan(&status, &provider); err != nil {
		t.Fatal(err)
	}
	var balance, ledgerCount int64
	if err := db.QueryRowContext(ctx, `
		SELECT
			COALESCE((
				SELECT SUM(delta_available) FROM wallet_ledger_entries
				WHERE user_id=? AND business_type='recharge' AND business_id=?
			),0),
			(SELECT COUNT(*) FROM wallet_ledger_entries
			 WHERE user_id=? AND business_type='recharge' AND business_id=?)`,
		userID, orderNo, userID, orderNo,
	).Scan(&balance, &ledgerCount); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus || provider.String != wantProvider ||
		balance != wantRechargeBalance || ledgerCount != wantLedgerCount {
		t.Fatalf(
			"recharge state status=%d provider=%q balance_delta=%d ledger=%d, want %d %q %d %d",
			status, provider.String, balance, ledgerCount,
			wantStatus, wantProvider, wantRechargeBalance, wantLedgerCount,
		)
	}
}

func createAtomicWithdrawal(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	service *wallet.Service,
	userID, amount int64,
	marker string,
) (int64, string, string) {
	t.Helper()
	orderNo, err := idgen.New()
	if err != nil {
		t.Fatal(err)
	}
	hold, err := service.PlaceHold(ctx, wallet.HoldRequest{
		UserID: userID, Amount: amount, BusinessType: "withdraw", BusinessID: orderNo,
		ExpiresAt: time.Now().Add(time.Hour), Description: "提现冻结",
		Metadata: map[string]any{"marker": marker},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO withdraw_orders
			(order_no,user_id,account_id,hold_no,coin_amount,fee_coin,
			 payout_currency,currency_scale,payout_amount_minor,
			 account_snapshot_ciphertext,account_masked,status)
		VALUES(?,?,0,?,?,0,'CNY',2,?,?,'****0000',0)`,
		orderNo, userID, hold.HoldNo, amount, amount*10, []byte("finance-atomicity-test"),
	)
	if err != nil {
		t.Fatal(err)
	}
	orderID, _ := result.LastInsertId()
	return orderID, orderNo, hold.HoldNo
}

func assertWithdrawalState(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	userID, orderID int64,
	orderNo, holdNo string,
	wantOrderStatus, wantHoldStatus int,
	wantAvailable, wantFrozen int64,
	entryBusinessType string,
	wantEntryCount int64,
	wantProvider string,
) {
	t.Helper()
	var orderStatus int
	var provider string
	if err := db.QueryRowContext(ctx, `
		SELECT status,provider_order_no FROM withdraw_orders WHERE id=? AND order_no=?`,
		orderID, orderNo,
	).Scan(&orderStatus, &provider); err != nil {
		t.Fatal(err)
	}
	var holdStatus int
	if err := db.QueryRowContext(ctx, `
		SELECT status FROM wallet_holds WHERE hold_no=?`,
		holdNo,
	).Scan(&holdStatus); err != nil {
		t.Fatal(err)
	}
	var available, frozen, entryCount int64
	if err := db.QueryRowContext(ctx, `
		SELECT account.available,account.frozen,
		       (SELECT COUNT(*) FROM wallet_ledger_entries
		        WHERE user_id=? AND business_type=? AND business_id=?)
		FROM wallet_accounts account
		WHERE account.user_id=? AND account.currency='COIN'`,
		userID, entryBusinessType, orderNo, userID,
	).Scan(&available, &frozen, &entryCount); err != nil {
		t.Fatal(err)
	}
	if orderStatus != wantOrderStatus || holdStatus != wantHoldStatus ||
		available != wantAvailable || frozen != wantFrozen ||
		entryCount != wantEntryCount || provider != wantProvider {
		t.Fatalf(
			"withdraw state order=%d hold=%d available=%d frozen=%d entry=%d provider=%q, want %d %d %d %d %d %q",
			orderStatus, holdStatus, available, frozen, entryCount, provider,
			wantOrderStatus, wantHoldStatus, wantAvailable, wantFrozen, wantEntryCount, wantProvider,
		)
	}
}
