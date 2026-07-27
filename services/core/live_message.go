package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type liveMessageRequest struct {
	UID     int64          `json:"uid"`
	Token   string         `json:"token"`
	LiveID  string         `json:"liveID"`
	Stream  string         `json:"stream"`
	Payload map[string]any `json:"payload"`
}

type liveActor struct {
	UserID      string
	Nickname    string
	Avatar      string
	Level       int64
	UserType    int
	GuardType   int
	Muted       bool
	Kicked      bool
	IsAnchor    bool
	IsSuper     bool
	IsManager   bool
	Consumption int64
}

type livePolicy struct {
	SpeakLimit     int64
	SensitiveWords []string
}

func (s *APIServer) imLiveMessage(w http.ResponseWriter, r *http.Request) {
	var request liveMessageRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, appError(400, "直播消息参数错误"))
		return
	}
	if err := s.auth.Verify(r.Context(), request.UID, request.Token); err != nil {
		writeAPIError(w, http.StatusUnauthorized, normalizeError(err))
		return
	}
	liveUID, err := strconv.ParseInt(strings.TrimSpace(request.LiveID), 10, 64)
	if err != nil || liveUID < 1 {
		writeAPIError(w, http.StatusBadRequest, appError(400, "直播间参数错误"))
		return
	}
	request.Stream = strings.TrimSpace(request.Stream)
	incoming, err := firstLiveMessage(request.Payload)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, appError(400, err.Error()))
		return
	}
	method := liveText(incoming["_method_"])
	allowEnded := method == "disconnect" || method == "StartEndLive"
	liveName, err := s.requireLiveRoom(r.Context(), liveUID, request.Stream, allowEnded)
	if err != nil {
		writeAPIError(w, http.StatusConflict, normalizeError(err))
		return
	}
	actor, err := s.loadLiveActor(r.Context(), request.UID, liveUID)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, normalizeError(err))
		return
	}
	payload, system, err := s.normalizeLiveMessage(r.Context(), request, liveUID, liveName, actor, incoming)
	if err != nil {
		status := http.StatusForbidden
		var target *AppError
		if errors.As(err, &target) && (target.Code == 400 || target.Code == 429) {
			status = http.StatusBadRequest
			if target.Code == 429 {
				status = http.StatusTooManyRequests
			}
		}
		writeAPIError(w, status, normalizeError(err))
		return
	}

	groupID := liveGroupID(request.LiveID)
	if method != "disconnect" {
		user, loadErr := loadIMUser(r.Context(), s.db, request.UID)
		if loadErr != nil {
			writeAPIError(w, http.StatusNotFound, normalizeError(loadErr))
			return
		}
		groupID, err = s.openIM.EnsureLiveGroup(r.Context(), request.LiveID, liveName, user)
		if err != nil {
			s.logger.Error("openim ensure live group before message", "uid", request.UID, "live_id", request.LiveID, "error", err)
			writeAPIError(w, http.StatusBadGateway, appError(502, "直播聊天服务暂不可用"))
			return
		}
	}
	serverMessageID, err := s.openIM.SendLiveCustomMessage(r.Context(), groupID, payload, system)
	if err != nil {
		s.logger.Error("openim send live message", "uid", request.UID, "live_id", request.LiveID, "method", method, "error", err)
		writeAPIError(w, http.StatusBadGateway, appError(502, "直播消息发送失败"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"data": map[string]any{
			"payload":     payload,
			"serverMsgID": serverMessageID,
		},
	})
}

func (s *APIServer) requireLiveRoom(ctx context.Context, liveUID int64, stream string, allowEnded bool) (string, error) {
	var title string
	var isLive int
	var currentStream string
	err := s.db.QueryRowContext(ctx,
		"SELECT title, islive, stream FROM cmf_live WHERE uid=? LIMIT 1",
		liveUID,
	).Scan(&title, &isLive, &currentStream)
	if errors.Is(err, sql.ErrNoRows) {
		return "", appError(1005, "直播间不存在")
	}
	if err != nil {
		return "", fmt.Errorf("load live room: %w", err)
	}
	if stream != "" && currentStream != stream {
		return "", appError(1005, "直播场次已更新，请重新进入")
	}
	if !allowEnded && isLive != 1 {
		return "", appError(1005, "直播已结束")
	}
	if strings.TrimSpace(title) == "" {
		title = "直播间 " + strconv.FormatInt(liveUID, 10)
	}
	return title, nil
}

func (s *APIServer) loadLiveActor(ctx context.Context, uid, liveUID int64) (liveActor, error) {
	var actor liveActor
	var super, manager, muted, kicked int
	var guardType sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT CAST(u.id AS CHAR),
		       COALESCE(NULLIF(u.user_nickname,''), NULLIF(u.user_login,''), CONCAT('用户',u.id)),
		       COALESCE(NULLIF(u.avatar_thumb,''),u.avatar,''),
		       u.consumption,
		       EXISTS(SELECT 1 FROM cmf_user_super us WHERE us.uid=u.id),
		       EXISTS(SELECT 1 FROM cmf_live_manager lm WHERE lm.uid=u.id AND lm.liveuid=?),
		       EXISTS(SELECT 1 FROM cmf_live_shut ls WHERE ls.uid=u.id AND ls.liveuid=?),
		       EXISTS(SELECT 1 FROM cmf_live_kick lk WHERE lk.uid=u.id AND lk.liveuid=?),
		       (SELECT gu.type FROM cmf_guard_user gu WHERE gu.uid=u.id AND gu.liveuid=? AND gu.endtime>? ORDER BY gu.type DESC LIMIT 1)
		  FROM cmf_user u
		 WHERE u.id=? AND u.user_status=1`,
		liveUID, liveUID, liveUID, liveUID, time.Now().Unix(), uid,
	).Scan(
		&actor.UserID, &actor.Nickname, &actor.Avatar, &actor.Consumption,
		&super, &manager, &muted, &kicked, &guardType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return liveActor{}, appError(404, "用户不存在或已停用")
	}
	if err != nil {
		return liveActor{}, fmt.Errorf("load live actor: %w", err)
	}
	actor.IsAnchor = uid == liveUID
	actor.IsSuper = super == 1
	actor.IsManager = manager == 1
	actor.Muted = muted == 1
	actor.Kicked = kicked == 1
	actor.GuardType = int(guardType.Int64)
	switch {
	case actor.IsAnchor:
		actor.UserType = 50
	case actor.IsSuper:
		actor.UserType = 60
	case actor.IsManager:
		actor.UserType = 40
	default:
		actor.UserType = 30
	}
	if err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(levelid),0) FROM cmf_level WHERE level_up<=?",
		actor.Consumption,
	).Scan(&actor.Level); err != nil {
		return liveActor{}, fmt.Errorf("load live actor level: %w", err)
	}
	return actor, nil
}

func (s *APIServer) normalizeLiveMessage(
	ctx context.Context,
	request liveMessageRequest,
	liveUID int64,
	liveName string,
	actor liveActor,
	incoming map[string]any,
) (map[string]any, bool, error) {
	method := liveText(incoming["_method_"])
	message := map[string]any{
		"_method_":   method,
		"uid":        actor.UserID,
		"uname":      actor.Nickname,
		"uhead":      actor.Avatar,
		"level":      strconv.FormatInt(actor.Level, 10),
		"usertype":   strconv.Itoa(actor.UserType),
		"isAnchor":   liveBoolText(actor.IsAnchor),
		"guard_type": strconv.Itoa(actor.GuardType),
		"vip_type":   "0",
		"liangname":  "0",
	}
	system := false

	switch method {
	case "SendMsg":
		if actor.Kicked {
			return nil, false, appError(1008, "您已被踢出房间")
		}
		msgType := liveText(incoming["msgtype"])
		switch msgType {
		case "0":
			message["action"] = "0"
			message["msgtype"] = "0"
			message["ct"] = map[string]any{
				"id":            actor.UserID,
				"user_nickname": actor.Nickname,
				"avatar":        actor.Avatar,
				"avatar_thumb":  actor.Avatar,
				"level":         strconv.FormatInt(actor.Level, 10),
				"vip_type":      "0",
				"liangname":     "0",
				"guard_type":    strconv.Itoa(actor.GuardType),
				"usertype":      strconv.Itoa(actor.UserType),
			}
			if s.auth.redis != nil {
				_ = s.auth.redis.ZAdd(ctx, "user_"+request.Stream, redis.Z{Score: 0, Member: actor.UserID}).Err()
			}
		case "2":
			if actor.Muted {
				return nil, false, appError(1002, "您已被禁言")
			}
			if err := s.enforceLiveChatRate(ctx, liveUID, actor.UserID); err != nil {
				return nil, false, err
			}
			policy, err := s.loadLivePolicy(ctx)
			if err != nil {
				return nil, false, err
			}
			if policy.SpeakLimit > 0 && actor.Level < policy.SpeakLimit {
				return nil, false, appError(1003, fmt.Sprintf("等级达到 %d 才能发言", policy.SpeakLimit))
			}
			content, err := normalizeChatContent(liveText(incoming["ct"]), policy.SensitiveWords)
			if err != nil {
				return nil, false, err
			}
			message["action"] = "0"
			message["msgtype"] = "2"
			message["ct"] = content
		default:
			return nil, false, appError(400, "不支持的聊天消息类型")
		}
	case "SendGift":
		if actor.Kicked {
			return nil, false, appError(1008, "您已被踢出房间")
		}
		gift, err := s.consumeGiftToken(ctx, liveText(incoming["ct"]), actor.UserID, request.LiveID, request.Stream)
		if err != nil {
			return nil, false, err
		}
		message["action"] = "0"
		message["msgtype"] = "1"
		message["ct"] = gift
		message["roomnum"] = request.LiveID
		message["livename"] = liveName
		message["evensend"] = liveText(gift["type"])
	case "KickUser":
		if actor.UserType == 30 {
			return nil, false, appError(1001, "无权执行踢人操作")
		}
		toUID, toName, err := s.verifyLiveTargetState(ctx, liveUID, incoming, "kick")
		if err != nil {
			return nil, false, err
		}
		message["action"] = "2"
		message["msgtype"] = "4"
		message["touid"] = toUID
		message["toname"] = toName
		message["ct"] = toName + "被踢出房间"
		message["ct_en"] = toName + " was kicked out of the room"
		system = true
	case "ShutUpUser":
		if actor.UserType == 30 {
			return nil, false, appError(1001, "无权执行禁言操作")
		}
		toUID, toName, err := s.verifyLiveTargetState(ctx, liveUID, incoming, "shut")
		if err != nil {
			return nil, false, err
		}
		var showID int
		if err := s.db.QueryRowContext(ctx,
			"SELECT showid FROM cmf_live_shut WHERE uid=? AND liveuid=? LIMIT 1",
			toUID, liveUID,
		).Scan(&showID); err != nil {
			return nil, false, appError(1003, "禁言状态尚未生效")
		}
		message["action"] = "1"
		message["msgtype"] = "4"
		message["touid"] = toUID
		message["toname"] = toName
		if showID == 0 {
			message["ct"] = toName + "被永久禁言"
			message["ct_en"] = toName + " is permanently banned"
		} else {
			message["ct"] = toName + "被本场禁言"
			message["ct_en"] = toName + " has been banned from this live"
		}
		system = true
	case "setAdmin":
		if !actor.IsAnchor {
			return nil, false, appError(1001, "只有主播可以设置房管")
		}
		toUID, toName, err := s.verifyLiveTargetState(ctx, liveUID, incoming, "admin")
		if err != nil {
			return nil, false, err
		}
		var isManager int
		err = s.db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM cmf_live_manager WHERE uid=? AND liveuid=?)",
			toUID, liveUID,
		).Scan(&isManager)
		if err != nil {
			return nil, false, fmt.Errorf("load live manager state: %w", err)
		}
		message["action"] = strconv.Itoa(isManager)
		message["msgtype"] = "1"
		message["touid"] = toUID
		message["toname"] = toName
		if isManager == 1 {
			message["ct"] = toName + "被设为管理员"
			message["ct_en"] = toName + " is set as administrator"
		} else {
			message["ct"] = toName + "被取消管理员"
			message["ct_en"] = toName + " was removed as administrator"
		}
		system = true
	case "disconnect":
		message["action"] = "0"
		message["msgtype"] = "0"
		message["ct"] = map[string]any{"id": actor.UserID}
		if s.auth.redis != nil && request.Stream != "" {
			_ = s.auth.redis.ZRem(ctx, "user_"+request.Stream, actor.UserID).Err()
		}
	case "StartEndLive":
		if !actor.IsAnchor && !actor.IsSuper {
			return nil, false, appError(1001, "无权关闭直播间")
		}
		message["action"] = "13"
		message["msgtype"] = "1"
		message["ct"] = "直播已结束"
		system = true
	default:
		return nil, false, appError(400, "不支持的直播消息")
	}

	payload := map[string]any{
		"retcode":    "000000",
		"retmsg":     "ok",
		"event_id":   newLiveEventID(),
		"created_at": time.Now().UnixMilli(),
		"msg":        []any{message},
	}
	return payload, system, nil
}

func (s *APIServer) loadLivePolicy(ctx context.Context) (livePolicy, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx,
		"SELECT option_value FROM cmf_option WHERE option_name='configpri' LIMIT 1",
	).Scan(&raw); err != nil {
		return livePolicy{}, fmt.Errorf("load live policy: %w", err)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return livePolicy{}, fmt.Errorf("decode live policy: %w", err)
	}
	policy := livePolicy{SpeakLimit: valueInt64(config["speak_limit"])}
	for _, word := range strings.Split(liveText(config["sensitive_words"]), ",") {
		if word = strings.TrimSpace(word); word != "" {
			policy.SensitiveWords = append(policy.SensitiveWords, word)
		}
	}
	return policy, nil
}

func (s *APIServer) enforceLiveChatRate(ctx context.Context, liveUID int64, userID string) error {
	if s.auth.redis == nil {
		return nil
	}
	cooldownKey := fmt.Sprintf("live_chat_cooldown:%d:%s", liveUID, userID)
	ok, err := s.auth.redis.SetNX(ctx, cooldownKey, "1", 650*time.Millisecond).Result()
	if err == nil && !ok {
		return appError(429, "发送太快，请稍后再试")
	}
	minuteKey := fmt.Sprintf("live_chat_minute:%d:%s:%d", liveUID, userID, time.Now().Unix()/60)
	count, countErr := s.auth.redis.Incr(ctx, minuteKey).Result()
	if countErr == nil {
		if count == 1 {
			_ = s.auth.redis.Expire(ctx, minuteKey, 2*time.Minute).Err()
		}
		if count > 40 {
			return appError(429, "发言过于频繁，请稍后再试")
		}
	}
	return nil
}

func (s *APIServer) consumeGiftToken(ctx context.Context, token, userID, liveID, stream string) (map[string]any, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 128 || s.auth.redis == nil {
		return nil, appError(1002, "礼物凭证无效")
	}
	raw, err := s.auth.redis.Get(ctx, token).Result()
	if errors.Is(err, redis.Nil) {
		return nil, appError(1002, "礼物凭证已失效或已使用")
	}
	if err != nil {
		return nil, fmt.Errorf("load gift token: %w", err)
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, appError(1002, "礼物凭证数据错误")
	}
	var list []any
	switch value := decoded.(type) {
	case []any:
		list = value
	case map[string]any:
		if uid := liveText(value["uid"]); uid != "" && uid != userID {
			return nil, appError(1002, "礼物凭证与用户不匹配")
		}
		if room := liveText(value["liveuid"]); room != "" && room != liveID {
			return nil, appError(1002, "礼物凭证与直播间不匹配")
		}
		if giftStream := liveText(value["stream"]); giftStream != "" && giftStream != stream {
			return nil, appError(1002, "礼物凭证与直播场次不匹配")
		}
		list, _ = value["list"].([]any)
	}
	if len(list) == 0 {
		return nil, appError(1002, "礼物凭证内容为空")
	}
	gift, ok := list[0].(map[string]any)
	if !ok || liveText(gift["uid"]) != userID {
		return nil, appError(1002, "礼物凭证与用户不匹配")
	}
	removed, err := s.auth.redis.Eval(ctx, `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`, []string{token}, raw).Int()
	if err != nil {
		return nil, fmt.Errorf("consume gift token: %w", err)
	}
	if removed != 1 {
		return nil, appError(1002, "礼物凭证已使用")
	}
	return gift, nil
}

func (s *APIServer) verifyLiveTargetState(ctx context.Context, liveUID int64, incoming map[string]any, kind string) (string, string, error) {
	toUID := liveText(incoming["touid"])
	targetID, err := strconv.ParseInt(toUID, 10, 64)
	if err != nil || targetID < 1 {
		return "", "", appError(400, "目标用户参数错误")
	}
	var name string
	err = s.db.QueryRowContext(ctx,
		"SELECT COALESCE(NULLIF(user_nickname,''), NULLIF(user_login,''), CONCAT('用户',id)) FROM cmf_user WHERE id=? LIMIT 1",
		targetID,
	).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", appError(404, "目标用户不存在")
	}
	if err != nil {
		return "", "", fmt.Errorf("load live target: %w", err)
	}
	var exists int
	switch kind {
	case "kick":
		err = s.db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM cmf_live_kick WHERE uid=? AND liveuid=?)",
			targetID, liveUID,
		).Scan(&exists)
	case "shut":
		err = s.db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM cmf_live_shut WHERE uid=? AND liveuid=?)",
			targetID, liveUID,
		).Scan(&exists)
	case "admin":
		return toUID, name, nil
	default:
		return "", "", appError(400, "不支持的管理动作")
	}
	if err != nil {
		return "", "", fmt.Errorf("load live target state: %w", err)
	}
	if exists != 1 {
		return "", "", appError(1003, "管理操作尚未生效")
	}
	return toUID, name, nil
}

func firstLiveMessage(payload map[string]any) (map[string]any, error) {
	if payload == nil {
		return nil, errors.New("直播消息不能为空")
	}
	list, ok := payload["msg"].([]any)
	if !ok || len(list) != 1 {
		return nil, errors.New("直播消息格式错误")
	}
	message, ok := list[0].(map[string]any)
	if !ok {
		return nil, errors.New("直播消息格式错误")
	}
	return message, nil
}

func normalizeChatContent(content string, sensitiveWords []string) (string, error) {
	content = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, content))
	if content == "" {
		return "", appError(400, "消息内容不能为空")
	}
	if len([]rune(content)) > 200 {
		return "", appError(400, "消息不能超过200个字")
	}
	for _, word := range sensitiveWords {
		content = strings.ReplaceAll(content, word, strings.Repeat("*", len([]rune(word))))
	}
	return content, nil
}

func liveText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func liveBoolText(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func newLiveEventID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
