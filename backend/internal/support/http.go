package support

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

type Handler struct {
	service *Service
	auth    *auth.Service
}

func NewHandler(service *Service, authService *auth.Service) *Handler {
	return &Handler{service: service, auth: authService}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v2/support/conversations/current", h.authenticated(h.current))
	mux.HandleFunc("GET /api/v2/support/conversations/{conversation_id}/messages", h.authenticated(h.messages))
	mux.HandleFunc("POST /api/v2/support/conversations/{conversation_id}/messages", h.authenticated(h.send))
}

func (h *Handler) current(w http.ResponseWriter, r *http.Request, user auth.User) {
	conversation, err := h.service.Current(r.Context(), user.ID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), conversation)
}

func (h *Handler) messages(w http.ResponseWriter, r *http.Request, user auth.User) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.Messages(
		r.Context(), user.ID, r.PathValue("conversation_id"),
		r.URL.Query().Get("before_id"), limit,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request, user auth.User) {
	var request struct {
		ClientMessageID string `json:"client_message_id"`
		MessageType     int    `json:"message_type"`
		TextContent     string `json:"text_content"`
		AssetID         int64  `json:"asset_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求格式错误")
		return
	}
	message, err := h.service.Send(r.Context(), SendRequest{
		ConversationID: r.PathValue("conversation_id"),
		UserID:         user.ID, ClientMessageID: request.ClientMessageID,
		MessageType: request.MessageType, TextContent: request.TextContent, AssetID: request.AssetID,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), message)
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
	switch {
	case errors.Is(err, ErrConversationNotFound):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "客服会话不存在")
	case errors.Is(err, ErrConversationClosed):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "客服会话已结束")
	case errors.Is(err, ErrInvalidMessage):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "消息内容无效")
	default:
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "客服服务暂不可用")
	}
}
