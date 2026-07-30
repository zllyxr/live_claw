package supportconsole

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/support"
)

var (
	ErrAgentNotFound       = errors.New("support agent not found")
	ErrPermissionDenied    = errors.New("support permission denied")
	ErrConversationMissing = errors.New("support conversation not found")
	ErrConversationClaimed = errors.New("support conversation already claimed")
	ErrConversationClosed  = errors.New("support conversation closed")
	ErrInvalidRequest      = errors.New("invalid support console request")
	ErrAgentCapacity       = errors.New("support agent active conversation limit reached")
)

type Service struct {
	db      *sql.DB
	support *support.Service
}

type Agent struct {
	ID           int64  `json:"id,string"`
	AgentNo      string `json:"agent_no"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Role         int    `json:"role"`
	RoleName     string `json:"role_name"`
	Presence     int    `json:"presence"`
	MaxActive    int    `json:"max_active"`
	SupportOnly  bool   `json:"support_only"`
	IsSupervisor bool   `json:"is_supervisor"`
}

type Dashboard struct {
	Waiting       int64 `json:"waiting"`
	Mine          int64 `json:"mine"`
	Urgent        int64 `json:"urgent"`
	ResolvedToday int64 `json:"resolved_today"`
	OnlineAgents  int64 `json:"online_agents"`
}

type Conversation struct {
	ID                 string `json:"id"`
	UserID             int64  `json:"user_id,string"`
	Username           string `json:"username"`
	Nickname           string `json:"nickname"`
	AvatarURL          string `json:"avatar_url"`
	Subject            string `json:"subject"`
	Category           string `json:"category"`
	Priority           int    `json:"priority"`
	Status             int    `json:"status"`
	AssignedAgentID    int64  `json:"assigned_agent_id,string"`
	AssignedAgentName  string `json:"assigned_agent_name"`
	LastMessagePreview string `json:"last_message_preview"`
	UnreadCount        int64  `json:"unread_count"`
	LastMessageAt      int64  `json:"last_message_at"`
	CreatedAt          int64  `json:"created_at"`
}

type UserCard struct {
	ID           int64  `json:"id,string"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	AvatarURL    string `json:"avatar_url"`
	Gender       int    `json:"gender"`
	Signature    string `json:"signature"`
	Status       int    `json:"status"`
	IsVirtual    bool   `json:"is_virtual"`
	CountryCode  string `json:"country_code"`
	RegisteredAt int64  `json:"registered_at"`
}

type Message struct {
	ID              string `json:"id"`
	ConversationID  string `json:"conversation_id"`
	SenderType      int    `json:"sender_type"`
	SenderID        int64  `json:"sender_id,string"`
	SenderName      string `json:"sender_name"`
	ClientMessageID string `json:"client_message_id"`
	MessageType     int    `json:"message_type"`
	TextContent     string `json:"text_content"`
	AssetID         int64  `json:"asset_id,string"`
	AssetURL        string `json:"asset_url"`
	MimeType        string `json:"mime_type"`
	Status          int    `json:"status"`
	CreatedAt       int64  `json:"created_at"`
}

type Note struct {
	ID        int64  `json:"id,string"`
	UserID    int64  `json:"user_id,string"`
	AgentID   int64  `json:"agent_id,string"`
	AgentName string `json:"agent_name"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
}

type QuickReply struct {
	ID       int64  `json:"id,string"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Category string `json:"category"`
}

type ActionMeta struct {
	RequestID string
	IP        string
	UserAgent string
}

type SendRequest struct {
	ClientMessageID string
	MessageType     int
	TextContent     string
	AssetID         int64
}

func NewService(db *sql.DB, supportService *support.Service) *Service {
	return &Service{db: db, support: supportService}
}

func (s *Service) Agent(ctx context.Context, admin adminauth.Admin) (Agent, error) {
	var agent Agent
	var status int
	err := s.db.QueryRowContext(ctx, `
		SELECT support_agent.admin_user_id,support_agent.agent_no,
		       admin.username,COALESCE(NULLIF(admin.display_name,''),admin.username),
		       support_agent.agent_role,support_agent.status,support_agent.presence,
		       support_agent.max_active,support_agent.support_only
		FROM support_agents support_agent
		JOIN admin_users admin ON admin.id=support_agent.admin_user_id AND admin.status=1
		WHERE support_agent.admin_user_id=?`,
		admin.ID,
	).Scan(
		&agent.ID, &agent.AgentNo, &agent.Username, &agent.DisplayName,
		&agent.Role, &status, &agent.Presence, &agent.MaxActive, &agent.SupportOnly,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Agent{}, ErrAgentNotFound
	}
	if err != nil {
		return Agent{}, err
	}
	if status != 1 ||
		!hasPermission(admin, "support.console") ||
		!hasPermission(admin, "support.read") ||
		!hasPermission(admin, "support.write") {
		return Agent{}, ErrAgentNotFound
	}
	agent.IsSupervisor = agent.Role == 2 && hasPermission(admin, "support.supervise")
	if agent.IsSupervisor {
		agent.RoleName = "客服主管"
	} else {
		agent.RoleName = "客服座席"
	}
	if agent.Presence == 0 {
		agent.Presence = 1
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE support_agents
		SET presence=IF(presence=0,1,presence),last_seen_at=CURRENT_TIMESTAMP(3)
		WHERE admin_user_id=?`,
		agent.ID,
	)
	return agent, nil
}

func (s *Service) SetPresence(ctx context.Context, agent Agent, presence int) error {
	if presence < 0 || presence > 3 {
		return ErrInvalidRequest
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE support_agents SET presence=?,last_seen_at=CURRENT_TIMESTAMP(3)
		WHERE admin_user_id=? AND status=1`,
		presence, agent.ID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ErrAgentNotFound
	}
	return nil
}

func (s *Service) canObserveEvent(
	ctx context.Context,
	agent Agent,
	event support.Event,
) bool {
	if agent.IsSupervisor || strings.TrimSpace(event.ConversationID) == "" {
		return true
	}
	conversation, err := s.conversation(ctx, agent.ID, event.ConversationID)
	return err == nil && canRead(agent, conversation)
}

func (s *Service) Dashboard(ctx context.Context, agent Agent) (Dashboard, error) {
	var result Dashboard
	err := s.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM support_conversations conversation
		   JOIN users user ON user.id=conversation.user_id
		   WHERE conversation.status=0 AND conversation.assigned_admin_id=0),
		  (SELECT COUNT(*) FROM support_conversations
		   WHERE assigned_admin_id=? AND status IN (0,1)),
		  (SELECT COUNT(*) FROM support_conversations
		   WHERE priority=3 AND status IN (0,1)
		     AND (assigned_admin_id=0 OR assigned_admin_id=?)),
		  (SELECT COUNT(*) FROM support_conversations
		   WHERE assigned_admin_id=? AND status=2
		     AND resolved_at>=CURRENT_DATE AND resolved_at<CURRENT_DATE+INTERVAL 1 DAY),
		  (SELECT COUNT(*) FROM support_agents
		   WHERE status=1 AND presence IN (1,3)
		     AND last_seen_at>CURRENT_TIMESTAMP(3)-INTERVAL 10 MINUTE)`,
		agent.ID, agent.ID, agent.ID,
	).Scan(
		&result.Waiting, &result.Mine, &result.Urgent,
		&result.ResolvedToday, &result.OnlineAgents,
	)
	return result, err
}

func (s *Service) Conversations(
	ctx context.Context,
	agent Agent,
	scope string,
	keyword string,
) ([]Conversation, error) {
	items, _, err := s.ConversationsPage(ctx, agent, scope, keyword, 1, 100)
	return items, err
}

func (s *Service) ConversationsPage(
	ctx context.Context,
	agent Agent,
	scope string,
	keyword string,
	page int,
	pageSize int,
) ([]Conversation, int64, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	keyword = strings.TrimSpace(keyword)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 100 {
		pageSize = 100
	}
	where := ""
	arguments := make([]any, 0, 6)
	switch scope {
	case "", "queue":
		where = "conversation.status=0 AND conversation.assigned_admin_id=0"
	case "mine":
		where = "conversation.assigned_admin_id=? AND conversation.status IN (0,1)"
		arguments = append(arguments, agent.ID)
	case "history":
		where = "conversation.assigned_admin_id=? AND conversation.status IN (2,3)"
		arguments = append(arguments, agent.ID)
	case "all":
		if !agent.IsSupervisor {
			return nil, 0, ErrPermissionDenied
		}
		where = "1=1"
	default:
		return nil, 0, ErrInvalidRequest
	}
	if keyword != "" {
		where += ` AND (conversation.id LIKE ? OR user.username LIKE ?
			OR user.nickname LIKE ? OR conversation.subject LIKE ?)`
		like := "%" + escapeLike(keyword) + "%"
		arguments = append(arguments, like, like, like, like)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM support_conversations conversation
		JOIN users user ON user.id=conversation.user_id
		WHERE `+where,
		arguments...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `
		SELECT conversation.id,conversation.user_id,user.username,
		       COALESCE(NULLIF(user.nickname,''),user.username),
		       COALESCE(CONCAT(avatar.bucket,'/',avatar.object_key),''),
		       conversation.subject,conversation.category,conversation.priority,
		       conversation.status,conversation.assigned_admin_id,
		       COALESCE(NULLIF(assigned.display_name,''),assigned.username,''),
		       COALESCE((
		         SELECT NULLIF(message.text_content,'')
		         FROM support_messages message
		         WHERE message.conversation_id=conversation.id AND message.status=1
		         ORDER BY message.created_at DESC,message.id DESC LIMIT 1
		       ),''),
		       (
		         SELECT COUNT(*) FROM support_messages unread
		         WHERE unread.conversation_id=conversation.id
		           AND unread.sender_type=1 AND unread.status=1
		           AND unread.id>COALESCE(read_state.last_read_message_id,'')
		       ),
		       CAST(UNIX_TIMESTAMP(conversation.last_message_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(conversation.created_at) AS UNSIGNED)
		FROM support_conversations conversation
		JOIN users user ON user.id=conversation.user_id
		LEFT JOIN media_assets avatar ON avatar.id=user.avatar_asset_id AND avatar.status=1
		LEFT JOIN admin_users assigned ON assigned.id=conversation.assigned_admin_id
		LEFT JOIN support_conversation_reads read_state
		  ON read_state.conversation_id=conversation.id AND read_state.admin_user_id=?
		WHERE ` + where + `
		ORDER BY
		  CASE conversation.status WHEN 0 THEN 0 WHEN 1 THEN 1 ELSE 2 END,
		  conversation.priority DESC,conversation.last_message_at DESC,conversation.id DESC
		LIMIT ? OFFSET ?`
	arguments = append([]any{agent.ID}, arguments...)
	arguments = append(arguments, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Conversation, 0, 32)
	for rows.Next() {
		var item Conversation
		if err = scanConversation(rows, &item); err != nil {
			return nil, 0, err
		}
		item.AvatarURL = mediaURL(item.AvatarURL)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Service) Conversation(
	ctx context.Context,
	agent Agent,
	conversationID string,
) (Conversation, UserCard, []Note, error) {
	item, err := s.conversation(ctx, agent.ID, conversationID)
	if err != nil {
		return Conversation{}, UserCard{}, nil, err
	}
	if !canRead(agent, item) {
		return Conversation{}, UserCard{}, nil, ErrPermissionDenied
	}
	var user UserCard
	err = s.db.QueryRowContext(ctx, `
		SELECT user.id,user.username,COALESCE(NULLIF(user.nickname,''),user.username),
		       COALESCE(CONCAT(avatar.bucket,'/',avatar.object_key),''),
		       user.gender,user.signature,user.status,user.is_virtual,user.country_code,
		       CAST(UNIX_TIMESTAMP(user.created_at) AS UNSIGNED)
		FROM users user
		LEFT JOIN media_assets avatar ON avatar.id=user.avatar_asset_id AND avatar.status=1
		WHERE user.id=?`,
		item.UserID,
	).Scan(
		&user.ID, &user.Username, &user.Nickname, &user.AvatarURL,
		&user.Gender, &user.Signature, &user.Status, &user.IsVirtual,
		&user.CountryCode, &user.RegisteredAt,
	)
	if err != nil {
		return Conversation{}, UserCard{}, nil, err
	}
	user.AvatarURL = mediaURL(user.AvatarURL)
	notes, err := s.notes(ctx, item.UserID)
	return item, user, notes, err
}

func (s *Service) Messages(
	ctx context.Context,
	agent Agent,
	conversationID string,
	beforeID string,
	limit int,
) ([]Message, error) {
	items, _, _, err := s.MessagesPage(
		ctx, agent, conversationID, beforeID, limit, "",
	)
	return items, err
}

func (s *Service) MessagesPage(
	ctx context.Context,
	agent Agent,
	conversationID string,
	beforeID string,
	limit int,
	keyword string,
) ([]Message, int64, bool, error) {
	conversation, err := s.conversation(ctx, agent.ID, conversationID)
	if err != nil {
		return nil, 0, false, err
	}
	if !canRead(agent, conversation) {
		return nil, 0, false, ErrPermissionDenied
	}
	if limit < 1 || limit > 100 {
		limit = 60
	}
	keyword = strings.TrimSpace(keyword)
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		conversationID,
		keyword, like, like, like, like, like, like,
	}
	var total int64
	if err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM support_messages message
		LEFT JOIN users user ON message.sender_type=1 AND user.id=message.sender_id
		LEFT JOIN admin_users admin ON message.sender_type=2 AND admin.id=message.sender_id
		WHERE message.conversation_id=? AND message.status=1
		  AND (?='' OR message.id LIKE ? OR message.text_content LIKE ?
		       OR user.username LIKE ? OR user.nickname LIKE ?
		       OR admin.username LIKE ? OR admin.display_name LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		return nil, 0, false, err
	}
	query := `
		SELECT message.id,message.conversation_id,message.sender_type,message.sender_id,
		       CASE message.sender_type
		         WHEN 1 THEN COALESCE(NULLIF(user.nickname,''),user.username,'用户')
		         WHEN 2 THEN COALESCE(NULLIF(admin.display_name,''),admin.username,'客服')
		         ELSE '系统'
		       END,
		       message.client_message_id,message.message_type,message.text_content,
		       message.asset_id,COALESCE(CONCAT(asset.bucket,'/',asset.object_key),''),
		       COALESCE(asset.mime_type,''),message.status,
		       CAST(UNIX_TIMESTAMP(message.created_at) AS UNSIGNED)
		FROM support_messages message
		LEFT JOIN users user ON message.sender_type=1 AND user.id=message.sender_id
		LEFT JOIN admin_users admin ON message.sender_type=2 AND admin.id=message.sender_id
		LEFT JOIN media_assets asset ON asset.id=message.asset_id AND asset.status=1
		WHERE message.conversation_id=? AND message.status=1
		  AND (?='' OR message.id LIKE ? OR message.text_content LIKE ?
		       OR user.username LIKE ? OR user.nickname LIKE ?
		       OR admin.username LIKE ? OR admin.display_name LIKE ?)`
	arguments := append([]any{}, filterArguments...)
	if strings.TrimSpace(beforeID) != "" {
		query += " AND message.id<?"
		arguments = append(arguments, strings.TrimSpace(beforeID))
	}
	query += " ORDER BY message.id DESC LIMIT ?"
	arguments = append(arguments, limit+1)
	rows, err := s.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()
	items := make([]Message, 0, limit+1)
	lastReadID := ""
	for rows.Next() {
		var item Message
		if err = rows.Scan(
			&item.ID, &item.ConversationID, &item.SenderType, &item.SenderID,
			&item.SenderName, &item.ClientMessageID, &item.MessageType,
			&item.TextContent, &item.AssetID, &item.AssetURL, &item.MimeType,
			&item.Status, &item.CreatedAt,
		); err != nil {
			return nil, 0, false, err
		}
		item.AssetURL = mediaURL(item.AssetURL)
		if item.ID > lastReadID {
			lastReadID = item.ID
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	if lastReadID != "" {
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO support_conversation_reads
				(conversation_id,admin_user_id,last_read_message_id,read_at)
			VALUES(?,?,?,CURRENT_TIMESTAMP(3))
			ON DUPLICATE KEY UPDATE
				last_read_message_id=IF(last_read_message_id<VALUES(last_read_message_id),
					VALUES(last_read_message_id),last_read_message_id),
				read_at=CURRENT_TIMESTAMP(3)`,
			conversationID, agent.ID, lastReadID,
		)
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, total, hasMore, nil
}

func (s *Service) Claim(
	ctx context.Context,
	agent Agent,
	conversationID string,
	meta ActionMeta,
) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var maxActive int
	err = tx.QueryRowContext(ctx, `
		SELECT max_active FROM support_agents
		WHERE admin_user_id=? AND status=1
		FOR UPDATE`,
		agent.ID,
	).Scan(&maxActive)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAgentNotFound
	}
	if err != nil {
		return err
	}
	var status int
	var assigned int64
	err = tx.QueryRowContext(ctx, `
		SELECT status,assigned_admin_id FROM support_conversations
		WHERE id=? FOR UPDATE`,
		conversationID,
	).Scan(&status, &assigned)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConversationMissing
	}
	if err != nil {
		return err
	}
	if status == 1 && assigned == agent.ID {
		return nil
	}
	if status != 0 || assigned != 0 {
		return ErrConversationClaimed
	}
	var active int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM support_conversations
		WHERE assigned_admin_id=? AND status IN (0,1)`,
		agent.ID,
	).Scan(&active); err != nil {
		return err
	}
	if active >= maxActive {
		return ErrAgentCapacity
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE support_conversations
		SET status=1,assigned_admin_id=?,assigned_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND status=0 AND assigned_admin_id=0`,
		agent.ID, conversationID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ErrConversationClaimed
	}
	if err = insertSystemMessage(ctx, tx, conversationID, agent.DisplayName+" 已接入会话"); err != nil {
		return err
	}
	if err = auditAction(
		ctx, tx, agent.ID, "support.claim", conversationID, meta,
		map[string]any{"status": 0, "assigned_agent_id": 0},
		map[string]any{"status": 1, "assigned_agent_id": agent.ID},
	); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.support.Notify(support.Event{Type: "conversation.claimed", ConversationID: conversationID})
	return nil
}

func (s *Service) Send(
	ctx context.Context,
	agent Agent,
	conversationID string,
	request SendRequest,
	meta ActionMeta,
) (Message, error) {
	conversationID = strings.TrimSpace(conversationID)
	request.ClientMessageID = strings.TrimSpace(request.ClientMessageID)
	request.TextContent = strings.TrimSpace(request.TextContent)
	if conversationID == "" || request.ClientMessageID == "" ||
		len(request.ClientMessageID) > 100 || request.MessageType < 1 ||
		request.MessageType > 3 || len(request.TextContent) > 5000 ||
		(request.TextContent == "" && request.AssetID < 1) {
		return Message{}, ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var status int
	var assignedAgentID int64
	err = tx.QueryRowContext(ctx, `
		SELECT status,assigned_admin_id FROM support_conversations
		WHERE id=? FOR UPDATE`,
		conversationID,
	).Scan(&status, &assignedAgentID)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrConversationMissing
	}
	if err != nil {
		return Message{}, err
	}
	if status > 1 {
		return Message{}, ErrConversationClosed
	}
	if assignedAgentID != agent.ID {
		return Message{}, ErrPermissionDenied
	}
	if request.AssetID > 0 {
		var mediaType string
		if err = tx.QueryRowContext(ctx, `
			SELECT media_type FROM media_assets WHERE id=? AND status=1`,
			request.AssetID,
		).Scan(&mediaType); errors.Is(err, sql.ErrNoRows) {
			return Message{}, ErrInvalidRequest
		} else if err != nil {
			return Message{}, err
		}
		if request.MessageType == 2 && mediaType != "image" {
			return Message{}, ErrInvalidRequest
		}
	}
	messageID, err := idgen.New()
	if err != nil {
		return Message{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO support_messages
			(id,conversation_id,sender_type,sender_id,client_message_id,
			 message_type,text_content,asset_id,status)
		VALUES(?,?,2,?,?,?,?,?,1)`,
		messageID, conversationID, agent.ID, request.ClientMessageID,
		request.MessageType, request.TextContent, request.AssetID,
	)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			_ = tx.Rollback()
			message, duplicateErr := s.messageByClientID(
				ctx, agent.ID, conversationID, request.ClientMessageID,
			)
			if errors.Is(duplicateErr, sql.ErrNoRows) {
				return Message{}, ErrInvalidRequest
			}
			return message, duplicateErr
		}
		return Message{}, err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE support_conversations
		SET status=1,last_message_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		conversationID,
	); err != nil {
		return Message{}, err
	}
	if err = auditAction(
		ctx, tx, agent.ID, "support.reply", conversationID, meta, nil,
		map[string]any{"message_id": messageID, "message_type": request.MessageType},
	); err != nil {
		return Message{}, err
	}
	if err = tx.Commit(); err != nil {
		return Message{}, err
	}
	s.support.Notify(support.Event{Type: "message.created", ConversationID: conversationID})
	return s.messageByClientID(ctx, agent.ID, conversationID, request.ClientMessageID)
}

func (s *Service) Resolve(
	ctx context.Context,
	agent Agent,
	conversationID string,
	meta ActionMeta,
) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var status int
	var assigned int64
	err = tx.QueryRowContext(ctx, `
		SELECT status,assigned_admin_id FROM support_conversations
		WHERE id=? FOR UPDATE`,
		conversationID,
	).Scan(&status, &assigned)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConversationMissing
	}
	if err != nil {
		return err
	}
	if status > 1 {
		return ErrConversationClosed
	}
	if assigned != agent.ID && !agent.IsSupervisor {
		return ErrPermissionDenied
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE support_conversations
		SET status=2,resolved_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		conversationID,
	); err != nil {
		return err
	}
	if err = insertSystemMessage(ctx, tx, conversationID, "会话已解决"); err != nil {
		return err
	}
	if err = auditAction(
		ctx, tx, agent.ID, "support.resolve", conversationID, meta,
		map[string]any{"status": status}, map[string]any{"status": 2},
	); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.support.Notify(support.Event{Type: "conversation.resolved", ConversationID: conversationID})
	return nil
}

func (s *Service) Transfer(
	ctx context.Context,
	agent Agent,
	conversationID string,
	targetAgentID int64,
	meta ActionMeta,
) error {
	if targetAgentID < 1 || targetAgentID == agent.ID {
		return ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var targetName string
	var targetMaxActive int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(NULLIF(admin.display_name,''),admin.username),
		       support_agent.max_active
		FROM support_agents support_agent
		JOIN admin_users admin
		  ON admin.id=support_agent.admin_user_id AND admin.status=1
		WHERE support_agent.admin_user_id=? AND support_agent.status=1
		  AND (
		    SELECT COUNT(DISTINCT permission.permission_key)
		    FROM admin_user_roles assignment
		    JOIN admin_roles role
		      ON role.id=assignment.role_id AND role.status=1
		    JOIN admin_role_permissions role_permission
		      ON role_permission.role_id=role.id
		    JOIN admin_permissions permission
		      ON permission.id=role_permission.permission_id
		    WHERE assignment.admin_user_id=support_agent.admin_user_id
		      AND permission.permission_key IN
		        ('support.console','support.read','support.write')
		  )=3
		FOR UPDATE`,
		targetAgentID,
	).Scan(&targetName, &targetMaxActive)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAgentNotFound
	}
	if err != nil {
		return err
	}
	var status int
	var assigned int64
	err = tx.QueryRowContext(ctx, `
		SELECT status,assigned_admin_id FROM support_conversations
		WHERE id=? FOR UPDATE`,
		conversationID,
	).Scan(&status, &assigned)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConversationMissing
	}
	if err != nil {
		return err
	}
	if status > 1 {
		return ErrConversationClosed
	}
	if assigned != agent.ID && !agent.IsSupervisor {
		return ErrPermissionDenied
	}
	if assigned == targetAgentID {
		return ErrInvalidRequest
	}
	var targetActive int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM support_conversations
		WHERE assigned_admin_id=? AND status IN (0,1)`,
		targetAgentID,
	).Scan(&targetActive); err != nil {
		return err
	}
	if targetActive >= targetMaxActive {
		return ErrAgentCapacity
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE support_conversations
		SET status=1,assigned_admin_id=?,assigned_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		targetAgentID, conversationID,
	); err != nil {
		return err
	}
	if err = insertSystemMessage(ctx, tx, conversationID, "会话已转接给 "+targetName); err != nil {
		return err
	}
	if err = auditAction(
		ctx, tx, agent.ID, "support.transfer", conversationID, meta,
		map[string]any{"assigned_agent_id": assigned},
		map[string]any{"assigned_agent_id": targetAgentID},
	); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.support.Notify(support.Event{Type: "conversation.transferred", ConversationID: conversationID})
	return nil
}

func (s *Service) SetPriority(
	ctx context.Context,
	agent Agent,
	conversationID string,
	priority int,
	meta ActionMeta,
) error {
	if priority < 1 || priority > 3 {
		return ErrInvalidRequest
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ErrInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var previousPriority, status int
	var assigned int64
	err = tx.QueryRowContext(ctx, `
		SELECT priority,status,assigned_admin_id
		FROM support_conversations WHERE id=? FOR UPDATE`,
		conversationID,
	).Scan(&previousPriority, &status, &assigned)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConversationMissing
	}
	if err != nil {
		return err
	}
	canModify := agent.IsSupervisor ||
		assigned == agent.ID ||
		(status == 0 && assigned == 0)
	if !canModify {
		return ErrPermissionDenied
	}
	if status > 1 {
		return ErrConversationClosed
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE support_conversations SET priority=? WHERE id=?`,
		priority, conversationID,
	); err != nil {
		return err
	}
	if err = auditAction(
		ctx, tx, agent.ID, "support.priority", conversationID, meta,
		map[string]any{"priority": previousPriority},
		map[string]any{"priority": priority},
	); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.support.Notify(support.Event{Type: "conversation.updated", ConversationID: conversationID})
	return nil
}

func (s *Service) Agents(ctx context.Context) ([]Agent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT support_agent.admin_user_id,support_agent.agent_no,
		       admin.username,COALESCE(NULLIF(admin.display_name,''),admin.username),
		       support_agent.agent_role,support_agent.presence,support_agent.max_active,
		       support_agent.support_only
			FROM support_agents support_agent
			JOIN admin_users admin ON admin.id=support_agent.admin_user_id AND admin.status=1
			WHERE support_agent.status=1
			  AND (
			    SELECT COUNT(DISTINCT permission.permission_key)
			    FROM admin_user_roles assignment
			    JOIN admin_roles role
			      ON role.id=assignment.role_id AND role.status=1
			    JOIN admin_role_permissions role_permission
			      ON role_permission.role_id=role.id
			    JOIN admin_permissions permission
			      ON permission.id=role_permission.permission_id
			    WHERE assignment.admin_user_id=support_agent.admin_user_id
			      AND permission.permission_key IN
			        ('support.console','support.read','support.write')
			  )=3
			ORDER BY support_agent.presence DESC,support_agent.agent_role DESC,admin.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Agent, 0, 8)
	for rows.Next() {
		var item Agent
		if err = rows.Scan(
			&item.ID, &item.AgentNo, &item.Username, &item.DisplayName,
			&item.Role, &item.Presence, &item.MaxActive, &item.SupportOnly,
		); err != nil {
			return nil, err
		}
		item.IsSupervisor = item.Role == 2
		if item.IsSupervisor {
			item.RoleName = "客服主管"
		} else {
			item.RoleName = "客服座席"
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) AgentsPage(
	ctx context.Context,
	keyword string,
	page int,
	pageSize int,
) ([]Agent, int64, error) {
	keyword = strings.TrimSpace(keyword)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 100 {
		pageSize = 100
	}
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{keyword, like, like, like}
	var total int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM support_agents support_agent
		JOIN admin_users admin ON admin.id=support_agent.admin_user_id AND admin.status=1
		WHERE support_agent.status=1
		  AND (
		    SELECT COUNT(DISTINCT permission.permission_key)
		    FROM admin_user_roles assignment
		    JOIN admin_roles role
		      ON role.id=assignment.role_id AND role.status=1
		    JOIN admin_role_permissions role_permission
		      ON role_permission.role_id=role.id
		    JOIN admin_permissions permission
		      ON permission.id=role_permission.permission_id
		    WHERE assignment.admin_user_id=support_agent.admin_user_id
		      AND permission.permission_key IN
		        ('support.console','support.read','support.write')
		  )=3
		  AND (?='' OR support_agent.agent_no LIKE ? OR admin.username LIKE ?
		       OR admin.display_name LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT support_agent.admin_user_id,support_agent.agent_no,
		       admin.username,COALESCE(NULLIF(admin.display_name,''),admin.username),
		       support_agent.agent_role,support_agent.presence,support_agent.max_active,
		       support_agent.support_only
		FROM support_agents support_agent
		JOIN admin_users admin ON admin.id=support_agent.admin_user_id AND admin.status=1
		WHERE support_agent.status=1
		  AND (
		    SELECT COUNT(DISTINCT permission.permission_key)
		    FROM admin_user_roles assignment
		    JOIN admin_roles role
		      ON role.id=assignment.role_id AND role.status=1
		    JOIN admin_role_permissions role_permission
		      ON role_permission.role_id=role.id
		    JOIN admin_permissions permission
		      ON permission.id=role_permission.permission_id
		    WHERE assignment.admin_user_id=support_agent.admin_user_id
		      AND permission.permission_key IN
		        ('support.console','support.read','support.write')
		  )=3
		  AND (?='' OR support_agent.agent_no LIKE ? OR admin.username LIKE ?
		       OR admin.display_name LIKE ?)
		ORDER BY support_agent.presence DESC,support_agent.agent_role DESC,admin.id
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Agent, 0, pageSize)
	for rows.Next() {
		var item Agent
		if err = rows.Scan(
			&item.ID, &item.AgentNo, &item.Username, &item.DisplayName,
			&item.Role, &item.Presence, &item.MaxActive, &item.SupportOnly,
		); err != nil {
			return nil, 0, err
		}
		item.IsSupervisor = item.Role == 2
		if item.IsSupervisor {
			item.RoleName = "客服主管"
		} else {
			item.RoleName = "客服座席"
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Service) QuickReplies(ctx context.Context) ([]QuickReply, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,title,content,category FROM support_quick_replies
		WHERE status=1 ORDER BY sort_order DESC,id ASC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]QuickReply, 0, 16)
	for rows.Next() {
		var item QuickReply
		if err = rows.Scan(&item.ID, &item.Title, &item.Content, &item.Category); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) QuickRepliesPage(
	ctx context.Context,
	keyword string,
	page int,
	pageSize int,
) ([]QuickReply, int64, error) {
	keyword = strings.TrimSpace(keyword)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 100 {
		pageSize = 100
	}
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{keyword, like, like, like}
	var total int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM support_quick_replies
		WHERE status=1
		  AND (?='' OR title LIKE ? OR content LIKE ? OR category LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,title,content,category
		FROM support_quick_replies
		WHERE status=1
		  AND (?='' OR title LIKE ? OR content LIKE ? OR category LIKE ?)
		ORDER BY sort_order DESC,id ASC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]QuickReply, 0, pageSize)
	for rows.Next() {
		var item QuickReply
		if err = rows.Scan(&item.ID, &item.Title, &item.Content, &item.Category); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Service) AddNote(
	ctx context.Context,
	agent Agent,
	userID int64,
	content string,
	meta ActionMeta,
) (Note, error) {
	content = strings.TrimSpace(content)
	if userID < 1 || content == "" || len(content) > 1000 {
		return Note{}, ErrInvalidRequest
	}
	var allowed int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM support_conversations
		WHERE user_id=? AND (
		  assigned_admin_id=? OR (?=1) OR (status=0 AND assigned_admin_id=0)
		) LIMIT 1`,
		userID, agent.ID, agent.IsSupervisor,
	).Scan(&allowed)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrPermissionDenied
	}
	if err != nil {
		return Note{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Note{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(ctx, `
		INSERT INTO support_user_notes(user_id,admin_user_id,content)
		VALUES(?,?,?)`,
		userID, agent.ID, content,
	)
	if err != nil {
		return Note{}, err
	}
	noteID, _ := result.LastInsertId()
	if err = auditAction(
		ctx, tx, agent.ID, "support.note.create", fmt.Sprint(userID), meta,
		nil, map[string]any{"note_id": noteID},
	); err != nil {
		return Note{}, err
	}
	if err = tx.Commit(); err != nil {
		return Note{}, err
	}
	return Note{
		ID: noteID, UserID: userID, AgentID: agent.ID,
		AgentName: agent.DisplayName, Content: content, CreatedAt: time.Now().Unix(),
	}, nil
}

func (s *Service) conversation(
	ctx context.Context,
	agentID int64,
	conversationID string,
) (Conversation, error) {
	var item Conversation
	err := s.db.QueryRowContext(ctx, `
		SELECT conversation.id,conversation.user_id,user.username,
		       COALESCE(NULLIF(user.nickname,''),user.username),
		       COALESCE(CONCAT(avatar.bucket,'/',avatar.object_key),''),
		       conversation.subject,conversation.category,conversation.priority,
		       conversation.status,conversation.assigned_admin_id,
		       COALESCE(NULLIF(assigned.display_name,''),assigned.username,''),
		       COALESCE((
		         SELECT NULLIF(message.text_content,'')
		         FROM support_messages message
		         WHERE message.conversation_id=conversation.id AND message.status=1
		         ORDER BY message.created_at DESC,message.id DESC LIMIT 1
		       ),''),
		       (
		         SELECT COUNT(*) FROM support_messages unread
		         WHERE unread.conversation_id=conversation.id
		           AND unread.sender_type=1 AND unread.status=1
		           AND unread.id>COALESCE(read_state.last_read_message_id,'')
		       ),
		       CAST(UNIX_TIMESTAMP(conversation.last_message_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(conversation.created_at) AS UNSIGNED)
		FROM support_conversations conversation
		JOIN users user ON user.id=conversation.user_id
		LEFT JOIN media_assets avatar ON avatar.id=user.avatar_asset_id AND avatar.status=1
		LEFT JOIN admin_users assigned ON assigned.id=conversation.assigned_admin_id
		LEFT JOIN support_conversation_reads read_state
		  ON read_state.conversation_id=conversation.id AND read_state.admin_user_id=?
		WHERE conversation.id=?`,
		agentID, strings.TrimSpace(conversationID),
	).Scan(conversationScanTargets(&item)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, ErrConversationMissing
	}
	if err != nil {
		return Conversation{}, err
	}
	item.AvatarURL = mediaURL(item.AvatarURL)
	return item, nil
}

func (s *Service) notes(ctx context.Context, userID int64) ([]Note, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT note.id,note.user_id,note.admin_user_id,
		       COALESCE(NULLIF(admin.display_name,''),admin.username,'客服'),
		       note.content,CAST(UNIX_TIMESTAMP(note.created_at) AS UNSIGNED)
		FROM support_user_notes note
		LEFT JOIN admin_users admin ON admin.id=note.admin_user_id
		WHERE note.user_id=? ORDER BY note.id DESC LIMIT 50`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Note, 0, 8)
	for rows.Next() {
		var item Note
		if err = rows.Scan(
			&item.ID, &item.UserID, &item.AgentID, &item.AgentName,
			&item.Content, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) messageByClientID(
	ctx context.Context,
	agentID int64,
	conversationID string,
	clientMessageID string,
) (Message, error) {
	var item Message
	err := s.db.QueryRowContext(ctx, `
		SELECT message.id,message.conversation_id,message.sender_type,message.sender_id,
		       COALESCE(NULLIF(admin.display_name,''),admin.username,'客服'),
		       message.client_message_id,message.message_type,message.text_content,
		       message.asset_id,COALESCE(CONCAT(asset.bucket,'/',asset.object_key),''),
		       COALESCE(asset.mime_type,''),message.status,
		       CAST(UNIX_TIMESTAMP(message.created_at) AS UNSIGNED)
		FROM support_messages message
		LEFT JOIN admin_users admin ON admin.id=message.sender_id
		LEFT JOIN media_assets asset ON asset.id=message.asset_id AND asset.status=1
		WHERE message.sender_type=2 AND message.sender_id=?
		  AND message.conversation_id=? AND message.client_message_id=?`,
		agentID, conversationID, clientMessageID,
	).Scan(
		&item.ID, &item.ConversationID, &item.SenderType, &item.SenderID,
		&item.SenderName, &item.ClientMessageID, &item.MessageType,
		&item.TextContent, &item.AssetID, &item.AssetURL, &item.MimeType,
		&item.Status, &item.CreatedAt,
	)
	item.AssetURL = mediaURL(item.AssetURL)
	return item, err
}

func scanConversation(scanner interface{ Scan(...any) error }, item *Conversation) error {
	return scanner.Scan(conversationScanTargets(item)...)
}

func conversationScanTargets(item *Conversation) []any {
	return []any{
		&item.ID, &item.UserID, &item.Username, &item.Nickname,
		&item.AvatarURL, &item.Subject, &item.Category, &item.Priority,
		&item.Status, &item.AssignedAgentID, &item.AssignedAgentName,
		&item.LastMessagePreview, &item.UnreadCount,
		&item.LastMessageAt, &item.CreatedAt,
	}
}

func canRead(agent Agent, conversation Conversation) bool {
	if agent.IsSupervisor {
		return true
	}
	if conversation.AssignedAgentID == agent.ID {
		return true
	}
	return conversation.Status == 0 && conversation.AssignedAgentID == 0
}

func hasPermission(admin adminauth.Admin, permission string) bool {
	for _, granted := range admin.Permissions {
		if granted == permission {
			return true
		}
	}
	return false
}

func insertSystemMessage(
	ctx context.Context,
	tx *sql.Tx,
	conversationID string,
	content string,
) error {
	messageID, err := idgen.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO support_messages
			(id,conversation_id,sender_type,sender_id,client_message_id,
			 message_type,text_content,asset_id,status)
		VALUES(?,?,3,0,?,1,?,0,1)`,
		messageID, conversationID, "system_"+messageID, content,
	)
	return err
}

func auditAction(
	ctx context.Context,
	tx *sql.Tx,
	agentID int64,
	action string,
	resourceID string,
	meta ActionMeta,
	before any,
	after any,
) error {
	if strings.TrimSpace(meta.RequestID) == "" {
		generated, err := idgen.New()
		if err != nil {
			return err
		}
		meta.RequestID = generated
	}
	beforeJSON, err := nullableJSON(before)
	if err != nil {
		return err
	}
	afterJSON, err := nullableJSON(after)
	if err != nil {
		return err
	}
	if len(meta.IP) > 45 {
		meta.IP = meta.IP[:45]
	}
	if len(meta.UserAgent) > 500 {
		meta.UserAgent = meta.UserAgent[:500]
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs
			(request_id,actor_type,actor_id,action,resource_type,resource_id,
			 before_data,after_data,ip,user_agent)
		VALUES(?,1,?,?,'support_conversation',?,?,?,?,?)`,
		meta.RequestID, agentID, action, resourceID,
		beforeJSON, afterJSON, meta.IP, meta.UserAgent,
	)
	return err
}

func nullableJSON(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func mediaURL(object string) string {
	object = strings.TrimLeft(strings.TrimSpace(object), "/")
	if object == "" {
		return ""
	}
	return "/" + object
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
