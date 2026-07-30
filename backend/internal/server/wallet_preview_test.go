package server

import (
	"strconv"
	"testing"
)

func TestRechargeCatalogWithPreview(t *testing.T) {
	payList, rules := rechargeCatalogWithPreview(nil, nil)

	if len(payList) != len(rechargePreviewPayments) {
		t.Fatalf(
			"preview payment methods = %d, want %d",
			len(payList),
			len(rechargePreviewPayments),
		)
	}
	for index, id := range []string{"ali", "wx", "paypal"} {
		paymentMethod := payList[index]
		if paymentMethod["id"] != id ||
			paymentMethod["available"] != false ||
			paymentMethod["preview"] != true ||
			paymentMethod["status_text"] != "待接入" {
			t.Fatalf("unexpected preview payment method %d: %#v", index, paymentMethod)
		}
	}
	paymentMethod := payList[len(payList)-1]
	if paymentMethod["id"] != "usdt" ||
		paymentMethod["provider"] != "bepusdt" ||
		paymentMethod["trade_type"] != "usdt.trc20" ||
		paymentMethod["network"] != "tron" ||
		paymentMethod["available"] != false ||
		paymentMethod["preview"] != true ||
		paymentMethod["status_text"] != "配置中" {
		t.Fatalf("unexpected preview payment method: %#v", paymentMethod)
	}

	if len(rules) != len(rechargePreviewAmounts) {
		t.Fatalf("preview rules = %d, want %d", len(rules), len(rechargePreviewAmounts))
	}
	for index, amount := range rechargePreviewAmounts {
		rule := rules[index]
		if rule["id"] != "preview-"+strconv.FormatInt(amount, 10) ||
			rule["coin"] != amount ||
			rule["money_minor"] != amount*100 ||
			rule["money"] != formatMinorAmount(amount*100, 2) ||
			rule["currency_scale"] != 2 ||
			rule["give"] != int64(0) ||
			rule["available"] != false ||
			rule["preview"] != true ||
			rule["status_text"] != "配置中" {
			t.Fatalf("unexpected preview rule %d: %#v", index, rule)
		}
	}
}

func TestRechargeCatalogWithPreviewKeepsRealCatalog(t *testing.T) {
	realPayList := []map[string]any{{
		"id": "usdt", "name": "已启用 USDT",
	}}
	realRules := []map[string]any{{
		"id": int64(42), "coin": int64(500), "money": "500.00",
	}}

	payList, rules := rechargeCatalogWithPreview(realPayList, realRules)

	if len(payList) != 1 || payList[0]["id"] != "usdt" ||
		payList[0]["name"] != "已启用 USDT" {
		t.Fatalf("real payment method was replaced: %#v", payList)
	}
	if _, exists := payList[0]["preview"]; exists {
		t.Fatalf("real payment method was marked as preview: %#v", payList[0])
	}
	if len(rules) != 1 || rules[0]["id"] != int64(42) ||
		rules[0]["money"] != "500.00" {
		t.Fatalf("real recharge rule was replaced: %#v", rules)
	}
	if _, exists := rules[0]["preview"]; exists {
		t.Fatalf("real recharge rule was marked as preview: %#v", rules[0])
	}
}

func TestRechargeCatalogWithPreviewOnlyFillsMissingSide(t *testing.T) {
	realPayList := []map[string]any{{"id": "usdt", "name": "已启用 USDT"}}
	payList, rules := rechargeCatalogWithPreview(realPayList, nil)
	if len(payList) != 1 || payList[0]["name"] != "已启用 USDT" {
		t.Fatalf("real payment method was replaced: %#v", payList)
	}
	if len(rules) != len(rechargePreviewAmounts) || rules[0]["preview"] != true {
		t.Fatalf("missing rules did not receive previews: %#v", rules)
	}

	realRules := []map[string]any{{"id": int64(7), "coin": int64(100)}}
	payList, rules = rechargeCatalogWithPreview(nil, realRules)
	if len(payList) != len(rechargePreviewPayments) || payList[0]["preview"] != true {
		t.Fatalf("missing payment method did not receive preview: %#v", payList)
	}
	if len(rules) != 1 || rules[0]["id"] != int64(7) {
		t.Fatalf("real rules were replaced: %#v", rules)
	}
}
