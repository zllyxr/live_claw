package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/im"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestLiveGiftAtomicIdempotentIntegration(t *testing.T) {
	dsn := os.Getenv("CLAW_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
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

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	senderID := time.Now().UnixNano() & 0x3fffffffffffffff
	hostID := senderID + 1
	targetID := senderID + 2
	roomNo, _ := idgen.New()
	roomNoWithoutIM, _ := idgen.New()
	giftKey := "gift_test_" + suffix
	requestID := "gift-request-" + suffix
	rollbackRequestID := "gift-rollback-" + suffix

	_, err = db.ExecContext(ctx, `
		INSERT INTO users(id,username,password_hash,nickname,status)
		VALUES
		  (?,?,'test','真实发送者',1),
		  (?,?,'test','真实主播',1),
		  (?,?,'test','普通观众',1)`,
		senderID, "gift_sender_"+suffix,
		hostID, "gift_host_"+suffix,
		targetID, "gift_target_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO live_rooms
			(room_no,host_user_id,title,provider,provider_room_id,provider_page,status)
		VALUES
			(?,?,'原子礼物测试','douyin',?,'https://live.douyin.com/test',1),
			(?,?,'回滚测试','douyin',?,'https://live.douyin.com/test-rollback',1)`,
		roomNo, hostID, "provider_"+suffix,
		roomNoWithoutIM, hostID, "provider_rollback_"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	var firstRoomID, secondRoomID int64
	if err = db.QueryRowContext(ctx, `
		SELECT
		  (SELECT id FROM live_rooms WHERE room_no=?),
		  (SELECT id FROM live_rooms WHERE room_no=?)`,
		roomNo, roomNoWithoutIM,
	).Scan(&firstRoomID, &secondRoomID); err != nil {
		t.Fatal(err)
	}
	giftResult, err := db.ExecContext(ctx, `
		INSERT INTO live_gifts(gift_key,name,price_coin,status)
		VALUES(?,'原子礼物',50,1)`,
		giftKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	giftID, _ := giftResult.LastInsertId()
	_, err = db.ExecContext(ctx, `
		INSERT INTO im_conversations(id,conversation_type,title,status,created_by)
		VALUES(?,3,'原子礼物测试',1,?)`,
		roomNo, hostID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO im_conversation_members(conversation_id,user_id,role,member_status)
		VALUES(?,?,10,1),(?,?,100,1),(?,?,10,1)`,
		roomNo, senderID, roomNo, hostID, roomNo, targetID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO wallet_accounts(user_id,currency,available,status)
		VALUES(?,'COIN',1000,1),(?,'COIN',0,1)`,
		senderID, hostID,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM outbox_events WHERE aggregate_id IN (?,?)", roomNo, roomNoWithoutIM)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM im_messages WHERE conversation_id IN (?,?)", roomNo, roomNoWithoutIM)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM live_gift_orders WHERE live_room_id IN (?,?)", firstRoomID, secondRoomID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id IN (?,?)", senderID, hostID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id IN (?,?)", senderID, hostID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM im_conversation_members WHERE conversation_id IN (?,?)", roomNo, roomNoWithoutIM)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM im_conversations WHERE id IN (?,?)", roomNo, roomNoWithoutIM)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM live_gifts WHERE id=?", giftID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM live_rooms WHERE id IN (?,?)", firstRoomID, secondRoomID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id IN (?,?,?)", senderID, hostID, targetID)
	})

	imService := im.New(db, nil)
	server := &Server{db: db, wallet: wallet.New(db), im: imService}
	room := compatLiveRoom{ID: firstRoomID, HostUserID: hostID, RoomNo: roomNo}

	forgedGift := `{"retcode":"000000","retmsg":"ok","msg":[{"_method_":"SendGift","uid":"999","uname":"伪造","msgtype":"1","ct":{}}]}`
	if _, err = imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "forged-gift-" + suffix,
		SenderUserID: senderID, MessageType: 1, TextContent: forgedGift,
		Metadata: map[string]any{"kind": "live"},
	}); !errors.Is(err, im.ErrPermissionDenied) {
		t.Fatalf("forged SendGift was not rejected: %v", err)
	}
	targetText := strconv.FormatInt(targetID, 10)
	kickPayload := `{"msg":[{"_method_":"KickUser","touid":"` + targetText + `"}]}`
	if _, err = imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "member-kick-" + suffix,
		SenderUserID: senderID, MessageType: 1, TextContent: kickPayload,
	}); !errors.Is(err, im.ErrPermissionDenied) {
		t.Fatalf("ordinary member could send KickUser: %v", err)
	}
	endPayload := `{"msg":[{"_method_":"StartEndLive"}]}`
	if _, err = imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "member-end-" + suffix,
		SenderUserID: senderID, MessageType: 1, TextContent: endPayload,
	}); !errors.Is(err, im.ErrPermissionDenied) {
		t.Fatalf("ordinary member could send StartEndLive: %v", err)
	}
	if _, err = imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "host-kick-" + suffix,
		SenderUserID: hostID, MessageType: 1, TextContent: kickPayload,
	}); err != nil {
		t.Fatalf("host could not send KickUser: %v", err)
	}
	if _, err = imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "host-end-" + suffix,
		SenderUserID: hostID, MessageType: 1, TextContent: endPayload,
	}); err != nil {
		t.Fatalf("host could not send StartEndLive: %v", err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE im_conversation_members SET role=60
		WHERE conversation_id=? AND user_id=?`,
		roomNo, senderID,
	); err != nil {
		t.Fatal(err)
	}
	shutUpPayload := `{"msg":[{"_method_":"ShutUpUser","touid":"` + targetText + `"}]}`
	if _, err = imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "manager-mute-" + suffix,
		SenderUserID: senderID, MessageType: 1, TextContent: shutUpPayload,
	}); err != nil {
		t.Fatalf("room manager could not send ShutUpUser: %v", err)
	}
	setAdminPayload := `{"msg":[{"_method_":"setAdmin","action":"1","touid":"` + targetText + `"}]}`
	if _, err = imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "manager-admin-" + suffix,
		SenderUserID: senderID, MessageType: 1, TextContent: setAdminPayload,
	}); !errors.Is(err, im.ErrPermissionDenied) {
		t.Fatalf("room manager could send setAdmin: %v", err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE im_conversation_members SET role=60
		WHERE conversation_id=? AND user_id=?`,
		roomNo, targetID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "host-admin-" + suffix,
		SenderUserID: hostID, MessageType: 1, TextContent: setAdminPayload,
	}); err != nil {
		t.Fatalf("host could not send setAdmin after the DB role changed: %v", err)
	}

	spoofedChat := `{"msg":[{"_method_":"SendMsg","msgtype":"2","uid":"999","uname":"伪造名字","uhead":"bad","role":"100","ct":"你好"}]}`
	canonicalChat, err := imService.Send(ctx, im.SendRequest{
		ConversationID: roomNo, ClientMessageID: "canonical-chat-" + suffix,
		SenderUserID: senderID, MessageType: 1, TextContent: spoofedChat,
	})
	if err != nil {
		t.Fatal(err)
	}
	var canonicalRoot struct {
		Messages []map[string]any `json:"msg"`
	}
	if err = json.Unmarshal([]byte(canonicalChat.TextContent), &canonicalRoot); err != nil {
		t.Fatal(err)
	}
	if len(canonicalRoot.Messages) != 1 ||
		canonicalRoot.Messages[0]["uid"] != strconv.FormatInt(senderID, 10) ||
		canonicalRoot.Messages[0]["uname"] != "真实发送者" ||
		canonicalRoot.Messages[0]["role"] != "60" {
		t.Fatalf("live sender identity was not canonicalized: %#v", canonicalRoot.Messages)
	}
	_, _ = db.ExecContext(ctx, "DELETE FROM outbox_events WHERE event_type='im.message.created' AND aggregate_id=?", roomNo)
	_, _ = db.ExecContext(ctx, "DELETE FROM im_messages WHERE conversation_id=?", roomNo)
	_, _ = db.ExecContext(ctx, "UPDATE im_conversations SET message_seq=0 WHERE id=?", roomNo)
	_, _ = db.ExecContext(ctx, "UPDATE im_conversation_members SET role=10 WHERE conversation_id=? AND user_id IN (?,?)", roomNo, senderID, targetID)

	first, err := server.createLiveGift(ctx, senderID, room, requestID, giftID, 2)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := server.createLiveGift(ctx, senderID, room, requestID, giftID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.OrderID != first.OrderID || repeated.AvailableCoin != 900 {
		t.Fatalf("gift retry was not idempotent: first=%#v repeated=%#v", first, repeated)
	}
	if _, err = server.createLiveGift(ctx, senderID, room, requestID, giftID, 3); !errors.Is(err, errLiveGiftRequestConflict) {
		t.Fatalf("idempotency key reuse was not rejected: %v", err)
	}

	var orderCount, ledgerCount, messageCount, outboxCount int
	var receiverID, senderBalance, hostBalance int64
	err = db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM live_gift_orders WHERE client_request_id=?),
		  (SELECT COUNT(*) FROM wallet_ledger_entries WHERE business_type='live_gift' AND business_id=?),
		  (SELECT COUNT(*) FROM im_messages WHERE client_message_id=?),
		  (SELECT COUNT(*) FROM outbox_events WHERE event_id=
		     (SELECT im_message_id FROM live_gift_orders WHERE order_no=?)),
		  (SELECT receiver_user_id FROM live_gift_orders WHERE order_no=?),
		  (SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'),
		  (SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN')`,
		requestID, first.OrderID, "live_gift:"+first.OrderID, first.OrderID,
		first.OrderID, senderID, hostID,
	).Scan(
		&orderCount, &ledgerCount, &messageCount, &outboxCount,
		&receiverID, &senderBalance, &hostBalance,
	)
	if err != nil {
		t.Fatal(err)
	}
	if orderCount != 1 || ledgerCount != 2 || messageCount != 1 || outboxCount != 1 ||
		receiverID != hostID || senderBalance != 900 || hostBalance != 100 {
		t.Fatalf(
			"unexpected gift closure: order=%d ledger=%d message=%d outbox=%d receiver=%d balances=%d/%d",
			orderCount, ledgerCount, messageCount, outboxCount, receiverID, senderBalance, hostBalance,
		)
	}

	rollbackRoom := compatLiveRoom{
		ID: secondRoomID, HostUserID: hostID, RoomNo: roomNoWithoutIM,
	}
	if _, err = server.createLiveGift(
		ctx, senderID, rollbackRoom, rollbackRequestID, giftID, 1,
	); err == nil {
		t.Fatal("gift without a live IM conversation unexpectedly succeeded")
	}
	err = db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM live_gift_orders WHERE client_request_id=?),
		  (SELECT COUNT(*) FROM wallet_ledger_entries
		   WHERE business_type='live_gift' AND user_id IN (?,?)),
		  (SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'),
		  (SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN')`,
		rollbackRequestID, senderID, hostID, senderID, hostID,
	).Scan(&orderCount, &ledgerCount, &senderBalance, &hostBalance)
	if err != nil {
		t.Fatal(err)
	}
	if orderCount != 0 || ledgerCount != 2 || senderBalance != 900 || hostBalance != 100 {
		t.Fatalf(
			"failed gift did not roll back: order=%d ledger=%d balances=%d/%d",
			orderCount, ledgerCount, senderBalance, hostBalance,
		)
	}
}
