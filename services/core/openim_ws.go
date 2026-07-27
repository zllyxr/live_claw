package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

type openIMWebSocketParams struct {
	UserID       string `json:"userID"`
	Token        string `json:"token"`
	PlatformID   int    `json:"platformID"`
	OperationID  string `json:"operationID"`
	Background   bool   `json:"background"`
	SendResponse bool   `json:"sendResponse"`
	SDKType      string `json:"sdkType"`
}

func newOpenIMWebSocketProxy(upstream string, logger *slog.Logger) http.Handler {
	target, err := url.Parse(upstream)
	if err != nil || target.Host == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "OpenIM gateway is not configured", http.StatusServiceUnavailable)
		})
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.URL.Path = "/"
		r.URL.RawPath = ""
		r.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		logger.Error("openim websocket proxy", "path", r.URL.Path, "error", proxyErr)
		http.Error(w, "OpenIM gateway unavailable", http.StatusBadGateway)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "WebSocket upgrade required", http.StatusUpgradeRequired)
			return
		}
		if err := expandOpenIMWebSocketQuery(r.URL); err != nil {
			http.Error(w, "Invalid OpenIM handshake", http.StatusBadRequest)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

func expandOpenIMWebSocketQuery(requestURL *url.URL) error {
	query := requestURL.Query()
	packed := strings.TrimSpace(query.Get("v"))
	if packed == "" {
		if query.Get("sendID") == "" || query.Get("token") == "" || query.Get("platformID") == "" {
			return errors.New("missing handshake parameters")
		}
		return nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(packed)
	if err != nil {
		return err
	}
	var params openIMWebSocketParams
	if err := json.Unmarshal(payload, &params); err != nil {
		return err
	}
	if strings.TrimSpace(params.UserID) == "" || strings.TrimSpace(params.Token) == "" || params.PlatformID < 1 {
		return errors.New("incomplete handshake parameters")
	}

	query.Del("v")
	query.Set("sendID", params.UserID)
	query.Set("token", params.Token)
	query.Set("platformID", strconv.Itoa(params.PlatformID))
	if params.OperationID != "" {
		query.Set("operationID", params.OperationID)
	}
	query.Set("background", strconv.FormatBool(params.Background))
	query.Set("sendResponse", strconv.FormatBool(params.SendResponse))
	if params.SDKType != "" {
		query.Set("sdkType", params.SDKType)
	}
	requestURL.RawQuery = query.Encode()
	return nil
}
