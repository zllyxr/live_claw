package im

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
)

func TestLiveIMSmoke(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("CLAW_LIVE_IM_SMOKE_URL"), "/")
	dsn := os.Getenv("CLAW_LIVE_IM_SMOKE_DSN")
	if baseURL == "" || dsn == "" {
		t.Skip("CLAW_LIVE_IM_SMOKE_URL and CLAW_LIVE_IM_SMOKE_DSN are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	userA := int64(8_000_000_000) + time.Now().UnixMilli()%1_000_000_000
	userB := userA + 1
	password := "IM-Smoke-2026!"
	passwordHash, err := adminauth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		INSERT INTO users
			(id,username,country_code,password_hash,password_algo,nickname,status)
		VALUES(?,?,'86',?,'argon2id','IM Smoke A',1),
		      (?,?,'86',?,'argon2id','IM Smoke B',1)`,
		userA, "im_smoke_a_"+suffix, passwordHash,
		userB, "im_smoke_b_"+suffix, passwordHash,
	); err != nil {
		t.Fatal(err)
	}
	var conversations []string
	defer cleanupLiveSmoke(db, conversations, userA, userB)

	tokenA := liveSmokeLogin(t, ctx, baseURL, "im_smoke_a_"+suffix, password)
	tokenB := liveSmokeLogin(t, ctx, baseURL, "im_smoke_b_"+suffix, password)
	direct := liveSmokeRequest[Conversation](
		t, ctx, http.MethodPost, baseURL+"/api/v2/im/direct",
		userA, tokenA, map[string]any{"peer_user_id": userB},
	)
	conversations = append(conversations, direct.ID)
	socketA := liveSmokeSocket(t, baseURL, userA, tokenA)
	defer socketA.Close()
	socketB := liveSmokeSocket(t, baseURL, userB, tokenB)
	defer socketB.Close()

	directClientID := "smoke-direct-a-" + suffix
	if err = socketA.WriteJSON(map[string]any{
		"type": "send", "conversation_id": direct.ID,
		"client_message_id": directClientID, "message_type": 1,
		"text_content": "A 发出的单聊实时消息",
	}); err != nil {
		t.Fatal(err)
	}
	ack := liveSmokeReadUntil(t, socketA, 5*time.Second, func(value map[string]any) bool {
		return value["type"] == "ack" && value["client_message_id"] == directClientID
	})
	if number(ack["code"]) != 0 {
		t.Fatalf("direct websocket send was rejected: %#v", ack)
	}
	directPush := liveSmokeReadMessage(
		t, socketB, 5*time.Second, direct.ID, "A 发出的单聊实时消息",
	)
	directMessageID := fmt.Sprint(directPush["id"])
	liveSmokeAssertNoDuplicate(t, socketB, directMessageID, 1300*time.Millisecond)
	_ = socketB.Close()
	socketB = liveSmokeSocket(t, baseURL, userB, tokenB)
	defer socketB.Close()

	reply := liveSmokeRequest[Message](
		t, ctx, http.MethodPost,
		baseURL+"/api/v2/im/conversations/"+direct.ID+"/messages",
		userB, tokenB, map[string]any{
			"client_message_id": "smoke-direct-b-" + suffix,
			"message_type":      1, "text_content": "B 回复的单聊消息",
		},
	)
	liveSmokeReadMessage(t, socketA, 5*time.Second, direct.ID, reply.TextContent)
	history := liveSmokeRequest[struct {
		Items []Message `json:"items"`
	}](
		t, ctx, http.MethodGet,
		baseURL+"/api/v2/im/conversations/"+direct.ID+"/messages?limit=20",
		userB, tokenB, nil,
	)
	if len(history.Items) != 2 || history.Items[0].Sequence != 2 || history.Items[1].Sequence != 1 {
		t.Fatalf("unexpected direct history: %#v", history.Items)
	}

	group := liveSmokeRequest[Conversation](
		t, ctx, http.MethodPost, baseURL+"/api/v2/im/groups",
		userA, tokenA, map[string]any{
			"title": "IM 端到端测试群 " + suffix, "max_members": 20,
			"member_ids": []int64{userB},
		},
	)
	conversations = append(conversations, group.ID)
	members := liveSmokeRequest[struct {
		Items []map[string]any `json:"items"`
	}](
		t, ctx, http.MethodGet,
		baseURL+"/api/v2/im/groups/"+group.ID+"/members?limit=20",
		userA, tokenA, nil,
	)
	if len(members.Items) != 2 {
		t.Fatalf("group member count is %d, want 2", len(members.Items))
	}
	groupFirst := liveSmokeRequest[Message](
		t, ctx, http.MethodPost,
		baseURL+"/api/v2/im/conversations/"+group.ID+"/messages",
		userA, tokenA, map[string]any{
			"client_message_id": "smoke-group-a-" + suffix,
			"message_type":      1, "text_content": "群主发送的群消息",
		},
	)
	liveSmokeReadMessage(t, socketB, 5*time.Second, group.ID, groupFirst.TextContent)

	groupClientID := "smoke-group-b-" + suffix
	if err = socketB.WriteJSON(map[string]any{
		"type": "send", "conversation_id": group.ID,
		"client_message_id": groupClientID, "message_type": 1,
		"text_content": "群成员回复的群消息",
	}); err != nil {
		t.Fatal(err)
	}
	groupAck := liveSmokeReadUntil(t, socketB, 5*time.Second, func(value map[string]any) bool {
		return value["type"] == "ack" && value["client_message_id"] == groupClientID
	})
	if number(groupAck["code"]) != 0 {
		t.Fatalf("group websocket send was rejected: %#v", groupAck)
	}
	liveSmokeReadMessage(t, socketA, 5*time.Second, group.ID, "群成员回复的群消息")

	groupHistory := liveSmokeRequest[struct {
		Items []Message `json:"items"`
	}](
		t, ctx, http.MethodGet,
		baseURL+"/api/v2/im/conversations/"+group.ID+"/messages?limit=20",
		userA, tokenA, nil,
	)
	if len(groupHistory.Items) != 2 {
		t.Fatalf("group history contains %d messages, want 2", len(groupHistory.Items))
	}
	liveSmokeRequest[map[string]bool](
		t, ctx, http.MethodPost,
		baseURL+"/api/v2/im/conversations/"+group.ID+"/read",
		userB, tokenB, map[string]any{"sequence": 0},
	)
	conversationList := liveSmokeRequest[struct {
		Items []map[string]any `json:"items"`
	}](
		t, ctx, http.MethodGet, baseURL+"/api/v2/im/conversations",
		userB, tokenB, nil,
	)
	if len(conversationList.Items) != 2 {
		t.Fatalf("user B has %d conversations, want 2", len(conversationList.Items))
	}

	var messageCount, groupMemberCount int
	if err = db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM im_messages WHERE conversation_id IN (?,?)),
			(SELECT COUNT(*) FROM im_conversation_members
			 WHERE conversation_id=? AND member_status=1)`,
		direct.ID, group.ID, group.ID,
	).Scan(&messageCount, &groupMemberCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 4 || groupMemberCount != 2 {
		t.Fatalf(
			"database verification failed: messages=%d group_members=%d",
			messageCount, groupMemberCount,
		)
	}
	t.Logf(
		"live IM passed: users=%d/%d direct=%s group=%s messages=%d members=%d",
		userA, userB, direct.ID, group.ID, messageCount, groupMemberCount,
	)
}

func liveSmokeLogin(
	t *testing.T,
	ctx context.Context,
	baseURL, username, password string,
) string {
	t.Helper()
	form := url.Values{
		"service": {"Login.userLogin"}, "country_code": {"86"},
		"user_login": {username}, "user_pass": {password},
		"device_id": {"im-smoke"}, "source": {"test"},
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/appapi/", strings.NewReader(form.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Data struct {
			Code int              `json:"code"`
			Msg  string           `json:"msg"`
			Info []map[string]any `json:"info"`
		} `json:"data"`
	}
	if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || envelope.Data.Code != 0 ||
		len(envelope.Data.Info) != 1 {
		t.Fatalf("login failed: status=%d payload=%#v", response.StatusCode, envelope)
	}
	token := fmt.Sprint(envelope.Data.Info[0]["token"])
	if token == "" {
		t.Fatal("login returned an empty token")
	}
	return token
}

func liveSmokeRequest[T any](
	t *testing.T,
	ctx context.Context,
	method, endpoint string,
	userID int64,
	token string,
	payload any,
) T {
	t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-User-ID", strconv.FormatInt(userID, 10))
	request.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || envelope.Code != 0 {
		t.Fatalf(
			"%s %s failed: status=%d code=%d message=%q",
			method, endpoint, response.StatusCode, envelope.Code, envelope.Message,
		)
	}
	var result T
	if err = json.Unmarshal(envelope.Data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func liveSmokeSocket(
	t *testing.T,
	baseURL string,
	userID int64,
	token string,
) *websocket.Conn {
	t.Helper()
	socketURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/ws/im"
	connection, _, err := websocket.DefaultDialer.Dial(socketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = connection.WriteJSON(map[string]any{
		"type": "auth", "uid": userID, "token": token,
	}); err != nil {
		connection.Close()
		t.Fatal(err)
	}
	ready := liveSmokeReadUntil(t, connection, 5*time.Second, func(value map[string]any) bool {
		return value["type"] == "ready"
	})
	if number(ready["user_id"]) != userID {
		connection.Close()
		t.Fatalf("websocket authenticated as the wrong user: %#v", ready)
	}
	return connection
}

func liveSmokeReadMessage(
	t *testing.T,
	connection *websocket.Conn,
	timeout time.Duration,
	conversationID string,
	text string,
) map[string]any {
	t.Helper()
	envelope := liveSmokeReadUntil(t, connection, timeout, func(value map[string]any) bool {
		if value["type"] != "message" {
			return false
		}
		message, _ := value["data"].(map[string]any)
		return fmt.Sprint(message["conversation_id"]) == conversationID &&
			fmt.Sprint(message["text_content"]) == text
	})
	message, _ := envelope["data"].(map[string]any)
	return message
}

func liveSmokeReadUntil(
	t *testing.T,
	connection *websocket.Conn,
	timeout time.Duration,
	match func(map[string]any) bool,
) map[string]any {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(timeout))
	for {
		var value map[string]any
		if err := connection.ReadJSON(&value); err != nil {
			t.Fatal(err)
		}
		if match(value) {
			return value
		}
	}
}

func liveSmokeAssertNoDuplicate(
	t *testing.T,
	connection *websocket.Conn,
	messageID string,
	wait time.Duration,
) {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(wait))
	for {
		var value map[string]any
		err := connection.ReadJSON(&value)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				t.Fatal(err)
			}
			return
		}
		if value["type"] != "message" {
			continue
		}
		message, _ := value["data"].(map[string]any)
		if fmt.Sprint(message["id"]) == messageID {
			t.Fatalf("message %s was pushed more than once", messageID)
		}
	}
}

func cleanupLiveSmoke(db *sql.DB, conversations []string, userA, userB int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, conversationID := range conversations {
		_, _ = db.ExecContext(ctx, "DELETE FROM outbox_events WHERE aggregate_id=?", conversationID)
		_, _ = db.ExecContext(ctx, "DELETE FROM im_messages WHERE conversation_id=?", conversationID)
		_, _ = db.ExecContext(ctx, "DELETE FROM im_group_applications WHERE conversation_id=?", conversationID)
		_, _ = db.ExecContext(ctx, "DELETE FROM im_moderation_actions WHERE conversation_id=?", conversationID)
		_, _ = db.ExecContext(ctx, "DELETE FROM im_groups WHERE conversation_id=?", conversationID)
		_, _ = db.ExecContext(ctx, "DELETE FROM im_conversation_members WHERE conversation_id=?", conversationID)
		_, _ = db.ExecContext(ctx, "DELETE FROM im_conversations WHERE id=?", conversationID)
	}
	_, _ = db.ExecContext(ctx, "DELETE FROM user_sessions WHERE user_id IN (?,?)", userA, userB)
	_, _ = db.ExecContext(ctx, "DELETE FROM wallet_accounts WHERE user_id IN (?,?)", userA, userB)
	_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id IN (?,?)", userA, userB)
}

func number(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case json.Number:
		result, _ := typed.Int64()
		return result
	case string:
		result, _ := strconv.ParseInt(typed, 10, 64)
		return result
	default:
		return 0
	}
}
