package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/mailer"
)

func (h *Handler) getEmailSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.emailSettings.Read(r.Context())
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取邮箱发送设置失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), settings.Public())
}

func (h *Handler) saveEmailSettings(w http.ResponseWriter, r *http.Request) {
	var request struct {
		mailer.Settings
		Version int64 `json:"version"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Version < 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "配置版本无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "保存邮箱发送设置失败")
		return
	}
	defer tx.Rollback()
	actor, _ := adminFromRequest(r)
	before, after, err := h.emailSettings.Save(r.Context(), tx, request.Settings, request.Version, actor.ID)
	if errors.Is(err, mailer.ErrSettingsConflict) {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "设置已被更新，请刷新后重试")
		return
	}
	if errors.Is(err, mailer.ErrInvalidSettings) {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, strings.TrimPrefix(err.Error(), mailer.ErrInvalidSettings.Error()+": "))
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "保存邮箱发送设置失败")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "system.email.update", "system_setting", mailer.SettingKey, before.Public(), after.Public()); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录邮箱设置审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "保存邮箱发送设置失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), after.Public())
}

func (h *Handler) testEmailSettings(w http.ResponseWriter, r *http.Request) {
	var request struct {
		To string `json:"to"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.To = strings.TrimSpace(request.To)
	if !mailer.ValidAddress(request.To) {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "请输入有效的测试收件邮箱")
		return
	}
	sender, err := h.emailSettings.Sender(r.Context())
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取邮箱发送设置失败")
		return
	}
	if sender == nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "请先保存并启用邮箱发送设置")
		return
	}
	now := time.Now().Unix()
	next := h.emailTestNext.Load()
	if now < next || !h.emailTestNext.CompareAndSwap(next, now+60) {
		httpx.Error(w, httpx.RequestID(r.Context()), 429, 429, "测试发送每分钟最多一次，请稍后重试")
		return
	}
	// Record the explicit operator request before causing an external send.
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录测试发送失败")
		return
	}
	defer tx.Rollback()
	if err = auditAdmin(r.Context(), tx, r, "system.email.test", "system_setting", mailer.SettingKey, nil, map[string]any{"recipient": request.To, "status": "requested"}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录测试发送失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录测试发送失败")
		return
	}
	if err = sender.SendTest(r.Context(), request.To); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 502, 502, "邮件发送失败，请检查服务器、端口、加密方式和应用专用密码")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]string{"message": "邮件服务器已接受测试邮件，请检查收件箱及垃圾邮件"})
}
