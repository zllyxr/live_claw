package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

var teamCodePattern = regexp.MustCompile(`^[0-9a-z]{3}$`)

func (h *Handler) listTeams(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT team.id,team.code,team.name,team.owner_user_id,team.status,team.created_at,
		       COALESCE(member_count.count_value,0)
		FROM teams team
		LEFT JOIN (
			SELECT team_id,COUNT(*) count_value
			FROM team_members WHERE status=1 GROUP BY team_id
		) member_count ON member_count.team_id=team.id
		ORDER BY team.status DESC,team.code`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 32)
	for rows.Next() {
		var id, ownerUserID, memberCount int64
		var code, name string
		var status int
		var createdAt time.Time
		if err = rows.Scan(&id, &code, &name, &ownerUserID, &status, &createdAt, &memberCount); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队失败")
			return
		}
		items = append(items, map[string]any{
			"id": id, "code": code, "name": name, "owner_user_id": ownerUserID,
			"status": status, "member_count": memberCount, "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		OwnerUserID int64  `json:"owner_user_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Code = strings.ToLower(strings.TrimSpace(request.Code))
	request.Name = strings.TrimSpace(request.Name)
	if !teamCodePattern.MatchString(request.Code) || request.Name == "" ||
		len(request.Name) > 100 || request.OwnerUserID < 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队参数无效")
		return
	}
	adminUser, _ := adminFromRequest(r)
	result, err := h.db.ExecContext(r.Context(), `
		INSERT INTO teams(code,name,owner_user_id,status,created_by)
		SELECT ?,?,?,1,?
		WHERE ?=0 OR EXISTS(SELECT 1 FROM users WHERE id=? AND status=1)`,
		request.Code, request.Name, request.OwnerUserID, adminUser.ID,
		request.OwnerUserID, request.OwnerUserID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "团队代码已存在")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队负责人不存在")
		return
	}
	teamID, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), h.db, r, "team.create", "team", teamID, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录团队审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": teamID, "code": request.Code})
}

func (h *Handler) updateTeam(w http.ResponseWriter, r *http.Request) {
	teamID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || teamID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队编号无效")
		return
	}
	var request struct {
		Name        string `json:"name"`
		OwnerUserID int64  `json:"owner_user_id"`
		Status      int    `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > 100 || request.OwnerUserID < 0 ||
		request.Status < 0 || request.Status > 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队参数无效")
		return
	}
	var beforeName string
	var beforeOwner int64
	var beforeStatus int
	if err = h.db.QueryRowContext(r.Context(), `
		SELECT name,owner_user_id,status FROM teams WHERE id=?`,
		teamID,
	).Scan(&beforeName, &beforeOwner, &beforeStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "团队不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新团队失败")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE teams SET name=?,owner_user_id=?,status=?
		WHERE id=? AND (?=0 OR EXISTS(SELECT 1 FROM users WHERE id=? AND status=1))`,
		request.Name, request.OwnerUserID, request.Status,
		teamID, request.OwnerUserID, request.OwnerUserID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新团队失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队负责人不存在")
		return
	}
	if err = auditAdmin(
		r.Context(), h.db, r, "team.update", "team", teamID,
		map[string]any{"name": beforeName, "owner_user_id": beforeOwner, "status": beforeStatus},
		request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录团队审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": teamID, "updated": true})
}

func (h *Handler) assignUserTeam(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户编号无效")
		return
	}
	var request struct {
		TeamID int64  `json:"team_id"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.TeamID < 1 || request.Reason == "" || len(request.Reason) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "调整团队参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var oldTeamID int64
	if err = tx.QueryRowContext(r.Context(), `
		SELECT team_id FROM users WHERE id=? FOR UPDATE`,
		userID,
	).Scan(&oldTeamID); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "用户不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
		return
	}
	var teamCode string
	if err = tx.QueryRowContext(r.Context(), `
		SELECT code FROM teams WHERE id=? AND status=1 FOR UPDATE`,
		request.TeamID,
	).Scan(&teamCode); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "目标团队不存在或已停用")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
		return
	}
	if oldTeamID == request.TeamID {
		httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
			"user_id": userID, "team_id": request.TeamID, "team_code": teamCode, "unchanged": true,
		})
		return
	}
	var oldInviteCode sql.NullString
	if err = tx.QueryRowContext(r.Context(), `
		SELECT full_code FROM invite_codes WHERE user_id=? FOR UPDATE`,
		userID,
	).Scan(&oldInviteCode); err != nil && !errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
		return
	}
	if oldInviteCode.Valid {
		if _, err = tx.ExecContext(r.Context(), `
			INSERT INTO invite_code_aliases(alias_code,user_id,expires_at)
			VALUES(?,?,CURRENT_TIMESTAMP(3)+INTERVAL 180 DAY)
			ON DUPLICATE KEY UPDATE user_id=VALUES(user_id),expires_at=VALUES(expires_at)`,
			oldInviteCode.String, userID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
			return
		}
		if _, err = tx.ExecContext(r.Context(), "DELETE FROM invite_codes WHERE user_id=?", userID); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE users SET team_id=? WHERE id=?", request.TeamID, userID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO team_members(user_id,team_id,inviter_user_id,status,joined_at,left_at)
		VALUES(?,?,0,1,CURRENT_TIMESTAMP(3),NULL)
		ON DUPLICATE KEY UPDATE
			team_id=VALUES(team_id),inviter_user_id=0,status=1,
			joined_at=CURRENT_TIMESTAMP(3),left_at=NULL`,
		userID, request.TeamID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "user.team.assign", "user", userID,
		map[string]any{"team_id": oldTeamID, "invite_code": oldInviteCode.String},
		map[string]any{"team_id": request.TeamID, "team_code": teamCode, "reason": request.Reason},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录团队审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调整用户团队失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"user_id": userID, "team_id": request.TeamID, "team_code": teamCode,
		"invite_code_regenerated": oldInviteCode.Valid,
	})
}
