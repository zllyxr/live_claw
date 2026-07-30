package admin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/payment"
	"github.com/zllyxr/live_claw/backend/internal/paymentconfig"
)

func validAdminPaymentConfig(apiBaseURL string) paymentconfig.ChannelConfig {
	return paymentconfig.ChannelConfig{
		APIBaseURL: apiBaseURL, PublicBaseURL: "https://pay.example.com",
		APIToken: "admin-payment-test-token", TradeType: "usdt.trc20",
		Fiat: "CNY", TimeoutSeconds: 1200,
	}
}

func TestCheckBEpusdtCredentialsUsesSignedCancelForMissingOrder(t *testing.T) {
	var requestCount atomic.Int32
	var method, path, tradeID, signature string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		method = r.Method
		path = r.URL.Path
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(payload) != 2 {
			t.Errorf("unexpected protocol check payload: %#v", payload)
		}
		tradeID = payload["trade_id"]
		signature = payload["signature"]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":400,"message":"订单不存在"}`))
	}))
	defer provider.Close()

	config := validAdminPaymentConfig(provider.URL)
	result, err := checkBEpusdtCredentials(
		context.Background(), provider.Client(), validAdminPaymentConfig(provider.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusBadRequest || result.Message == "" {
		t.Fatalf("unexpected protocol result: %#v", result)
	}
	if requestCount.Load() != 1 || method != http.MethodPost ||
		path != "/api/v1/order/cancel-transaction" ||
		!strings.HasPrefix(tradeID, "claw_config_check_") {
		t.Fatalf(
			"unsafe protocol check count=%d method=%q path=%q trade_id=%q",
			requestCount.Load(), method, path, tradeID,
		)
	}
	expectedSignature, err := payment.SignBEpusdtRequest(
		map[string]any{"trade_id": tradeID}, config.APIToken,
	)
	if err != nil {
		t.Fatal(err)
	}
	if signature == "" || signature != expectedSignature {
		t.Fatalf("protocol check was not signed correctly: %q", signature)
	}
}

func TestCheckBEpusdtCredentialsRefusesRedirectToProviderRoot(t *testing.T) {
	var rootHits atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			rootHits.Add(1)
			_, _ = w.Write([]byte("installation secret"))
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	defer provider.Close()

	_, err := checkBEpusdtCredentials(
		context.Background(), newPaymentHTTPClient(), validAdminPaymentConfig(provider.URL),
	)
	if err == nil {
		t.Fatal("redirecting provider was accepted")
	}
	if rootHits.Load() != 0 {
		t.Fatalf("protocol check followed redirect to provider root %d time(s)", rootHits.Load())
	}
}

func TestVerifyBEpusdtPaidOrderRequiresExactPaidProviderState(t *testing.T) {
	status := 2
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/pay/info" {
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode provider info request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status_code": 200,
			"message":     "success",
			"data": map[string]any{
				"trade_id": "provider-paid-1", "order_id": "local-order-1",
				"trade_type": "usdt.trc20", "status": status,
				"money": "28.8800", "actual_amount": "4.25",
				"token": "TRx-paid-address", "fiat": "CNY",
			},
		})
	}))
	defer provider.Close()
	config := validAdminPaymentConfig(provider.URL)
	expected := bepusdtPaidOrderExpectation{
		TradeID: "provider-paid-1", OrderID: "local-order-1",
		Fiat: "CNY", TradeType: "usdt.trc20",
		AmountMinor: 2888, CurrencyScale: 2,
		ActualAmount: "4.25", PaymentAddress: "TRx-paid-address",
	}
	if err := verifyBEpusdtPaidOrder(
		context.Background(), provider.Client(), config, expected,
	); err != nil {
		t.Fatalf("verified paid order was rejected: %v", err)
	}
	status = 1
	if err := verifyBEpusdtPaidOrder(
		context.Background(), provider.Client(), config, expected,
	); !errors.Is(err, errBEpusdtPaymentNotConfirmed) {
		t.Fatalf("unpaid provider order error = %v", err)
	}
	status = 2
	expected.AmountMinor = 2889
	if err := verifyBEpusdtPaidOrder(
		context.Background(), provider.Client(), config, expected,
	); !errors.Is(err, errBEpusdtPaymentNotConfirmed) {
		t.Fatalf("mismatched provider amount error = %v", err)
	}
}

func TestPaymentChannelAuditDataNeverContainsToken(t *testing.T) {
	const secret = "audit-must-never-contain-this-token"
	request := paymentChannelUpdateRequest{
		Name: "BEpusdt", APIBaseURL: "http://bepusdt:8080",
		PublicBaseURL: "https://pay.example.com", APIToken: secret,
		TradeType: "usdt.trc20", Fiat: "CNY", TimeoutSeconds: 1200,
		CurrencyScale: 2, MinAmountMinor: 100, MaxAmountMinor: 100000,
		Status: 1, SortOrder: 100,
	}
	encoded, err := json.Marshal(paymentChannelAuditData("usdt", "bepusdt", request, true))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) ||
		strings.Contains(string(encoded), "api_token") ||
		!strings.Contains(string(encoded), `"token_configured":true`) {
		t.Fatalf("unsafe payment audit payload: %s", encoded)
	}
}

func TestPaymentConfigAPIDataReturnsOnlyTokenConfigured(t *testing.T) {
	const secret = "api-must-never-return-this-token"
	cipher, err := paymentconfig.NewCipher("0123456789abcdef-admin-payment-test-key")
	if err != nil {
		t.Fatal(err)
	}
	config := validAdminPaymentConfig("http://bepusdt:8080")
	config.APIToken = secret
	ciphertext, err := cipher.Encrypt("usdt", config)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(paymentConfigAPIData(cipher, "usdt", ciphertext))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) ||
		strings.Contains(string(encoded), "api_token") ||
		strings.Contains(string(encoded), "ciphertext") ||
		!strings.Contains(string(encoded), `"token_configured":true`) {
		t.Fatalf("unsafe payment API data: %s", encoded)
	}
}

func TestPaymentProtocolEndpointCannotTargetProviderRootOrCreateOrder(t *testing.T) {
	endpoint, err := paymentProtocolEndpoint(
		"https://provider.example.com/", "/api/v1/order/cancel-transaction",
	)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "https://provider.example.com/api/v1/order/cancel-transaction" {
		t.Fatalf("unexpected payment info endpoint: %s", endpoint)
	}
	if strings.Contains(endpoint, "create-transaction") || strings.HasSuffix(endpoint, "/") {
		t.Fatalf("unsafe payment config check endpoint: %s", endpoint)
	}
}

func TestPaymentConfigVerifiedRequiresMatchingCiphertextAndTimestamp(t *testing.T) {
	ciphertext := []byte("encrypted-payment-config")
	sum := sha256.Sum256(ciphertext)
	hash := hex.EncodeToString(sum[:])
	verifiedAt := sql.NullTime{Time: time.Now(), Valid: true}

	if !paymentConfigVerified(ciphertext, hash, verifiedAt) {
		t.Fatal("matching verified payment config was rejected")
	}
	if paymentConfigVerified([]byte("changed"), hash, verifiedAt) {
		t.Fatal("changed payment config retained verification")
	}
	if paymentConfigVerified(ciphertext, hash, sql.NullTime{}) {
		t.Fatal("payment config without verification timestamp was accepted")
	}
}

func TestAdminPaymentTimeoutAndFiatScaleBoundaries(t *testing.T) {
	config := validAdminPaymentConfig("http://bepusdt:8080")
	for _, timeout := range []int{180, 3600} {
		config.TimeoutSeconds = timeout
		if err := paymentconfig.Validate(config); err != nil {
			t.Fatalf("valid timeout %d rejected: %v", timeout, err)
		}
	}
	for _, timeout := range []int{179, 3601} {
		config.TimeoutSeconds = timeout
		if err := paymentconfig.Validate(config); err == nil {
			t.Fatalf("invalid timeout %d accepted", timeout)
		}
	}

	channel := paymentChannelUpdateRequest{
		Name: "BEpusdt", Fiat: "CNY", CurrencyScale: 2,
		MinAmountMinor: 100, MaxAmountMinor: 100000, Status: 0,
	}
	if err := channel.validate(); err != nil {
		t.Fatalf("CNY scale 2 rejected: %v", err)
	}
	channel.CurrencyScale = 0
	if err := channel.validate(); err == nil {
		t.Fatal("CNY channel with scale 0 was accepted")
	}

	product := rechargeProductWriteRequest{
		Name: "100 元", FiatCurrency: "CNY", CurrencyScale: 2,
		AmountMinor: 10000, CoinAmount: 100, Status: 1,
	}
	if err := product.validate(); err != nil {
		t.Fatalf("CNY product scale 2 rejected: %v", err)
	}
	product.CurrencyScale = 6
	if err := product.validate(); err == nil {
		t.Fatal("CNY product with scale 6 was accepted")
	}
}
