package support

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestSupportConversationIntegration(t *testing.T) {
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
	suffix, err := idgen.New()
	if err != nil {
		t.Fatal(err)
	}
	username := "support_" + suffix[len(suffix)-10:]
	result, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,nickname,status)
		VALUES(?,'integration-test-only','客服联调用户',1)`,
		username,
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := result.LastInsertId()
	var conversationID string
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if conversationID != "" {
			_, _ = db.ExecContext(cleanupContext, "DELETE FROM support_messages WHERE conversation_id=?", conversationID)
			_, _ = db.ExecContext(cleanupContext, "DELETE FROM support_conversations WHERE id=?", conversationID)
		}
		_, _ = db.ExecContext(cleanupContext, "DELETE FROM users WHERE id=?", userID)
	})

	service := New(db)
	conversation, err := service.Current(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	conversationID = conversation.ID
	repeated, err := service.Current(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.ID != conversation.ID {
		t.Fatalf("current conversation is not stable: %q != %q", repeated.ID, conversation.ID)
	}
	clientMessageID := "support-test-" + suffix
	message, err := service.Send(ctx, SendRequest{
		ConversationID: conversation.ID, UserID: userID,
		ClientMessageID: clientMessageID, MessageType: 1, TextContent: "需要人工协助",
	})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.Send(ctx, SendRequest{
		ConversationID: conversation.ID, UserID: userID,
		ClientMessageID: clientMessageID, MessageType: 1, TextContent: "需要人工协助",
	})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ID != message.ID {
		t.Fatalf("message idempotency failed: %q != %q", duplicate.ID, message.ID)
	}
	messages, err := service.Messages(ctx, userID, conversation.ID, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].TextContent != "需要人工协助" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
}
