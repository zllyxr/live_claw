package im

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

type jsonInt64 int64

func (value *jsonInt64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("empty integer")
	}
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return err
		}
		*value = jsonInt64(parsed)
		return nil
	}
	parsed, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*value = jsonInt64(parsed)
	return nil
}

type Handler struct {
	service  *Service
	auth     *auth.Service
	redis    *redis.Client
	upgrader websocket.Upgrader
}

func NewHandler(service *Service, authService *auth.Service, redisClient *redis.Client) *Handler {
	handler := &Handler{service: service, auth: authService, redis: redisClient}
	handler.upgrader = websocket.Upgrader{
		HandshakeTimeout: 8 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		CheckOrigin:      sameOrigin,
	}
	return handler
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws/im", h.socket)
	mux.HandleFunc("GET /api/v2/im/conversations", h.authenticated(h.listConversations))
	mux.HandleFunc("POST /api/v2/im/direct", h.authenticated(h.createDirect))
	mux.HandleFunc("POST /api/v2/im/groups", h.authenticated(h.createGroup))
	mux.HandleFunc("GET /api/v2/im/groups", h.authenticated(h.listGroups))
	mux.HandleFunc("GET /api/v2/im/groups/{conversation_id}", h.authenticated(h.getGroup))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}", h.authenticated(h.updateGroup))
	mux.HandleFunc("GET /api/v2/im/groups/{conversation_id}/members", h.authenticated(h.listGroupMembers))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/members", h.authenticated(h.addGroupMember))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/members/{user_id}/mute", h.authenticated(h.muteGroupMember))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/members/{user_id}/role", h.authenticated(h.setGroupMemberRole))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/members/{user_id}/remove", h.authenticated(h.removeGroupMember))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/all-mute", h.authenticated(h.setGroupAllMute))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/transfer", h.authenticated(h.transferGroupOwner))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/leave", h.authenticated(h.leaveGroup))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/dissolve", h.authenticated(h.dissolveGroup))
	mux.HandleFunc("POST /api/v2/im/groups/{conversation_id}/join", h.authenticated(h.joinGroup))
	mux.HandleFunc("GET /api/v2/im/group-applications", h.authenticated(h.listGroupApplications))
	mux.HandleFunc("POST /api/v2/im/group-applications/{application_id}", h.authenticated(h.handleGroupApplication))
	mux.HandleFunc("GET /api/v2/im/blocks", h.authenticated(h.listBlocks))
	mux.HandleFunc("POST /api/v2/im/blocks/{user_id}", h.authenticated(h.setBlocked))
	mux.HandleFunc("POST /api/v2/im/conversations/{conversation_id}/read", h.authenticated(h.markRead))
	mux.HandleFunc("POST /api/v2/im/conversations/{conversation_id}/hide", h.authenticated(h.hideConversation))
	mux.HandleFunc("POST /api/v2/im/conversations/{conversation_id}/messages", h.authenticated(h.sendMessage))
	mux.HandleFunc("GET /api/v2/im/conversations/{conversation_id}/messages", h.authenticated(h.listMessages))
	mux.HandleFunc("POST /api/v2/im/conversations/{conversation_id}/messages/{message_id}/revoke", h.authenticated(h.revokeMessage))
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request, user auth.User) {
	items, err := h.service.Conversations(r.Context(), user.ID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) createDirect(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		PeerUserID jsonInt64 `json:"peer_user_id"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	conversation, err := h.service.DirectConversation(r.Context(), user.ID, int64(request.PeerUserID))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), conversation)
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		Title      string      `json:"title"`
		MaxMembers int         `json:"max_members"`
		MemberIDs  []jsonInt64 `json:"member_ids"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	conversation, err := h.service.CreateGroup(r.Context(), user.ID, request.Title, request.MaxMembers)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	for _, memberID := range request.MemberIDs {
		if int64(memberID) == user.ID {
			continue
		}
		if err = h.service.AddGroupMember(r.Context(), user.ID, conversation.ID, int64(memberID)); err != nil {
			h.writeError(w, r, err)
			return
		}
	}
	httpx.OK(w, httpx.RequestID(r.Context()), conversation)
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request, user auth.User) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.Groups(r.Context(), user.ID, offset, limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) getGroup(w http.ResponseWriter, r *http.Request, user auth.User) {
	item, err := h.service.Group(r.Context(), user.ID, r.PathValue("conversation_id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), item)
}

func (h *Handler) updateGroup(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		Title        string `json:"title"`
		Introduction string `json:"introduction"`
		Announcement string `json:"announcement"`
		JoinPolicy   int    `json:"join_policy"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	err := h.service.UpdateGroup(
		r.Context(), user.ID, r.PathValue("conversation_id"), request.Title,
		request.Introduction, request.Announcement, request.JoinPolicy,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"updated": true})
}

func (h *Handler) listGroupMembers(w http.ResponseWriter, r *http.Request, user auth.User) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.GroupMembers(
		r.Context(), user.ID, r.PathValue("conversation_id"), offset, limit,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) addGroupMember(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		UserID jsonInt64 `json:"user_id"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	err := h.service.AddGroupMember(r.Context(), user.ID, r.PathValue("conversation_id"), int64(request.UserID))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"added": true})
}

func (h *Handler) muteGroupMember(w http.ResponseWriter, r *http.Request, user auth.User) {
	targetUserID, err := parseUserID(r.PathValue("user_id"))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户编号无效")
		return
	}
	var request struct {
		DurationSeconds int64 `json:"duration_seconds"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	var until time.Time
	if request.DurationSeconds > 0 {
		if request.DurationSeconds > int64((365 * 24 * time.Hour).Seconds()) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "禁言时长无效")
			return
		}
		until = time.Now().Add(time.Duration(request.DurationSeconds) * time.Second)
	}
	err = h.service.MuteMember(
		r.Context(), user.ID, r.PathValue("conversation_id"), targetUserID, until,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"muted": request.DurationSeconds > 0, "mute_until": unixOrZero(until),
	})
}

func (h *Handler) setGroupMemberRole(w http.ResponseWriter, r *http.Request, user auth.User) {
	targetUserID, err := parseUserID(r.PathValue("user_id"))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户编号无效")
		return
	}
	var request struct {
		Role int `json:"role"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	err = h.service.SetMemberRole(
		r.Context(), user.ID, r.PathValue("conversation_id"), targetUserID, request.Role,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"updated": true})
}

func (h *Handler) removeGroupMember(w http.ResponseWriter, r *http.Request, user auth.User) {
	targetUserID, err := parseUserID(r.PathValue("user_id"))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户编号无效")
		return
	}
	if err = h.service.RemoveMember(
		r.Context(), user.ID, r.PathValue("conversation_id"), targetUserID,
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"removed": true})
}

func (h *Handler) setGroupAllMute(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		Muted bool `json:"muted"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	if err := h.service.SetAllMuted(
		r.Context(), user.ID, r.PathValue("conversation_id"), request.Muted,
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"muted": request.Muted})
}

func (h *Handler) transferGroupOwner(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		UserID jsonInt64 `json:"user_id"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	if err := h.service.TransferOwner(
		r.Context(), user.ID, r.PathValue("conversation_id"), int64(request.UserID),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"transferred": true})
}

func (h *Handler) leaveGroup(w http.ResponseWriter, r *http.Request, user auth.User) {
	if err := h.service.LeaveGroup(r.Context(), user.ID, r.PathValue("conversation_id")); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"left": true})
}

func (h *Handler) dissolveGroup(w http.ResponseWriter, r *http.Request, user auth.User) {
	if err := h.service.DissolveGroup(r.Context(), user.ID, r.PathValue("conversation_id")); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"dissolved": true})
}

func (h *Handler) joinGroup(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		Message string `json:"message"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	result, err := h.service.JoinGroup(
		r.Context(), user.ID, r.PathValue("conversation_id"), request.Message,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), result)
}

func (h *Handler) listGroupApplications(w http.ResponseWriter, r *http.Request, user auth.User) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.GroupApplications(r.Context(), user.ID, offset, limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) handleGroupApplication(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		Accept  bool   `json:"accept"`
		Message string `json:"message"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	if err := h.service.HandleGroupApplication(
		r.Context(), user.ID, r.PathValue("application_id"), request.Accept, request.Message,
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"handled": true})
}

func (h *Handler) listBlocks(w http.ResponseWriter, r *http.Request, user auth.User) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.BlockList(r.Context(), user.ID, offset, limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) setBlocked(w http.ResponseWriter, r *http.Request, user auth.User) {
	targetUserID, err := parseUserID(r.PathValue("user_id"))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户编号无效")
		return
	}
	var request struct {
		Blocked bool `json:"blocked"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	if err = h.service.SetBlocked(r.Context(), user.ID, targetUserID, request.Blocked); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"blocked": request.Blocked})
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		Sequence int64 `json:"sequence"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	if err := h.service.MarkRead(
		r.Context(), user.ID, r.PathValue("conversation_id"), request.Sequence,
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"read": true})
}

func (h *Handler) hideConversation(w http.ResponseWriter, r *http.Request, user auth.User) {
	if err := h.service.HideConversation(
		r.Context(), user.ID, r.PathValue("conversation_id"),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"hidden": true})
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		ClientMessageID string         `json:"client_message_id"`
		MessageType     int            `json:"message_type"`
		TextContent     string         `json:"text_content"`
		AssetID         int64          `json:"asset_id"`
		Metadata        map[string]any `json:"metadata"`
	}
	if !decodeRequest(w, r, &request) {
		return
	}
	message, err := h.service.Send(r.Context(), SendRequest{
		ConversationID: r.PathValue("conversation_id"), ClientMessageID: request.ClientMessageID,
		SenderUserID: user.ID, MessageType: request.MessageType, TextContent: request.TextContent,
		AssetID: request.AssetID, Metadata: request.Metadata,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), message)
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request, user auth.User) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before_sequence"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	messages, err := h.service.Messages(
		r.Context(), user.ID, r.PathValue("conversation_id"), before, limit,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": messages})
}

func (h *Handler) revokeMessage(w http.ResponseWriter, r *http.Request, user auth.User) {
	if err := h.service.RevokeMessage(
		r.Context(), user.ID, r.PathValue("conversation_id"), r.PathValue("message_id"),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"revoked": true})
}

func (h *Handler) socket(w http.ResponseWriter, r *http.Request) {
	connection, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(64 << 10)
	_ = connection.SetReadDeadline(time.Now().Add(8 * time.Second))
	var first struct {
		Type  string    `json:"type"`
		UID   jsonInt64 `json:"uid"`
		Token string    `json:"token"`
	}
	if err = connection.ReadJSON(&first); err != nil || first.Type != "auth" {
		_ = connection.WriteJSON(map[string]any{"type": "error", "code": 401, "message": "authentication required"})
		return
	}
	user, err := h.auth.Authenticate(r.Context(), int64(first.UID), first.Token)
	if err != nil {
		_ = connection.WriteJSON(map[string]any{"type": "error", "code": 401, "message": "invalid session"})
		return
	}
	_ = connection.SetReadDeadline(time.Now().Add(70 * time.Second))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(70 * time.Second))
	})
	if err = connection.WriteJSON(map[string]any{
		"type": "ready", "user_id": strconv.FormatInt(user.ID, 10), "server_time": time.Now().Unix(),
	}); err != nil {
		return
	}

	socketContext, cancel := context.WithCancel(r.Context())
	defer cancel()
	pubsub := h.redis.Subscribe(socketContext, "im:v2:user:"+strconv.FormatInt(user.ID, 10))
	defer pubsub.Close()
	if _, err = pubsub.Receive(socketContext); err != nil {
		return
	}
	incoming := make(chan socketCommand, 8)
	readErrors := make(chan error, 1)
	go func() {
		for {
			var command socketCommand
			if readErr := connection.ReadJSON(&command); readErr != nil {
				readErrors <- readErr
				return
			}
			select {
			case incoming <- command:
			case <-socketContext.Done():
				return
			}
		}
	}()

	redisMessages := pubsub.Channel(redis.WithChannelSize(128))
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-socketContext.Done():
			return
		case <-readErrors:
			return
		case message, ok := <-redisMessages:
			if !ok {
				return
			}
			_ = connection.SetWriteDeadline(time.Now().Add(8 * time.Second))
			if err = connection.WriteMessage(websocket.TextMessage, []byte(message.Payload)); err != nil {
				return
			}
		case command := <-incoming:
			if command.Type != "send" {
				_ = connection.WriteJSON(map[string]any{"type": "error", "code": 400, "message": "unsupported command"})
				continue
			}
			message, sendErr := h.service.Send(socketContext, SendRequest{
				ConversationID: command.ConversationID, ClientMessageID: command.ClientMessageID,
				SenderUserID: user.ID, MessageType: command.MessageType,
				TextContent: command.TextContent, AssetID: command.AssetID, Metadata: command.Metadata,
			})
			_ = connection.SetWriteDeadline(time.Now().Add(8 * time.Second))
			if sendErr != nil {
				code, text := PublicError(sendErr)
				if connection.WriteJSON(map[string]any{
					"type": "ack", "client_message_id": command.ClientMessageID,
					"code": code, "message": text,
				}) != nil {
					return
				}
				continue
			}
			if connection.WriteJSON(map[string]any{
				"type": "ack", "client_message_id": command.ClientMessageID,
				"code": 0, "data": message,
			}) != nil {
				return
			}
		case <-ping.C:
			_ = connection.SetWriteDeadline(time.Now().Add(8 * time.Second))
			if err = connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type socketCommand struct {
	Type            string         `json:"type"`
	ConversationID  string         `json:"conversation_id"`
	ClientMessageID string         `json:"client_message_id"`
	MessageType     int            `json:"message_type"`
	TextContent     string         `json:"text_content"`
	AssetID         int64          `json:"asset_id"`
	Metadata        map[string]any `json:"metadata"`
}

type authenticatedHandler func(http.ResponseWriter, *http.Request, auth.User)

func (h *Handler) authenticated(next authenticatedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, token := auth.Bearer(r)
		user, err := h.auth.Authenticate(r.Context(), userID, token)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 700, "登录已失效")
			return
		}
		next(w, r, user)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	code, message := PublicError(err)
	status := http.StatusBadRequest
	if code == 403 {
		status = http.StatusForbidden
	} else if code == 404 {
		status = http.StatusNotFound
	}
	httpx.Error(w, httpx.RequestID(r.Context()), status, code, message)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求格式错误")
		return false
	}
	return true
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}

func unixOrZero(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}
