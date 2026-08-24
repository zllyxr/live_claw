package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

var platformAgentPermissionAllowlist = map[string]struct{}{
	"games.read": {}, "games.write": {},
	"live.read": {}, "live.write": {},
	"lottery.read": {}, "lottery.write": {},
	"sports.read": {}, "sports.write": {},
	"bets.read": {},
	"app.read":  {}, "app.write": {},
}

var platformAgentWriteDependencies = map[string]string{
	"games.write":   "games.read",
	"live.write":    "live.read",
	"lottery.write": "lottery.read",
	"sports.write":  "sports.read",
	"app.write":     "app.read",
}

func normalizeAgentPermissionKeys(values []string) ([]string, error) {
	set := make(map[string]struct{}, len(values)+len(platformAgentWriteDependencies))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if _, allowed := platformAgentPermissionAllowlist[key]; !allowed {
			return nil, errors.New("permission is not available to platform agents")
		}
		set[key] = struct{}{}
		if dependency := platformAgentWriteDependencies[key]; dependency != "" {
			set[dependency] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	sort.Strings(result)
	return result, nil
}

func platformAgentAllowedPermissions() []string {
	keys := make([]string, 0, len(platformAgentPermissionAllowlist))
	for key := range platformAgentPermissionAllowlist {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func splitPermissionCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func (h *Handler) listPlatformAgents(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		if parsed, err := strconv.Atoi(rawStatus); err == nil && (parsed == 0 || parsed == 1) {
			status = parsed
		}
	}
	args := []any{keyword, like, like, like, status, status}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM platform_agents agent
		JOIN admin_users admin ON admin.id=agent.admin_user_id
		WHERE (?='' OR admin.username LIKE ? OR admin.display_name LIKE ? OR agent.agent_no LIKE ?)
		  AND (? < 0 OR agent.status=?)`, args...).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取代理失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT agent.admin_user_id,agent.agent_no,admin.username,
		       COALESCE(NULLIF(admin.display_name,''),admin.username),COALESCE(admin.email,''),
		       agent.status,admin.last_login_at,agent.created_at,
		       (SELECT COUNT(*) FROM platform_agent_teams owned WHERE owned.admin_user_id=agent.admin_user_id),
		       COALESCE((
		         SELECT GROUP_CONCAT(permission.permission_key ORDER BY permission.permission_key SEPARATOR ',')
		         FROM platform_agent_permissions grant_row
		         JOIN admin_permissions permission ON permission.id=grant_row.permission_id
		         WHERE grant_row.admin_user_id=agent.admin_user_id
		       ),'')
		FROM platform_agents agent
		JOIN admin_users admin ON admin.id=agent.admin_user_id
		WHERE (?='' OR admin.username LIKE ? OR admin.display_name LIKE ? OR agent.agent_no LIKE ?)
		  AND (? < 0 OR agent.status=?)
		ORDER BY agent.created_at DESC,agent.admin_user_id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取代理失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, prefixCount int64
		var agentNo, username, displayName, email, permissionCSV string
		var agentStatus int
		var lastLogin sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &agentNo, &username, &displayName, &email, &agentStatus,
			&lastLogin, &createdAt, &prefixCount, &permissionCSV,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取代理失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "agent_no": agentNo, "username": username,
			"display_name": displayName, "email": email, "status": agentStatus,
			"permissions": splitPermissionCSV(permissionCSV), "prefix_count": prefixCount,
			"last_login_at": nullTime(lastLogin), "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取代理失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
		"allowed_permissions": platformAgentAllowedPermissions(),
	})
}

type createPlatformAgentRequest struct {
	Username       string   `json:"username"`
	Password       string   `json:"password"`
	DisplayName    string   `json:"display_name"`
	Email          string   `json:"email"`
	PermissionKeys []string `json:"permission_keys"`
}

func (h *Handler) createPlatformAgent(w http.ResponseWriter, r *http.Request) {
	var request createPlatformAgentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	permissions, err := normalizeAgentPermissionKeys(request.PermissionKeys)
	if !adminNamePattern.MatchString(request.Username) || !validManagedPassword(request.Password) ||
		request.DisplayName == "" || len(request.DisplayName) > 100 ||
		(request.Email != "" && !validUserEmail(request.Email)) || err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理参数无效")
		return
	}
	passwordHash, err := adminauth.HashPassword(request.Password)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理密码不符合安全要求")
		return
	}
	agentNo, err := idgen.New()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建代理失败")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建代理失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO admin_users(username,password_hash,display_name,email,status)
		VALUES(?,?,?,?,1)`,
		request.Username, passwordHash, request.DisplayName, nullableUserField(request.Email),
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "代理账号或邮箱已存在")
		return
	}
	agentID, _ := result.LastInsertId()
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO platform_agents(admin_user_id,agent_no,status) VALUES(?,?,1)`,
		agentID, agentNo,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建代理失败")
		return
	}
	if err = replacePlatformAgentPermissions(r, tx, agentID, permissions); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理权限无效")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "agent.create", "platform_agent", agentID, nil, map[string]any{
		"username": request.Username, "display_name": request.DisplayName,
		"permissions": permissions, "status": 1,
	}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录代理审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建代理失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(agentID), "agent_no": agentNo,
	})
}

type updatePlatformAgentRequest struct {
	DisplayName    string   `json:"display_name"`
	Email          string   `json:"email"`
	Status         int      `json:"status"`
	PermissionKeys []string `json:"permission_keys"`
}

func (h *Handler) updatePlatformAgent(w http.ResponseWriter, r *http.Request) {
	agentID, err := parsePositivePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理编号无效")
		return
	}
	var request updatePlatformAgentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	permissions, permissionErr := normalizeAgentPermissionKeys(request.PermissionKeys)
	if request.DisplayName == "" || len(request.DisplayName) > 100 ||
		(request.Email != "" && !validUserEmail(request.Email)) ||
		(request.Status != 0 && request.Status != 1) || permissionErr != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新代理失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var username, beforeName, beforeEmail, beforePermissions string
	var beforeStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT admin.username,admin.display_name,COALESCE(admin.email,''),agent.status,
		       COALESCE((SELECT GROUP_CONCAT(permission.permission_key ORDER BY permission.permission_key SEPARATOR ',')
		         FROM platform_agent_permissions grant_row
		         JOIN admin_permissions permission ON permission.id=grant_row.permission_id
		         WHERE grant_row.admin_user_id=agent.admin_user_id),'')
		FROM platform_agents agent
		JOIN admin_users admin ON admin.id=agent.admin_user_id
		WHERE agent.admin_user_id=? FOR UPDATE`, agentID,
	).Scan(&username, &beforeName, &beforeEmail, &beforeStatus, &beforePermissions); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "代理不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新代理失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE admin_users SET display_name=?,email=?,status=? WHERE id=?`,
		request.DisplayName, nullableUserField(request.Email), request.Status, agentID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "代理邮箱已存在")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE platform_agents SET status=? WHERE admin_user_id=?`, request.Status, agentID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新代理失败")
		return
	}
	if err = replacePlatformAgentPermissions(r, tx, agentID, permissions); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理权限无效")
		return
	}
	if request.Status == 0 {
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE admin_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
			WHERE admin_user_id=? AND portal='agent' AND revoked_at IS NULL`, agentID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "撤销代理会话失败")
			return
		}
	}
	if err = auditAdmin(r.Context(), tx, r, "agent.update", "platform_agent", agentID,
		map[string]any{"username": username, "display_name": beforeName, "email": beforeEmail, "status": beforeStatus, "permissions": splitPermissionCSV(beforePermissions)},
		map[string]any{"username": username, "display_name": request.DisplayName, "email": request.Email, "status": request.Status, "permissions": permissions},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录代理审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新代理失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(agentID), "updated": true})
}

func (h *Handler) resetPlatformAgentPassword(w http.ResponseWriter, r *http.Request) {
	agentID, err := parsePositivePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理编号无效")
		return
	}
	var request resetAdministratorPasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err = request.normalize(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "密码须为 12 至 128 个字符，且必须填写重置原因")
		return
	}
	passwordHash, err := adminauth.HashPassword(request.Password)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理密码不符合安全要求")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置代理密码失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var username, previousHash string
	if err = tx.QueryRowContext(r.Context(), `
		SELECT admin.username,admin.password_hash
		FROM platform_agents agent JOIN admin_users admin ON admin.id=agent.admin_user_id
		WHERE agent.admin_user_id=? FOR UPDATE`, agentID,
	).Scan(&username, &previousHash); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "代理不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置代理密码失败")
		return
	}
	if adminauth.VerifyPassword(previousHash, request.Password) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "新密码不能与当前密码相同")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE admin_users SET password_hash=?,password_changed_at=CURRENT_TIMESTAMP(3) WHERE id=?`,
		passwordHash, agentID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置代理密码失败")
		return
	}
	revokeResult, err := tx.ExecContext(r.Context(), `
		UPDATE admin_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
		WHERE admin_user_id=? AND revoked_at IS NULL`, agentID)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "撤销代理会话失败")
		return
	}
	revokedSessions, _ := revokeResult.RowsAffected()
	if err = auditAdmin(r.Context(), tx, r, "agent.password.reset", "platform_agent", agentID, nil, map[string]any{
		"username": username, "password_changed": true, "reason": request.Reason,
		"revoked_sessions": revokedSessions,
	}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录代理审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置代理密码失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(agentID), "password_reset": true, "revoked_sessions": revokedSessions,
	})
}

func (h *Handler) listPlatformAgentTeamPrefixes(w http.ResponseWriter, r *http.Request) {
	agentID, err := parsePositivePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理编号无效")
		return
	}
	var exists int
	if err = h.db.QueryRowContext(r.Context(), `SELECT 1 FROM platform_agents WHERE admin_user_id=?`, agentID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "代理不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取代理团队失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT team.id,team.code,team.name,team.status,owned.assigned_at,
		       COALESCE((SELECT COUNT(*) FROM team_members member WHERE member.team_id=team.id AND member.status=1),0)
		FROM platform_agent_teams owned
		JOIN teams team ON team.id=owned.team_id
		WHERE owned.admin_user_id=? ORDER BY team.code`, agentID)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取代理团队失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 16)
	for rows.Next() {
		var teamID, count int64
		var code, name string
		var status int
		var assignedAt time.Time
		if err = rows.Scan(&teamID, &code, &name, &status, &assignedAt, &count); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取代理团队失败")
			return
		}
		items = append(items, map[string]any{
			"team_id": apiDecimalID(teamID), "code": code, "name": name,
			"status": status, "member_count": count, "assigned_at": assignedAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取代理团队失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

type agentTeamAssignmentRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) assignPlatformAgentTeamPrefix(w http.ResponseWriter, r *http.Request) {
	h.changePlatformAgentTeamPrefix(w, r, true)
}

func (h *Handler) unassignPlatformAgentTeamPrefix(w http.ResponseWriter, r *http.Request) {
	h.changePlatformAgentTeamPrefix(w, r, false)
}

func (h *Handler) changePlatformAgentTeamPrefix(w http.ResponseWriter, r *http.Request, assign bool) {
	agentID, agentErr := parsePositivePathID(r, "agent_id")
	teamID, teamErr := parsePositivePathID(r, "team_id")
	var request agentTeamAssignmentRequest
	if agentErr != nil || teamErr != nil || !decodeJSON(w, r, &request) {
		if agentErr != nil || teamErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "代理或团队编号无效")
		}
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" || len(request.Reason) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "必须填写归属变更原因")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "变更团队归属失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var agentStatus int
	if err = tx.QueryRowContext(r.Context(), `SELECT status FROM platform_agents WHERE admin_user_id=? FOR UPDATE`, agentID).Scan(&agentStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "代理不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "变更团队归属失败")
		return
	}
	var code string
	if err = tx.QueryRowContext(r.Context(), `SELECT code FROM teams WHERE id=? FOR UPDATE`, teamID).Scan(&code); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "团队不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "变更团队归属失败")
		return
	}
	if code == "sys" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "系统默认团队不能分配给代理")
		return
	}
	var previousAgentID int64
	err = tx.QueryRowContext(r.Context(), `SELECT admin_user_id FROM platform_agent_teams WHERE team_id=? FOR UPDATE`, teamID).Scan(&previousAgentID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "变更团队归属失败")
		return
	}
	adminUser, _ := adminFromRequest(r)
	if assign {
		if _, err = tx.ExecContext(r.Context(), `
			INSERT INTO platform_agent_teams(team_id,admin_user_id,assigned_by,assigned_at)
			VALUES(?,?,?,CURRENT_TIMESTAMP(3))
			ON DUPLICATE KEY UPDATE admin_user_id=VALUES(admin_user_id),assigned_by=VALUES(assigned_by),assigned_at=VALUES(assigned_at)`,
			teamID, agentID, adminUser.ID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "变更团队归属失败")
			return
		}
	} else {
		if errors.Is(err, sql.ErrNoRows) || previousAgentID != agentID {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该团队当前不属于此代理")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM platform_agent_teams WHERE team_id=? AND admin_user_id=?`, teamID, agentID); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "变更团队归属失败")
			return
		}
	}
	action := "agent.team.assign"
	if !assign {
		action = "agent.team.unassign"
	}
	afterAgentID := int64(0)
	if assign {
		afterAgentID = agentID
	}
	if err = auditAdmin(r.Context(), tx, r, action, "team", teamID,
		map[string]any{"agent_id": apiDecimalID(previousAgentID), "code": code},
		map[string]any{"agent_id": apiDecimalID(afterAgentID), "code": code, "reason": request.Reason},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录团队归属审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "变更团队归属失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"agent_id": apiDecimalID(agentID), "team_id": apiDecimalID(teamID), "assigned": assign,
	})
}

func replacePlatformAgentPermissions(r *http.Request, tx *sql.Tx, agentID int64, keys []string) error {
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM platform_agent_permissions WHERE admin_user_id=?`, agentID); err != nil {
		return err
	}
	for _, key := range keys {
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO platform_agent_permissions(admin_user_id,permission_id)
			SELECT ?,id FROM admin_permissions WHERE permission_key=?`, agentID, key)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return errors.New("permission does not exist")
		}
	}
	return nil
}

func parsePositivePathID(r *http.Request, name string) (int64, error) {
	value, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || value < 1 {
		return 0, errors.New("invalid path id")
	}
	return value, nil
}

func (h *Handler) platformAgentExists(ctx context.Context, adminID int64) (bool, error) {
	var exists int
	err := h.db.QueryRowContext(ctx, `SELECT 1 FROM platform_agents WHERE admin_user_id=?`, adminID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && exists == 1, err
}
