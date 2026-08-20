package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/remoteassist"
)

func (s *Server) remoteEnroll(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.remoteUser(w, r)
	if !ok {
		return
	}
	var request remoteassist.EnrollRequest
	if !decodeRemoteJSON(w, r, &request, 32<<10) {
		return
	}
	result, err := s.remote.Enroll(r.Context(), userID, request)
	if err != nil {
		s.writeRemoteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.OK(w, httpx.RequestID(r.Context()), result)
}

func (s *Server) remoteCurrent(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.remoteUser(w, r)
	if !ok {
		return
	}
	result, err := s.remote.Current(r.Context(), userID, remoteInstallID(r))
	if err != nil {
		s.writeRemoteError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), result)
}

func (s *Server) remoteUnbind(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.remoteUser(w, r)
	if !ok {
		return
	}
	if err := s.remote.Unbind(r.Context(), userID, remoteInstallID(r)); err != nil {
		s.writeRemoteError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"unbound": true})
}

func (s *Server) remoteHeartbeat(w http.ResponseWriter, r *http.Request) {
	device, ok := s.remoteDevice(w, r)
	if !ok {
		return
	}
	var request remoteassist.Heartbeat
	if !decodeRemoteJSON(w, r, &request, 64<<10) {
		return
	}
	commands, err := s.remote.Heartbeat(r.Context(), device, request)
	if err != nil {
		s.writeRemoteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"commands": commands, "next_heartbeat_seconds": 5})
}

func (s *Server) remoteCommandAck(w http.ResponseWriter, r *http.Request) {
	device, ok := s.remoteDevice(w, r)
	if !ok {
		return
	}
	var request remoteassist.Ack
	if !decodeRemoteJSON(w, r, &request, 16<<10) {
		return
	}
	if err := s.remote.AckCommand(r.Context(), device, r.PathValue("id"), request); err != nil {
		s.writeRemoteError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"acknowledged": true})
}

func (s *Server) remoteEvents(w http.ResponseWriter, r *http.Request) {
	device, ok := s.remoteDevice(w, r)
	if !ok {
		return
	}
	var request struct {
		Events []remoteassist.Event `json:"events"`
	}
	if !decodeRemoteJSON(w, r, &request, 64<<10) {
		return
	}
	if err := s.remote.RecordEvents(r.Context(), device, request.Events); err != nil {
		s.writeRemoteError(w, r, err)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]int{"accepted": len(request.Events)})
}

func (s *Server) remoteUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if s.remote == nil || !s.remote.Enabled() {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusServiceUnavailable, 503, "远程协助暂未开放")
		return 0, false
	}
	userID, token := auth.Bearer(r)
	user, err := s.auth.Authenticate(r.Context(), userID, token)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 700, "登录已失效")
		return 0, false
	}
	return user.ID, true
}

func (s *Server) remoteDevice(w http.ResponseWriter, r *http.Request) (remoteassist.Device, bool) {
	if s.remote == nil || !s.remote.Enabled() {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusServiceUnavailable, 503, "远程协助暂未开放")
		return remoteassist.Device{}, false
	}
	scheme, token, found := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	if !found || scheme != "Device" || strings.TrimSpace(token) == "" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "设备凭据无效")
		return remoteassist.Device{}, false
	}
	device, err := s.remote.AuthenticateDevice(r.Context(), token)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "设备凭据无效")
		return remoteassist.Device{}, false
	}
	return device, true
}

func (s *Server) writeRemoteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, remoteassist.ErrDisabled):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusServiceUnavailable, 503, "远程协助暂未开放")
	case errors.Is(err, remoteassist.ErrInvalid):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求参数无效")
	case errors.Is(err, remoteassist.ErrNotFound):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "远程设备不存在")
	case errors.Is(err, remoteassist.ErrConflict):
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "当前安装实例已绑定其他账号")
	default:
		s.logger.Error("remote assistance request", "request_id", httpx.RequestID(r.Context()), "error", err)
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "远程协助暂不可用")
	}
}

func remoteInstallID(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Install-ID")); value != "" {
		return value
	}
	return strings.TrimSpace(r.URL.Query().Get("install_id"))
}

func decodeRemoteJSON(w http.ResponseWriter, r *http.Request, target any, limit int64) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求格式错误")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求格式错误")
		return false
	}
	return true
}
