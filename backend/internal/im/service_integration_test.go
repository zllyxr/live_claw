package im

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestDirectAndGroupMessagingIntegration(t *testing.T) {
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
		Addr: redisAddress, DB: 14,
		MaintNotificationsConfig: &maintnotifications.Config{Mode: maintnotifications.ModeDisabled},
	})
	defer redisClient.Close()
	userA := time.Now().UnixNano() & 0x3fffffffffffffff
	userB := userA + 1
	if _, err = db.ExecContext(ctx, `
		INSERT INTO users(id,username,password_hash,nickname,status)
		VALUES(?,?,'test','用户 A',1),(?,?,'test','用户 B',1)`,
		userA, "im_a_"+time.Now().Format("150405.000000"),
		userB, "im_b_"+time.Now().Format("150405.000000"),
	); err != nil {
		t.Fatal(err)
	}
	var conversations []string
	defer func() {
		for _, conversationID := range conversations {
			db.ExecContext(context.Background(), "DELETE FROM outbox_events WHERE aggregate_id=?", conversationID)              //nolint:errcheck
			db.ExecContext(context.Background(), "DELETE FROM im_messages WHERE conversation_id=?", conversationID)             //nolint:errcheck
			db.ExecContext(context.Background(), "DELETE FROM im_group_applications WHERE conversation_id=?", conversationID)   //nolint:errcheck
			db.ExecContext(context.Background(), "DELETE FROM im_moderation_actions WHERE conversation_id=?", conversationID)   //nolint:errcheck
			db.ExecContext(context.Background(), "DELETE FROM im_groups WHERE conversation_id=?", conversationID)               //nolint:errcheck
			db.ExecContext(context.Background(), "DELETE FROM im_conversation_members WHERE conversation_id=?", conversationID) //nolint:errcheck
			db.ExecContext(context.Background(), "DELETE FROM im_conversations WHERE id=?", conversationID)                     //nolint:errcheck
		}
		db.ExecContext(context.Background(), "DELETE FROM user_blocks WHERE user_id IN (?,?) OR target_user_id IN (?,?)", userA, userB, userA, userB) //nolint:errcheck
		db.ExecContext(context.Background(), "DELETE FROM users WHERE id IN (?,?)", userA, userB)                                                     //nolint:errcheck
	}()

	service := New(db, redisClient)
	direct, err := service.DirectConversation(ctx, userA, userB)
	if err != nil {
		t.Fatal(err)
	}
	conversations = append(conversations, direct.ID)
	repeatedDirect, err := service.DirectConversation(ctx, userB, userA)
	if err != nil || repeatedDirect.ID != direct.ID {
		t.Fatalf("direct conversation was not idempotent: %#v %v", repeatedDirect, err)
	}
	message, err := service.Send(ctx, SendRequest{
		ConversationID: direct.ID, ClientMessageID: "client-1",
		SenderUserID: userA, MessageType: 1, TextContent: "你好",
	})
	if err != nil {
		t.Fatal(err)
	}
	repeatedMessage, err := service.Send(ctx, SendRequest{
		ConversationID: direct.ID, ClientMessageID: "client-1",
		SenderUserID: userA, MessageType: 1, TextContent: "你好",
	})
	if err != nil || repeatedMessage.ID != message.ID {
		t.Fatalf("message was not idempotent: %#v %v", repeatedMessage, err)
	}
	history, err := service.Messages(ctx, userB, direct.ID, 0, 30)
	if err != nil || len(history) != 1 || history[0].TextContent != "你好" {
		t.Fatalf("unexpected direct history: %#v %v", history, err)
	}

	group, err := service.CreateGroup(ctx, userA, "测试群", 20)
	if err != nil {
		t.Fatal(err)
	}
	conversations = append(conversations, group.ID)
	if err = service.AddGroupMember(ctx, userA, group.ID, userB); err != nil {
		t.Fatal(err)
	}
	if err = service.MuteMember(ctx, userA, group.ID, userB, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Send(ctx, SendRequest{
		ConversationID: group.ID, ClientMessageID: "muted-1",
		SenderUserID: userB, MessageType: 1, TextContent: "不应发送",
	}); !errors.Is(err, ErrMuted) {
		t.Fatalf("muted member could send: %v", err)
	}
	if err = service.MuteMember(ctx, userA, group.ID, userB, time.Time{}); err != nil {
		t.Fatal(err)
	}
	groupMessage, err := service.Send(ctx, SendRequest{
		ConversationID: group.ID, ClientMessageID: "group-1",
		SenderUserID: userB, MessageType: 1, TextContent: "解除禁言",
	})
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := service.Conversations(ctx, userB)
	if err != nil || len(inbox) != 2 {
		t.Fatalf("unexpected inbox: %#v %v", inbox, err)
	}
	if err = service.MarkRead(ctx, userB, group.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err = service.HideConversation(ctx, userB, direct.ID); err != nil {
		t.Fatal(err)
	}
	inbox, err = service.Conversations(ctx, userB)
	if err != nil || len(inbox) != 1 || inbox[0]["id"] != group.ID {
		t.Fatalf("conversation hide failed: %#v %v", inbox, err)
	}
	if err = service.UpdateGroup(ctx, userA, group.ID, "新群名", "群介绍", "群公告", 1); err != nil {
		t.Fatal(err)
	}
	if err = service.SetMemberRole(ctx, userA, group.ID, userB, 60); err != nil {
		t.Fatal(err)
	}
	if err = service.SetAllMuted(ctx, userA, group.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = service.SetMemberRole(ctx, userA, group.ID, userB, 10); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Send(ctx, SendRequest{
		ConversationID: group.ID, ClientMessageID: "all-muted",
		SenderUserID: userB, MessageType: 1, TextContent: "不应发送",
	}); !errors.Is(err, ErrMuted) {
		t.Fatalf("all-muted member could send: %v", err)
	}
	if err = service.SetAllMuted(ctx, userA, group.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = service.LeaveGroup(ctx, userB, group.ID); err != nil {
		t.Fatal(err)
	}
	groupAfterLeave, err := service.Group(ctx, userA, group.ID)
	if err != nil || groupAfterLeave["member_count"] != 1 {
		t.Fatalf("group member count was not committed after leave: %#v %v", groupAfterLeave, err)
	}
	application, err := service.JoinGroup(ctx, userB, group.ID, "申请重新加入")
	if err != nil || application["status"] != 0 {
		t.Fatalf("unexpected group application: %#v %v", application, err)
	}
	applications, err := service.GroupApplications(ctx, userA, 0, 20)
	if err != nil || len(applications) != 1 {
		t.Fatalf("unexpected applications: %#v %v", applications, err)
	}
	applicationID, _ := applications[0]["id"].(string)
	if err = service.HandleGroupApplication(ctx, userA, applicationID, true, "同意"); err != nil {
		t.Fatal(err)
	}
	members, err := service.GroupMembers(ctx, userA, group.ID, 0, 20)
	if err != nil || len(members) != 2 {
		t.Fatalf("unexpected members after approval: %#v %v", members, err)
	}
	if err = service.SetBlocked(ctx, userA, userB, true); err != nil {
		t.Fatal(err)
	}
	blocks, err := service.BlockList(ctx, userA, 0, 20)
	if err != nil || len(blocks) != 1 {
		t.Fatalf("unexpected block list: %#v %v", blocks, err)
	}
	if err = service.SetBlocked(ctx, userA, userB, false); err != nil {
		t.Fatal(err)
	}
	if err = service.RevokeMessage(ctx, userB, group.ID, groupMessage.ID); err != nil {
		t.Fatal(err)
	}
	var revokedStatus int
	if err = db.QueryRowContext(ctx, "SELECT status FROM im_messages WHERE id=?", groupMessage.ID).
		Scan(&revokedStatus); err != nil || revokedStatus != 2 {
		t.Fatalf("message revoke was not committed: status=%d error=%v", revokedStatus, err)
	}
}
