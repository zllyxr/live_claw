package main

import (
	"encoding/base64"
	"net/url"
	"testing"
)

func TestExpandOpenIMWebSocketQuery(t *testing.T) {
	payload := `{"userID":"10000","token":"signed-token","platformID":5,"operationID":"op-1","background":false,"sendResponse":true,"sdkType":"js"}`
	packed := base64.RawURLEncoding.EncodeToString([]byte(payload))
	requestURL, err := url.Parse("/v1/im/ws?v=" + url.QueryEscape(packed))
	if err != nil {
		t.Fatal(err)
	}
	if err := expandOpenIMWebSocketQuery(requestURL); err != nil {
		t.Fatal(err)
	}
	query := requestURL.Query()
	if query.Get("v") != "" || query.Get("sendID") != "10000" || query.Get("token") != "signed-token" || query.Get("platformID") != "5" {
		t.Fatalf("unexpected expanded query: %s", requestURL.RawQuery)
	}
}

func TestExpandOpenIMWebSocketQueryRejectsIncompletePayload(t *testing.T) {
	packed := base64.RawURLEncoding.EncodeToString([]byte(`{"userID":"10000"}`))
	requestURL := &url.URL{RawQuery: "v=" + packed}
	if err := expandOpenIMWebSocketQuery(requestURL); err == nil {
		t.Fatal("expected incomplete payload to be rejected")
	}
}
