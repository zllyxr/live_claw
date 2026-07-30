package payment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/paymentconfig"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestBEpusdtPaymentIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	const (
		masterKey = "payment-integration-master-key-32-bytes"
		apiToken  = "payment-integration-api-token"
	)
	fixture := preparePaymentFixture(t, ctx, db, masterKey)
	var providerCalls atomic.Int64
	var blockNext atomic.Bool
	var failNextAfterAccept atomic.Bool
	blockedOrder := make(chan string, 1)
	releaseProvider := make(chan struct{})
	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls.Add(1)
		var fields map[string]any
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&fields); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if _, _, err := verifyFields(fields, apiToken); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		amount, amountIncluded, _ := providerString(fields["amount"])
		if amount != "28.88" || !amountIncluded ||
			fields["fiat"] != "CNY" ||
			fields["trade_type"] != "usdt.trc20" ||
			fields["name"] != "BEpusdt联调档位" {
			http.Error(w, "unexpected immutable order snapshot", http.StatusBadRequest)
			return
		}
		orderNo, _ := fields["order_id"].(string)
		if failNextAfterAccept.CompareAndSwap(true, false) {
			http.Error(w, "simulated response loss", http.StatusBadGateway)
			return
		}
		if blockNext.CompareAndSwap(true, false) {
			blockedOrder <- orderNo
			select {
			case <-releaseProvider:
			case <-r.Context().Done():
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status_code": 200, "message": "success",
			"data": map[string]any{
				"trade_id": "trade-" + orderNo, "order_id": orderNo,
				"fiat": "CNY", "trade_type": "usdt.trc20",
				"amount": "28.88", "actual_amount": "4.25",
				"token": "TRx-" + orderNo, "status": 1, "expiration_time": 1200,
				"payment_url": provider.URL + "/pay/checkout-counter/trade-" + orderNo,
			},
		})
	}))
	defer provider.Close()
	originalConfig := paymentconfig.ChannelConfig{
		APIBaseURL: provider.URL, PublicBaseURL: "https://payments.example.com",
		APIToken: apiToken, TradeType: "usdt.trc20", Fiat: "CNY",
		TimeoutSeconds: 1200,
	}
	fixture.configureChannel(t, ctx, db, originalConfig)
	service, err := New(
		db, wallet.New(db), masterKey, "https://app.example.com",
		Options{HTTPClient: provider.Client()},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE payment_channels SET config_verified_hash=REPEAT('0',64)
		WHERE channel_key='usdt'`,
	); err != nil {
		t.Fatal(err)
	}
	if _, createErr := service.CreateRecharge(ctx, CreateRequest{
		UserID: fixture.userID, ProductID: fixture.productID,
		ClientTraceID: "unverified-trace-" + fixture.suffix, ClientIP: "127.0.0.1",
	}); !errors.Is(createErr, ErrChannelNotReady) {
		t.Fatalf("new order bypassed config verification hash: %v", createErr)
	}
	fixture.configureChannel(t, ctx, db, originalConfig)

	const createConcurrency = 12
	createResults := make(chan Order, createConcurrency)
	createErrors := make(chan error, createConcurrency)
	var createGroup sync.WaitGroup
	for index := 0; index < createConcurrency; index++ {
		createGroup.Add(1)
		go func() {
			defer createGroup.Done()
			order, createErr := service.CreateRecharge(ctx, CreateRequest{
				UserID: fixture.userID, ProductID: fixture.productID,
				ClientTraceID: "same-trace-" + fixture.suffix, ClientIP: "127.0.0.1",
			})
			createResults <- order
			createErrors <- createErr
		}()
	}
	createGroup.Wait()
	close(createResults)
	close(createErrors)
	for createErr := range createErrors {
		if createErr != nil {
			t.Fatalf("concurrent create error: %v", createErr)
		}
	}
	var paidOrder Order
	for order := range createResults {
		if paidOrder.OrderNo == "" {
			paidOrder = order
		} else if order.OrderNo != paidOrder.OrderNo {
			t.Fatalf("same trace created different orders: %s and %s", paidOrder.OrderNo, order.OrderNo)
		}
	}
	if calls := providerCalls.Load(); calls != 1 {
		t.Fatalf("same trace made %d provider calls, want 1", calls)
	}
	if _, err = service.OrderStatus(ctx, fixture.otherUserID, paidOrder.OrderNo); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("other user could inspect the order: %v", err)
	}

	paidCallback := signedCallback(t, apiToken, map[string]any{
		"trade_id": "trade-" + paidOrder.OrderNo, "order_id": paidOrder.OrderNo,
		"amount": json.Number("28.88"), "actual_amount": "4.25",
		"token": "TRx-" + paidOrder.OrderNo, "block_transaction_id": "block-" + paidOrder.OrderNo,
		"status": 2,
	})
	const callbackConcurrency = 20
	callbackErrors := make(chan error, callbackConcurrency)
	var callbackGroup sync.WaitGroup
	for index := 0; index < callbackConcurrency; index++ {
		callbackGroup.Add(1)
		go func() {
			defer callbackGroup.Done()
			_, callbackErr := service.HandleBEpusdtCallback(ctx, paidCallback)
			callbackErrors <- callbackErr
		}()
	}
	callbackGroup.Wait()
	close(callbackErrors)
	for callbackErr := range callbackErrors {
		if callbackErr != nil {
			t.Fatalf("concurrent callback error: %v", callbackErr)
		}
	}
	assertPaidOnce(t, ctx, db, fixture.userID, paidOrder.OrderNo, 288)

	for _, providerStatus := range []int{1, 3, 6} {
		raw := signedCallback(t, apiToken, map[string]any{
			"trade_id": "trade-" + paidOrder.OrderNo, "order_id": paidOrder.OrderNo,
			"amount": json.Number("28.88"), "status": providerStatus,
		})
		if _, err = service.HandleBEpusdtCallback(ctx, raw); err != nil {
			t.Fatalf("post-paid status %d callback: %v", providerStatus, err)
		}
		current, statusErr := service.OrderStatus(ctx, fixture.userID, paidOrder.OrderNo)
		if statusErr != nil || current.Status != OrderStatusPaid {
			t.Fatalf("paid order downgraded by status %d: status=%d err=%v", providerStatus, current.Status, statusErr)
		}
	}

	failedOrder, err := service.CreateRecharge(ctx, CreateRequest{
		UserID: fixture.userID, ProductID: fixture.productID,
		ClientTraceID: "failed-trace-" + fixture.suffix, ClientIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	failedCallback := signedCallback(t, apiToken, map[string]any{
		"trade_id": "trade-" + failedOrder.OrderNo, "order_id": failedOrder.OrderNo,
		"amount": json.Number("28.88"), "status": 6,
	})
	result, err := service.HandleBEpusdtCallback(ctx, failedCallback)
	if err != nil || result.Status != OrderStatusFailed {
		t.Fatalf("status=6 transition: result=%#v err=%v", result, err)
	}

	closedOrder, err := service.CreateRecharge(ctx, CreateRequest{
		UserID: fixture.userID, ProductID: fixture.productID,
		ClientTraceID: "closed-trace-" + fixture.suffix, ClientIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	closedCallback := signedCallback(t, apiToken, map[string]any{
		"trade_id": "trade-" + closedOrder.OrderNo, "order_id": closedOrder.OrderNo,
		"amount": json.Number("28.88"), "status": 3,
	})
	result, err = service.HandleBEpusdtCallback(ctx, closedCallback)
	if err != nil || result.Status != OrderStatusClosed {
		t.Fatalf("status=3 transition: result=%#v err=%v", result, err)
	}

	progressOrder, err := service.CreateRecharge(ctx, CreateRequest{
		UserID: fixture.otherUserID, ProductID: fixture.productID,
		ClientTraceID: "progress-trace-" + fixture.suffix, ClientIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	progressBlock := "block-progress-" + progressOrder.OrderNo
	progressPaying := signedCallback(t, apiToken, map[string]any{
		"trade_id": "trade-" + progressOrder.OrderNo, "order_id": progressOrder.OrderNo,
		"amount": json.Number("28.88"), "block_transaction_id": progressBlock,
		"status": 1,
	})
	if result, err = service.HandleBEpusdtCallback(ctx, progressPaying); err != nil ||
		result.Status != OrderStatusPaying {
		t.Fatalf("same-block status=1 transition: result=%#v err=%v", result, err)
	}
	progressPaid := signedCallback(t, apiToken, map[string]any{
		"trade_id": "trade-" + progressOrder.OrderNo, "order_id": progressOrder.OrderNo,
		"amount": json.Number("28.88"), "actual_amount": "4.25",
		"token":                "TRx-" + progressOrder.OrderNo,
		"block_transaction_id": progressBlock, "status": 2,
	})
	if result, err = service.HandleBEpusdtCallback(ctx, progressPaid); err != nil ||
		result.Status != OrderStatusPaid {
		t.Fatalf("same-block status=2 transition: result=%#v err=%v", result, err)
	}
	assertPaidOnceWithCallbackCount(
		t, ctx, db, fixture.otherUserID, progressOrder.OrderNo, 288, 2,
	)

	blockNext.Store(true)
	earlyResult := make(chan Order, 1)
	earlyError := make(chan error, 1)
	go func() {
		order, createErr := service.CreateRecharge(ctx, CreateRequest{
			UserID: fixture.userID, ProductID: fixture.productID,
			ClientTraceID: "early-trace-" + fixture.suffix, ClientIP: "127.0.0.1",
		})
		earlyResult <- order
		earlyError <- createErr
	}()
	var earlyOrderNo string
	select {
	case earlyOrderNo = <-blockedOrder:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	earlyCallback := signedCallback(t, apiToken, map[string]any{
		"trade_id": "trade-" + earlyOrderNo, "order_id": earlyOrderNo,
		"amount": json.Number("28.88"), "actual_amount": "4.25",
		"token": "TRx-" + earlyOrderNo, "block_transaction_id": "block-" + earlyOrderNo,
		"status": 2,
	})
	if _, err = service.HandleBEpusdtCallback(ctx, earlyCallback); err != nil {
		t.Fatalf("callback before provider response: %v", err)
	}
	close(releaseProvider)
	if err = <-earlyError; err != nil {
		t.Fatalf("create after early callback: %v", err)
	}
	earlyOrder := <-earlyResult
	if earlyOrder.Status != OrderStatusPaid || earlyOrder.PaymentURL == "" {
		t.Fatalf("early callback result was not preserved: %#v", earlyOrder)
	}
	assertPaidOnce(t, ctx, db, fixture.userID, earlyOrderNo, 576)

	// Simulate the provider accepting a create request while its response is
	// lost. The local row must already contain everything needed for a retry.
	failNextAfterAccept.Store(true)
	recoveryTrace := "recovery-trace-" + fixture.suffix
	if _, createErr := service.CreateRecharge(ctx, CreateRequest{
		UserID: fixture.userID, ProductID: fixture.productID,
		ClientTraceID: recoveryTrace, ClientIP: "127.0.0.1",
	}); !errors.Is(createErr, ErrProvider) {
		t.Fatalf("simulated response loss error = %v", createErr)
	}
	var recoveryOrderNo, snapshotProductName string
	var snapshotBytes int
	var snapshotVersion int
	if err = db.QueryRowContext(ctx, `
		SELECT order_no,OCTET_LENGTH(provider_config_ciphertext),
		       provider_config_key_version,product_name_snapshot
		FROM recharge_orders
		WHERE user_id=? AND client_trace_id=?`,
		fixture.userID, recoveryTrace,
	).Scan(
		&recoveryOrderNo, &snapshotBytes, &snapshotVersion, &snapshotProductName,
	); err != nil {
		t.Fatal(err)
	}
	if snapshotBytes < 1 || snapshotVersion != providerConfigSnapshotVersion ||
		snapshotProductName != "BEpusdt联调档位" {
		t.Fatalf(
			"missing immutable provider snapshot: bytes=%d version=%d name=%q",
			snapshotBytes, snapshotVersion, snapshotProductName,
		)
	}

	rotatedCipher, cipherErr := paymentconfig.NewCipher(masterKey)
	if cipherErr != nil {
		t.Fatal(cipherErr)
	}
	rotatedConfig, cipherErr := rotatedCipher.Encrypt(USDTChannelKey, paymentconfig.ChannelConfig{
		APIBaseURL: provider.URL, PublicBaseURL: "https://rotated-payments.example.com",
		APIToken: "rotated-payment-api-token", TradeType: "usdc.trc20", Fiat: "CNY",
		TimeoutSeconds: 600,
	})
	if cipherErr != nil {
		t.Fatal(cipherErr)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE payment_channels
		SET config_ciphertext=?,config_verified_hash='',config_verified_at=NULL,status=0
		WHERE channel_key='usdt'`,
		rotatedConfig,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE recharge_products
		SET name='轮换后的档位',amount_minor=9999,coin_amount=999,
		    bonus_coin=99,status=0
		WHERE id=?`,
		fixture.productID,
	); err != nil {
		t.Fatal(err)
	}
	if _, createErr := service.CreateRecharge(ctx, CreateRequest{
		UserID: fixture.userID, ProductID: fixture.productID,
		ClientTraceID: "new-after-rotation-" + fixture.suffix, ClientIP: "127.0.0.1",
	}); !errors.Is(createErr, ErrChannelDisabled) {
		t.Fatalf("new order bypassed disabled current channel: %v", createErr)
	}
	recovered, err := service.CreateRecharge(ctx, CreateRequest{
		UserID: fixture.userID, ProductID: fixture.productID,
		ClientTraceID: recoveryTrace, ClientIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("retry after channel/product rotation: %v", err)
	}
	if recovered.OrderNo != recoveryOrderNo || recovered.PaymentURL == "" ||
		recovered.AmountMinor != 2888 || recovered.CoinAmount != 268 ||
		recovered.BonusCoin != 20 {
		t.Fatalf("retry did not preserve order snapshot: %#v", recovered)
	}

	// BEpusdt can still retry a notification signed with the order's original
	// token, so the immutable snapshot must remain accepted after rotation.
	originalTokenProgress := signedCallback(t, apiToken, map[string]any{
		"trade_id": "trade-" + recoveryOrderNo, "order_id": recoveryOrderNo,
		"amount": json.Number("28.88"), "status": 1,
	})
	if _, err = service.HandleBEpusdtCallback(ctx, originalTokenProgress); err != nil {
		t.Fatalf("snapshot-token callback after rotation: %v", err)
	}

	// BEpusdt signs new notifications with its current global token. A newly
	// saved but unverified channel token must not be trusted for old orders.
	rotatedToken := "rotated-payment-api-token"
	recoveryCallback := signedCallback(t, rotatedToken, map[string]any{
		"trade_id": "trade-" + recoveryOrderNo, "order_id": recoveryOrderNo,
		"amount": json.Number("28.88"), "actual_amount": "4.25",
		"token":                "TRx-" + recoveryOrderNo,
		"block_transaction_id": "block-" + recoveryOrderNo,
		"status":               2,
	})
	if _, err = service.HandleBEpusdtCallback(ctx, recoveryCallback); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("unverified rotated token callback error = %v", err)
	}
	var recoveryStatus, recoveryLedgerCount int
	var recoveryCallbackCount uint64
	if err = db.QueryRowContext(ctx, `
		SELECT recharge.status,recharge.callback_count,
		       (SELECT COUNT(*) FROM wallet_ledger_entries
		        WHERE user_id=? AND business_type='recharge'
		          AND business_id=recharge.order_no)
		FROM recharge_orders recharge
		WHERE recharge.user_id=? AND recharge.order_no=?`,
		fixture.userID, fixture.userID, recoveryOrderNo,
	).Scan(
		&recoveryStatus, &recoveryCallbackCount, &recoveryLedgerCount,
	); err != nil {
		t.Fatal(err)
	}
	if recoveryStatus != OrderStatusPaying || recoveryCallbackCount != 1 ||
		recoveryLedgerCount != 0 {
		t.Fatalf(
			"unverified token changed order: status=%d callbacks=%d ledger=%d",
			recoveryStatus, recoveryCallbackCount, recoveryLedgerCount,
		)
	}

	rotatedHash := sha256.Sum256(rotatedConfig)
	if _, err = db.ExecContext(ctx, `
		UPDATE payment_channels
		SET config_verified_hash=?,
		    config_verified_at=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 11 MINUTE)
		WHERE channel_key='usdt'`,
		hex.EncodeToString(rotatedHash[:]),
	); err != nil {
		t.Fatal(err)
	}
	expiredOldTokenCallback := signedCallback(t, apiToken, map[string]any{
		"trade_id": "trade-" + recoveryOrderNo, "order_id": recoveryOrderNo,
		"amount": json.Number("28.88"), "actual_amount": "4.25",
		"token":                "TRx-" + recoveryOrderNo,
		"block_transaction_id": "block-" + recoveryOrderNo,
		"status":               2,
	})
	if _, err = service.HandleBEpusdtCallback(
		ctx, expiredOldTokenCallback,
	); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expired old-token callback error = %v", err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE payment_channels
		SET config_verified_at=CURRENT_TIMESTAMP(3)
		WHERE channel_key='usdt'`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = service.HandleBEpusdtCallback(ctx, recoveryCallback); err != nil {
		t.Fatalf("verified current-token callback after rotation: %v", err)
	}
	assertPaidOnceWithCallbackCount(
		t, ctx, db, fixture.userID, recoveryOrderNo, 864, 2,
	)
}

type paymentFixture struct {
	suffix      string
	userID      int64
	otherUserID int64
	productID   int64
	masterKey   string
}

func preparePaymentFixture(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	masterKey string,
) paymentFixture {
	t.Helper()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	var (
		oldName, oldProvider, oldCurrency           string
		oldScale, oldKeyVersion, oldStatus, oldSort int
		oldMin, oldMax                              int64
		oldConfig                                   []byte
		oldVerifiedHash                             string
		oldVerifiedAt                               sql.NullTime
	)
	err := db.QueryRowContext(ctx, `
		SELECT name,provider,currency,currency_scale,min_amount_minor,max_amount_minor,
		       config_ciphertext,key_version,config_verified_hash,
		       config_verified_at,status,sort_order
		FROM payment_channels WHERE channel_key='usdt'`).Scan(
		&oldName, &oldProvider, &oldCurrency, &oldScale, &oldMin, &oldMax,
		&oldConfig, &oldKeyVersion, &oldVerifiedHash, &oldVerifiedAt,
		&oldStatus, &oldSort,
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture := paymentFixture{suffix: suffix, masterKey: masterKey}
	for index, target := range []*int64{&fixture.userID, &fixture.otherUserID} {
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO users(username,password_hash,nickname,status)
			VALUES(?,'integration-test-only',?,1)`,
			fmt.Sprintf("payment_%d_%s", index, suffix), "支付联调用户",
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		*target, _ = result.LastInsertId()
	}
	productResult, err := db.ExecContext(ctx, `
		INSERT INTO recharge_products
			(name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
		VALUES('BEpusdt联调档位','CNY',2,2888,268,20,1,0)`)
	if err != nil {
		t.Fatal(err)
	}
	fixture.productID, _ = productResult.LastInsertId()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM payment_callback_block_bindings
			WHERE order_no IN (SELECT order_no FROM recharge_orders WHERE user_id IN (?,?))`,
			fixture.userID, fixture.otherUserID,
		)
		_, _ = db.ExecContext(cleanupCtx, `
			DELETE FROM payment_callback_events
			WHERE order_no IN (SELECT order_no FROM recharge_orders WHERE user_id IN (?,?))`,
			fixture.userID, fixture.otherUserID,
		)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id IN (?,?)", fixture.userID, fixture.otherUserID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id IN (?,?)", fixture.userID, fixture.otherUserID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM recharge_orders WHERE user_id IN (?,?)", fixture.userID, fixture.otherUserID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM recharge_products WHERE id=?", fixture.productID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id IN (?,?)", fixture.userID, fixture.otherUserID)
		_, _ = db.ExecContext(cleanupCtx, `
			UPDATE payment_channels
			SET name=?,provider=?,currency=?,currency_scale=?,min_amount_minor=?,
			    max_amount_minor=?,config_ciphertext=?,key_version=?,
			    config_verified_hash=?,config_verified_at=?,status=?,sort_order=?
			WHERE channel_key='usdt'`,
			oldName, oldProvider, oldCurrency, oldScale, oldMin, oldMax,
			oldConfig, oldKeyVersion, oldVerifiedHash, pointerTimeValue(nullTimePointer(oldVerifiedAt)),
			oldStatus, oldSort,
		)
	})
	return fixture
}

func (fixture paymentFixture) configureChannel(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	config paymentconfig.ChannelConfig,
) {
	t.Helper()
	cipher, err := paymentconfig.NewCipher(fixture.masterKey)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := cipher.Encrypt(USDTChannelKey, config)
	if err != nil {
		t.Fatal(err)
	}
	configHash := sha256.Sum256(ciphertext)
	_, err = db.ExecContext(ctx, `
		UPDATE payment_channels
		SET name='USDT.TRC20',provider='bepusdt',currency='CNY',currency_scale=2,
		    min_amount_minor=1,max_amount_minor=100000000,config_ciphertext=?,
		    key_version=1,config_verified_hash=?,
		    config_verified_at=CURRENT_TIMESTAMP(3),status=1
		WHERE channel_key='usdt'`,
		ciphertext, hex.EncodeToString(configHash[:]),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func signedCallback(t *testing.T, token string, fields map[string]any) []byte {
	t.Helper()
	signature, err := signFields(fields, token)
	if err != nil {
		t.Fatal(err)
	}
	fields["signature"] = signature
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Clone(raw)
}

func assertPaidOnce(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	userID int64,
	orderNo string,
	expectedBalance int64,
) {
	assertPaidOnceWithCallbackCount(t, ctx, db, userID, orderNo, expectedBalance, 1)
}

func assertPaidOnceWithCallbackCount(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	userID int64,
	orderNo string,
	expectedBalance int64,
	expectedCallbackCount uint64,
) {
	t.Helper()
	var status int
	var callbackCount uint64
	if err := db.QueryRowContext(ctx, `
		SELECT status,callback_count FROM recharge_orders
		WHERE user_id=? AND order_no=?`,
		userID, orderNo,
	).Scan(&status, &callbackCount); err != nil {
		t.Fatal(err)
	}
	if status != OrderStatusPaid || callbackCount != expectedCallbackCount {
		t.Fatalf("order status/callback count = %d/%d", status, callbackCount)
	}
	var ledgerCount, eventCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM wallet_ledger_entries
		WHERE user_id=? AND business_type='recharge' AND business_id=?`,
		userID, orderNo,
	).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM payment_callback_events
		WHERE order_no=? AND provider_status=2`,
		orderNo,
	).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	var balance int64
	if err := db.QueryRowContext(ctx, `
		SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'`,
		userID,
	).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 || eventCount != 1 || balance != expectedBalance {
		t.Fatalf(
			"paid-once invariant failed: ledger=%d event=%d balance=%d want=%d",
			ledgerCount, eventCount, balance, expectedBalance,
		)
	}
}
