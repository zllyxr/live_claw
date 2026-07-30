package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/paymentconfig"
)

func TestBEpusdtCreateTransactionSignsAndRewritesCheckoutURL(t *testing.T) {
	const token = "provider-secret-token"
	var received map[string]any
	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/order/create-transaction" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		if r.Host != "payments.example.com" ||
			r.Header.Get("X-Forwarded-Host") != "payments.example.com" ||
			r.Header.Get("X-Forwarded-Proto") != "https" {
			http.Error(w, "unexpected forwarding headers", http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&received); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if _, _, err := verifyFields(received, token); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status_code": 200,
			"message":     "success",
			"data": map[string]any{
				"trade_id": "provider-trade-1", "order_id": "1234567890123456789",
				"fiat": "CNY", "trade_type": "usdt.trc20",
				"amount": "28.88", "actual_amount": "4.25",
				"token": "TRx-payment-address", "status": 1,
				"expiration_time": 1200,
				"payment_url":     provider.URL + "/pay/checkout-counter/provider-trade-1",
			},
		})
	}))
	defer provider.Close()

	client := &bepusdtClient{httpClient: &http.Client{Timeout: time.Second}}
	order, err := client.createTransaction(context.Background(), paymentconfig.ChannelConfig{
		APIBaseURL: provider.URL, PublicBaseURL: "https://payments.example.com",
		APIToken: token, TradeType: "usdt.trc20", Fiat: "CNY", TimeoutSeconds: 1200,
	}, providerCreateRequest{
		OrderNo: "1234567890123456789", Amount: "28.88", Fiat: "CNY",
		TradeType: "usdt.trc20", Name: "28元充值",
		NotifyURL:      "https://app.example.com/api/v2/payments/bepusdt/notify",
		RedirectURL:    "https://app.example.com/h5/#/wallet",
		TimeoutSeconds: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.PaymentURL != "https://payments.example.com/pay/checkout-counter/provider-trade-1" {
		t.Fatalf("payment URL = %q", order.PaymentURL)
	}
	if received["order_id"] != "1234567890123456789" {
		t.Fatalf("19-digit order ID changed type/value: %#v", received["order_id"])
	}
}

func TestBEpusdtProviderErrorsNeverExposeAPIToken(t *testing.T) {
	const token = "never-print-this-token"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, token, http.StatusInternalServerError)
	}))
	defer provider.Close()
	client := &bepusdtClient{httpClient: provider.Client()}
	_, err := client.createTransaction(context.Background(), paymentconfig.ChannelConfig{
		APIBaseURL: provider.URL, PublicBaseURL: "https://payments.example.com",
		APIToken: token, TradeType: "usdt.trc20", Fiat: "CNY", TimeoutSeconds: 1200,
	}, providerCreateRequest{
		OrderNo: "1234567890123456789", Amount: "28.88", Fiat: "CNY",
		TradeType: "usdt.trc20", Name: "充值",
		NotifyURL:   "https://app.example.com/notify",
		RedirectURL: "https://app.example.com/success", TimeoutSeconds: 1200,
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("provider error leaked API token: %v", err)
	}
}

func TestBEpusdtCreateTransactionRejectsMismatchedTradeType(t *testing.T) {
	var provider *httptest.Server
	provider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status_code": 200,
			"message":     "success",
			"data": map[string]any{
				"trade_id": "provider-trade-wrong-chain",
				"order_id": "1234567890123456789",
				"fiat":     "CNY", "trade_type": "usdt.erc20",
				"amount": "28.88", "actual_amount": "4.25",
				"token": "0x-payment-address", "status": 1,
				"expiration_time": 1200,
				"payment_url":     provider.URL + "/pay/checkout-counter/wrong-chain",
			},
		})
	}))
	defer provider.Close()

	client := &bepusdtClient{httpClient: provider.Client()}
	_, err := client.createTransaction(context.Background(), paymentconfig.ChannelConfig{
		APIBaseURL: provider.URL, PublicBaseURL: "https://payments.example.com",
		APIToken: "provider-secret-token", TradeType: "usdt.trc20",
		Fiat: "CNY", TimeoutSeconds: 1200,
	}, providerCreateRequest{
		OrderNo: "1234567890123456789", Amount: "28.88", Fiat: "CNY",
		TradeType: "usdt.trc20", Name: "充值",
		NotifyURL:   "https://app.example.com/notify",
		RedirectURL: "https://app.example.com/success", TimeoutSeconds: 1200,
	})
	if !errors.Is(err, ErrProvider) {
		t.Fatalf("trade-type mismatch error = %v", err)
	}
}
