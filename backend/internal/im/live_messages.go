package im

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

type TrustedLiveGiftRequest struct {
	ConversationID  string
	OrderID         string
	ClientRequestID string
	SenderUserID    int64
	ReceiverUserID  int64
	GiftID          int64
	GiftName        string
	GiftIcon        string
	GiftCount       int64
	TotalCoin       int64
}

// AppendTrustedLiveGiftTx persists the canonical gift message and its outbox
// event in the business transaction that charged the wallet.
func (s *Service) AppendTrustedLiveGiftTx(
	ctx context.Context,
	tx *sql.Tx,
	request TrustedLiveGiftRequest,
) (Message, error) {
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.OrderID = strings.TrimSpace(request.OrderID)
	request.ClientRequestID = strings.TrimSpace(request.ClientRequestID)
	request.GiftName = strings.TrimSpace(request.GiftName)
	request.GiftIcon = strings.TrimSpace(request.GiftIcon)
	if tx == nil ||
		request.ConversationID == "" ||
		request.OrderID == "" ||
		request.ClientRequestID == "" ||
		request.SenderUserID < 1 ||
		request.ReceiverUserID < 1 ||
		request.SenderUserID == request.ReceiverUserID ||
		request.GiftID < 1 ||
		request.GiftName == "" ||
		len(request.GiftName) > 100 ||
		len(request.GiftIcon) > 1000 ||
		request.GiftCount < 1 ||
		request.TotalCoin < 1 {
		return Message{}, errors.New("invalid trusted live gift")
	}

	var sequence int64
	var conversationType, conversationStatus int
	var senderRole, senderStatus, receiverRole, receiverStatus int
	var senderName, senderAvatar, receiverName string
	err := tx.QueryRowContext(ctx, `
		SELECT conversation.message_seq,conversation.conversation_type,conversation.status,
		       sender_member.role,sender_member.member_status,
		       receiver_member.role,receiver_member.member_status,
		       COALESCE(NULLIF(sender.nickname,''),sender.username),
		       COALESCE(sender_asset.object_key,''),
		       COALESCE(NULLIF(receiver.nickname,''),receiver.username)
		FROM im_conversations conversation
		JOIN im_conversation_members sender_member
		  ON sender_member.conversation_id=conversation.id
		 AND sender_member.user_id=?
		JOIN im_conversation_members receiver_member
		  ON receiver_member.conversation_id=conversation.id
		 AND receiver_member.user_id=?
		JOIN users sender ON sender.id=sender_member.user_id AND sender.status=1
		JOIN users receiver ON receiver.id=receiver_member.user_id AND receiver.status=1
		LEFT JOIN media_assets sender_asset
		  ON sender_asset.id=sender.avatar_asset_id AND sender_asset.status=1
		WHERE conversation.id=?
		FOR UPDATE`,
		request.SenderUserID, request.ReceiverUserID, request.ConversationID,
	).Scan(
		&sequence, &conversationType, &conversationStatus,
		&senderRole, &senderStatus, &receiverRole, &receiverStatus,
		&senderName, &senderAvatar, &receiverName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrNotConversationMember
	}
	if err != nil {
		return Message{}, err
	}
	if conversationType != 3 || conversationStatus != 1 {
		return Message{}, ErrConversationNotFound
	}
	if senderStatus != 1 || receiverStatus != 1 {
		return Message{}, ErrNotConversationMember
	}
	if receiverRole < 100 {
		return Message{}, ErrPermissionDenied
	}

	messageID, err := idgen.New()
	if err != nil {
		return Message{}, err
	}
	senderAvatar = s.assetURL(senderAvatar)
	senderID := strconv.FormatInt(request.SenderUserID, 10)
	receiverID := strconv.FormatInt(request.ReceiverUserID, 10)
	giftID := strconv.FormatInt(request.GiftID, 10)
	textPayload := map[string]any{
		"retcode": "000000",
		"retmsg":  "ok",
		"msg": []any{map[string]any{
			"_method_":    "SendGift",
			"action":      "0",
			"msgtype":     "1",
			"uid":         senderID,
			"uname":       senderName,
			"uhead":       senderAvatar,
			"roomnum":     receiverID,
			"livename":    receiverName,
			"usertype":    liveUserType(senderRole),
			"isAnchor":    liveAnchor(senderRole),
			"role":        strconv.Itoa(senderRole),
			"paintedPath": []any{},
			"ct": map[string]any{
				"orderid":           request.OrderID,
				"client_request_id": request.ClientRequestID,
				"giftid":            giftID,
				"giftname":          request.GiftName,
				"gifticon":          request.GiftIcon,
				"giftcount":         request.GiftCount,
				"totalcoin":         request.TotalCoin,
			},
		}},
	}
	textContent, err := json.Marshal(textPayload)
	if err != nil {
		return Message{}, err
	}
	if len(textContent) > 5000 {
		return Message{}, errors.New("trusted live gift payload is too large")
	}
	metadata := map[string]any{
		"kind":              "live_gift",
		"trusted":           true,
		"order_id":          request.OrderID,
		"client_request_id": request.ClientRequestID,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return Message{}, err
	}

	sequence++
	createdAt := s.now()
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_conversations SET message_seq=?,updated_at=? WHERE id=?`,
		sequence, createdAt, request.ConversationID,
	); err != nil {
		return Message{}, err
	}
	clientMessageID := "live_gift:" + request.OrderID
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_messages
			(id,conversation_id,sequence,client_message_id,sender_user_id,message_type,
			 text_content,asset_id,metadata,status,created_at)
		VALUES(?,?,?,?,?,100,?,0,?,1,?)`,
		messageID, request.ConversationID, sequence, clientMessageID,
		request.SenderUserID, string(textContent), nullableJSON(metadataJSON), createdAt,
	); err != nil {
		return Message{}, err
	}
	message := Message{
		ID: messageID, ConversationID: request.ConversationID, Sequence: sequence,
		ClientMessageID: clientMessageID, SenderUserID: request.SenderUserID,
		MessageType: 100, TextContent: string(textContent), Metadata: metadata,
		SenderName: senderName, SenderAvatar: senderAvatar, CreatedAt: createdAt.UnixMilli(),
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return Message{}, err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events
			(event_id,aggregate_type,aggregate_id,event_type,payload,status,available_at)
		VALUES(?,'im_conversation',?,'im.message.created',?,0,CURRENT_TIMESTAMP(3))`,
		messageID, request.ConversationID, payload,
	); err != nil {
		return Message{}, err
	}
	return message, nil
}

func (s *Service) normalizeLiveClientMessage(
	ctx context.Context,
	tx *sql.Tx,
	request *SendRequest,
	memberRole int,
	senderName string,
	senderAvatar string,
) (string, error) {
	if request.MessageType != 1 || request.AssetID != 0 {
		return "", ErrPermissionDenied
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(request.TextContent), &root); err != nil {
		return "", errors.New("invalid live message")
	}
	rawMessages, ok := root["msg"].([]any)
	if !ok || len(rawMessages) != 1 {
		return "", errors.New("invalid live message")
	}
	incoming, ok := rawMessages[0].(map[string]any)
	if !ok {
		return "", errors.New("invalid live message")
	}
	method := strings.TrimSpace(liveText(incoming["_method_"]))
	senderID := strconv.FormatInt(request.SenderUserID, 10)
	senderAvatar = s.assetURL(senderAvatar)
	identity := func(message map[string]any) map[string]any {
		message["uid"] = senderID
		message["uname"] = senderName
		message["uhead"] = senderAvatar
		message["role"] = strconv.Itoa(memberRole)
		message["usertype"] = liveUserType(memberRole)
		if memberRole >= 100 {
			message["isAnchor"] = "1"
		} else {
			message["isAnchor"] = "0"
		}
		return message
	}

	var canonical map[string]any
	switch method {
	case "SendMsg":
		messageType := liveText(incoming["msgtype"])
		switch messageType {
		case "0":
			canonical = identity(map[string]any{
				"_method_": "SendMsg",
				"action":   "0",
				"msgtype":  "0",
				"ct": map[string]any{
					"id":            senderID,
					"user_nickname": senderName,
					"avatar":        senderAvatar,
					"avatar_thumb":  senderAvatar,
					"role":          strconv.Itoa(memberRole),
					"usertype":      liveUserType(memberRole),
				},
			})
		case "2":
			content, contentOK := incoming["ct"].(string)
			content = strings.TrimSpace(content)
			if !contentOK || content == "" || len(content) > 2000 {
				return "", errors.New("invalid live chat message")
			}
			canonical = identity(map[string]any{
				"_method_": "SendMsg",
				"action":   "0",
				"msgtype":  "2",
				"ct":       content,
			})
		default:
			return "", ErrPermissionDenied
		}
	case "disconnect":
		canonical = identity(map[string]any{
			"_method_": "disconnect",
			"action":   "0",
			"msgtype":  "0",
			"ct":       map[string]any{"id": senderID},
		})
	case "KickUser", "ShutUpUser":
		if memberRole < 60 {
			return "", ErrPermissionDenied
		}
		targetID, err := liveTargetID(incoming["touid"])
		if err != nil || targetID == request.SenderUserID {
			return "", ErrPermissionDenied
		}
		targetRole, targetStatus, targetName, err := liveTarget(ctx, tx, request.ConversationID, targetID)
		if err != nil || targetRole >= memberRole {
			return "", ErrPermissionDenied
		}
		if method == "KickUser" && targetStatus != 1 && targetStatus != 3 {
			return "", ErrPermissionDenied
		}
		if method == "ShutUpUser" && targetStatus != 1 {
			return "", ErrPermissionDenied
		}
		content := targetName + "被禁言"
		action := "1"
		if method == "KickUser" {
			content = targetName + "被踢出房间"
			action = "2"
		}
		canonical = identity(map[string]any{
			"_method_": method,
			"action":   action,
			"msgtype":  "4",
			"touid":    strconv.FormatInt(targetID, 10),
			"toname":   targetName,
			"ct":       content,
		})
	case "setAdmin":
		if memberRole < 100 {
			return "", ErrPermissionDenied
		}
		targetID, err := liveTargetID(incoming["touid"])
		if err != nil || targetID == request.SenderUserID {
			return "", ErrPermissionDenied
		}
		action := liveText(incoming["action"])
		if action != "0" && action != "1" {
			return "", ErrPermissionDenied
		}
		targetRole, targetStatus, targetName, err := liveTarget(ctx, tx, request.ConversationID, targetID)
		if err != nil || targetStatus != 1 || targetRole >= memberRole {
			return "", ErrPermissionDenied
		}
		if (action == "1" && targetRole != 60) || (action == "0" && targetRole != 10) {
			return "", ErrPermissionDenied
		}
		content := targetName + "被取消管理员"
		if action == "1" {
			content = targetName + "被设为管理员"
		}
		canonical = identity(map[string]any{
			"_method_": "setAdmin",
			"action":   action,
			"msgtype":  "1",
			"touid":    strconv.FormatInt(targetID, 10),
			"toname":   targetName,
			"ct":       content,
		})
	case "StartEndLive":
		if memberRole < 100 {
			return "", ErrPermissionDenied
		}
		canonical = identity(map[string]any{
			"_method_": "StartEndLive",
			"action":   "0",
			"msgtype":  "1",
			"ct":       "直播已结束",
		})
	case "SendGift", "BuyGuard", "SendRed", "SystemNot", "warning", "requestFans", "SendBarrage":
		return "", ErrPermissionDenied
	default:
		return "", ErrPermissionDenied
	}

	encoded, err := json.Marshal(map[string]any{
		"retcode": "000000",
		"retmsg":  "ok",
		"msg":     []any{canonical},
	})
	if err != nil {
		return "", err
	}
	if len(encoded) > 5000 {
		return "", errors.New("live message is too large")
	}
	request.TextContent = string(encoded)
	request.Metadata = map[string]any{"kind": "live", "legacy_method": method}
	return method, nil
}

func liveTarget(
	ctx context.Context,
	tx *sql.Tx,
	conversationID string,
	targetUserID int64,
) (int, int, string, error) {
	var role, status int
	var name string
	err := tx.QueryRowContext(ctx, `
		SELECT member.role,member.member_status,
		       COALESCE(NULLIF(user.nickname,''),user.username)
		FROM im_conversation_members member
		JOIN users user ON user.id=member.user_id AND user.status=1
		WHERE member.conversation_id=? AND member.user_id=?`,
		conversationID, targetUserID,
	).Scan(&role, &status, &name)
	return role, status, name, err
}

func liveTargetID(value any) (int64, error) {
	result, err := strconv.ParseInt(strings.TrimSpace(liveText(value)), 10, 64)
	if err != nil || result < 1 {
		return 0, errors.New("invalid live target")
	}
	return result, nil
}

func liveText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func liveUserType(role int) string {
	switch {
	case role >= 100:
		return "50"
	case role >= 60:
		return "40"
	default:
		return "30"
	}
}

func liveAnchor(role int) string {
	if role >= 100 {
		return "1"
	}
	return "0"
}
