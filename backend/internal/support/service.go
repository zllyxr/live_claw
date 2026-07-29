package support

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

var (
	ErrConversationNotFound = errors.New("support conversation not found")
	ErrConversationClosed   = errors.New("support conversation is closed")
	ErrInvalidMessage       = errors.New("invalid support message")
)

type Service struct {
	db     *sql.DB
	events *eventBroker
}

type Conversation struct {
	ID              string `json:"id"`
	UserID          int64  `json:"user_id"`
	Subject         string `json:"subject"`
	Category        string `json:"category"`
	Priority        int    `json:"priority"`
	Status          int    `json:"status"`
	AssignedAdminID int64  `json:"assigned_admin_id"`
	LastMessageAt   int64  `json:"last_message_at"`
	CreatedAt       int64  `json:"created_at"`
}

type Message struct {
	ID              string `json:"id"`
	ConversationID  string `json:"conversation_id"`
	SenderType      int    `json:"sender_type"`
	SenderID        int64  `json:"sender_id"`
	ClientMessageID string `json:"client_message_id"`
	MessageType     int    `json:"message_type"`
	TextContent     string `json:"text_content"`
	AssetID         int64  `json:"asset_id"`
	Status          int    `json:"status"`
	CreatedAt       int64  `json:"created_at"`
}

type SendRequest struct {
	ConversationID  string
	UserID          int64
	ClientMessageID string
	MessageType     int
	TextContent     string
	AssetID         int64
}

func New(db *sql.DB) *Service {
	return &Service{db: db, events: newEventBroker()}
}

func (s *Service) Subscribe() (<-chan Event, func()) {
	return s.events.subscribe()
}

func (s *Service) Notify(event Event) {
	s.events.publish(event)
}

func (s *Service) Current(ctx context.Context, userID int64) (Conversation, error) {
	conversation, err := s.current(ctx, userID)
	if err == nil {
		return conversation, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, err
	}
	conversationID, err := idgen.New()
	if err != nil {
		return Conversation{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO support_conversations(id,user_id,subject,category,priority,status)
		VALUES(?,?,'在线客服','general',1,0)`,
		conversationID, userID,
	)
	if err != nil {
		var mysqlError *mysql.MySQLError
		if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
			return Conversation{}, err
		}
	} else {
		s.Notify(Event{Type: "conversation.created", ConversationID: conversationID, UserID: userID})
	}
	return s.current(ctx, userID)
}

func (s *Service) current(ctx context.Context, userID int64) (Conversation, error) {
	var conversation Conversation
	err := s.db.QueryRowContext(ctx, `
		SELECT id,user_id,subject,category,priority,status,assigned_admin_id,
		       CAST(UNIX_TIMESTAMP(last_message_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(created_at) AS UNSIGNED)
		FROM support_conversations
		WHERE user_id=? AND status IN (0,1)
		ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(
		&conversation.ID, &conversation.UserID, &conversation.Subject,
		&conversation.Category, &conversation.Priority, &conversation.Status,
		&conversation.AssignedAdminID, &conversation.LastMessageAt, &conversation.CreatedAt,
	)
	return conversation, err
}

func (s *Service) Messages(
	ctx context.Context,
	userID int64,
	conversationID string,
	beforeID string,
	limit int,
) ([]Message, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if err := s.authorize(ctx, userID, conversationID, false); err != nil {
		return nil, err
	}
	query := `
		SELECT id,conversation_id,sender_type,sender_id,client_message_id,
		       message_type,text_content,asset_id,status,
		       CAST(UNIX_TIMESTAMP(created_at) AS UNSIGNED)
		FROM support_messages
		WHERE conversation_id=? AND status=1`
	arguments := []any{conversationID}
	if strings.TrimSpace(beforeID) != "" {
		query += ` AND id<?`
		arguments = append(arguments, strings.TrimSpace(beforeID))
	}
	query += ` ORDER BY id DESC LIMIT ?`
	arguments = append(arguments, limit)
	rows, err := s.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Message, 0, limit)
	for rows.Next() {
		var item Message
		if err = rows.Scan(
			&item.ID, &item.ConversationID, &item.SenderType, &item.SenderID,
			&item.ClientMessageID, &item.MessageType, &item.TextContent,
			&item.AssetID, &item.Status, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Send(ctx context.Context, request SendRequest) (Message, error) {
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.ClientMessageID = strings.TrimSpace(request.ClientMessageID)
	request.TextContent = strings.TrimSpace(request.TextContent)
	if request.ConversationID == "" || request.UserID < 1 ||
		request.ClientMessageID == "" || len(request.ClientMessageID) > 100 ||
		request.MessageType < 1 || request.MessageType > 3 ||
		(request.TextContent == "" && request.AssetID < 1) ||
		len(request.TextContent) > 5000 {
		return Message{}, ErrInvalidMessage
	}
	if err := s.authorize(ctx, request.UserID, request.ConversationID, true); err != nil {
		return Message{}, err
	}
	messageID, err := idgen.New()
	if err != nil {
		return Message{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO support_messages(
			id,conversation_id,sender_type,sender_id,client_message_id,
			message_type,text_content,asset_id,status
		) VALUES(?,?,1,?,?,?,?,?,1)`,
		messageID, request.ConversationID, request.UserID, request.ClientMessageID,
		request.MessageType, request.TextContent, request.AssetID,
	)
	if err != nil {
		var mysqlError *mysql.MySQLError
		if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
			return Message{}, err
		}
	}
	if _, err = s.db.ExecContext(ctx, `
		UPDATE support_conversations
		SET status=IF(assigned_admin_id=0,0,1),last_message_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND user_id=? AND status IN (0,1)`,
		request.ConversationID, request.UserID,
	); err != nil {
		return Message{}, err
	}
	message, err := s.messageByClientID(ctx, request.UserID, request.ClientMessageID)
	if err == nil {
		s.Notify(Event{
			Type: "message.created", ConversationID: request.ConversationID,
			UserID: request.UserID,
		})
	}
	return message, err
}

func (s *Service) authorize(
	ctx context.Context,
	userID int64,
	conversationID string,
	requireOpen bool,
) error {
	var status int
	err := s.db.QueryRowContext(ctx, `
		SELECT status FROM support_conversations WHERE id=? AND user_id=?`,
		conversationID, userID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConversationNotFound
	}
	if err != nil {
		return err
	}
	if requireOpen && status > 1 {
		return ErrConversationClosed
	}
	return nil
}

func (s *Service) messageByClientID(
	ctx context.Context,
	userID int64,
	clientMessageID string,
) (Message, error) {
	var item Message
	err := s.db.QueryRowContext(ctx, `
		SELECT id,conversation_id,sender_type,sender_id,client_message_id,
		       message_type,text_content,asset_id,status,
		       CAST(UNIX_TIMESTAMP(created_at) AS UNSIGNED)
		FROM support_messages
		WHERE sender_type=1 AND sender_id=? AND client_message_id=?`,
		userID, clientMessageID,
	).Scan(
		&item.ID, &item.ConversationID, &item.SenderType, &item.SenderID,
		&item.ClientMessageID, &item.MessageType, &item.TextContent,
		&item.AssetID, &item.Status, &item.CreatedAt,
	)
	return item, err
}
