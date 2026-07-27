package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeChatContent(t *testing.T) {
	content, err := normalizeChatContent("  正常词和违禁词  ", []string{"违禁"})
	if err != nil {
		t.Fatal(err)
	}
	if content != "正常词和**词" {
		t.Fatalf("unexpected sanitized content %q", content)
	}
	if _, err := normalizeChatContent(strings.Repeat("字", 201), nil); err == nil {
		t.Fatal("expected oversized chat message to be rejected")
	}
}

func TestFirstLiveMessageRejectsMultipleMessages(t *testing.T) {
	_, err := firstLiveMessage(map[string]any{
		"msg": []any{
			map[string]any{"_method_": "SendMsg"},
			map[string]any{"_method_": "KickUser"},
		},
	})
	if err == nil {
		t.Fatal("expected a batched live message to be rejected")
	}
}

func TestSendLiveCustomMessageUsesServerBotAndOnlineCustomPayload(t *testing.T) {
	var sent map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/get_admin_token":
			_, _ = io.WriteString(w, `{"errCode":0,"errMsg":"","data":{"token":"admin-token","expireTimeSeconds":3600}}`)
		case "/user/get_users_info":
			_, _ = io.WriteString(w, `{"errCode":0,"errMsg":"","data":{"usersInfo":[{"userID":"claw_live_bot"}]}}`)
		case "/msg/send_msg":
			if r.Header.Get("token") != "admin-token" {
				t.Errorf("missing admin token")
			}
			if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
				t.Errorf("decode request: %v", err)
			}
			_, _ = io.WriteString(w, `{"errCode":0,"errMsg":"","data":{"serverMsgID":"server-1"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewOpenIMClient(Config{
		OpenIMAPIURL:      server.URL,
		OpenIMSecret:      "secret",
		OpenIMAdminUserID: "imAdmin",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	messageID, err := client.SendLiveCustomMessage(context.Background(), "group-1", map[string]any{"event_id": "event-1"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if messageID != "server-1" {
		t.Fatalf("unexpected server message ID %q", messageID)
	}
	if sent["sendID"] != "claw_live_bot" || sent["recvID"] != "group-1" || sent["groupID"] != "group-1" {
		t.Fatalf("unexpected sender/receiver: %#v", sent)
	}
	if sent["contentType"] != float64(110) || sent["sessionType"] != float64(3) {
		t.Fatalf("unexpected message types: %#v", sent)
	}
	if sent["isOnlineOnly"] != true || sent["notOfflinePush"] != true {
		t.Fatalf("live message must be online-only without push: %#v", sent)
	}
	content, ok := sent["content"].(map[string]any)
	if !ok || !strings.Contains(liveText(content["data"]), `"event_id":"event-1"`) {
		t.Fatalf("unexpected custom content: %#v", sent["content"])
	}
}
