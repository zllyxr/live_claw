package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

func (h *Handler) listIMConversations(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	conversationType, _ := strconv.Atoi(r.URL.Query().Get("type"))
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT conversation.id,conversation.conversation_type,conversation.title,
		       conversation.message_seq,conversation.status,conversation.created_by,
		       conversation.created_at,conversation.updated_at,
		       COALESCE(group_info.group_no,''),COALESCE(group_info.owner_user_id,0),
		       COALESCE(group_info.member_count,
		                (SELECT COUNT(*) FROM im_conversation_members member
		                 WHERE member.conversation_id=conversation.id AND member.member_status=1)),
		       COALESCE(group_info.max_members,2),COALESCE(group_info.all_muted,0),
		       group_info.dissolved_at
		FROM im_conversations conversation
		LEFT JOIN im_groups group_info ON group_info.conversation_id=conversation.id
		WHERE (?=0 OR conversation.conversation_type=?)
		  AND (?='' OR conversation.id LIKE ? OR conversation.title LIKE ? OR group_info.group_no LIKE ?)
		ORDER BY conversation.updated_at DESC,conversation.id DESC
		LIMIT ? OFFSET ?`,
		conversationType, conversationType, keyword, like, like, like,
		pageSize, (page-1)*pageSize,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取 IM 会话失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, title, groupNo string
		var conversationTypeValue, status, maxMembers, allMuted int
		var messageSeq, createdBy, ownerUserID, memberCount int64
		var createdAt, updatedAt time.Time
		var dissolvedAt sql.NullTime
		if err = rows.Scan(
			&id, &conversationTypeValue, &title, &messageSeq, &status, &createdBy,
			&createdAt, &updatedAt, &groupNo, &ownerUserID, &memberCount,
			&maxMembers, &allMuted, &dissolvedAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取 IM 会话失败")
			return
		}
		items = append(items, map[string]any{
			"id": id, "conversation_type": conversationTypeValue, "title": title,
			"message_seq": messageSeq, "status": status, "created_by": createdBy,
			"group_no": groupNo, "owner_user_id": ownerUserID, "member_count": memberCount,
			"max_members": maxMembers, "all_muted": allMuted == 1,
			"dissolved_at": nullTime(dissolvedAt),
			"created_at":   createdAt.Unix(), "updated_at": updatedAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取 IM 会话失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "items": items,
	})
}

func (h *Handler) listIMMembers(w http.ResponseWriter, r *http.Request) {
	conversationID := strings.TrimSpace(r.PathValue("id"))
	if conversationID == "" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "会话编号无效")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT member.user_id,COALESCE(NULLIF(user.nickname,''),user.username),member.role,
		       member.member_status,member.mute_until,member.last_read_seq,
		       member.joined_at,member.left_at
		FROM im_conversation_members member
		LEFT JOIN users user ON user.id=member.user_id
		WHERE member.conversation_id=?
		ORDER BY member.member_status=1 DESC,member.role DESC,member.joined_at`,
		conversationID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取群成员失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 32)
	for rows.Next() {
		var userID, lastReadSeq int64
		var nickname string
		var role, status int
		var muteUntil, leftAt sql.NullTime
		var joinedAt time.Time
		if err = rows.Scan(
			&userID, &nickname, &role, &status, &muteUntil, &lastReadSeq, &joinedAt, &leftAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取群成员失败")
			return
		}
		items = append(items, map[string]any{
			"user_id": userID, "nickname": nickname, "role": role, "member_status": status,
			"mute_until": nullTime(muteUntil), "last_read_seq": lastReadSeq,
			"joined_at": joinedAt.Unix(), "left_at": nullTime(leftAt),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取群成员失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) listIMMessages(w http.ResponseWriter, r *http.Request) {
	conversationID := strings.TrimSpace(r.PathValue("id"))
	page, pageSize := pageParams(r)
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT message.id,message.sequence,message.sender_user_id,
		       COALESCE(NULLIF(user.nickname,''),user.username,'已注销用户'),
		       message.message_type,message.text_content,message.asset_id,message.metadata,
		       message.status,message.created_at,message.revoked_at
		FROM im_messages message
		LEFT JOIN users user ON user.id=message.sender_user_id
		WHERE message.conversation_id=?
		ORDER BY message.sequence DESC
		LIMIT ? OFFSET ?`,
		conversationID, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取 IM 消息失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, senderName, textContent string
		var sequence, senderUserID, assetID int64
		var messageType, status int
		var metadata []byte
		var createdAt time.Time
		var revokedAt sql.NullTime
		if err = rows.Scan(
			&id, &sequence, &senderUserID, &senderName, &messageType,
			&textContent, &assetID, &metadata, &status, &createdAt, &revokedAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取 IM 消息失败")
			return
		}
		items = append(items, map[string]any{
			"id": id, "sequence": sequence, "sender_user_id": senderUserID,
			"sender_name": senderName, "message_type": messageType, "text_content": textContent,
			"asset_id": assetID, "metadata": string(metadata), "status": status,
			"created_at": createdAt.Unix(), "revoked_at": nullTime(revokedAt),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取 IM 消息失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "items": items,
	})
}

func (h *Handler) moderateIMMember(w http.ResponseWriter, r *http.Request) {
	conversationID := strings.TrimSpace(r.PathValue("id"))
	userID, err := strconv.ParseInt(r.PathValue("user_id"), 10, 64)
	if err != nil || conversationID == "" || userID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "群成员参数无效")
		return
	}
	var request struct {
		Action          string `json:"action"`
		DurationSeconds int64  `json:"duration_seconds"`
		Reason          string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.Reason = strings.TrimSpace(request.Reason)
	if (request.Action != "mute" && request.Action != "unmute" && request.Action != "remove") ||
		len(request.Reason) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "群成员管理参数无效")
		return
	}
	if request.Action == "mute" && (request.DurationSeconds < 1 || request.DurationSeconds > 365*24*3600) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "禁言时长无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "管理群成员失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var role, status int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT role,member_status FROM im_conversation_members
		WHERE conversation_id=? AND user_id=? FOR UPDATE`,
		conversationID, userID,
	).Scan(&role, &status); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "群成员不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "管理群成员失败")
		return
	}
	if request.Action == "remove" && role >= 100 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "群主不能被直接移除，请先解散群组")
		return
	}
	var muteUntil any
	switch request.Action {
	case "mute":
		muteUntil = time.Now().Add(time.Duration(request.DurationSeconds) * time.Second)
		_, err = tx.ExecContext(r.Context(), `
			UPDATE im_conversation_members SET mute_until=?
			WHERE conversation_id=? AND user_id=? AND member_status=1`,
			muteUntil, conversationID, userID,
		)
	case "unmute":
		_, err = tx.ExecContext(r.Context(), `
			UPDATE im_conversation_members SET mute_until=NULL
			WHERE conversation_id=? AND user_id=? AND member_status=1`,
			conversationID, userID,
		)
	case "remove":
		_, err = tx.ExecContext(r.Context(), `
			UPDATE im_conversation_members
			SET member_status=3,left_at=CURRENT_TIMESTAMP(3),mute_until=NULL
			WHERE conversation_id=? AND user_id=? AND member_status=1`,
			conversationID, userID,
		)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `
				UPDATE im_groups
				SET member_count=(SELECT COUNT(*) FROM im_conversation_members
				                  WHERE conversation_id=? AND member_status=1)
				WHERE conversation_id=?`,
				conversationID, conversationID,
			)
		}
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "管理群成员失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO im_moderation_actions
			(conversation_id,target_user_id,action_type,reason,actor_type,actor_id)
		VALUES(?,?,?,?,1,?)`,
		conversationID, userID, request.Action, request.Reason, adminID(r),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录 IM 管理失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "im.member."+request.Action, "im_conversation", conversationID,
		map[string]any{"user_id": userID, "role": role, "status": status},
		map[string]any{"user_id": userID, "action": request.Action, "duration_seconds": request.DurationSeconds},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录 IM 审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "管理群成员失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"conversation_id": conversationID, "user_id": userID, "action": request.Action,
		"mute_until": muteUntil,
	})
}

func (h *Handler) updateIMGroup(w http.ResponseWriter, r *http.Request) {
	conversationID := strings.TrimSpace(r.PathValue("id"))
	var request struct {
		Action string `json:"action"`
		Value  bool   `json:"value"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.Reason = strings.TrimSpace(request.Reason)
	if conversationID == "" || (request.Action != "all_mute" && request.Action != "dissolve") ||
		len(request.Reason) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "群组管理参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "管理群组失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var dissolvedAt sql.NullTime
	var allMuted int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT all_muted,dissolved_at FROM im_groups WHERE conversation_id=? FOR UPDATE`,
		conversationID,
	).Scan(&allMuted, &dissolvedAt); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "群组不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "管理群组失败")
		return
	}
	if dissolvedAt.Valid {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "群组已解散")
		return
	}
	if request.Action == "all_mute" {
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE im_groups SET all_muted=? WHERE conversation_id=?`,
			request.Value, conversationID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "管理群组失败")
			return
		}
	} else {
		if !request.Value || request.Reason == "" {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "解散群组必须填写原因")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE im_groups SET dissolved_at=CURRENT_TIMESTAMP(3),member_count=0
			WHERE conversation_id=?`,
			conversationID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "解散群组失败")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE im_conversations SET status=0 WHERE id=?`,
			conversationID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "解散群组失败")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE im_conversation_members
			SET member_status=3,left_at=CURRENT_TIMESTAMP(3),mute_until=NULL
			WHERE conversation_id=? AND member_status=1`,
			conversationID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "解散群组失败")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO im_moderation_actions
			(conversation_id,action_type,reason,actor_type,actor_id)
		VALUES(?,?,?,1,?)`,
		conversationID, request.Action, request.Reason, adminID(r),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录 IM 管理失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "im.group."+request.Action, "im_conversation", conversationID,
		map[string]any{"all_muted": allMuted == 1, "dissolved": dissolvedAt.Valid}, request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录 IM 审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "管理群组失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"conversation_id": conversationID, "action": request.Action, "value": request.Value,
	})
}

func (h *Handler) deleteIMMessage(w http.ResponseWriter, r *http.Request) {
	messageID := strings.TrimSpace(r.PathValue("message_id"))
	var request struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if messageID == "" || request.Reason == "" || len(request.Reason) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "消息处置参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "处置消息失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var conversationID string
	var previousStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT conversation_id,status FROM im_messages WHERE id=? FOR UPDATE`,
		messageID,
	).Scan(&conversationID, &previousStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "消息不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "处置消息失败")
		return
	}
	if previousStatus != 3 {
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE im_messages SET status=3,revoked_at=CURRENT_TIMESTAMP(3) WHERE id=?`,
			messageID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "处置消息失败")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO im_moderation_actions
			(conversation_id,message_id,action_type,reason,actor_type,actor_id)
		VALUES(?,?,'delete_message',?,1,?)`,
		conversationID, messageID, request.Reason, adminID(r),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录消息处置失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "im.message.delete", "im_message", messageID,
		map[string]int{"status": previousStatus}, map[string]any{"status": 3, "reason": request.Reason},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录 IM 审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "处置消息失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"message_id": messageID, "status": 3})
}
