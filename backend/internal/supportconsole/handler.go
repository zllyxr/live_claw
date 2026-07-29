package supportconsole

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

const (
	sessionCookie = "claw_support_session"
	csrfCookie    = "claw_support_csrf"
	consolePath   = "/support-console"
)

//go:embed web
var webFiles embed.FS

type Handler struct {
	auth          *adminauth.Service
	service       *Service
	loginTemplate *template.Template
	appTemplate   *template.Template
	static        http.Handler
	secureCookies bool
}

type contextAgentKey struct{}

func NewHandler(
	authService *adminauth.Service,
	service *Service,
	environment string,
) (*Handler, error) {
	loginBody, err := webFiles.ReadFile("web/login.html")
	if err != nil {
		return nil, err
	}
	appBody, err := webFiles.ReadFile("web/app.html")
	if err != nil {
		return nil, err
	}
	staticFS, err := fs.Sub(webFiles, "web/static")
	if err != nil {
		return nil, err
	}
	return &Handler{
		auth:          authService,
		service:       service,
		loginTemplate: template.Must(template.New("support-login").Parse(string(loginBody))),
		appTemplate:   template.Must(template.New("support-app").Parse(string(appBody))),
		static: http.StripPrefix(
			consolePath+"/static/",
			http.FileServer(http.FS(staticFS)),
		),
		secureCookies: environment != "local" && environment != "development",
	}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET "+consolePath, h.root)
	mux.HandleFunc("GET "+consolePath+"/", h.root)
	mux.HandleFunc("GET "+consolePath+"/login", h.loginPage)
	mux.HandleFunc("GET "+consolePath+"/app", h.requirePage(h.appPage))
	mux.Handle("GET "+consolePath+"/static/", h.static)

	mux.HandleFunc("POST "+consolePath+"/api/login", h.login)
	mux.HandleFunc("POST "+consolePath+"/api/logout", h.requireAPI(false, true, h.logout))
	mux.HandleFunc("POST "+consolePath+"/api/csrf", h.requireAPI(false, false, h.refreshCSRF))
	mux.HandleFunc("GET "+consolePath+"/api/me", h.requireAPI(false, false, h.me))
	mux.HandleFunc("GET "+consolePath+"/api/dashboard", h.requireAPI(false, false, h.dashboard))
	mux.HandleFunc("POST "+consolePath+"/api/presence", h.requireAPI(false, true, h.presence))
	mux.HandleFunc("GET "+consolePath+"/api/events", h.requireAPI(false, false, h.events))
	mux.HandleFunc("GET "+consolePath+"/api/agents", h.requireAPI(false, false, h.agents))
	mux.HandleFunc("GET "+consolePath+"/api/quick-replies", h.requireAPI(false, false, h.quickReplies))
	mux.HandleFunc("GET "+consolePath+"/api/conversations", h.requireAPI(false, false, h.conversations))
	mux.HandleFunc("GET "+consolePath+"/api/conversations/{id}", h.requireAPI(false, false, h.conversation))
	mux.HandleFunc("GET "+consolePath+"/api/conversations/{id}/messages", h.requireAPI(false, false, h.messages))
	mux.HandleFunc("POST "+consolePath+"/api/conversations/{id}/claim", h.requireAPI(true, true, h.claim))
	mux.HandleFunc("POST "+consolePath+"/api/conversations/{id}/messages", h.requireAPI(true, true, h.send))
	mux.HandleFunc("POST "+consolePath+"/api/conversations/{id}/transfer", h.requireAPI(true, true, h.transfer))
	mux.HandleFunc("POST "+consolePath+"/api/conversations/{id}/resolve", h.requireAPI(true, true, h.resolve))
	mux.HandleFunc("POST "+consolePath+"/api/conversations/{id}/priority", h.requireAPI(true, true, h.priority))
	mux.HandleFunc("POST "+consolePath+"/api/users/{id}/notes", h.requireAPI(true, true, h.addNote))
}

func (h *Handler) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != consolePath && r.URL.Path != consolePath+"/" {
		http.NotFound(w, r)
		return
	}
	if _, _, err := h.current(r); err != nil {
		http.Redirect(w, r, consolePath+"/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, consolePath+"/app", http.StatusSeeOther)
}

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.current(r); err == nil {
		http.Redirect(w, r, consolePath+"/app", http.StatusSeeOther)
		return
	}
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.loginTemplate.Execute(w, nil)
}

func (h *Handler) appPage(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.appTemplate.Execute(w, agent)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, err := h.auth.LoginForPortal(
		r.Context(), adminauth.PortalSupport,
		request.Username, request.Password, clientIP(r), r.UserAgent(),
	)
	if errors.Is(err, adminauth.ErrInvalidCredentials) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "座席账号或密码错误")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "座席登录暂不可用")
		return
	}
	agent, err := h.service.Agent(r.Context(), session.Admin)
	if err != nil {
		_ = h.auth.LogoutForPortal(r.Context(), adminauth.PortalSupport, session.Token)
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "该账号未开通客服座席权限")
		return
	}
	h.setSessionCookies(w, session)
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"agent": agent, "csrf_token": session.CSRFToken,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	_ = h.service.SetPresence(r.Context(), agent, 0)
	if err := h.auth.LogoutForPortal(
		r.Context(), adminauth.PortalSupport, sessionToken(r),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "退出失败")
		return
	}
	h.clearSessionCookies(w)
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"logged_out": true})
}

func (h *Handler) refreshCSRF(w http.ResponseWriter, r *http.Request) {
	csrf, err := h.auth.RefreshCSRFForPortal(
		r.Context(), adminauth.PortalSupport, sessionToken(r),
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "请重新登录")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: csrf, Path: consolePath,
		HttpOnly: false, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		MaxAge: int((12 * time.Hour).Seconds()),
	})
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]string{"csrf_token": csrf})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	httpx.OK(w, httpx.RequestID(r.Context()), agent)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	result, err := h.service.Dashboard(r.Context(), agent)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), result)
}

func (h *Handler) presence(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	var request struct {
		Presence int `json:"presence"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := h.service.SetPresence(r.Context(), agent, request.Presence); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]int{"presence": request.Presence})
}

func (h *Handler) conversations(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	items, err := h.service.Conversations(
		r.Context(), agent, r.URL.Query().Get("scope"), r.URL.Query().Get("q"),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) conversation(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	item, user, notes, err := h.service.Conversation(r.Context(), agent, r.PathValue("id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"conversation": item, "user": user, "notes": notes,
	})
}

func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.service.Messages(
		r.Context(), agent, r.PathValue("id"),
		r.URL.Query().Get("before_id"), limit,
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) claim(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	if err := h.service.Claim(
		r.Context(), agent, r.PathValue("id"), actionMeta(r),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"claimed": true})
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	var request struct {
		ClientMessageID string `json:"client_message_id"`
		MessageType     int    `json:"message_type"`
		TextContent     string `json:"text_content"`
		AssetID         int64  `json:"asset_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	message, err := h.service.Send(
		r.Context(), agent, r.PathValue("id"),
		SendRequest{
			ClientMessageID: request.ClientMessageID,
			MessageType:     request.MessageType, TextContent: request.TextContent,
			AssetID: request.AssetID,
		},
		actionMeta(r),
	)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), message)
}

func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	var request struct {
		TargetAgentID int64 `json:"target_agent_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := h.service.Transfer(
		r.Context(), agent, r.PathValue("id"), request.TargetAgentID, actionMeta(r),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"transferred": true})
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	if err := h.service.Resolve(
		r.Context(), agent, r.PathValue("id"), actionMeta(r),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"resolved": true})
}

func (h *Handler) priority(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	var request struct {
		Priority int `json:"priority"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := h.service.SetPriority(
		r.Context(), agent, r.PathValue("id"), request.Priority, actionMeta(r),
	); err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]int{"priority": request.Priority})
}

func (h *Handler) agents(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Agents(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) quickReplies(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.QuickReplies(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) addNote(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userID < 1 {
		h.writeError(w, r, ErrInvalidRequest)
		return
	}
	var request struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	note, err := h.service.AddNote(r.Context(), agent, userID, request.Content, actionMeta(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), note)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	agent, _ := agentFromRequest(r)
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotImplemented, 501, "事件推送不可用")
		return
	}
	channel, unsubscribe := h.service.support.Subscribe()
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	responseController := http.NewResponseController(w)
	refreshWriteDeadline := func() {
		_ = responseController.SetWriteDeadline(time.Now().Add(30 * time.Second))
	}
	refreshWriteDeadline()
	_, _ = fmt.Fprint(w, "retry: 2000\nevent: ready\ndata: {\"type\":\"ready\"}\n\n")
	flusher.Flush()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case event, open := <-channel:
			if !open {
				return
			}
			if !h.service.canObserveEvent(r.Context(), agent, event) {
				continue
			}
			payload, _ := json.Marshal(struct {
				Type           string `json:"type"`
				ConversationID string `json:"conversation_id,omitempty"`
				CreatedAt      int64  `json:"created_at"`
			}{
				Type: event.Type, ConversationID: event.ConversationID,
				CreatedAt: event.CreatedAt,
			})
			refreshWriteDeadline()
			_, _ = fmt.Fprintf(w, "event: support\ndata: %s\n\n", payload)
			flusher.Flush()
		case <-heartbeat.C:
			refreshWriteDeadline()
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

type consoleHandler func(http.ResponseWriter, *http.Request)

func (h *Handler) requirePage(next consoleHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, agent, err := h.current(r)
		if err != nil {
			http.Redirect(w, r, consolePath+"/login", http.StatusSeeOther)
			return
		}
		next(w, withAgent(r, agent))
	}
}

func (h *Handler) requireAPI(
	write bool,
	requireCSRF bool,
	next consoleHandler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, agent, err := h.current(r)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "座席登录已失效")
			return
		}
		if !hasPermission(admin, "support.read") ||
			(write && !hasPermission(admin, "support.write")) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "无权执行此客服操作")
			return
		}
		if requireCSRF && !h.auth.VerifyCSRFForPortal(
			r.Context(), adminauth.PortalSupport, sessionToken(r),
			r.Header.Get("X-CSRF-Token"),
		) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "请求校验失败")
			return
		}
		next(w, withAgent(r, agent))
	}
}

func (h *Handler) current(r *http.Request) (adminauth.Admin, Agent, error) {
	admin, err := h.auth.AuthenticateForPortal(
		r.Context(), adminauth.PortalSupport, sessionToken(r),
	)
	if err != nil {
		return adminauth.Admin{}, Agent{}, err
	}
	agent, err := h.service.Agent(r.Context(), admin)
	return admin, agent, err
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrAgentNotFound):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "客服座席不可用")
	case errors.Is(err, ErrPermissionDenied):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "无权访问该客服会话")
	case errors.Is(err, ErrConversationMissing):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "客服会话不存在")
	case errors.Is(err, ErrConversationClaimed):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该会话已被其他座席接入")
	case errors.Is(err, ErrConversationClosed):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该会话已经结束")
	case errors.Is(err, ErrAgentCapacity):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "当前处理中的会话已达上限")
	case errors.Is(err, ErrInvalidRequest):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "客服操作参数无效")
	default:
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "客服座席服务暂不可用")
	}
}

func (h *Handler) setSessionCookies(w http.ResponseWriter, session adminauth.Session) {
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: session.Token, Path: consolePath,
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: maxAge,
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: session.CSRFToken, Path: consolePath,
		HttpOnly: false, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: maxAge,
	})
}

func (h *Handler) clearSessionCookies(w http.ResponseWriter) {
	for _, item := range []struct {
		name     string
		httpOnly bool
	}{
		{name: sessionCookie, httpOnly: true},
		{name: csrfCookie, httpOnly: false},
	} {
		http.SetCookie(w, &http.Cookie{
			Name: item.name, Value: "", Path: consolePath, HttpOnly: item.httpOnly,
			Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
			MaxAge: -1, Expires: time.Unix(1, 0),
		})
	}
}

func (h *Handler) securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; img-src 'self' data: blob:; style-src 'self'; "+
			"script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Cache-Control", "no-store")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求格式错误")
		return false
	}
	return true
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func withAgent(r *http.Request, agent Agent) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextAgentKey{}, agent))
}

func agentFromRequest(r *http.Request) (Agent, bool) {
	agent, ok := r.Context().Value(contextAgentKey{}).(Agent)
	return agent, ok
}

func actionMeta(r *http.Request) ActionMeta {
	return ActionMeta{
		RequestID: httpx.RequestID(r.Context()),
		IP:        clientIP(r), UserAgent: r.UserAgent(),
	}
}

func clientIP(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
