package im

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestWebSocketDeliveryIntegration(t *testing.T) {
	dsn := os.Getenv("CLAW_TEST_MYSQL_DSN")
	redisAddress := os.Getenv("CLAW_TEST_REDIS_ADDR")
	if dsn == "" || redisAddress == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN and CLAW_TEST_REDIS_ADDR are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddress, DB: 13,
		MaintNotificationsConfig: &maintnotifications.Config{Mode: maintnotifications.ModeDisabled},
	})
	defer redisClient.Close()
	if err = redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}

	userA := time.Now().UnixNano() & 0x3fffffffffffffff
	userB := userA + 1
	tokenA := "im-http-token-a"
	tokenB := "im-http-token-b"
	sessionA, _ := idgen.New()
	sessionB, _ := idgen.New()
	sumA := sha256.Sum256([]byte(tokenA))
	sumB := sha256.Sum256([]byte(tokenB))
	if _, err = db.ExecContext(ctx, `
		INSERT INTO users(id,username,password_hash,nickname,status)
		VALUES(?,?,'test','用户 A',1),(?,?,'test','用户 B',1)`,
		userA, "ws_a_"+time.Now().Format("150405.000000"),
		userB, "ws_b_"+time.Now().Format("150405.000000"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		INSERT INTO user_sessions(id,user_id,token_hash,expires_at)
		VALUES(?,?,?,CURRENT_TIMESTAMP(3)+INTERVAL 1 HOUR),
		      (?,?,?,CURRENT_TIMESTAMP(3)+INTERVAL 1 HOUR)`,
		sessionA, userA, hex.EncodeToString(sumA[:]),
		sessionB, userB, hex.EncodeToString(sumB[:]),
	); err != nil {
		t.Fatal(err)
	}
	service := New(db, redisClient)
	conversation, err := service.DirectConversation(ctx, userA, userB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.ExecContext(context.Background(), "DELETE FROM outbox_events WHERE aggregate_id=?", conversation.ID)              //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM im_messages WHERE conversation_id=?", conversation.ID)             //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM im_conversation_members WHERE conversation_id=?", conversation.ID) //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM im_conversations WHERE id=?", conversation.ID)                     //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM user_sessions WHERE user_id IN (?,?)", userA, userB)               //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM users WHERE id IN (?,?)", userA, userB)                            //nolint:errcheck
	}()

	authService := auth.New(db)
	handler := NewHandler(service, authService, redisClient)
	mux := http.NewServeMux()
	handler.Register(mux)
	server := httptest.NewServer(httpx.RequestContext(mux))
	defer server.Close()
	socketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/im"
	connection, _, err := websocket.DefaultDialer.Dial(socketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err = connection.WriteJSON(map[string]any{"type": "auth", "uid": userB, "token": tokenB}); err != nil {
		t.Fatal(err)
	}
	var ready map[string]any
	if err = connection.ReadJSON(&ready); err != nil || ready["type"] != "ready" {
		t.Fatalf("websocket did not authenticate: %#v %v", ready, err)
	}

	body, _ := json.Marshal(map[string]any{
		"client_message_id": "http-to-ws-1", "message_type": 1, "text_content": "实时消息",
	})
	request, _ := http.NewRequestWithContext(
		ctx, http.MethodPost,
		server.URL+"/api/v2/im/conversations/"+conversation.ID+"/messages",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", strconv.FormatInt(userA, 10))
	request.Header.Set("Authorization", "Bearer "+tokenA)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("send message returned %d", response.StatusCode)
	}
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	var pushed struct {
		Type string  `json:"type"`
		Data Message `json:"data"`
	}
	if err = connection.ReadJSON(&pushed); err != nil {
		t.Fatal(err)
	}
	if pushed.Type != "message" || pushed.Data.TextContent != "实时消息" ||
		pushed.Data.ConversationID != conversation.ID {
		t.Fatalf("unexpected websocket message: %#v", pushed)
	}

	senderConnection, _, err := websocket.DefaultDialer.Dial(socketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer senderConnection.Close()
	if err = senderConnection.WriteJSON(map[string]any{
		"type": "auth", "uid": userA, "token": tokenA,
	}); err != nil {
		t.Fatal(err)
	}
	ready = nil
	if err = senderConnection.ReadJSON(&ready); err != nil || ready["type"] != "ready" {
		t.Fatalf("sender websocket did not authenticate: %#v %v", ready, err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE user_sessions SET revoked_at=CURRENT_TIMESTAMP(3) WHERE id=?`,
		sessionA,
	); err != nil {
		t.Fatal(err)
	}
	blockedClientID := "revoked-ws-send-" + strconv.FormatInt(userA, 10)
	if err = senderConnection.WriteJSON(map[string]any{
		"type": "send", "conversation_id": conversation.ID,
		"client_message_id": blockedClientID, "message_type": 1,
		"text_content": "不应发送",
	}); err != nil {
		t.Fatal(err)
	}
	assertInvalidSocketSession(t, senderConnection)
	var blockedMessageCount int
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM im_messages
		WHERE sender_user_id=? AND client_message_id=?`,
		userA, blockedClientID,
	).Scan(&blockedMessageCount); err != nil {
		t.Fatal(err)
	}
	if blockedMessageCount != 0 {
		t.Fatalf("revoked websocket session inserted %d messages", blockedMessageCount)
	}

	if _, err = db.ExecContext(ctx, "UPDATE users SET status=2 WHERE id=?", userB); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Send(ctx, SendRequest{
		ConversationID:  conversation.ID,
		ClientMessageID: "disabled-recipient-" + strconv.FormatInt(userA, 10),
		SenderUserID:    userA,
		MessageType:     1,
		TextContent:     "不应推送",
	}); err != nil {
		t.Fatal(err)
	}
	assertInvalidSocketSession(t, connection)
}

func assertInvalidSocketSession(t *testing.T, connection *websocket.Conn) {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	var invalid struct {
		Type    string `json:"type"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := connection.ReadJSON(&invalid); err != nil {
		t.Fatalf("websocket did not report the invalid session: %v", err)
	}
	if invalid.Type != "error" || invalid.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected invalid-session response: %#v", invalid)
	}
	var afterClose map[string]any
	err := connection.ReadJSON(&afterClose)
	if err == nil {
		t.Fatalf("invalid websocket session remained open: %#v", afterClose)
	}
	var closeError *websocket.CloseError
	if !errors.As(err, &closeError) || closeError.Code != websocket.ClosePolicyViolation {
		t.Fatalf("invalid websocket session closed unexpectedly: %v", err)
	}
}
