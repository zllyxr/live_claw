package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/im"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

var (
	compatGiftRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,99}$`)
	errLiveGiftRequestConflict = errors.New("live gift idempotency key was reused")
)

type compatLiveRoom struct {
	ID         int64
	HostUserID int64
	RoomNo     string
}

type compatLiveGiftResult struct {
	OrderID          string
	ClientRequestID  string
	LiveRoomID       int64
	ReceiverUserID   int64
	GiftID           int64
	GiftName         string
	GiftIcon         string
	GiftCount        int64
	UnitPriceCoin    int64
	TotalCoin        int64
	AvailableCoin    int64
	BroadcastPending bool
}

func (s *Server) compatLiveFeatures(w http.ResponseWriter, r *http.Request, service string) bool {
	switch service {
	case "Live.getGiftList":
		s.compatLiveGiftList(w, r)
	case "Live.sendGift":
		s.compatSendLiveGift(w, r)
	case "Live.getAdminList":
		s.compatLiveAdminList(w, r)
	case "Live.setAdmin":
		s.compatSetLiveAdmin(w, r)
	case "Live.setReport":
		s.compatReportLiveUser(w, r)
	case "Live.setShutUp":
		s.compatLiveModeration(w, r, "mute")
	case "Live.kicking":
		s.compatLiveModeration(w, r, "kick")
	case "Guard.GetGuardList":
		s.compatLiveGuards(w, r)
	case "Live.getUserRank":
		s.compatLiveRank(w, r)
	case "Red.getRedList":
		s.compatRedPacketList(w, r)
	case "Red.sendRed":
		s.compatSendRedPacket(w, r)
	case "Red.robRed":
		s.compatClaimRedPacket(w, r)
	case "Red.getRedRobList":
		s.compatRedPacketClaims(w, r)
	default:
		return false
	}
	return true
}

func (s *Server) compatRoomFromRequest(r *http.Request) (compatLiveRoom, error) {
	liveUserID := compatInt64(r.FormValue("liveuid"))
	stream := strings.TrimSpace(r.FormValue("stream"))
	var room compatLiveRoom
	if stream != "" && liveUserID > 0 {
		err := s.db.QueryRowContext(r.Context(), `
			SELECT id,host_user_id,room_no FROM live_rooms
			WHERE room_no=? AND host_user_id=? AND status=1 AND provider='douyin'`,
			stream, liveUserID,
		).Scan(&room.ID, &room.HostUserID, &room.RoomNo)
		return room, err
	}
	if stream != "" {
		err := s.db.QueryRowContext(r.Context(), `
			SELECT id,host_user_id,room_no FROM live_rooms
			WHERE room_no=? AND status=1 AND provider='douyin'`, stream,
		).Scan(&room.ID, &room.HostUserID, &room.RoomNo)
		return room, err
	}
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id,host_user_id,room_no FROM live_rooms
		WHERE host_user_id=? AND status=1 AND provider='douyin'
		ORDER BY sort_order DESC,id DESC LIMIT 1`, liveUserID,
	).Scan(&room.ID, &room.HostUserID, &room.RoomNo)
	return room, err
}

func (s *Server) compatLiveGiftList(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT gift.id,gift.name,gift.price_coin,COALESCE(asset.object_key,'')
		FROM live_gifts gift
		LEFT JOIN media_assets asset ON asset.id=gift.icon_asset_id AND asset.status=1
		WHERE gift.status=1 ORDER BY gift.sort_order DESC,gift.id`)
	if err != nil {
		writeCompat(w, 500, "礼物列表加载失败", nil)
		return
	}
	defer rows.Close()
	gifts := make([]map[string]any, 0, 12)
	for rows.Next() {
		var id, price int64
		var name, iconKey string
		if err = rows.Scan(&id, &name, &price, &iconKey); err != nil {
			break
		}
		icon := s.mediaURL(iconKey)
		if icon == "" {
			icon = "/static/live/icon_live_gift.png"
		}
		gifts = append(gifts, map[string]any{
			"id": strconv.FormatInt(id, 10), "giftname": name, "name": name,
			"gifticon": icon, "needcoin": strconv.FormatInt(price, 10),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "礼物列表加载失败", nil)
		return
	}
	balance, _ := s.wallet.Balance(r.Context(), userID)
	writeCompat(w, 0, "", map[string]any{
		"giftlist": gifts, "proplist": []any{}, "coin": strconv.FormatInt(balance.Available, 10),
	})
}

func (s *Server) compatSendLiveGift(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	clientRequestID := strings.TrimSpace(r.FormValue("client_request_id"))
	if clientRequestID == "" {
		clientRequestID = strings.TrimSpace(r.FormValue("clientRequestId"))
	}
	if clientRequestID == "" {
		clientRequestID = strings.TrimSpace(r.FormValue("request_id"))
	}
	if !compatGiftRequestIDPattern.MatchString(clientRequestID) {
		writeCompat(w, 400, "缺少有效的送礼请求编号", nil)
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil {
		writeCompat(w, 404, "直播间不存在", nil)
		return
	}
	giftID := compatInt64(r.FormValue("giftid"))
	count := compatInt64(r.FormValue("giftcount"))
	if count < 1 {
		count = 1
	}
	if count > 999 {
		writeCompat(w, 400, "单次礼物数量过多", nil)
		return
	}
	if room.HostUserID == userID {
		writeCompat(w, 400, "不能给自己赠送礼物", nil)
		return
	}
	result, err := s.createLiveGift(
		r.Context(), userID, room, clientRequestID, giftID, count,
	)
	if errors.Is(err, wallet.ErrInsufficientFunds) {
		writeCompat(w, 400, "星币余额不足", nil)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 404, "礼物不存在或直播间不可用", nil)
		return
	}
	if errors.Is(err, errLiveGiftRequestConflict) ||
		errors.Is(err, wallet.ErrIdempotencyReuse) {
		writeCompat(w, 409, "送礼请求编号已用于其他订单", nil)
		return
	}
	if err != nil {
		s.logger.Error("send live gift", "user_id", userID, "room_id", room.ID, "error", err)
		writeCompat(w, 500, "赠送礼物失败", nil)
		return
	}
	writeCompat(w, 0, "赠送成功", map[string]any{
		"orderid":           result.OrderID,
		"client_request_id": result.ClientRequestID,
		"giftid":            strconv.FormatInt(result.GiftID, 10),
		"giftname":          result.GiftName,
		"gifticon":          result.GiftIcon,
		"giftcount":         result.GiftCount,
		"totalcoin":         result.TotalCoin,
		"coin":              strconv.FormatInt(result.AvailableCoin, 10),
		"broadcast_pending": result.BroadcastPending,
	})
}

func (s *Server) createLiveGift(
	ctx context.Context,
	senderUserID int64,
	room compatLiveRoom,
	clientRequestID string,
	giftID int64,
	count int64,
) (compatLiveGiftResult, error) {
	if s.wallet == nil || s.im == nil {
		return compatLiveGiftResult{}, errors.New("live gift dependencies are unavailable")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return compatLiveGiftResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	existing, found, err := loadCompatLiveGift(
		ctx, tx, senderUserID, clientRequestID,
	)
	if err != nil {
		return compatLiveGiftResult{}, err
	}
	if found {
		if existing.LiveRoomID != room.ID ||
			existing.ReceiverUserID != room.HostUserID ||
			existing.GiftID != giftID ||
			existing.GiftCount != count {
			return compatLiveGiftResult{}, errLiveGiftRequestConflict
		}
		return existing, nil
	}

	var currentHostUserID int64
	err = tx.QueryRowContext(ctx, `
		SELECT host_user_id FROM live_rooms
		WHERE id=? AND room_no=? AND status=1 AND provider='douyin'
		FOR SHARE`,
		room.ID, room.RoomNo,
	).Scan(&currentHostUserID)
	if err != nil {
		return compatLiveGiftResult{}, err
	}
	if currentHostUserID != room.HostUserID || currentHostUserID == senderUserID {
		return compatLiveGiftResult{}, errors.New("live gift receiver is invalid")
	}

	var giftName, giftIconKey string
	var unitPrice int64
	err = tx.QueryRowContext(ctx, `
		SELECT gift.name,gift.price_coin,COALESCE(asset.object_key,'')
		FROM live_gifts gift
		LEFT JOIN media_assets asset ON asset.id=gift.icon_asset_id AND asset.status=1
		WHERE gift.id=? AND gift.status=1
		FOR SHARE`,
		giftID,
	).Scan(&giftName, &unitPrice, &giftIconKey)
	if err != nil {
		return compatLiveGiftResult{}, err
	}
	if unitPrice < 1 || unitPrice > (1<<62)/count {
		return compatLiveGiftResult{}, errors.New("invalid live gift price")
	}
	total := unitPrice * count
	giftIcon := s.mediaURL(giftIconKey)
	if giftIcon == "" {
		giftIcon = "/h5/static/live/icon_live_gift.png"
	}
	if len(giftIcon) > 1000 {
		return compatLiveGiftResult{}, errors.New("live gift icon URL is too long")
	}
	orderNo, err := idgen.New()
	if err != nil {
		return compatLiveGiftResult{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO live_gift_orders
			(order_no,client_request_id,live_room_id,sender_user_id,receiver_user_id,
			 gift_id,gift_name,gift_icon_url,gift_count,unit_price_coin,total_coin)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		orderNo, clientRequestID, room.ID, senderUserID, room.HostUserID,
		giftID, giftName, giftIcon, count, unitPrice, total,
	)
	if err != nil {
		if !compatIsDuplicate(err) {
			return compatLiveGiftResult{}, err
		}
		existing, found, loadErr := loadCompatLiveGift(
			ctx, tx, senderUserID, clientRequestID,
		)
		if loadErr != nil {
			return compatLiveGiftResult{}, loadErr
		}
		if !found {
			return compatLiveGiftResult{}, err
		}
		if existing.LiveRoomID != room.ID ||
			existing.ReceiverUserID != room.HostUserID ||
			existing.GiftID != giftID ||
			existing.GiftCount != count {
			return compatLiveGiftResult{}, errLiveGiftRequestConflict
		}
		return existing, nil
	}

	transfer, err := s.wallet.TransferTx(ctx, tx, wallet.TransferRequest{
		FromUserID:   senderUserID,
		ToUserID:     room.HostUserID,
		Amount:       total,
		BusinessType: "live_gift",
		BusinessID:   orderNo,
		Description:  "直播间赠送礼物",
		Metadata: map[string]any{
			"room_id": room.ID, "gift_id": giftID, "count": count,
			"client_request_id": clientRequestID,
		},
	})
	if err != nil {
		return compatLiveGiftResult{}, err
	}
	message, err := s.im.AppendTrustedLiveGiftTx(ctx, tx, im.TrustedLiveGiftRequest{
		ConversationID: room.RoomNo, OrderID: orderNo, ClientRequestID: clientRequestID,
		SenderUserID: senderUserID, ReceiverUserID: room.HostUserID,
		GiftID: giftID, GiftName: giftName, GiftIcon: giftIcon,
		GiftCount: count, TotalCoin: total,
	})
	if err != nil {
		return compatLiveGiftResult{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE live_gift_orders
		SET ledger_entry_no=?,credit_ledger_entry_no=?,im_message_id=?
		WHERE order_no=?`,
		transfer.Debit.EntryNo, transfer.Credit.EntryNo, message.ID, orderNo,
	)
	if err != nil {
		return compatLiveGiftResult{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return compatLiveGiftResult{}, errors.New("live gift order disappeared")
	}
	if err = tx.Commit(); err != nil {
		return compatLiveGiftResult{}, err
	}
	return compatLiveGiftResult{
		OrderID: orderNo, ClientRequestID: clientRequestID,
		LiveRoomID: room.ID, ReceiverUserID: room.HostUserID,
		GiftID: giftID, GiftName: giftName, GiftIcon: giftIcon,
		GiftCount: count, UnitPriceCoin: unitPrice, TotalCoin: total,
		AvailableCoin: transfer.Debit.Available, BroadcastPending: true,
	}, nil
}

func loadCompatLiveGift(
	ctx context.Context,
	tx *sql.Tx,
	senderUserID int64,
	clientRequestID string,
) (compatLiveGiftResult, bool, error) {
	var result compatLiveGiftResult
	var broadcastPending int
	err := tx.QueryRowContext(ctx, `
		SELECT gift.order_no,gift.client_request_id,gift.live_room_id,gift.receiver_user_id,
		       gift.gift_id,gift.gift_name,gift.gift_icon_url,gift.gift_count,
		       gift.unit_price_coin,gift.total_coin,
		       COALESCE(ledger.balance_available,account.available,0),
		       CASE WHEN outbox.status=2 THEN 0 ELSE 1 END
		FROM live_gift_orders gift
		LEFT JOIN wallet_ledger_entries ledger ON ledger.entry_no=gift.ledger_entry_no
		LEFT JOIN wallet_accounts account
		  ON account.user_id=gift.sender_user_id AND account.currency='COIN'
		LEFT JOIN outbox_events outbox ON outbox.event_id=gift.im_message_id
		WHERE gift.sender_user_id=? AND gift.client_request_id=?`,
		senderUserID, clientRequestID,
	).Scan(
		&result.OrderID, &result.ClientRequestID, &result.LiveRoomID,
		&result.ReceiverUserID, &result.GiftID, &result.GiftName,
		&result.GiftIcon, &result.GiftCount, &result.UnitPriceCoin,
		&result.TotalCoin, &result.AvailableCoin, &broadcastPending,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return compatLiveGiftResult{}, false, nil
	}
	if err != nil {
		return compatLiveGiftResult{}, false, err
	}
	result.BroadcastPending = broadcastPending == 1
	return result, true, nil
}

func (s *Server) compatLiveAdminList(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil {
		writeCompat(w, 404, "直播间不存在", nil)
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT user_id FROM live_room_managers
		WHERE live_room_id=? ORDER BY created_at DESC`, room.ID)
	if err != nil {
		writeCompat(w, 500, "房管列表加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 10)
	for rows.Next() {
		var userID int64
		if err = rows.Scan(&userID); err != nil {
			break
		}
		profile, profileErr := s.compatRelationshipProfile(r.Context(), currentUserID, userID)
		if profileErr != nil {
			err = profileErr
			break
		}
		items = append(items, profile)
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "房管列表加载失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]any{
		"list": items, "nums": strconv.Itoa(len(items)), "total": strconv.Itoa(len(items)),
	})
}

func (s *Server) compatSetLiveAdmin(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil || room.HostUserID != userID {
		writeCompat(w, 403, "只有主播可以设置房管", nil)
		return
	}
	targetUserID := compatInt64(r.FormValue("touid"))
	if targetUserID < 1 || targetUserID == userID {
		writeCompat(w, 400, "房管用户无效", nil)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeCompat(w, 500, "设置房管失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var exists int
	err = tx.QueryRowContext(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM live_room_managers WHERE live_room_id=? AND user_id=?
		)`, room.ID, targetUserID).Scan(&exists)
	if err != nil {
		writeCompat(w, 500, "设置房管失败", nil)
		return
	}
	if exists == 1 {
		_, err = tx.ExecContext(r.Context(), `
			DELETE FROM live_room_managers WHERE live_room_id=? AND user_id=?`,
			room.ID, targetUserID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `
				UPDATE im_conversation_members SET role=10
				WHERE conversation_id=? AND user_id=? AND role=60`,
				room.RoomNo, targetUserID)
		}
		exists = 0
	} else {
		var result sql.Result
		result, err = tx.ExecContext(r.Context(), `
			INSERT INTO live_room_managers(live_room_id,user_id,created_by)
			SELECT ?,id,? FROM users WHERE id=? AND status=1`,
			room.ID, userID, targetUserID)
		if err == nil {
			if affected, _ := result.RowsAffected(); affected != 1 {
				err = errors.New("live manager user does not exist")
			}
		}
		if err == nil {
			result, err = tx.ExecContext(r.Context(), `
				INSERT INTO im_conversation_members
					(conversation_id,user_id,role,member_status,left_at)
				VALUES(?,?,60,2,CURRENT_TIMESTAMP(3))
				ON DUPLICATE KEY UPDATE
					role=IF(role<100,60,role)`,
				room.RoomNo, targetUserID)
		}
		exists = 1
	}
	if err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "设置房管失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]string{"isadmin": strconv.Itoa(exists)})
}

func (s *Server) compatReportLiveUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	targetUserID := compatInt64(r.FormValue("touid"))
	if targetUserID < 1 || targetUserID == userID {
		writeCompat(w, 400, "举报对象无效", nil)
		return
	}
	_, err := s.db.ExecContext(r.Context(), `
		INSERT INTO user_reports(reporter_user_id,target_type,target_id,reason_code,description,evidence,status)
		VALUES(?,'live_user',?,'user_report',?,JSON_OBJECT(),0)`,
		userID, strconv.FormatInt(targetUserID, 10), boundedCompat(r.FormValue("content"), 1000),
	)
	if err != nil {
		writeCompat(w, 500, "提交举报失败", nil)
		return
	}
	writeCompat(w, 0, "举报已提交", map[string]string{"reported": "1"})
}

func (s *Server) isLiveModerator(r *http.Request, room compatLiveRoom, userID int64) bool {
	if room.HostUserID == userID {
		return true
	}
	var exists int
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM live_room_managers WHERE live_room_id=? AND user_id=?
		)`, room.ID, userID).Scan(&exists)
	return exists == 1
}

func (s *Server) compatLiveModeration(w http.ResponseWriter, r *http.Request, action string) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil || !s.isLiveModerator(r, room, userID) {
		writeCompat(w, 403, "没有直播间管理权限", nil)
		return
	}
	targetUserID := compatInt64(r.FormValue("touid"))
	if targetUserID < 1 || targetUserID == room.HostUserID {
		writeCompat(w, 400, "不能操作该用户", nil)
		return
	}
	actionType := action
	var expires any
	if action == "mute" && compatInt(r.FormValue("type")) == 0 {
		actionType = "unmute"
		expires = nil
	} else if action == "mute" {
		expires = time.Now().Add(24 * time.Hour)
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeCompat(w, 500, "直播间管理操作失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO live_moderation_actions
			(live_room_id,target_user_id,action_type,reason,expires_at,actor_type,actor_id)
		VALUES(?,?,?,'',?,2,?)`,
		room.ID, targetUserID, actionType, expires, userID)
	if err == nil {
		switch actionType {
		case "mute":
			_, err = tx.ExecContext(r.Context(), `
				UPDATE im_conversation_members SET mute_until=?
				WHERE conversation_id=? AND user_id=? AND member_status=1`,
				expires, room.RoomNo, targetUserID)
		case "unmute":
			_, err = tx.ExecContext(r.Context(), `
				UPDATE im_conversation_members SET mute_until=NULL
				WHERE conversation_id=? AND user_id=?`, room.RoomNo, targetUserID)
		case "kick":
			_, err = tx.ExecContext(r.Context(), `
				UPDATE im_conversation_members
				SET member_status=3,left_at=CURRENT_TIMESTAMP(3),mute_until=NULL
				WHERE conversation_id=? AND user_id=? AND member_status=1`,
				room.RoomNo, targetUserID)
		}
	}
	if err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "直播间管理操作失败", nil)
		return
	}
	writeCompat(w, 0, "操作成功", map[string]string{"action": actionType})
}

func (s *Server) compatLiveGuards(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireCompatUser(w, r); !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil {
		writeCompat(w, 404, "直播间不存在", nil)
		return
	}
	limit, offset := compatPage(r.FormValue("p"))
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT guard.user_id,guard.level,guard.expires_at,
		       COALESCE(NULLIF(user.nickname,''),user.username),COALESCE(asset.object_key,'')
		FROM live_guards guard
		JOIN users user ON user.id=guard.user_id AND user.status=1
		LEFT JOIN media_assets asset ON asset.id=user.avatar_asset_id AND asset.status=1
		WHERE guard.live_room_id=? AND guard.expires_at>CURRENT_TIMESTAMP(3)
		ORDER BY guard.level DESC,guard.expires_at DESC LIMIT ? OFFSET ?`,
		room.ID, limit, offset)
	if err != nil {
		writeCompat(w, 500, "守护列表加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]any, 0, limit)
	for rows.Next() {
		var userID int64
		var level int
		var expiresAt time.Time
		var nickname, avatarKey string
		if err = rows.Scan(&userID, &level, &expiresAt, &nickname, &avatarKey); err != nil {
			break
		}
		items = append(items, map[string]any{
			"uid": strconv.FormatInt(userID, 10), "user_nicename": nickname,
			"avatar": s.mediaURL(avatarKey), "avatar_thumb": s.mediaURL(avatarKey),
			"guard_type": level, "level": level, "endtime": expiresAt.Unix(),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "守护列表加载失败", nil)
		return
	}
	writeCompatList(w, items)
}

func (s *Server) compatLiveRank(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireCompatUser(w, r); !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil {
		writeCompat(w, 404, "直播间不存在", nil)
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT gift.sender_user_id,SUM(gift.total_coin),
		       COALESCE(NULLIF(user.nickname,''),user.username),COALESCE(asset.object_key,'')
		FROM live_gift_orders gift
		JOIN users user ON user.id=gift.sender_user_id
		LEFT JOIN media_assets asset ON asset.id=user.avatar_asset_id AND asset.status=1
		WHERE gift.live_room_id=?
		GROUP BY gift.sender_user_id,user.nickname,user.username,asset.object_key
		ORDER BY SUM(gift.total_coin) DESC LIMIT 50`, room.ID)
	if err != nil {
		writeCompat(w, 500, "贡献榜加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]any, 0, 50)
	for rows.Next() {
		var userID, total int64
		var nickname, avatarKey string
		if err = rows.Scan(&userID, &total, &nickname, &avatarKey); err != nil {
			break
		}
		items = append(items, map[string]any{
			"uid": strconv.FormatInt(userID, 10), "user_nicename": nickname,
			"avatar": s.mediaURL(avatarKey), "avatar_thumb": s.mediaURL(avatarKey),
			"total": strconv.FormatInt(total, 10), "coin": strconv.FormatInt(total, 10),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "贡献榜加载失败", nil)
		return
	}
	writeCompatList(w, items)
}

func (s *Server) expireRedPackets(r *http.Request, roomID int64) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id,packet_no,sender_user_id,total_coin-claimed_coin
		FROM live_red_packets
		WHERE live_room_id=? AND status=0 AND expires_at<=CURRENT_TIMESTAMP(3)`, roomID)
	if err != nil {
		return
	}
	type expired struct {
		id, senderID, remaining int64
		packetNo                string
	}
	items := make([]expired, 0, 8)
	for rows.Next() {
		var item expired
		if rows.Scan(&item.id, &item.packetNo, &item.senderID, &item.remaining) == nil {
			items = append(items, item)
		}
	}
	rows.Close()
	for _, item := range items {
		result, updateErr := s.db.ExecContext(r.Context(), `
			UPDATE live_red_packets SET status=2
			WHERE id=? AND status=0 AND expires_at<=CURRENT_TIMESTAMP(3)`, item.id)
		affected, _ := result.RowsAffected()
		if updateErr == nil && affected == 1 && item.remaining > 0 {
			_, _ = s.wallet.Apply(r.Context(), wallet.ApplyRequest{
				UserID: item.senderID, Amount: item.remaining, BusinessType: "red_refund",
				BusinessID: item.packetNo, Description: "直播红包过期退回",
			})
		}
	}
}

func (s *Server) compatRedPacketList(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil {
		writeCompat(w, 404, "直播间不存在", nil)
		return
	}
	s.expireRedPackets(r, room.ID)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT packet.id,packet.sender_user_id,packet.total_coin,packet.packet_count,
		       packet.claimed_count,packet.expires_at,packet.created_at,
		       COALESCE(NULLIF(user.nickname,''),user.username),COALESCE(asset.object_key,''),
		       EXISTS(SELECT 1 FROM live_red_packet_claims claim
		              WHERE claim.packet_id=packet.id AND claim.user_id=?)
		FROM live_red_packets packet
		JOIN users user ON user.id=packet.sender_user_id
		LEFT JOIN media_assets asset ON asset.id=user.avatar_asset_id AND asset.status=1
		WHERE packet.live_room_id=? AND packet.status=0
		ORDER BY packet.created_at DESC`, userID, room.ID)
	if err != nil {
		writeCompat(w, 500, "红包列表加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]any, 0, 20)
	now := time.Now()
	for rows.Next() {
		var id, senderID, total, count, claimed int64
		var expiresAt, createdAt time.Time
		var nickname, avatarKey string
		var isRobbed int
		if err = rows.Scan(
			&id, &senderID, &total, &count, &claimed, &expiresAt, &createdAt,
			&nickname, &avatarKey, &isRobbed,
		); err != nil {
			break
		}
		seconds := int64(expiresAt.Sub(now).Seconds())
		if seconds < 0 {
			seconds = 0
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "uid": strconv.FormatInt(senderID, 10),
			"type": "0", "type_grant": "0", "coin": total, "nums": count,
			"claimed": claimed, "des": "恭喜发财，大吉大利", "second": seconds,
			"isrob": strconv.Itoa(isRobbed), "avatar": s.mediaURL(avatarKey),
			"avatar_thumb": s.mediaURL(avatarKey), "user_nickname": nickname,
			"user_nicename": nickname, "addtime": createdAt.Unix(),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "红包列表加载失败", nil)
		return
	}
	writeCompatList(w, items)
}

func (s *Server) compatSendRedPacket(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil {
		writeCompat(w, 404, "直播间不存在", nil)
		return
	}
	total := compatInt64(r.FormValue("coin"))
	count := compatInt64(r.FormValue("nums"))
	if total < 1 || count < 1 || count > 100 || total < count {
		writeCompat(w, 400, "红包金额或数量无效", nil)
		return
	}
	packetNo, err := idgen.New()
	if err != nil {
		writeCompat(w, 500, "发送红包失败", nil)
		return
	}
	hold, err := s.wallet.PlaceHold(r.Context(), wallet.HoldRequest{
		UserID: userID, Amount: total, BusinessType: "live_red",
		BusinessID: packetNo, ExpiresAt: time.Now().Add(25 * time.Hour),
		Description: "发送直播红包",
	})
	if errors.Is(err, wallet.ErrInsufficientFunds) {
		writeCompat(w, 400, "星币余额不足", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "发送红包失败", nil)
		return
	}
	expiresAt := time.Now().Add(10 * time.Minute)
	result, err := s.db.ExecContext(r.Context(), `
		INSERT INTO live_red_packets
			(packet_no,live_room_id,sender_user_id,total_coin,packet_count,hold_no,status,expires_at)
		VALUES(?,?,?,?,?,?,0,?)`,
		packetNo, room.ID, userID, total, count, hold.HoldNo, expiresAt)
	if err != nil {
		_, _ = s.wallet.ReleaseHold(r.Context(), hold.HoldNo, "红包创建失败退款", nil)
		writeCompat(w, 500, "发送红包失败", nil)
		return
	}
	packetID, _ := result.LastInsertId()
	if _, err = s.wallet.CommitHold(r.Context(), wallet.CommitRequest{
		HoldNo: hold.HoldNo, Payout: 0, Description: "直播红包资金扣除",
	}); err != nil {
		_, _ = s.db.ExecContext(r.Context(), `DELETE FROM live_red_packets WHERE id=?`, packetID)
		_, _ = s.wallet.ReleaseHold(r.Context(), hold.HoldNo, "红包扣款失败退款", nil)
		writeCompat(w, 500, "红包扣款失败", nil)
		return
	}
	writeCompat(w, 0, "红包已发送", map[string]any{
		"id": strconv.FormatInt(packetID, 10), "packet_no": packetNo,
		"coin": total, "nums": count, "second": 600,
	})
}

func (s *Server) compatClaimRedPacket(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil {
		writeCompat(w, 404, "直播间不存在", nil)
		return
	}
	packetID := compatInt64(r.FormValue("redid"))
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeCompat(w, 500, "领取红包失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var packetNo string
	var total, count, claimedCoin, claimedCount int64
	var status int
	var expiresAt time.Time
	err = tx.QueryRowContext(r.Context(), `
		SELECT packet_no,total_coin,packet_count,claimed_coin,claimed_count,status,expires_at
		FROM live_red_packets WHERE id=? AND live_room_id=? FOR UPDATE`,
		packetID, room.ID,
	).Scan(&packetNo, &total, &count, &claimedCoin, &claimedCount, &status, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 404, "红包不存在", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "领取红包失败", nil)
		return
	}
	var existingAmount int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT amount_coin FROM live_red_packet_claims WHERE packet_id=? AND user_id=?`,
		packetID, userID).Scan(&existingAmount)
	if err == nil {
		writeCompat(w, 0, "已领取过该红包", map[string]any{"win": existingAmount, "msg": "已领取"})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 500, "领取红包失败", nil)
		return
	}
	if status != 0 || !expiresAt.After(time.Now()) || claimedCount >= count {
		writeCompat(w, 409, "红包已领完或已过期", nil)
		return
	}
	remainingCoin := total - claimedCoin
	remainingCount := count - claimedCount
	amount := remainingCoin / remainingCount
	if amount < 1 {
		amount = 1
	}
	if remainingCount == 1 {
		amount = remainingCoin
	}
	claimPlaceholder, _ := idgen.New()
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO live_red_packet_claims(packet_id,user_id,amount_coin,ledger_entry_no)
		VALUES(?,?,?,?)`, packetID, userID, amount, claimPlaceholder)
	finished := claimedCount+1 >= count || claimedCoin+amount >= total
	if err == nil {
		nextStatus := 0
		if finished {
			nextStatus = 1
		}
		_, err = tx.ExecContext(r.Context(), `
			UPDATE live_red_packets
			SET claimed_coin=claimed_coin+?,claimed_count=claimed_count+1,status=?
			WHERE id=?`, amount, nextStatus, packetID)
	}
	if err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "领取红包失败", nil)
		return
	}
	entry, err := s.wallet.Apply(r.Context(), wallet.ApplyRequest{
		UserID: userID, Amount: amount, BusinessType: "red_claim",
		BusinessID:  fmt.Sprintf("%s:%d", packetNo, userID),
		Description: "领取直播红包", Metadata: map[string]any{"packet_id": packetID},
	})
	if err != nil {
		_, _ = s.db.ExecContext(r.Context(), `
			DELETE FROM live_red_packet_claims WHERE packet_id=? AND user_id=?`, packetID, userID)
		_, _ = s.db.ExecContext(r.Context(), `
			UPDATE live_red_packets
			SET claimed_coin=GREATEST(claimed_coin-?,0),
			    claimed_count=GREATEST(claimed_count-1,0),status=0
			WHERE id=?`, amount, packetID)
		writeCompat(w, 500, "发放红包失败，请重试", nil)
		return
	}
	_, _ = s.db.ExecContext(r.Context(), `
		UPDATE live_red_packet_claims SET ledger_entry_no=?
		WHERE packet_id=? AND user_id=?`, entry.EntryNo, packetID, userID)
	writeCompat(w, 0, "领取成功", map[string]any{
		"win": amount, "msg": "恭喜领取成功", "coin": entry.Available,
	})
}

func (s *Server) compatRedPacketClaims(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	room, err := s.compatRoomFromRequest(r)
	if err != nil {
		writeCompat(w, 404, "直播间不存在", nil)
		return
	}
	packetID := compatInt64(r.FormValue("redid"))
	var total, count, claimedCoin, claimedCount int64
	var senderID int64
	err = s.db.QueryRowContext(r.Context(), `
		SELECT total_coin,packet_count,claimed_coin,claimed_count,sender_user_id
		FROM live_red_packets WHERE id=? AND live_room_id=?`, packetID, room.ID,
	).Scan(&total, &count, &claimedCoin, &claimedCount, &senderID)
	if err != nil {
		writeCompat(w, 404, "红包不存在", nil)
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT claim.user_id,claim.amount_coin,claim.created_at,
		       COALESCE(NULLIF(user.nickname,''),user.username),COALESCE(asset.object_key,'')
		FROM live_red_packet_claims claim
		JOIN users user ON user.id=claim.user_id
		LEFT JOIN media_assets asset ON asset.id=user.avatar_asset_id AND asset.status=1
		WHERE claim.packet_id=? ORDER BY claim.created_at ASC`, packetID)
	if err != nil {
		writeCompat(w, 500, "红包领取记录加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, count)
	var ownWin int64
	for rows.Next() {
		var claimUserID, amount int64
		var createdAt time.Time
		var nickname, avatarKey string
		if err = rows.Scan(&claimUserID, &amount, &createdAt, &nickname, &avatarKey); err != nil {
			break
		}
		if claimUserID == userID {
			ownWin = amount
		}
		items = append(items, map[string]any{
			"uid": strconv.FormatInt(claimUserID, 10), "coin": amount,
			"user_nicename": nickname, "avatar": s.mediaURL(avatarKey),
			"avatar_thumb": s.mediaURL(avatarKey), "addtime": createdAt.Unix(),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "红包领取记录加载失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]any{
		"redinfo": map[string]any{
			"id": strconv.FormatInt(packetID, 10), "uid": strconv.FormatInt(senderID, 10),
			"coin": total, "nums": count, "claimed_coin": claimedCoin, "claimed_count": claimedCount,
		},
		"list": items, "win": ownWin, "msg": "",
	})
}
