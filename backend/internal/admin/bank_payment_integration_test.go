package admin

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/bankpayment"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestBankRechargeAssignmentAndApprovalIntegration(t *testing.T) {
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
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'bank-payment-test','银行卡充值联调用户',1)`, "bank_payment_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	_, err = db.ExecContext(ctx, `
		INSERT INTO wallet_accounts(user_id,currency,available,frozen)
		VALUES(?,'COIN',0,0)`, userID)
	if err != nil {
		t.Fatal(err)
	}
	var channelID, productID int64
	if err = db.QueryRowContext(ctx, "SELECT id FROM payment_channels WHERE channel_key='bank'").Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	productResult, err := db.ExecContext(ctx, `
		INSERT INTO recharge_products
			(name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
		VALUES(?,'CNY',2,1000,100,10,1,0)`, "银行卡联调档位"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	productID, _ = productResult.LastInsertId()
	cipher, err := bankpayment.NewCipher("bank-payment-integration-key-32-bytes")
	if err != nil {
		t.Fatal(err)
	}
	cardNumber := "6222021234567890"
	accountHash := bankpayment.CardHash(cardNumber + suffix)
	accountCiphertext, err := cipher.EncryptAccount(accountHash, bankpayment.AccountSecret{
		HolderName: "联调收款人", CardNumber: cardNumber,
	})
	if err != nil {
		t.Fatal(err)
	}
	accountResult, err := db.ExecContext(ctx, `
		INSERT INTO payment_bank_accounts
			(display_name,bank_name,branch_name,account_ciphertext,account_hash,
			 account_masked,key_version,instructions,status,sort_order)
		VALUES('联调收款卡','联调银行','联调支行',?,?,?,?,'仅用于自动化测试',1,0)`,
		accountCiphertext, accountHash, bankpayment.MaskCardNumber(cardNumber), bankpayment.KeyVersion)
	if err != nil {
		t.Fatal(err)
	}
	accountID, _ := accountResult.LastInsertId()
	orderNo, _ := idgen.New()
	orderResult, err := db.ExecContext(ctx, `
		INSERT INTO recharge_orders
			(order_no,user_id,product_id,channel_id,client_trace_id,product_name_snapshot,
			 fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,
			 client_ip,provider_payload,expires_at)
		VALUES(?,?,?,?,?,'银行卡联调档位','CNY',2,1000,100,10,0,'127.0.0.1',
		       JSON_OBJECT('payment_method','bank_transfer'),CURRENT_TIMESTAMP(3)+INTERVAL 10 MINUTE)`,
		orderNo, userID, productID, channelID, "BANK_TEST_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	orderID, _ := orderResult.LastInsertId()
	if _, err = db.ExecContext(ctx, `INSERT INTO payment_bank_order_details(recharge_order_id) VALUES(?)`, orderID); err != nil {
		t.Fatal(err)
	}
	actor := adminauth.Admin{ID: time.Now().UnixNano() & 0x1fffffffffffffff, Username: "bank-reviewer"}
	handler := &Handler{db: db, wallet: wallet.New(db), bankPaymentCipher: cipher}
	t.Cleanup(func() {
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanup, "DELETE FROM audit_logs WHERE actor_id=?", actor.ID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM payment_bank_proofs WHERE recharge_order_id=?", orderID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM payment_bank_order_details WHERE recharge_order_id=?", orderID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM media_assets WHERE owner_user_id=? AND object_key LIKE 'bank-proof-test/%'", userID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM recharge_orders WHERE id=?", orderID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM recharge_products WHERE id=?", productID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM payment_bank_accounts WHERE id=?", accountID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM wallet_accounts WHERE user_id=?", userID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM users WHERE id=?", userID)
	})

	assigned := invokeFinanceAdminHandler(
		handler.assignRechargeBankAccount, actor, "POST", "/assign", strconv.FormatInt(orderID, 10),
		map[string]any{"bank_account_id": strconv.FormatInt(accountID, 10)}, "",
	)
	if assigned.Code != 200 {
		t.Fatalf("assign status=%d body=%s", assigned.Code, assigned.Body.String())
	}
	var snapshot []byte
	var snapshotVersion, orderStatus int
	var expiresAt time.Time
	if err = db.QueryRowContext(ctx, `
		SELECT detail.account_snapshot_ciphertext,detail.snapshot_key_version,
		       recharge.status,recharge.expires_at
		FROM payment_bank_order_details detail
		JOIN recharge_orders recharge ON recharge.id=detail.recharge_order_id
		WHERE detail.recharge_order_id=?`, orderID).Scan(&snapshot, &snapshotVersion, &orderStatus, &expiresAt); err != nil {
		t.Fatal(err)
	}
	decoded, err := cipher.DecryptSnapshot(orderNo, snapshot)
	if err != nil || decoded.CardNumber != cardNumber || snapshotVersion != bankpayment.KeyVersion ||
		orderStatus != 1 || time.Until(expiresAt) < 29*time.Minute {
		t.Fatalf("snapshot=%#v version=%d status=%d expires=%s err=%v", decoded, snapshotVersion, orderStatus, expiresAt, err)
	}
	secondAssign := invokeFinanceAdminHandler(
		handler.assignRechargeBankAccount, actor, "POST", "/assign", strconv.FormatInt(orderID, 10),
		map[string]any{"bank_account_id": strconv.FormatInt(accountID, 10)}, "",
	)
	if secondAssign.Code != 409 {
		t.Fatalf("second assignment status=%d body=%s", secondAssign.Code, secondAssign.Body.String())
	}

	assetResult, err := db.ExecContext(ctx, `
		INSERT INTO media_assets
			(owner_user_id,bucket,object_key,media_type,mime_type,size_bytes,sha256,status)
		VALUES(?,'claw-private',?,'image','image/png',4,REPEAT('a',64),1)`,
		userID, "bank-proof-test/"+suffix+".png")
	if err != nil {
		t.Fatal(err)
	}
	assetID, _ := assetResult.LastInsertId()
	if _, err = db.ExecContext(ctx, `
		INSERT INTO payment_bank_proofs(recharge_order_id,user_id,asset_id,status)
		VALUES(?,?,?,0)`, orderID, userID, assetID); err != nil {
		t.Fatal(err)
	}
	approved := invokeFinanceAdminHandler(
		handler.approveBankRechargeProof, actor, "POST", "/approve", strconv.FormatInt(orderID, 10),
		map[string]any{"reason": "已核对银行流水到账"}, "",
	)
	if approved.Code != 200 {
		t.Fatalf("approve status=%d body=%s", approved.Code, approved.Body.String())
	}
	duplicateApproval := invokeFinanceAdminHandler(
		handler.approveBankRechargeProof, actor, "POST", "/approve", strconv.FormatInt(orderID, 10),
		map[string]any{"reason": "重复确认"}, "",
	)
	if duplicateApproval.Code != 409 {
		t.Fatalf("duplicate approval status=%d body=%s", duplicateApproval.Code, duplicateApproval.Body.String())
	}
	var available, ledgerCount int64
	if err = db.QueryRowContext(ctx, `
		SELECT account.available,
		       (SELECT COUNT(*) FROM wallet_ledger_entries WHERE user_id=? AND business_type='recharge' AND business_id=?)
		FROM wallet_accounts account WHERE account.user_id=? AND account.currency='COIN'`,
		userID, orderNo, userID).Scan(&available, &ledgerCount); err != nil {
		t.Fatal(err)
	}
	if available != 110 || ledgerCount != 1 {
		t.Fatalf("available=%d ledger_count=%d, want 110 and 1", available, ledgerCount)
	}
	var finalStatus, proofStatus int
	if err = db.QueryRowContext(ctx, `
		SELECT recharge.status,proof.status FROM recharge_orders recharge
		JOIN payment_bank_proofs proof ON proof.recharge_order_id=recharge.id
		WHERE recharge.id=?`, orderID).Scan(&finalStatus, &proofStatus); err != nil {
		t.Fatal(err)
	}
	if finalStatus != 2 || proofStatus != 1 {
		t.Fatalf("order status=%d proof status=%d", finalStatus, proofStatus)
	}
}
