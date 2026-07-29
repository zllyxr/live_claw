package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

func (h *Handler) listSupportConversations(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	status := -1
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		status, _ = strconv.Atoi(raw)
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT conversation.id,conversation.user_id,
		       COALESCE(NULLIF(user.nickname,''),user.username),
		       conversation.subject,conversation.category,conversation.priority,
		       conversation.status,conversation.assigned_admin_id,
		       COALESCE(NULLIF(admin.display_name,''),admin.username,''),
		       conversation.last_message_at,conversation.created_at,
		       (SELECT COUNT(*) FROM support_messages message
		        WHERE message.conversation_id=conversation.id AND message.status=1)
		FROM support_conversations conversation
		JOIN users user ON user.id=conversation.user_id
		LEFT JOIN admin_users admin ON admin.id=conversation.assigned_admin_id
		WHERE (? < 0 OR conversation.status=?)
		  AND (?='' OR conversation.id LIKE ? OR user.username LIKE ?
		       OR user.nickname LIKE ? OR conversation.subject LIKE ?)
		ORDER BY
		  CASE conversation.status WHEN 0 THEN 0 WHEN 1 THEN 1 ELSE 2 END,
		  conversation.priority DESC,conversation.last_message_at DESC
		LIMIT ? OFFSET ?`,
		status, status, keyword, like, like, like, like, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取客服会话失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, username, subject, category, assigneeName string
		var userID, assignedAdminID, messageCount int64
		var priority, conversationStatus int
		var lastMessageAt, createdAt time.Time
		if err = rows.Scan(
			&id, &userID, &username, &subject, &category, &priority,
			&conversationStatus, &assignedAdminID, &assigneeName,
			&lastMessageAt, &createdAt, &messageCount,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取客服会话失败")
			return
		}
		items = append(items, map[string]any{
			"id": id, "user_id": userID, "username": username, "subject": subject,
			"category": category, "priority": priority, "status": conversationStatus,
			"assigned_admin_id": assignedAdminID, "assignee_name": assigneeName,
			"message_count": messageCount, "last_message_at": lastMessageAt.Unix(),
			"created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取客服会话失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "items": items,
	})
}

func (h *Handler) listSupportMessages(w http.ResponseWriter, r *http.Request) {
	conversationID := strings.TrimSpace(r.PathValue("id"))
	if conversationID == "" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "客服会话编号无效")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT message.id,message.sender_type,message.sender_id,
		       CASE message.sender_type
		         WHEN 1 THEN COALESCE(NULLIF(user.nickname,''),user.username,'用户')
		         WHEN 2 THEN COALESCE(NULLIF(admin.display_name,''),admin.username,'客服')
		         ELSE '系统'
		       END,
		       message.message_type,message.text_content,message.asset_id,
		       message.status,message.created_at
		FROM support_messages message
		LEFT JOIN users user ON message.sender_type=1 AND user.id=message.sender_id
		LEFT JOIN admin_users admin ON message.sender_type=2 AND admin.id=message.sender_id
		WHERE message.conversation_id=?
		ORDER BY message.created_at ASC,message.id ASC
		LIMIT 500`,
		conversationID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取客服消息失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 64)
	for rows.Next() {
		var id, senderName, textContent string
		var senderType, messageType, messageStatus int
		var senderID, assetID int64
		var createdAt time.Time
		if err = rows.Scan(
			&id, &senderType, &senderID, &senderName, &messageType,
			&textContent, &assetID, &messageStatus, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取客服消息失败")
			return
		}
		items = append(items, map[string]any{
			"id": id, "sender_type": senderType, "sender_id": senderID,
			"sender_name": senderName, "message_type": messageType,
			"text_content": textContent, "asset_id": assetID,
			"status": messageStatus, "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取客服消息失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) replySupportConversation(w http.ResponseWriter, r *http.Request) {
	conversationID := strings.TrimSpace(r.PathValue("id"))
	var request struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Content = strings.TrimSpace(request.Content)
	if conversationID == "" || request.Content == "" || len(request.Content) > 5000 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "客服回复内容无效")
		return
	}
	adminUser, _ := adminFromRequest(r)
	messageID, err := idgen.New()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成客服消息失败")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发送客服回复失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var previousStatus int
	err = tx.QueryRowContext(r.Context(), `
		SELECT status FROM support_conversations WHERE id=? FOR UPDATE`,
		conversationID,
	).Scan(&previousStatus)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "客服会话不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发送客服回复失败")
		return
	}
	if previousStatus > 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "已结束的客服会话不能回复")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO support_messages
			(id,conversation_id,sender_type,sender_id,client_message_id,
			 message_type,text_content,asset_id,status)
		VALUES(?,?,2,?,?,1,?,0,1)`,
		messageID, conversationID, adminUser.ID, "admin_"+messageID, request.Content,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发送客服回复失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE support_conversations
		SET status=1,assigned_admin_id=?,last_message_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		adminUser.ID, conversationID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发送客服回复失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "support.reply", "support_conversation", conversationID,
		map[string]any{"status": previousStatus},
		map[string]any{"status": 1, "message_id": messageID},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录客服操作失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发送客服回复失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": messageID, "conversation_id": conversationID, "status": 1,
	})
}

func (h *Handler) updateSupportConversation(w http.ResponseWriter, r *http.Request) {
	conversationID := strings.TrimSpace(r.PathValue("id"))
	var request struct {
		Status   int `json:"status"`
		Priority int `json:"priority"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if conversationID == "" || request.Status < 0 || request.Status > 3 ||
		request.Priority < 1 || request.Priority > 3 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "客服会话状态无效")
		return
	}
	adminUser, _ := adminFromRequest(r)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新客服会话失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var beforeStatus, beforePriority int
	var beforeAssignee int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT status,priority,assigned_admin_id
		FROM support_conversations WHERE id=? FOR UPDATE`,
		conversationID,
	).Scan(&beforeStatus, &beforePriority, &beforeAssignee)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "客服会话不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新客服会话失败")
		return
	}
	assignee := beforeAssignee
	if request.Status == 1 {
		assignee = adminUser.ID
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE support_conversations
		SET status=?,priority=?,assigned_admin_id=?,
		    resolved_at=IF(?=2,CURRENT_TIMESTAMP(3),NULL),
		    closed_at=IF(?=3,CURRENT_TIMESTAMP(3),NULL)
		WHERE id=?`,
		request.Status, request.Priority, assignee,
		request.Status, request.Status, conversationID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新客服会话失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "support.conversation.update", "support_conversation", conversationID,
		map[string]any{"status": beforeStatus, "priority": beforePriority, "assigned_admin_id": beforeAssignee},
		map[string]any{"status": request.Status, "priority": request.Priority, "assigned_admin_id": assignee},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录客服操作失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新客服会话失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": conversationID, "status": request.Status, "priority": request.Priority,
		"assigned_admin_id": assignee,
	})
}
