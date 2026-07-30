package im

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestDispatcherRecoversStaleClaimIntegration(t *testing.T) {
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
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddress, DB: 14,
		MaintNotificationsConfig: &maintnotifications.Config{Mode: maintnotifications.ModeDisabled},
	})
	defer redisClient.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	userID := time.Now().UnixNano() & 0x3fffffffffffffff
	conversationID, _ := idgen.New()
	messageID, _ := idgen.New()
	staleEventID, _ := idgen.New()
	freshEventID, _ := idgen.New()
	message := Message{
		ID: messageID, ConversationID: conversationID, Sequence: 1,
		ClientMessageID: "outbox-recovery-" + suffix, SenderUserID: userID,
		MessageType: 1, TextContent: "恢复派发", CreatedAt: time.Now().UnixMilli(),
	}
	payload, _ := json.Marshal(message)
	_, err = db.ExecContext(ctx, `
		INSERT INTO users(id,username,password_hash,nickname,status)
		VALUES(?,?,'test','Outbox User',1)`,
		userID, "outbox_user_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO im_conversations(id,conversation_type,title,status,created_by)
		VALUES(?,2,'Outbox Recovery',1,?)`,
		conversationID, userID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO im_conversation_members(conversation_id,user_id,role,member_status)
		VALUES(?,?,100,1)`,
		conversationID, userID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO outbox_events
			(event_id,aggregate_type,aggregate_id,event_type,payload,status,attempts,
			 available_at,processing_started_at)
		VALUES
			(?,'im_conversation',?,'im.message.created',?,1,1,
			 CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3)),
			(?,'im_conversation',?,'im.message.created',?,1,1,
			 CURRENT_TIMESTAMP(3)-INTERVAL 1 DAY,
			 CURRENT_TIMESTAMP(3)-INTERVAL 1 MINUTE)`,
		freshEventID, conversationID, payload,
		staleEventID, conversationID, payload,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM outbox_events WHERE event_id IN (?,?)", staleEventID, freshEventID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM im_conversation_members WHERE conversation_id=?", conversationID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM im_conversations WHERE id=?", conversationID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", userID)
	})

	channel := "im:v2:user:" + strconv.FormatInt(userID, 10)
	subscription := redisClient.Subscribe(ctx, channel)
	defer subscription.Close()
	if _, err = subscription.Receive(ctx); err != nil {
		t.Fatal(err)
	}

	dispatcher := NewDispatcher(
		db, redisClient, slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	processed, err := dispatcher.processOne(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("stale processing claim was not recovered")
	}
	receiveCtx, receiveCancel := context.WithTimeout(ctx, 5*time.Second)
	defer receiveCancel()
	published, err := subscription.ReceiveMessage(receiveCtx)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Type string  `json:"type"`
		Data Message `json:"data"`
	}
	if err = json.Unmarshal([]byte(published.Payload), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Type != "message" || envelope.Data.ID != messageID {
		t.Fatalf("unexpected recovered event payload: %#v", envelope)
	}

	var staleStatus, staleAttempts, freshStatus, freshAttempts int
	var processingStartedAt any
	err = db.QueryRowContext(ctx, `
		SELECT stale.status,stale.attempts,stale.processing_started_at,
		       fresh.status,fresh.attempts
		FROM outbox_events stale
		JOIN outbox_events fresh ON fresh.event_id=?
		WHERE stale.event_id=?`,
		freshEventID, staleEventID,
	).Scan(
		&staleStatus, &staleAttempts, &processingStartedAt,
		&freshStatus, &freshAttempts,
	)
	if err != nil {
		t.Fatal(err)
	}
	if staleStatus != 2 || staleAttempts != 2 || processingStartedAt != nil ||
		freshStatus != 1 || freshAttempts != 1 {
		t.Fatalf(
			"unexpected recovery state: stale=%d/%d/%v fresh=%d/%d",
			staleStatus, staleAttempts, processingStartedAt, freshStatus, freshAttempts,
		)
	}
}
