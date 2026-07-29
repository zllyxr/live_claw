package im

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

var (
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrNotConversationMember = errors.New("not a conversation member")
	ErrMuted                 = errors.New("conversation member is muted")
	ErrPermissionDenied      = errors.New("conversation permission denied")
)

type Service struct {
	db           *sql.DB
	redis        *redis.Client
	now          func() time.Time
	mediaBaseURL string
}

type Conversation struct {
	ID               string `json:"id"`
	ConversationType int    `json:"conversation_type"`
	Title            string `json:"title"`
	MessageSeq       int64  `json:"message_seq"`
}

type Message struct {
	ID              string         `json:"id"`
	ConversationID  string         `json:"conversation_id"`
	Sequence        int64          `json:"sequence"`
	ClientMessageID string         `json:"client_message_id"`
	SenderUserID    int64          `json:"sender_user_id,string"`
	MessageType     int            `json:"message_type"`
	TextContent     string         `json:"text_content,omitempty"`
	AssetID         int64          `json:"asset_id,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	SenderName      string         `json:"sender_name,omitempty"`
	SenderAvatar    string         `json:"sender_avatar,omitempty"`
	CreatedAt       int64          `json:"created_at"`
}

type SendRequest struct {
	ConversationID  string
	ClientMessageID string
	SenderUserID    int64
	MessageType     int
	TextContent     string
	AssetID         int64
	Metadata        map[string]any
}

func New(db *sql.DB, redisClient *redis.Client) *Service {
	return &Service{db: db, redis: redisClient, now: time.Now}
}

func (s *Service) SetMediaBaseURL(value string) {
	s.mediaBaseURL = strings.TrimRight(strings.TrimSpace(value), "/")
}

func (s *Service) assetURL(objectKey string) string {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" ||
		strings.HasPrefix(objectKey, "http://") ||
		strings.HasPrefix(objectKey, "https://") ||
		strings.HasPrefix(objectKey, "data:") ||
		strings.HasPrefix(objectKey, "blob:") {
		return objectKey
	}
	if s.mediaBaseURL == "" {
		return objectKey
	}
	return s.mediaBaseURL + "/" + strings.TrimLeft(objectKey, "/")
}

func (s *Service) DirectConversation(ctx context.Context, userID, peerUserID int64) (Conversation, error) {
	if userID < 1 || peerUserID < 1 || userID == peerUserID {
		return Conversation{}, errors.New("invalid direct conversation")
	}
	left, right := userID, peerUserID
	if left > right {
		left, right = right, left
	}
	directKey := strconv.FormatInt(left, 10) + ":" + strconv.FormatInt(right, 10)
	var existing Conversation
	err := s.db.QueryRowContext(ctx, `
		SELECT id,conversation_type,title,message_seq
		FROM im_conversations WHERE direct_key=?`,
		directKey,
	).Scan(&existing.ID, &existing.ConversationType, &existing.Title, &existing.MessageSeq)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, err
	}
	conversationID, err := idgen.New()
	if err != nil {
		return Conversation{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Conversation{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var userCount int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE id IN (?,?) AND status=1`,
		userID, peerUserID,
	).Scan(&userCount); err != nil || userCount != 2 {
		return Conversation{}, errors.New("direct conversation user does not exist")
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_conversations
			(id,conversation_type,direct_key,title,status,created_by)
		VALUES(?,1,?,'',1,?)`,
		conversationID, directKey, userID,
	); err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			_ = tx.Rollback()
			return s.DirectConversation(ctx, userID, peerUserID)
		}
		return Conversation{}, err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_conversation_members(conversation_id,user_id,role,member_status)
		VALUES(?,?,10,1),(?,?,10,1)`,
		conversationID, userID, conversationID, peerUserID,
	); err != nil {
		return Conversation{}, err
	}
	if err = tx.Commit(); err != nil {
		return Conversation{}, err
	}
	return Conversation{ID: conversationID, ConversationType: 1}, nil
}

func (s *Service) CreateGroup(
	ctx context.Context,
	ownerUserID int64,
	title string,
	maxMembers int,
) (Conversation, error) {
	title = strings.TrimSpace(title)
	if ownerUserID < 1 || title == "" || len(title) > 200 {
		return Conversation{}, errors.New("invalid group")
	}
	if maxMembers < 2 {
		maxMembers = 500
	}
	if maxMembers > 2000 {
		return Conversation{}, errors.New("group member limit is too large")
	}
	conversationID, err := idgen.New()
	if err != nil {
		return Conversation{}, err
	}
	groupNo, err := idgen.New()
	if err != nil {
		return Conversation{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Conversation{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(ctx, `
		INSERT INTO im_conversations
			(id,conversation_type,direct_key,title,status,created_by)
		SELECT ?,2,NULL,?,1,? FROM users WHERE id=? AND status=1`,
		conversationID, title, ownerUserID, ownerUserID,
	)
	if err != nil {
		return Conversation{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return Conversation{}, errors.New("group owner does not exist")
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_groups
			(conversation_id,group_no,owner_user_id,max_members,member_count)
		VALUES(?,?,?,?,1)`,
		conversationID, groupNo, ownerUserID, maxMembers,
	); err != nil {
		return Conversation{}, err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_conversation_members(conversation_id,user_id,role,member_status)
		VALUES(?,?,100,1)`,
		conversationID, ownerUserID,
	); err != nil {
		return Conversation{}, err
	}
	if err = tx.Commit(); err != nil {
		return Conversation{}, err
	}
	return Conversation{ID: conversationID, ConversationType: 2, Title: title}, nil
}

func (s *Service) AddGroupMember(ctx context.Context, actorUserID int64, conversationID string, userID int64) error {
	if actorUserID < 1 || userID < 1 || conversationID == "" {
		return errors.New("invalid group member")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var role int
	var memberCount, maxMembers int
	err = tx.QueryRowContext(ctx, `
		SELECT member.role,group_info.member_count,group_info.max_members
		FROM im_conversation_members member
		JOIN im_groups group_info ON group_info.conversation_id=member.conversation_id
		WHERE member.conversation_id=? AND member.user_id=? AND member.member_status=1
		FOR UPDATE`,
		conversationID, actorUserID,
	).Scan(&role, &memberCount, &maxMembers)
	if errors.Is(err, sql.ErrNoRows) || role < 60 {
		return ErrPermissionDenied
	}
	if err != nil {
		return err
	}
	if memberCount >= maxMembers {
		return errors.New("group is full")
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO im_conversation_members
			(conversation_id,user_id,role,member_status,is_hidden)
		SELECT ?,?,10,1,0 FROM users WHERE id=? AND status=1
		ON DUPLICATE KEY UPDATE member_status=1,is_hidden=0,left_at=NULL`,
		conversationID, userID, userID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errors.New("group member does not exist")
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_groups
		SET member_count=(SELECT COUNT(*) FROM im_conversation_members
		                  WHERE conversation_id=? AND member_status=1)
		WHERE conversation_id=?`,
		conversationID, conversationID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) MuteMember(
	ctx context.Context,
	actorUserID int64,
	conversationID string,
	targetUserID int64,
	until time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var actorRole, targetRole int
	err = tx.QueryRowContext(ctx, `
		SELECT actor.role,target.role
		FROM im_conversation_members actor
		JOIN im_conversation_members target ON target.conversation_id=actor.conversation_id
		WHERE actor.conversation_id=? AND actor.user_id=? AND actor.member_status=1
		  AND target.user_id=? AND target.member_status=1
		FOR UPDATE`,
		conversationID, actorUserID, targetUserID,
	).Scan(&actorRole, &targetRole)
	if errors.Is(err, sql.ErrNoRows) || actorRole < 60 || actorRole <= targetRole {
		return ErrPermissionDenied
	}
	if err != nil {
		return err
	}
	var muteValue any
	if until.After(s.now()) {
		muteValue = until
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_conversation_members SET mute_until=?
		WHERE conversation_id=? AND user_id=?`,
		muteValue, conversationID, targetUserID,
	); err != nil {
		return err
	}
	action := "unmute"
	if muteValue != nil {
		action = "mute"
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_moderation_actions
			(conversation_id,target_user_id,action_type,actor_type,actor_id)
		VALUES(?,?,?,2,?)`,
		conversationID, targetUserID, action, actorUserID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) Send(ctx context.Context, request SendRequest) (Message, error) {
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.ClientMessageID = strings.TrimSpace(request.ClientMessageID)
	request.TextContent = strings.TrimSpace(request.TextContent)
	if request.SenderUserID < 1 || request.ConversationID == "" ||
		request.ClientMessageID == "" || len(request.ClientMessageID) > 100 ||
		request.MessageType < 1 || request.MessageType > 100 {
		return Message{}, errors.New("invalid message")
	}
	if request.MessageType == 1 && (request.TextContent == "" || len(request.TextContent) > 5000) {
		return Message{}, errors.New("invalid text message")
	}
	if request.MessageType >= 2 && request.MessageType <= 5 && request.AssetID < 1 {
		sourceURL, _ := request.Metadata["source_url"].(string)
		if sourceURL = strings.TrimSpace(sourceURL); sourceURL == "" || len(sourceURL) > 2000 {
			return Message{}, errors.New("message asset is required")
		}
	}
	messageID, err := idgen.New()
	if err != nil {
		return Message{}, err
	}
	eventID, err := idgen.New()
	if err != nil {
		return Message{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var sequence int64
	var conversationType, memberRole, memberStatus int
	var muteUntil sql.NullTime
	var allMuted bool
	var senderName, senderAvatar string
	err = tx.QueryRowContext(ctx, `
		SELECT conversation.message_seq,conversation.conversation_type,
		       member.role,member.member_status,member.mute_until,
		       COALESCE(group_info.all_muted,0),
		       COALESCE(NULLIF(sender.nickname,''),sender.username),
		       COALESCE(sender_asset.object_key,'')
		FROM im_conversations conversation
		JOIN im_conversation_members member ON member.conversation_id=conversation.id
		JOIN users sender ON sender.id=member.user_id
		LEFT JOIN media_assets sender_asset
		  ON sender_asset.id=sender.avatar_asset_id AND sender_asset.status=1
		LEFT JOIN im_groups group_info ON group_info.conversation_id=conversation.id
		WHERE conversation.id=? AND conversation.status=1 AND member.user_id=?
		FOR UPDATE`,
		request.ConversationID, request.SenderUserID,
	).Scan(
		&sequence, &conversationType, &memberRole, &memberStatus, &muteUntil, &allMuted,
		&senderName, &senderAvatar,
	)
	if errors.Is(err, sql.ErrNoRows) || memberStatus != 1 {
		return Message{}, ErrNotConversationMember
	}
	if err != nil {
		return Message{}, err
	}
	liveMethod := ""
	if conversationType == 3 {
		liveMethod, err = s.normalizeLiveClientMessage(
			ctx, tx, &request, memberRole, senderName, senderAvatar,
		)
		if err != nil {
			return Message{}, err
		}
	}
	if conversationType != 3 || liveMethod == "SendMsg" {
		if muteUntil.Valid && muteUntil.Time.After(s.now()) {
			return Message{}, ErrMuted
		}
		if allMuted && memberRole < 60 {
			return Message{}, ErrMuted
		}
	}
	metadataJSON, err := json.Marshal(request.Metadata)
	if err != nil {
		return Message{}, err
	}
	var existing Message
	var existingCreatedAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT id,conversation_id,sequence,client_message_id,sender_user_id,
		       message_type,text_content,asset_id,created_at
		FROM im_messages WHERE sender_user_id=? AND client_message_id=?`,
		request.SenderUserID, request.ClientMessageID,
	).Scan(
		&existing.ID, &existing.ConversationID, &existing.Sequence, &existing.ClientMessageID,
		&existing.SenderUserID, &existing.MessageType, &existing.TextContent,
		&existing.AssetID, &existingCreatedAt,
	)
	if err == nil {
		if existing.ConversationID != request.ConversationID {
			return Message{}, errors.New("client message id was reused")
		}
		existing.Metadata = request.Metadata
		existing.SenderName = senderName
		existing.SenderAvatar = s.assetURL(senderAvatar)
		existing.CreatedAt = existingCreatedAt.UnixMilli()
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
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
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_messages
			(id,conversation_id,sequence,client_message_id,sender_user_id,message_type,
			 text_content,asset_id,metadata,status,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,1,?)`,
		messageID, request.ConversationID, sequence, request.ClientMessageID,
		request.SenderUserID, request.MessageType, request.TextContent, request.AssetID,
		nullableJSON(metadataJSON), createdAt,
	); err != nil {
		return Message{}, err
	}
	message := Message{
		ID: messageID, ConversationID: request.ConversationID, Sequence: sequence,
		ClientMessageID: request.ClientMessageID, SenderUserID: request.SenderUserID,
		MessageType: request.MessageType, TextContent: request.TextContent,
		AssetID: request.AssetID, Metadata: request.Metadata,
		SenderName: senderName, SenderAvatar: s.assetURL(senderAvatar), CreatedAt: createdAt.UnixMilli(),
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return Message{}, err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events
			(event_id,aggregate_type,aggregate_id,event_type,payload,status,available_at)
		VALUES(?,'im_conversation',?,'im.message.created',?,0,
		       CURRENT_TIMESTAMP(3)+INTERVAL 2 SECOND)`,
		eventID, request.ConversationID, payload,
	); err != nil {
		return Message{}, err
	}
	if err = tx.Commit(); err != nil {
		return Message{}, err
	}
	if publishErr := s.publishMessage(ctx, message); publishErr == nil {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE outbox_events
			SET status=2,processed_at=CURRENT_TIMESTAMP(3),last_error=''
			WHERE event_id=? AND status=0`,
			eventID,
		)
	}
	return message, nil
}

func (s *Service) Messages(
	ctx context.Context,
	userID int64,
	conversationID string,
	beforeSequence int64,
	limit int,
) ([]Message, error) {
	if limit < 1 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var member int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM im_conversation_members
		WHERE conversation_id=? AND user_id=? AND member_status=1`,
		conversationID, userID,
	).Scan(&member)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotConversationMember
	}
	if err != nil {
		return nil, err
	}
	if beforeSequence <= 0 {
		beforeSequence = 1<<63 - 1
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT message.id,message.conversation_id,message.sequence,message.client_message_id,
		       message.sender_user_id,message.message_type,message.text_content,
		       message.asset_id,message.metadata,
		       COALESCE(NULLIF(sender.nickname,''),sender.username),
		       COALESCE(sender_asset.object_key,''),message.created_at
		FROM im_messages message
		JOIN users sender ON sender.id=message.sender_user_id
		LEFT JOIN media_assets sender_asset
		  ON sender_asset.id=sender.avatar_asset_id AND sender_asset.status=1
		WHERE message.conversation_id=? AND message.sequence<? AND message.status=1
		ORDER BY message.sequence DESC LIMIT ?`,
		conversationID, beforeSequence, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Message, 0, limit)
	for rows.Next() {
		var message Message
		var metadata []byte
		var createdAt time.Time
		if err = rows.Scan(
			&message.ID, &message.ConversationID, &message.Sequence, &message.ClientMessageID,
			&message.SenderUserID, &message.MessageType, &message.TextContent,
			&message.AssetID, &metadata, &message.SenderName, &message.SenderAvatar, &createdAt,
		); err != nil {
			return nil, err
		}
		message.CreatedAt = createdAt.UnixMilli()
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &message.Metadata)
		}
		message.SenderAvatar = s.assetURL(message.SenderAvatar)
		items = append(items, message)
	}
	return items, rows.Err()
}

func (s *Service) publishMessage(ctx context.Context, message Message) error {
	if s.redis == nil {
		return errors.New("redis is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id FROM im_conversation_members
		WHERE conversation_id=? AND member_status=1`,
		message.ConversationID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	payload, err := json.Marshal(map[string]any{"type": "message", "data": message})
	if err != nil {
		return err
	}
	pipe := s.redis.Pipeline()
	for rows.Next() {
		var userID int64
		if err = rows.Scan(&userID); err != nil {
			return err
		}
		pipe.Publish(ctx, "im:v2:user:"+strconv.FormatInt(userID, 10), payload)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	_, err = pipe.Exec(ctx)
	return err
}

func nullableJSON(value []byte) any {
	if len(value) == 0 || string(value) == "null" {
		return nil
	}
	return value
}

func PublicError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrConversationNotFound):
		return 404, "会话不存在"
	case errors.Is(err, ErrNotConversationMember):
		return 403, "你不是该会话成员"
	case errors.Is(err, ErrMuted):
		return 403, "你已被禁言"
	case errors.Is(err, ErrPermissionDenied):
		return 403, "无权执行此操作"
	default:
		return 400, "IM 操作失败"
	}
}
