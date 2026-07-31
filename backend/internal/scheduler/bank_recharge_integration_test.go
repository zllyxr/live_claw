package scheduler

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestExpiredBankRechargesSkipSubmittedProofsIntegration(t *testing.T) {
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
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'bank-expiry-test','银行卡超时联调用户',1)`, "bank_expiry_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := userResult.LastInsertId()
	var channelID, productID int64
	if err = db.QueryRowContext(ctx, "SELECT id FROM payment_channels WHERE channel_key='bank'").Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	productResult, err := db.ExecContext(ctx, `
		INSERT INTO recharge_products
			(name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
		VALUES(?,'CNY',2,1000,100,0,1,0)`, "银行卡超时联调档位"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	productID, _ = productResult.LastInsertId()
	createOrder := func(trace string, status int) (int64, string) {
		orderNo, _ := idgen.New()
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO recharge_orders
				(order_no,user_id,product_id,channel_id,client_trace_id,product_name_snapshot,
				 fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,
				 client_ip,provider_payload,expires_at)
			VALUES(?,?,?,?,?,'银行卡超时联调','CNY',2,1000,100,0,?,'127.0.0.1',
			       JSON_OBJECT('payment_method','bank_transfer'),CURRENT_TIMESTAMP(3)-INTERVAL 1 MINUTE)`,
			orderNo, userID, productID, channelID, trace, status)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		orderID, _ := result.LastInsertId()
		if _, insertErr = db.ExecContext(ctx, `INSERT INTO payment_bank_order_details(recharge_order_id) VALUES(?)`, orderID); insertErr != nil {
			t.Fatal(insertErr)
		}
		return orderID, orderNo
	}
	expiredID, _ := createOrder("BANK_EXPIRED_"+suffix, 0)
	proofID, _ := createOrder("BANK_PROOF_"+suffix, 1)
	assetResult, err := db.ExecContext(ctx, `
		INSERT INTO media_assets
			(owner_user_id,bucket,object_key,media_type,mime_type,size_bytes,sha256,status)
		VALUES(?,'claw-private',?,'image','image/png',4,REPEAT('b',64),1)`,
		userID, "bank-proof-expiry-test/"+suffix+".png")
	if err != nil {
		t.Fatal(err)
	}
	assetID, _ := assetResult.LastInsertId()
	if _, err = db.ExecContext(ctx, `
		INSERT INTO payment_bank_proofs(recharge_order_id,user_id,asset_id,status)
		VALUES(?,?,?,0)`, proofID, userID, assetID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanup, "DELETE FROM payment_bank_proofs WHERE recharge_order_id IN (?,?)", expiredID, proofID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM payment_bank_order_details WHERE recharge_order_id IN (?,?)", expiredID, proofID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM media_assets WHERE id=?", assetID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM recharge_orders WHERE id IN (?,?)", expiredID, proofID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM recharge_products WHERE id=?", productID)
		_, _ = db.ExecContext(cleanup, "DELETE FROM users WHERE id=?", userID)
	})
	runner := &Runner{db: db, now: time.Now}
	if err = runner.closeExpiredBankRecharges(ctx); err != nil {
		t.Fatal(err)
	}
	var expiredStatus, protectedStatus int
	var closeReason string
	if err = db.QueryRowContext(ctx, `
		SELECT recharge.status,detail.close_reason
		FROM recharge_orders recharge JOIN payment_bank_order_details detail
		  ON detail.recharge_order_id=recharge.id WHERE recharge.id=?`, expiredID).Scan(&expiredStatus, &closeReason); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, "SELECT status FROM recharge_orders WHERE id=?", proofID).Scan(&protectedStatus); err != nil {
		t.Fatal(err)
	}
	if expiredStatus != 4 || closeReason != "银行卡订单已超时" || protectedStatus != 1 {
		t.Fatalf("expired status=%d reason=%q protected status=%d", expiredStatus, closeReason, protectedStatus)
	}
}
