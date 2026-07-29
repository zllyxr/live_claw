package supportconsole

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/support"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestSupportConsoleClaimIsolationAndReplyIdempotencyIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	var (
		agentIDs       []int64
		userID         int64
		conversationID string
	)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if conversationID != "" {
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM support_conversation_reads WHERE conversation_id=?", conversationID)
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM support_messages WHERE conversation_id=?", conversationID)
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM support_conversations WHERE id=?", conversationID)
			_, _ = db.ExecContext(cleanupCtx, `
				DELETE FROM audit_logs
				WHERE resource_type='support_conversation' AND resource_id=?`,
				conversationID,
			)
		}
		if userID > 0 {
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM support_user_notes WHERE user_id=?", userID)
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM user_sessions WHERE user_id=?", userID)
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM users WHERE id=?", userID)
		}
		for _, agentID := range agentIDs {
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM audit_logs WHERE actor_type=1 AND actor_id=?", agentID)
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM admin_sessions WHERE admin_user_id=?", agentID)
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM admin_user_roles WHERE admin_user_id=?", agentID)
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM support_agents WHERE admin_user_id=?", agentID)
			_, _ = db.ExecContext(cleanupCtx,
				"DELETE FROM admin_users WHERE id=?", agentID)
		}
	})

	suffix := mustSupportConsoleTestID(t)
	agents := make([]Agent, 0, 2)
	for index := 1; index <= 2; index++ {
		username := "support_it_" + string(rune('a'+index-1)) + "_" + suffix[len(suffix)-10:]
		displayName := "集成测试座席" + string(rune('A'+index-1))
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO admin_users(username,password_hash,display_name,status)
			VALUES(?,'integration-test-only',?,1)`,
			username, displayName,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		agentID, insertErr := result.LastInsertId()
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		agentIDs = append(agentIDs, agentID)
		if _, insertErr = db.ExecContext(ctx, `
			INSERT INTO admin_user_roles(admin_user_id,role_id)
			SELECT ?,id FROM admin_roles WHERE role_key='support_agent' AND status=1`,
			agentID,
		); insertErr != nil {
			t.Fatal(insertErr)
		}
		agentNo := mustSupportConsoleTestID(t)
		if _, insertErr = db.ExecContext(ctx, `
			INSERT INTO support_agents(
				admin_user_id,agent_no,agent_role,status,presence,max_active,support_only
			) VALUES(?,?,1,1,1,8,1)`,
			agentID, agentNo,
		); insertErr != nil {
			t.Fatal(insertErr)
		}
		agents = append(agents, Agent{
			ID: agentID, AgentNo: agentNo, Username: username,
			DisplayName: displayName, Role: 1, RoleName: "客服座席",
			Presence: 1, MaxActive: 8, SupportOnly: true,
			IsSupervisor: false,
		})
	}

	userResult, err := db.ExecContext(ctx, `
		INSERT INTO users(username,password_hash,password_algo,nickname,status)
		VALUES(?,'integration-test-only','argon2id','客服座席联调用户',1)`,
		"support_console_user_"+suffix[len(suffix)-12:],
	)
	if err != nil {
		t.Fatal(err)
	}
	userID, err = userResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	supportService := support.New(db)
	service := NewService(db, supportService)
	conversation, err := supportService.Current(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	conversationID = conversation.ID
	if _, err = supportService.Send(ctx, support.SendRequest{
		ConversationID:  conversationID,
		UserID:          userID,
		ClientMessageID: "user_" + suffix,
		MessageType:     1,
		TextContent:     "需要人工协助",
	}); err != nil {
		t.Fatal(err)
	}
	queue, err := service.Conversations(ctx, agents[0], "queue", "")
	if err != nil {
		t.Fatal(err)
	}
	foundInQueue := false
	for _, item := range queue {
		if item.ID == conversationID {
			foundInQueue = true
			break
		}
	}
	if !foundInQueue {
		t.Fatalf("waiting conversation was not visible in the queue: %#v", queue)
	}

	type claimResult struct {
		agentIndex int
		err        error
	}
	claimRequestIDs := []string{
		mustSupportConsoleTestID(t),
		mustSupportConsoleTestID(t),
	}
	start := make(chan struct{})
	results := make(chan claimResult, len(agents))
	var waitGroup sync.WaitGroup
	for index := range agents {
		waitGroup.Add(1)
		go func(agentIndex int) {
			defer waitGroup.Done()
			<-start
			results <- claimResult{
				agentIndex: agentIndex,
				err: service.Claim(
					ctx, agents[agentIndex], conversationID,
					ActionMeta{RequestID: claimRequestIDs[agentIndex]},
				),
			}
		}(index)
	}
	close(start)
	waitGroup.Wait()
	close(results)

	winnerIndex := -1
	loserIndex := -1
	successCount := 0
	claimedCount := 0
	for result := range results {
		switch {
		case result.err == nil:
			successCount++
			winnerIndex = result.agentIndex
		case errors.Is(result.err, ErrConversationClaimed):
			claimedCount++
			loserIndex = result.agentIndex
		default:
			t.Fatalf("unexpected claim result for agent %d: %v", result.agentIndex, result.err)
		}
	}
	if successCount != 1 || claimedCount != 1 || winnerIndex < 0 || loserIndex < 0 {
		t.Fatalf(
			"concurrent claim must have one winner and one conflict: success=%d conflict=%d winner=%d loser=%d",
			successCount, claimedCount, winnerIndex, loserIndex,
		)
	}
	winner := agents[winnerIndex]
	loser := agents[loserIndex]
	if _, err = db.ExecContext(ctx,
		"UPDATE support_agents SET max_active=1 WHERE admin_user_id=?",
		winner.ID,
	); err != nil {
		t.Fatal(err)
	}
	if err = service.Claim(
		ctx, winner, conversationID,
		ActionMeta{RequestID: mustSupportConsoleTestID(t)},
	); err != nil {
		t.Fatalf("repeating an already successful claim must be idempotent: %v", err)
	}

	var storedStatus int
	var storedAgentID int64
	if err = db.QueryRowContext(ctx, `
		SELECT status,assigned_admin_id
		FROM support_conversations WHERE id=?`,
		conversationID,
	).Scan(&storedStatus, &storedAgentID); err != nil {
		t.Fatal(err)
	}
	if storedStatus != 1 || storedAgentID != winner.ID {
		t.Fatalf(
			"claim assignment was not persisted: status=%d assigned_agent_id=%d winner=%d",
			storedStatus, storedAgentID, winner.ID,
		)
	}

	if _, _, _, err = service.Conversation(ctx, loser, conversationID); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("non-owner conversation read must be denied, got %v", err)
	}
	if _, err = service.Messages(ctx, loser, conversationID, "", 20); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("non-owner message read must be denied, got %v", err)
	}
	if _, err = service.Send(
		ctx, loser, conversationID,
		SendRequest{
			ClientMessageID: "loser_" + suffix,
			MessageType:     1,
			TextContent:     "不应发送成功",
		},
		ActionMeta{RequestID: mustSupportConsoleTestID(t)},
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("non-owner reply must be denied, got %v", err)
	}

	clientMessageID := "agent_reply_" + suffix
	request := SendRequest{
		ClientMessageID: clientMessageID,
		MessageType:     1,
		TextContent:     "您好，我正在为您核实。",
	}
	first, err := service.Send(
		ctx, winner, conversationID, request,
		ActionMeta{RequestID: mustSupportConsoleTestID(t)},
	)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.Send(
		ctx, winner, conversationID, request,
		ActionMeta{RequestID: mustSupportConsoleTestID(t)},
	)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ID != first.ID {
		t.Fatalf("reply idempotency failed: first=%q duplicate=%q", first.ID, duplicate.ID)
	}
	if duplicate.ConversationID != conversationID || duplicate.SenderID != winner.ID {
		t.Fatalf("duplicate reply returned the wrong message: %#v", duplicate)
	}

	var replyCount int
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM support_messages
		WHERE sender_type=2 AND sender_id=? AND client_message_id=?`,
		winner.ID, clientMessageID,
	).Scan(&replyCount); err != nil {
		t.Fatal(err)
	}
	if replyCount != 1 {
		t.Fatalf("idempotent reply must persist once, got %d rows", replyCount)
	}

	var replyAuditCount int
	if err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE actor_type=1 AND actor_id=? AND action='support.reply'
		  AND resource_type='support_conversation' AND resource_id=?`,
		winner.ID, conversationID,
	).Scan(&replyAuditCount); err != nil {
		t.Fatal(err)
	}
	if replyAuditCount != 1 {
		t.Fatalf("idempotent reply must audit once, got %d rows", replyAuditCount)
	}

	if err = service.Transfer(
		ctx, winner, conversationID, loser.ID,
		ActionMeta{RequestID: mustSupportConsoleTestID(t)},
	); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = service.Conversation(ctx, winner, conversationID); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("previous owner must lose access after transfer, got %v", err)
	}
	if _, _, _, err = service.Conversation(ctx, loser, conversationID); err != nil {
		t.Fatalf("transfer target must gain access, got %v", err)
	}
	if err = service.Resolve(
		ctx, loser, conversationID,
		ActionMeta{RequestID: mustSupportConsoleTestID(t)},
	); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx,
		"SELECT status FROM support_conversations WHERE id=?",
		conversationID,
	).Scan(&storedStatus); err != nil {
		t.Fatal(err)
	}
	if storedStatus != 2 {
		t.Fatalf("resolved conversation must have status 2, got %d", storedStatus)
	}
}

func mustSupportConsoleTestID(t *testing.T) string {
	t.Helper()
	value, err := idgen.New()
	if err != nil {
		t.Fatal(err)
	}
	return value
}
