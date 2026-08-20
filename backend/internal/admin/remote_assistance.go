package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/remoteassist"
)

func (h *Handler) listRemoteDevices(w http.ResponseWriter, r *http.Request) {
	if !h.remoteAvailable(w, r) {
		return
	}
	page, pageSize := pageParams(r)
	var online *bool
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("online"))) {
	case "1", "true":
		value := true
		online = &value
	case "0", "false":
		value := false
		online = &value
	}
	permission := strings.TrimSpace(r.URL.Query().Get("permission"))
	allowedPermissions := map[string]struct{}{
		"notification": {}, "media_projection": {}, "system_audio": {}, "accessibility": {},
		"overlay": {}, "all_files": {}, "microphone": {}, "battery": {},
	}
	var permissionGranted *bool
	if permission != "" {
		if _, valid := allowedPermissions[permission]; !valid {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "权限筛选条件无效")
			return
		}
		switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("permission_granted"))) {
		case "1", "true":
			value := true
			permissionGranted = &value
		case "0", "false":
			value := false
			permissionGranted = &value
		default:
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "权限筛选状态无效")
			return
		}
	}
	items, total, err := h.remote.ListDevices(r.Context(), r.URL.Query().Get("q"), online, permission, permissionGranted, pageSize, (page-1)*pageSize)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取远程设备失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": page*pageSize < total, "items": items,
	})
}

func (h *Handler) createRemoteCredential(w http.ResponseWriter, r *http.Request) {
	if !h.remoteAvailable(w, r) {
		return
	}
	adminUser, _ := adminFromRequest(r)
	if err := auditAdmin(r.Context(), h.db, r, "remote.authorization.request", "remote_device", r.PathValue("id"), nil, map[string]any{"requested": true}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录远程协助审计失败")
		return
	}
	credential, err := h.remote.CreateCredential(r.Context(), r.PathValue("id"), adminUser.ID)
	if err != nil {
		h.writeAdminRemoteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.OK(w, httpx.RequestID(r.Context()), credential)
}

func (h *Handler) remoteCredentialStatus(w http.ResponseWriter, r *http.Request) {
	if !h.remoteAvailable(w, r) {
		return
	}
	adminUser, _ := adminFromRequest(r)
	credential, err := h.remote.CredentialStatus(r.Context(), r.PathValue("id"), adminUser.ID)
	if err != nil {
		h.writeAdminRemoteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.OK(w, httpx.RequestID(r.Context()), credential)
}

func (h *Handler) revealRemoteCredential(w http.ResponseWriter, r *http.Request) {
	if !h.remoteAvailable(w, r) {
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	adminUser, _ := adminFromRequest(r)
	if err := h.auth.Reauthenticate(r.Context(), adminUser.ID, request.Password); err != nil {
		if errors.Is(err, adminauth.ErrInvalidCredentials) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "当前密码错误")
			return
		}
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "身份复核失败")
		return
	}
	// Persist the authorization audit before decrypting or returning a secret.
	// A failed reveal is still a security-relevant, reauthenticated attempt.
	if err := auditAdmin(r.Context(), h.db, r, "remote.authorization.activate", "remote_credential_request", r.PathValue("id"), nil, map[string]any{"identity_reverified": true}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录远程协助审计失败")
		return
	}
	credential, err := h.remote.RevealCredential(r.Context(), r.PathValue("id"), adminUser.ID)
	if err != nil {
		h.writeAdminRemoteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	httpx.OK(w, httpx.RequestID(r.Context()), credential)
}

func (h *Handler) revokeRemoteDevice(w http.ResponseWriter, r *http.Request) {
	if !h.remoteAvailable(w, r) {
		return
	}
	if err := auditAdmin(r.Context(), h.db, r, "remote.device.revoke", "remote_device", r.PathValue("id"), nil, map[string]any{"requested": true}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录远程协助审计失败")
		return
	}
	if err := h.remote.RevokeDevice(r.Context(), r.PathValue("id")); err != nil {
		h.writeAdminRemoteError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"revoking": true})
}

func (h *Handler) remoteFrame(w http.ResponseWriter, r *http.Request) {
	if !h.remoteAvailable(w, r) {
		return
	}
	adminUser, _ := adminFromRequest(r)
	frame, err := h.remote.Frame(r.Context(), r.PathValue("id"), adminUser.ID, r.Header.Get("X-Remote-Session"))
	if errors.Is(err, remoteassist.ErrNoFrame) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		h.writeAdminRemoteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(frame.JPEG)))
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("X-Frame-Width", strconv.Itoa(frame.Width))
	w.Header().Set("X-Frame-Height", strconv.Itoa(frame.Height))
	w.Header().Set("X-Frame-Rotation", strconv.Itoa(frame.Rotation))
	w.Header().Set("X-Frame-Sequence", strconv.FormatInt(frame.Sequence, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(frame.JPEG)
}

func (h *Handler) remoteControl(w http.ResponseWriter, r *http.Request) {
	if !h.remoteAvailable(w, r) {
		return
	}
	var request struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Payload == nil {
		request.Payload = map[string]any{}
	}
	adminUser, _ := adminFromRequest(r)
	commandType := strings.ToLower(strings.TrimSpace(request.Type))
	allowedCommands := map[string]struct{}{
		"tap": {}, "swipe": {}, "system_action": {}, "text": {}, "clipboard_set": {},
	}
	if _, allowed := allowedCommands[commandType]; !allowed {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "远程控制参数无效")
		return
	}
	if err := auditAdmin(r.Context(), h.db, r, "remote.control."+commandType, "remote_device", r.PathValue("id"), nil, map[string]any{"command_type": commandType}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录远程控制审计失败")
		return
	}
	if err := h.remote.QueueControlCommand(r.Context(), r.PathValue("id"), adminUser.ID, r.Header.Get("X-Remote-Session"), commandType, request.Payload); err != nil {
		h.writeAdminRemoteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"queued": true})
}

func (h *Handler) endRemoteControl(w http.ResponseWriter, r *http.Request) {
	if !h.remoteAvailable(w, r) {
		return
	}
	adminUser, _ := adminFromRequest(r)
	if err := auditAdmin(r.Context(), h.db, r, "remote.control.end", "remote_device", r.PathValue("id"), nil, map[string]any{"ended": true}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录远程控制审计失败")
		return
	}
	token := r.Header.Get("X-Remote-Session")
	if err := h.remote.QueueControlCommand(r.Context(), r.PathValue("id"), adminUser.ID, token, "end_session", map[string]any{}); err != nil {
		endErr := h.remote.EndControlSession(r.Context(), r.PathValue("id"), adminUser.ID, token)
		if !errors.Is(err, remoteassist.ErrOffline) || endErr != nil {
			h.writeAdminRemoteError(w, r, err)
			return
		}
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"ended": true})
}

func (h *Handler) remoteAvailable(w http.ResponseWriter, r *http.Request) bool {
	if h.remote != nil && h.remote.Enabled() {
		return true
	}
	httpx.Error(w, httpx.RequestID(r.Context()), http.StatusServiceUnavailable, 503, "远程协助暂未开放")
	return false
}

func (h *Handler) writeAdminRemoteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, remoteassist.ErrInvalid):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "远程控制参数无效")
	case errors.Is(err, remoteassist.ErrNotFound):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "远程设备或请求不存在")
	case errors.Is(err, remoteassist.ErrOffline):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "设备不在线或远程服务未运行")
	case errors.Is(err, remoteassist.ErrNotReady):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "远程控制授权尚未就绪或已过期")
	case errors.Is(err, remoteassist.ErrAlreadyRevealed):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusGone, 410, "远程控制授权已使用，不能再次开启")
	case errors.Is(err, remoteassist.ErrDisabled):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusServiceUnavailable, 503, "远程协助暂未开放")
	default:
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "远程协助操作失败")
	}
}
