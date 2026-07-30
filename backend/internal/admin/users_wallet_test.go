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
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestDirectWalletAdjustmentRequestNormalize(t *testing.T) {
	tests := []struct {
		name       string
		request    directWalletAdjustmentRequest
		wantAmount int64
		wantError  bool
	}{
		{
			name: "credit",
			request: directWalletAdjustmentRequest{
				Direction: " CREDIT ", Amount: 12, Reason: "  人工充值  ", RequestID: " request-1 ",
			},
			wantAmount: 12,
		},
		{
			name: "debit",
			request: directWalletAdjustmentRequest{
				Direction: "debit", Amount: 9, Reason: "重复派彩追回", RequestID: "request-2",
			},
			wantAmount: -9,
		},
		{
			name: "zero amount",
			request: directWalletAdjustmentRequest{
				Direction: "credit", Amount: 0, Reason: "无效", RequestID: "request-3",
			},
			wantError: true,
		},
		{
			name: "negative amount",
			request: directWalletAdjustmentRequest{
				Direction: "debit", Amount: -1, Reason: "无效", RequestID: "request-4",
			},
			wantError: true,
		},
		{
			name: "missing reason",
			request: directWalletAdjustmentRequest{
				Direction: "credit", Amount: 1, Reason: " ", RequestID: "request-5",
			},
			wantError: true,
		},
		{
			name: "missing request id",
			request: directWalletAdjustmentRequest{
				Direction: "credit", Amount: 1, Reason: "测试", RequestID: " ",
			},
			wantError: true,
		},
		{
			name: "invalid direction",
			request: directWalletAdjustmentRequest{
				Direction: "refund", Amount: 1, Reason: "测试", RequestID: "request-6",
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			amount, err := test.request.normalize()
			if test.wantError {
				if err == nil {
					t.Fatalf("expected validation error, got amount %d", amount)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if amount != test.wantAmount {
				t.Fatalf("unexpected signed amount: %d", amount)
			}
		})
	}
}

func TestDirectWalletAdjustmentNo(t *testing.T) {
	first := directWalletAdjustmentNo(12, "browser-request-1")
	repeated := directWalletAdjustmentNo(12, "browser-request-1")
	if first != repeated || len(first) != 26 || !strings.HasPrefix(first, "Q") {
		t.Fatalf("unexpected deterministic adjustment number: %q %q", first, repeated)
	}
	if first == directWalletAdjustmentNo(13, "browser-request-1") {
		t.Fatal("idempotency key must be scoped to the administrator")
	}
	if first == directWalletAdjustmentNo(12, "browser-request-2") {
		t.Fatal("different request ids must produce different adjustment numbers")
	}
}

func TestDirectWalletAdjustmentIntegration(t *testing.T) {
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
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'integration-test-only','后台调账联调用户',1)`,
		"admin_adjust_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	adminID := time.Now().UnixNano() & 0x3fffffffffffffff
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM audit_logs
			WHERE actor_type=1 AND actor_id=? AND action='wallet.adjustment.direct'`,
			adminID,
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_adjustments WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", userID)
	})

	handler := &Handler{db: db, wallet: wallet.New(db)}
	adminUser := adminauth.Admin{ID: adminID, Username: "integration-admin"}
	run := func(direction string, amount int64, reason, requestID string) (int, directAdjustmentResponse) {
		t.Helper()
		body, marshalErr := json.Marshal(map[string]any{
			"direction": direction, "amount": amount, "reason": reason, "request_id": requestID,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		request := httptest.NewRequest(http.MethodPost, "/admin/api/users/"+strconv.FormatInt(userID, 10)+"/wallet-adjustments", bytes.NewReader(body))
		request.SetPathValue("id", strconv.FormatInt(userID, 10))
		recorder := httptest.NewRecorder()
		httpx.RequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.adjustUserWallet(w, r.WithContext(withAdmin(r, adminUser)))
		})).ServeHTTP(recorder, request)
		var response directAdjustmentResponse
		if decodeErr := json.Unmarshal(recorder.Body.Bytes(), &response); decodeErr != nil {
			t.Fatalf("decode response %q: %v", recorder.Body.String(), decodeErr)
		}
		return recorder.Code, response
	}

	status, credited := run("credit", 100, "联调充值", "credit-"+suffix)
	if status != http.StatusOK || credited.Data.Balance.Available != 100 || credited.Data.Idempotent {
		t.Fatalf("unexpected credit response (%d): %#v", status, credited)
	}
	status, debited := run("debit", 40, "联调扣款", "debit-"+suffix)
	if status != http.StatusOK || debited.Data.Balance.Available != 60 {
		t.Fatalf("unexpected debit response (%d): %#v", status, debited)
	}
	status, repeated := run("credit", 100, "联调充值", "credit-"+suffix)
	if status != http.StatusOK || repeated.Data.Balance.Available != 60 || !repeated.Data.Idempotent ||
		repeated.Data.LedgerEntryNo != credited.Data.LedgerEntryNo {
		t.Fatalf("credit was not idempotent or returned a stale balance (%d): %#v", status, repeated)
	}
	status, conflict := run("credit", 101, "联调充值", "credit-"+suffix)
	if status != http.StatusConflict || conflict.Code != http.StatusConflict {
		t.Fatalf("expected request id conflict, got (%d): %#v", status, conflict)
	}
	status, insufficient := run("debit", 61, "余额不足", "insufficient-"+suffix)
	if status != http.StatusConflict || insufficient.Code != http.StatusConflict {
		t.Fatalf("expected insufficient funds conflict, got (%d): %#v", status, insufficient)
	}

	var available, adjustmentCount, ledgerCount, auditCount int64
	if err = db.QueryRowContext(ctx, `
		SELECT
			(SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'),
			(SELECT COUNT(*) FROM wallet_adjustments WHERE user_id=?),
			(SELECT COUNT(*) FROM wallet_ledger_entries WHERE user_id=? AND business_type='admin_adjustment'),
			(SELECT COUNT(*) FROM audit_logs WHERE actor_type=1 AND actor_id=? AND action='wallet.adjustment.direct')`,
		userID, userID, userID, adminID,
	).Scan(&available, &adjustmentCount, &ledgerCount, &auditCount); err != nil {
		t.Fatal(err)
	}
	if available != 60 || adjustmentCount != 2 || ledgerCount != 2 || auditCount != 2 {
		t.Fatalf(
			"unexpected persisted state: available=%d adjustments=%d ledger=%d audit=%d",
			available, adjustmentCount, ledgerCount, auditCount,
		)
	}
}

type directAdjustmentResponse struct {
	Code int `json:"code"`
	Data struct {
		Balance struct {
			Available int64 `json:"available"`
			Frozen    int64 `json:"frozen"`
		} `json:"balance"`
		LedgerEntryNo string `json:"ledger_entry_no"`
		Idempotent    bool   `json:"idempotent"`
	} `json:"data"`
}
