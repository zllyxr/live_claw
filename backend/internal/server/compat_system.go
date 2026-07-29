package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type compatContentPage struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (s *Server) compatSystem(w http.ResponseWriter, r *http.Request, service string) bool {
	if service != "System.getPage" {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(r.FormValue("key")))
	if key != "recharge_agreement" {
		writeCompat(w, 404, "页面不存在", nil)
		return true
	}
	var raw []byte
	err := s.db.QueryRowContext(r.Context(), `
		SELECT JSON_EXTRACT(setting_value, CONCAT('$.', ?))
		FROM system_settings
		WHERE setting_key='content.pages' AND is_secret=0`,
		key,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || string(raw) == "null" {
		writeCompat(w, 404, "页面内容尚未配置", nil)
		return true
	}
	if err != nil {
		s.logger.Error("read public content page", "key", key, "error", err)
		writeCompat(w, 500, "页面内容加载失败", nil)
		return true
	}
	var page compatContentPage
	if err = json.Unmarshal(raw, &page); err != nil || strings.TrimSpace(page.Title) == "" ||
		strings.TrimSpace(page.Content) == "" {
		writeCompat(w, 500, "页面内容配置无效", nil)
		return true
	}
	writeCompat(w, 0, "", page)
	return true
}
