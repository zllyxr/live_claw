package im

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

func (s *Service) Conversations(ctx context.Context, userID int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT conversation.id,conversation.conversation_type,conversation.title,
		       conversation.message_seq,member.last_read_seq,conversation.updated_at,
		       COALESCE(peer.user_id,0),
		       COALESCE(NULLIF(peer_user.nickname,''),peer_user.username,''),
		       COALESCE(peer_asset.object_key,''),
		       COALESCE(latest.id,''),COALESCE(latest.sender_user_id,0),
		       COALESCE(latest.message_type,0),COALESCE(latest.text_content,''),
		       COALESCE(latest.metadata,JSON_OBJECT()),latest.created_at
		FROM im_conversation_members member
		JOIN im_conversations conversation
		  ON conversation.id=member.conversation_id AND conversation.status=1
		LEFT JOIN im_conversation_members peer
		  ON conversation.conversation_type=1
		 AND peer.conversation_id=conversation.id
		 AND peer.user_id<>member.user_id AND peer.member_status=1
		LEFT JOIN users peer_user ON peer_user.id=peer.user_id
		LEFT JOIN media_assets peer_asset
		  ON peer_asset.id=peer_user.avatar_asset_id AND peer_asset.status=1
		LEFT JOIN im_messages latest
		  ON latest.conversation_id=conversation.id
		 AND latest.sequence=conversation.message_seq AND latest.status=1
		WHERE member.user_id=? AND member.member_status=1 AND member.is_hidden=0
		ORDER BY conversation.updated_at DESC,conversation.id DESC
		LIMIT 200`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 32)
	for rows.Next() {
		var conversationID, title, peerName, peerAvatar, messageID, text string
		var conversationType, messageType int
		var messageSeq, lastReadSeq, peerUserID, senderUserID int64
		var metadata []byte
		var updatedAt time.Time
		var messageAt sql.NullTime
		if err = rows.Scan(
			&conversationID, &conversationType, &title, &messageSeq, &lastReadSeq, &updatedAt,
			&peerUserID, &peerName, &peerAvatar, &messageID, &senderUserID, &messageType,
			&text, &metadata, &messageAt,
		); err != nil {
			return nil, err
		}
		if conversationType == 1 && peerName != "" {
			title = peerName
		}
		var metadataValue map[string]any
		_ = json.Unmarshal(metadata, &metadataValue)
		unread := messageSeq - lastReadSeq
		if unread < 0 {
			unread = 0
		}
		items = append(items, map[string]any{
			"id": conversationID, "conversation_type": conversationType, "title": title,
			"message_seq": messageSeq, "last_read_seq": lastReadSeq, "unread_count": unread,
			"updated_at": updatedAt.UnixMilli(), "peer_user_id": strconv.FormatInt(peerUserID, 10),
			"peer_nickname": peerName, "peer_avatar": s.assetURL(peerAvatar),
			"latest_message": map[string]any{
				"id": messageID, "sender_user_id": strconv.FormatInt(senderUserID, 10), "message_type": messageType,
				"text_content": text, "metadata": metadataValue, "created_at": nullTimeMillis(messageAt),
			},
		})
	}
	return items, rows.Err()
}

func (s *Service) MarkRead(ctx context.Context, userID int64, conversationID string, sequence int64) error {
	if sequence < 0 {
		return errors.New("invalid read sequence")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE im_conversation_members member
		JOIN im_conversations conversation ON conversation.id=member.conversation_id
		SET member.last_read_seq=LEAST(
		        GREATEST(member.last_read_seq,IF(?=0,conversation.message_seq,?)),
		        conversation.message_seq
		    ),
		    member.is_hidden=0
		WHERE member.conversation_id=? AND member.user_id=? AND member.member_status=1`,
		sequence, sequence, strings.TrimSpace(conversationID), userID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotConversationMember
	}
	return nil
}

func (s *Service) HideConversation(ctx context.Context, userID int64, conversationID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE im_conversation_members SET is_hidden=1
		WHERE conversation_id=? AND user_id=? AND member_status=1`,
		strings.TrimSpace(conversationID), userID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotConversationMember
	}
	return nil
}

func (s *Service) Groups(ctx context.Context, userID int64, offset, limit int) ([]map[string]any, error) {
	offset, limit = pageWindow(offset, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT conversation.id,conversation.title,group_info.group_no,
		       group_info.owner_user_id,group_info.introduction,group_info.announcement,
		       group_info.join_policy,group_info.all_muted,group_info.max_members,
		       group_info.member_count,member.role,conversation.created_at
		FROM im_conversation_members member
		JOIN im_conversations conversation
		  ON conversation.id=member.conversation_id
		 AND conversation.conversation_type=2 AND conversation.status=1
		JOIN im_groups group_info ON group_info.conversation_id=conversation.id
		WHERE member.user_id=? AND member.member_status=1
		ORDER BY conversation.updated_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		item, scanErr := scanGroup(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Group(ctx context.Context, userID int64, conversationID string) (map[string]any, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT conversation.id,conversation.title,group_info.group_no,
		       group_info.owner_user_id,group_info.introduction,group_info.announcement,
		       group_info.join_policy,group_info.all_muted,group_info.max_members,
		       group_info.member_count,member.role,conversation.created_at
		FROM im_conversations conversation
		JOIN im_groups group_info ON group_info.conversation_id=conversation.id
		JOIN im_conversation_members member
		  ON member.conversation_id=conversation.id
		 AND member.user_id=? AND member.member_status=1
		WHERE conversation.id=? AND conversation.conversation_type=2 AND conversation.status=1`,
		userID, strings.TrimSpace(conversationID),
	)
	item, err := scanGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	return item, err
}

func (s *Service) GroupMembers(
	ctx context.Context,
	userID int64,
	conversationID string,
	offset, limit int,
) ([]map[string]any, error) {
	if _, err := s.Group(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	offset, limit = pageWindow(offset, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT member.user_id,COALESCE(NULLIF(app_user.nickname,''),app_user.username),
		       COALESCE(asset.object_key,''),member.role,member.mute_until,
		       member.joined_at
		FROM im_conversation_members member
		JOIN users app_user ON app_user.id=member.user_id
		LEFT JOIN media_assets asset ON asset.id=app_user.avatar_asset_id AND asset.status=1
		WHERE member.conversation_id=? AND member.member_status=1
		ORDER BY member.role DESC,member.joined_at LIMIT ? OFFSET ?`,
		conversationID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var memberID int64
		var nickname, avatar string
		var role int
		var muteUntil sql.NullTime
		var joinedAt time.Time
		if err = rows.Scan(&memberID, &nickname, &avatar, &role, &muteUntil, &joinedAt); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"user_id": strconv.FormatInt(memberID, 10), "nickname": nickname, "avatar": s.assetURL(avatar), "role": role,
			"mute_until": nullTimeMillis(muteUntil), "joined_at": joinedAt.UnixMilli(),
		})
	}
	return items, rows.Err()
}

func (s *Service) UpdateGroup(
	ctx context.Context,
	actorUserID int64,
	conversationID string,
	title, introduction, announcement string,
	joinPolicy int,
) error {
	title = strings.TrimSpace(title)
	introduction = strings.TrimSpace(introduction)
	announcement = strings.TrimSpace(announcement)
	if title == "" || len(title) > 200 || len(introduction) > 1000 ||
		len(announcement) > 2000 || joinPolicy < 1 || joinPolicy > 3 {
		return errors.New("invalid group information")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = requireGroupRole(ctx, tx, actorUserID, conversationID, 60); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_conversations SET title=? WHERE id=? AND conversation_type=2 AND status=1`,
		title, conversationID,
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_groups SET introduction=?,announcement=?,join_policy=?
		WHERE conversation_id=?`,
		introduction, announcement, joinPolicy, conversationID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) SetMemberRole(
	ctx context.Context,
	ownerUserID int64,
	conversationID string,
	targetUserID int64,
	role int,
) error {
	if role != 10 && role != 60 {
		return errors.New("invalid group role")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	actorRole, err := requireGroupRole(ctx, tx, ownerUserID, conversationID, 100)
	if err != nil || actorRole != 100 || ownerUserID == targetUserID {
		return ErrPermissionDenied
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE im_conversation_members SET role=?
		WHERE conversation_id=? AND user_id=? AND member_status=1 AND role<100`,
		role, conversationID, targetUserID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrConversationNotFound
	}
	return tx.Commit()
}

func (s *Service) SetAllMuted(ctx context.Context, actorUserID int64, conversationID string, muted bool) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = requireGroupRole(ctx, tx, actorUserID, conversationID, 60); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_groups SET all_muted=? WHERE conversation_id=?`,
		muted, conversationID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) RemoveMember(
	ctx context.Context,
	actorUserID int64,
	conversationID string,
	targetUserID int64,
) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	actorRole, err := requireGroupRole(ctx, tx, actorUserID, conversationID, 60)
	if err != nil {
		return err
	}
	var targetRole int
	if err = tx.QueryRowContext(ctx, `
		SELECT role FROM im_conversation_members
		WHERE conversation_id=? AND user_id=? AND member_status=1 FOR UPDATE`,
		conversationID, targetUserID,
	).Scan(&targetRole); errors.Is(err, sql.ErrNoRows) {
		return ErrConversationNotFound
	} else if err != nil {
		return err
	}
	if actorUserID == targetUserID || actorRole <= targetRole {
		return ErrPermissionDenied
	}
	if err = removeMemberTx(ctx, tx, conversationID, targetUserID, 3); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) TransferOwner(
	ctx context.Context,
	ownerUserID int64,
	conversationID string,
	targetUserID int64,
) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	role, err := requireGroupRole(ctx, tx, ownerUserID, conversationID, 100)
	if err != nil || role != 100 || ownerUserID == targetUserID {
		return ErrPermissionDenied
	}
	var targetRole int
	if err = tx.QueryRowContext(ctx, `
		SELECT role FROM im_conversation_members
		WHERE conversation_id=? AND user_id=? AND member_status=1 FOR UPDATE`,
		conversationID, targetUserID,
	).Scan(&targetRole); err != nil {
		return ErrConversationNotFound
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_conversation_members
		SET role=CASE WHEN user_id=? THEN 60 WHEN user_id=? THEN 100 ELSE role END
		WHERE conversation_id=? AND user_id IN (?,?)`,
		ownerUserID, targetUserID, conversationID, ownerUserID, targetUserID,
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_groups SET owner_user_id=? WHERE conversation_id=?`,
		targetUserID, conversationID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) LeaveGroup(ctx context.Context, userID int64, conversationID string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	role, err := requireGroupRole(ctx, tx, userID, conversationID, 10)
	if err != nil {
		return err
	}
	if role == 100 {
		return errors.New("owner must transfer ownership before leaving")
	}
	if err = removeMemberTx(ctx, tx, conversationID, userID, 2); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) DissolveGroup(ctx context.Context, ownerUserID int64, conversationID string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	role, err := requireGroupRole(ctx, tx, ownerUserID, conversationID, 100)
	if err != nil || role != 100 {
		return ErrPermissionDenied
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_conversations SET status=0 WHERE id=? AND conversation_type=2`,
		conversationID,
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_groups SET dissolved_at=CURRENT_TIMESTAMP(3) WHERE conversation_id=?`,
		conversationID,
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_conversation_members SET member_status=3,left_at=CURRENT_TIMESTAMP(3)
		WHERE conversation_id=? AND member_status=1`,
		conversationID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) JoinGroup(
	ctx context.Context,
	userID int64,
	conversationID string,
	message string,
) (map[string]any, error) {
	message = strings.TrimSpace(message)
	if len(message) > 500 {
		return nil, errors.New("join message is too long")
	}
	var policy int
	var status int
	err := s.db.QueryRowContext(ctx, `
		SELECT group_info.join_policy,conversation.status
		FROM im_groups group_info
		JOIN im_conversations conversation ON conversation.id=group_info.conversation_id
		WHERE group_info.conversation_id=?`,
		conversationID,
	).Scan(&policy, &status)
	if errors.Is(err, sql.ErrNoRows) || status != 1 {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}
	if policy == 3 {
		return nil, ErrPermissionDenied
	}
	if policy == 2 {
		if err = s.addGroupMemberAsSystem(ctx, conversationID, userID); err != nil {
			return nil, err
		}
		return map[string]any{"status": 1, "joined": true}, nil
	}
	applicationID, err := idgen.New()
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO im_group_applications
			(id,conversation_id,applicant_user_id,request_message,status)
		SELECT ?,?,?,?,0 FROM users WHERE id=? AND status=1
		ON DUPLICATE KEY UPDATE request_message=VALUES(request_message),created_at=CURRENT_TIMESTAMP(3)`,
		applicationID, conversationID, userID, message, userID,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": applicationID, "status": 0, "joined": false}, nil
}

func (s *Service) GroupApplications(
	ctx context.Context,
	userID int64,
	offset, limit int,
) ([]map[string]any, error) {
	offset, limit = pageWindow(offset, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT application.id,application.conversation_id,conversation.title,
		       application.applicant_user_id,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username),
		       COALESCE(asset.object_key,''),application.request_message,
		       application.status,application.created_at
		FROM im_group_applications application
		JOIN im_conversations conversation ON conversation.id=application.conversation_id
		JOIN im_conversation_members actor
		  ON actor.conversation_id=application.conversation_id
		 AND actor.user_id=? AND actor.member_status=1 AND actor.role>=60
		JOIN users app_user ON app_user.id=application.applicant_user_id
		LEFT JOIN media_assets asset ON asset.id=app_user.avatar_asset_id AND asset.status=1
		ORDER BY application.created_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, conversationID, title, nickname, avatar, message string
		var applicantID int64
		var status int
		var createdAt time.Time
		if err = rows.Scan(
			&id, &conversationID, &title, &applicantID, &nickname, &avatar,
			&message, &status, &createdAt,
		); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": id, "conversation_id": conversationID, "group_name": title,
			"user_id": strconv.FormatInt(applicantID, 10), "nickname": nickname, "avatar": s.assetURL(avatar),
			"request_message": message, "status": status, "created_at": createdAt.UnixMilli(),
		})
	}
	return items, rows.Err()
}

func (s *Service) HandleGroupApplication(
	ctx context.Context,
	actorUserID int64,
	applicationID string,
	accept bool,
	message string,
) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var conversationID string
	var applicantID int64
	var status int
	if err = tx.QueryRowContext(ctx, `
		SELECT conversation_id,applicant_user_id,status
		FROM im_group_applications WHERE id=? FOR UPDATE`,
		applicationID,
	).Scan(&conversationID, &applicantID, &status); errors.Is(err, sql.ErrNoRows) {
		return ErrConversationNotFound
	} else if err != nil {
		return err
	}
	if _, err = requireGroupRole(ctx, tx, actorUserID, conversationID, 60); err != nil {
		return err
	}
	if status != 0 {
		return nil
	}
	nextStatus := 2
	if accept {
		nextStatus = 1
		if err = addGroupMemberTx(ctx, tx, conversationID, applicantID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_group_applications
		SET status=?,handled_by=?,handle_message=?,handled_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND status=0`,
		nextStatus, actorUserID, strings.TrimSpace(message), applicationID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) BlockList(ctx context.Context, userID int64, offset, limit int) ([]map[string]any, error) {
	offset, limit = pageWindow(offset, limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT block.target_user_id,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username),
		       COALESCE(asset.object_key,''),block.created_at
		FROM user_blocks block
		JOIN users app_user ON app_user.id=block.target_user_id
		LEFT JOIN media_assets asset ON asset.id=app_user.avatar_asset_id AND asset.status=1
		WHERE block.user_id=? ORDER BY block.created_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var blockedID int64
		var nickname, avatar string
		var createdAt time.Time
		if err = rows.Scan(&blockedID, &nickname, &avatar, &createdAt); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"user_id": strconv.FormatInt(blockedID, 10), "nickname": nickname, "avatar": s.assetURL(avatar),
			"created_at": createdAt.UnixMilli(),
		})
	}
	return items, rows.Err()
}

func (s *Service) SetBlocked(ctx context.Context, userID, targetUserID int64, blocked bool) error {
	if userID < 1 || targetUserID < 1 || userID == targetUserID {
		return errors.New("invalid blocked user")
	}
	if blocked {
		result, err := s.db.ExecContext(ctx, `
			INSERT IGNORE INTO user_blocks(user_id,target_user_id)
			SELECT ?,id FROM users WHERE id=? AND status=1`,
			userID, targetUserID,
		)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			var exists int
			if scanErr := s.db.QueryRowContext(ctx, `
				SELECT 1 FROM user_blocks WHERE user_id=? AND target_user_id=?`,
				userID, targetUserID,
			).Scan(&exists); scanErr != nil {
				return ErrConversationNotFound
			}
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM user_blocks WHERE user_id=? AND target_user_id=?`,
		userID, targetUserID,
	)
	return err
}

func (s *Service) RevokeMessage(
	ctx context.Context,
	actorUserID int64,
	conversationID, messageID string,
) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var senderID int64
	var status int
	var createdAt time.Time
	if err = tx.QueryRowContext(ctx, `
		SELECT sender_user_id,status,created_at FROM im_messages
		WHERE id=? AND conversation_id=? FOR UPDATE`,
		messageID, conversationID,
	).Scan(&senderID, &status, &createdAt); errors.Is(err, sql.ErrNoRows) {
		return ErrConversationNotFound
	} else if err != nil {
		return err
	}
	if status != 1 {
		return nil
	}
	if senderID != actorUserID {
		if _, err = requireGroupRole(ctx, tx, actorUserID, conversationID, 60); err != nil {
			return err
		}
	} else if s.now().Sub(createdAt) > 2*time.Minute {
		return ErrPermissionDenied
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_messages SET status=2,revoked_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND status=1`,
		messageID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) addGroupMemberAsSystem(ctx context.Context, conversationID string, userID int64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if err = addGroupMemberTx(ctx, tx, conversationID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func addGroupMemberTx(ctx context.Context, tx *sql.Tx, conversationID string, userID int64) error {
	var memberCount, maxMembers int
	if err := tx.QueryRowContext(ctx, `
		SELECT member_count,max_members FROM im_groups
		WHERE conversation_id=? AND dissolved_at IS NULL FOR UPDATE`,
		conversationID,
	).Scan(&memberCount, &maxMembers); err != nil {
		return ErrConversationNotFound
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
	_, err = tx.ExecContext(ctx, `
		UPDATE im_groups SET member_count=(
		    SELECT COUNT(*) FROM im_conversation_members
		    WHERE conversation_id=? AND member_status=1
		) WHERE conversation_id=?`,
		conversationID, conversationID,
	)
	return err
}

func removeMemberTx(
	ctx context.Context,
	tx *sql.Tx,
	conversationID string,
	targetUserID int64,
	status int,
) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE im_conversation_members
		SET member_status=?,left_at=CURRENT_TIMESTAMP(3)
		WHERE conversation_id=? AND user_id=? AND member_status=1`,
		status, conversationID, targetUserID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE im_groups SET member_count=(
		    SELECT COUNT(*) FROM im_conversation_members
		    WHERE conversation_id=? AND member_status=1
		) WHERE conversation_id=?`,
		conversationID, conversationID,
	); err != nil {
		return err
	}
	return nil
}

func requireGroupRole(
	ctx context.Context,
	tx *sql.Tx,
	userID int64,
	conversationID string,
	minimum int,
) (int, error) {
	var role int
	err := tx.QueryRowContext(ctx, `
		SELECT member.role
		FROM im_conversation_members member
		JOIN im_conversations conversation
		  ON conversation.id=member.conversation_id
		 AND conversation.conversation_type=2 AND conversation.status=1
		WHERE member.conversation_id=? AND member.user_id=? AND member.member_status=1
		FOR UPDATE`,
		conversationID, userID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) || role < minimum {
		return 0, ErrPermissionDenied
	}
	return role, err
}

type groupScanner interface {
	Scan(...any) error
}

func scanGroup(scanner groupScanner) (map[string]any, error) {
	var conversationID, title, groupNo, introduction, announcement string
	var ownerID int64
	var joinPolicy, maxMembers, memberCount, role int
	var allMuted bool
	var createdAt time.Time
	err := scanner.Scan(
		&conversationID, &title, &groupNo, &ownerID, &introduction, &announcement,
		&joinPolicy, &allMuted, &maxMembers, &memberCount, &role, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": conversationID, "group_no": groupNo, "title": title,
		"owner_user_id": strconv.FormatInt(ownerID, 10),
		"introduction":  introduction, "announcement": announcement,
		"join_policy": joinPolicy, "all_muted": allMuted, "max_members": maxMembers,
		"member_count": memberCount, "role": role, "created_at": createdAt.UnixMilli(),
	}, nil
}

func pageWindow(offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	return offset, limit
}

func nullTimeMillis(value sql.NullTime) int64 {
	if !value.Valid {
		return 0
	}
	return value.Time.UnixMilli()
}

func parseUserID(value string) (int64, error) {
	result, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || result < 1 {
		return 0, errors.New("invalid user id")
	}
	return result, nil
}
