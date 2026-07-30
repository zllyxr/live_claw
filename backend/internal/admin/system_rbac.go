package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

var (
	settingKeyPattern = regexp.MustCompile(`^[0-9a-z][0-9a-z._-]{1,119}$`)
	roleKeyPattern    = regexp.MustCompile(`^[0-9a-z][0-9a-z_-]{1,79}$`)
	adminNamePattern  = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z_.-]{2,79}$`)
)

func (h *Handler) listSystemSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT setting_key,setting_value,is_secret,version,updated_by,updated_at
		FROM system_settings ORDER BY setting_key`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取系统设置失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 32)
	for rows.Next() {
		var key string
		var raw []byte
		var secret int
		var version, updatedBy int64
		var updatedAt time.Time
		if err = rows.Scan(&key, &raw, &secret, &version, &updatedBy, &updatedAt); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取系统设置失败")
			return
		}
		var value any
		if secret == 1 {
			value = map[string]bool{"configured": string(raw) != "null" && string(raw) != `""`}
		} else {
			value = jsonOrNil(raw)
		}
		items = append(items, map[string]any{
			"key": key, "value": value, "is_secret": secret == 1, "version": version,
			"updated_by": apiDecimalID(updatedBy), "updated_at": updatedAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取系统设置失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items})
}

func (h *Handler) updateSystemSetting(w http.ResponseWriter, r *http.Request) {
	key := strings.ToLower(strings.TrimSpace(r.PathValue("key")))
	var request struct {
		Value    json.RawMessage `json:"value"`
		IsSecret bool            `json:"is_secret"`
		Version  int64           `json:"version"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if !settingKeyPattern.MatchString(key) || len(request.Value) == 0 ||
		len(request.Value) > 256<<10 || !json.Valid(request.Value) || request.Version < 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "系统设置参数无效")
		return
	}
	var normalized any
	decoder := json.NewDecoder(strings.NewReader(string(request.Value)))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "系统设置参数无效")
		return
	}
	canonical, _ := json.Marshal(normalized)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新系统设置失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var previous []byte
	var previousSecret int
	var previousVersion int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT setting_value,is_secret,version FROM system_settings WHERE setting_key=? FOR UPDATE`,
		key,
	).Scan(&previous, &previousSecret, &previousVersion)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新系统设置失败")
		return
	}
	if err == nil && request.Version != previousVersion {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "系统设置已被其他管理员更新，请刷新后重试")
		return
	}
	if errors.Is(err, sql.ErrNoRows) && request.Version != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "新系统设置版本必须为 0")
		return
	}
	adminUser, _ := adminFromRequest(r)
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO system_settings(setting_key,setting_value,is_secret,version,updated_by)
		VALUES(?,?,?,1,?)
		ON DUPLICATE KEY UPDATE
			setting_value=VALUES(setting_value),is_secret=VALUES(is_secret),
			version=system_settings.version+1,updated_by=VALUES(updated_by)`,
		key, canonical, request.IsSecret, adminUser.ID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新系统设置失败")
		return
	}
	beforeAudit := any(map[string]any{"exists": false})
	if previousVersion > 0 {
		beforeValue := any("***")
		if previousSecret == 0 {
			beforeValue = jsonOrNil(previous)
		}
		beforeAudit = map[string]any{
			"value": beforeValue, "is_secret": previousSecret == 1, "version": previousVersion,
		}
	}
	afterValue := any("***")
	if !request.IsSecret {
		afterValue = normalized
	}
	if err = auditAdmin(
		r.Context(), tx, r, "system.setting.update", "system_setting", key,
		beforeAudit, map[string]any{
			"value": afterValue, "is_secret": request.IsSecret, "version": previousVersion + 1,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录系统设置审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新系统设置失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"key": key, "version": previousVersion + 1, "is_secret": request.IsSecret,
	})
}

func (h *Handler) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	actionLike := "%" + escapeLike(action) + "%"
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		action, actionLike,
		keyword, like, like, like, like, like, like,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM audit_logs audit
		LEFT JOIN admin_users admin ON audit.actor_type=1 AND admin.id=audit.actor_id
		WHERE (?='' OR audit.action LIKE ?)
		  AND (?='' OR audit.request_id LIKE ? OR admin.username LIKE ?
		       OR audit.action LIKE ? OR audit.resource_type LIKE ?
		       OR audit.resource_id LIKE ? OR audit.ip LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取审计日志失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT audit.id,audit.request_id,audit.actor_id,
		       COALESCE(admin.username,''),audit.action,audit.resource_type,audit.resource_id,
		       audit.before_data,audit.after_data,audit.ip,audit.created_at
		FROM audit_logs audit
		LEFT JOIN admin_users admin ON audit.actor_type=1 AND admin.id=audit.actor_id
		WHERE (?='' OR audit.action LIKE ?)
		  AND (?='' OR audit.request_id LIKE ? OR admin.username LIKE ?
		       OR audit.action LIKE ? OR audit.resource_type LIKE ?
		       OR audit.resource_id LIKE ? OR audit.ip LIKE ?)
		ORDER BY audit.created_at DESC,audit.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取审计日志失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, actorID int64
		var requestID, actorName, actionValue, resourceType, resourceID, ip string
		var before, after []byte
		var createdAt time.Time
		if err = rows.Scan(
			&id, &requestID, &actorID, &actorName, &actionValue,
			&resourceType, &resourceID, &before, &after, &ip, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取审计日志失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "request_id": requestID,
			"actor_id": apiDecimalID(actorID), "actor_name": actorName,
			"action": actionValue, "resource_type": resourceType, "resource_id": resourceID,
			"before": jsonOrNil(before), "after": jsonOrNil(after), "ip": ip,
			"created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取审计日志失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) listRBAC(w http.ResponseWriter, r *http.Request) {
	permissionRows, err := h.db.QueryContext(r.Context(), `
		SELECT id,permission_key,name,module,action,description
		FROM admin_permissions ORDER BY module,action,permission_key`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取权限失败")
		return
	}
	defer permissionRows.Close()
	permissions := make([]map[string]any, 0, 32)
	for permissionRows.Next() {
		var id int64
		var key, name, module, action, description string
		if err = permissionRows.Scan(&id, &key, &name, &module, &action, &description); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取权限失败")
			return
		}
		permissions = append(permissions, map[string]any{
			"id": apiDecimalID(id), "permission_key": key, "name": name, "module": module,
			"action": action, "description": description,
		})
	}

	roleRows, err := h.db.QueryContext(r.Context(), `
		SELECT role.id,role.role_key,role.name,role.description,role.data_scope,role.status,
		       COALESCE(GROUP_CONCAT(permission.permission_key ORDER BY permission.permission_key SEPARATOR ','),'')
		FROM admin_roles role
		LEFT JOIN admin_role_permissions role_permission ON role_permission.role_id=role.id
		LEFT JOIN admin_permissions permission ON permission.id=role_permission.permission_id
		GROUP BY role.id ORDER BY role.id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取角色失败")
		return
	}
	defer roleRows.Close()
	roles := make([]map[string]any, 0, 16)
	for roleRows.Next() {
		var id int64
		var roleKey, name, description, permissionCSV string
		var dataScope, status int
		if err = roleRows.Scan(&id, &roleKey, &name, &description, &dataScope, &status, &permissionCSV); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取角色失败")
			return
		}
		roles = append(roles, map[string]any{
			"id": apiDecimalID(id), "role_key": roleKey, "name": name,
			"description": description,
			"data_scope":  dataScope, "status": status, "permissions": splitCSV(permissionCSV),
		})
	}

	adminRows, err := h.db.QueryContext(r.Context(), `
		SELECT admin.id,admin.username,admin.display_name,COALESCE(admin.email,''),
		       admin.status,admin.last_login_at,admin.created_at,
		       COALESCE(GROUP_CONCAT(role.role_key ORDER BY role.role_key SEPARATOR ','),'')
		FROM admin_users admin
		LEFT JOIN admin_user_roles assignment ON assignment.admin_user_id=admin.id
		LEFT JOIN admin_roles role ON role.id=assignment.role_id
		GROUP BY admin.id ORDER BY admin.id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取管理员失败")
		return
	}
	defer adminRows.Close()
	admins := make([]map[string]any, 0, 16)
	for adminRows.Next() {
		var id int64
		var username, displayName, email, roleCSV string
		var status int
		var lastLogin sql.NullTime
		var createdAt time.Time
		if err = adminRows.Scan(
			&id, &username, &displayName, &email, &status, &lastLogin, &createdAt, &roleCSV,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取管理员失败")
			return
		}
		admins = append(admins, map[string]any{
			"id": apiDecimalID(id), "username": username,
			"display_name": displayName, "email": email,
			"status": status, "last_login_at": nullTime(lastLogin), "created_at": createdAt.Unix(),
			"roles": splitCSV(roleCSV),
		})
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"permissions": permissions, "roles": roles, "admins": admins,
	})
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RoleKey       string             `json:"role_key"`
		Name          string             `json:"name"`
		Description   string             `json:"description"`
		DataScope     int                `json:"data_scope"`
		PermissionIDs decimalIDListInput `json:"permission_ids"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.RoleKey = strings.ToLower(strings.TrimSpace(request.RoleKey))
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	if !roleKeyPattern.MatchString(request.RoleKey) || request.RoleKey == "super_admin" ||
		request.Name == "" || len(request.Name) > 100 || len(request.Description) > 500 ||
		request.DataScope < 1 || request.DataScope > 3 || len(request.PermissionIDs) > 100 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "角色参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建角色失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO admin_roles(role_key,name,description,data_scope,status)
		VALUES(?,?,?,?,1)`,
		request.RoleKey, request.Name, request.Description, request.DataScope,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "角色标识已存在")
		return
	}
	roleID, _ := result.LastInsertId()
	if err = grantPermissions(r, tx, roleID, request.PermissionIDs.Int64s()); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "权限编号无效")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "rbac.role.create", "admin_role", roleID, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录权限审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建角色失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(roleID)})
}

func (h *Handler) updateRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || roleID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "角色编号无效")
		return
	}
	var request struct {
		PermissionIDs decimalIDListInput `json:"permission_ids"`
		Status        int                `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Status < 0 || request.Status > 1 || len(request.PermissionIDs) > 100 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "角色授权参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新角色失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var roleKey string
	var previousStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT role_key,status FROM admin_roles WHERE id=? FOR UPDATE`,
		roleID,
	).Scan(&roleKey, &previousStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "角色不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新角色失败")
		return
	}
	if roleKey == "super_admin" ||
		roleKey == "support_agent" ||
		roleKey == "support_supervisor" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "该角色为系统保留角色")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM admin_role_permissions WHERE role_id=?", roleID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新角色失败")
		return
	}
	if err = grantPermissions(r, tx, roleID, request.PermissionIDs.Int64s()); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "权限编号无效")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE admin_roles SET status=? WHERE id=?", request.Status, roleID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新角色失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "rbac.role.update", "admin_role", roleID,
		map[string]any{"status": previousStatus}, request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录权限审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新角色失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(roleID), "updated": true,
	})
}

func (h *Handler) createAdministrator(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username    string             `json:"username"`
		Password    string             `json:"password"`
		DisplayName string             `json:"display_name"`
		Email       string             `json:"email"`
		RoleIDs     decimalIDListInput `json:"role_ids"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if !adminNamePattern.MatchString(request.Username) || !validManagedPassword(request.Password) ||
		request.DisplayName == "" || len(request.DisplayName) > 100 ||
		len(request.Email) > 190 || len(request.RoleIDs) < 1 || len(request.RoleIDs) > 20 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "管理员参数无效")
		return
	}
	passwordHash, err := adminauth.HashPassword(request.Password)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建管理员失败")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建管理员失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var emailValue any
	if request.Email != "" {
		emailValue = request.Email
	}
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO admin_users(username,password_hash,display_name,email,status)
		VALUES(?,?,?,?,1)`,
		request.Username, passwordHash, request.DisplayName, emailValue,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "管理员账号或邮箱已存在")
		return
	}
	adminIDValue, _ := result.LastInsertId()
	if err = assignRoles(r, tx, adminIDValue, request.RoleIDs.Int64s()); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "角色编号无效")
		return
	}
	if err = syncSupportAgentProfile(r, tx, adminIDValue, 1); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建客服座席资料失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "rbac.admin.create", "admin_user", adminIDValue,
		nil, map[string]any{
			"username": request.Username, "display_name": request.DisplayName, "role_ids": request.RoleIDs,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录权限审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建管理员失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(adminIDValue)})
}

func (h *Handler) updateAdministrator(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || targetID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "管理员编号无效")
		return
	}
	var request struct {
		DisplayName string             `json:"display_name"`
		Status      int                `json:"status"`
		RoleIDs     decimalIDListInput `json:"role_ids"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	if request.DisplayName == "" || len(request.DisplayName) > 100 ||
		request.Status < 0 || request.Status > 1 || len(request.RoleIDs) < 1 || len(request.RoleIDs) > 20 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "管理员参数无效")
		return
	}
	currentAdmin, _ := adminFromRequest(r)
	if currentAdmin.ID == targetID && request.Status == 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "不能停用当前登录账号")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新管理员失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var username, beforeName string
	var beforeStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT username,display_name,status FROM admin_users WHERE id=? FOR UPDATE`,
		targetID,
	).Scan(&username, &beforeName, &beforeStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "管理员不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新管理员失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE admin_users SET display_name=?,status=? WHERE id=?`,
		request.DisplayName, request.Status, targetID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新管理员失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM admin_user_roles WHERE admin_user_id=?", targetID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新管理员失败")
		return
	}
	if err = assignRoles(r, tx, targetID, request.RoleIDs.Int64s()); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "角色编号无效")
		return
	}
	if err = syncSupportAgentProfile(r, tx, targetID, request.Status); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新客服座席资料失败")
		return
	}
	if request.Status == 0 {
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE admin_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
			WHERE admin_user_id=? AND revoked_at IS NULL`,
			targetID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新管理员失败")
			return
		}
	}
	if err = auditAdmin(
		r.Context(), tx, r, "rbac.admin.update", "admin_user", targetID,
		map[string]any{"username": username, "display_name": beforeName, "status": beforeStatus},
		request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录权限审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新管理员失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(targetID), "updated": true,
	})
}

type resetAdministratorPasswordRequest struct {
	Password string `json:"password"`
	Reason   string `json:"reason"`
}

func (request *resetAdministratorPasswordRequest) normalize() error {
	request.Reason = strings.TrimSpace(request.Reason)
	if !validManagedPassword(request.Password) ||
		strings.TrimSpace(request.Password) == "" ||
		request.Reason == "" || len(request.Reason) > 500 {
		return errors.New("invalid administrator password reset request")
	}
	return nil
}

func (h *Handler) resetAdministratorPassword(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || targetID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "管理员编号无效")
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
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "管理员密码不符合安全要求")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置管理员密码失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var username, previousHash string
	var status int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT username,password_hash,status
		FROM admin_users WHERE id=? FOR UPDATE`,
		targetID,
	).Scan(&username, &previousHash, &status); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "管理员不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置管理员密码失败")
		return
	}
	if adminauth.VerifyPassword(previousHash, request.Password) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "新密码不能与当前密码相同")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE admin_users
		SET password_hash=?,password_changed_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		passwordHash, targetID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置管理员密码失败")
		return
	}
	revokeResult, err := tx.ExecContext(r.Context(), `
		UPDATE admin_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
		WHERE admin_user_id=? AND revoked_at IS NULL`,
		targetID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "撤销管理员登录会话失败")
		return
	}
	revokedSessions, _ := revokeResult.RowsAffected()
	if err = auditAdmin(
		r.Context(), tx, r, "rbac.admin.password.reset", "admin_user", targetID,
		map[string]any{
			"username": username,
			"status":   status,
		},
		map[string]any{
			"username":         username,
			"status":           status,
			"password_changed": true,
			"reason":           request.Reason,
			"revoked_sessions": revokedSessions,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录管理员密码审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置管理员密码失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(targetID), "password_reset": true,
		"revoked_sessions": revokedSessions,
	})
}

func grantPermissions(r *http.Request, tx *sql.Tx, roleID int64, permissionIDs []int64) error {
	if !allPositiveIDs(permissionIDs) {
		return errors.New("permission id must be positive")
	}
	for _, permissionID := range uniquePositiveIDs(permissionIDs) {
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO admin_role_permissions(role_id,permission_id)
			SELECT ?,id FROM admin_permissions WHERE id=?`,
			roleID, permissionID,
		)
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

func assignRoles(r *http.Request, tx *sql.Tx, administratorID int64, roleIDs []int64) error {
	if len(roleIDs) == 0 || !allPositiveIDs(roleIDs) {
		return errors.New("role id must be positive")
	}
	for _, roleID := range uniquePositiveIDs(roleIDs) {
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO admin_user_roles(admin_user_id,role_id)
			SELECT ?,id FROM admin_roles WHERE id=? AND status=1`,
			administratorID, roleID,
		)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return errors.New("role does not exist")
		}
	}
	return nil
}

func syncSupportAgentProfile(
	r *http.Request,
	tx *sql.Tx,
	administratorID int64,
	administratorStatus int,
) error {
	var agentRole, otherRoles int
	err := tx.QueryRowContext(r.Context(), `
		SELECT
		  COALESCE(MAX(CASE role.role_key
		    WHEN 'support_supervisor' THEN 2
		    WHEN 'support_agent' THEN 1
		    ELSE 0 END),0),
		  COALESCE(SUM(role.role_key NOT IN ('support_agent','support_supervisor')),0)
		FROM admin_user_roles assignment
		JOIN admin_roles role ON role.id=assignment.role_id
		WHERE assignment.admin_user_id=?`,
		administratorID,
	).Scan(&agentRole, &otherRoles)
	if err != nil {
		return err
	}
	if agentRole == 0 {
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE support_agents SET status=0,presence=0,support_only=0 WHERE admin_user_id=?`,
			administratorID,
		); err != nil {
			return err
		}
		_, err = tx.ExecContext(r.Context(), `
			UPDATE admin_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
			WHERE admin_user_id=? AND portal='support' AND revoked_at IS NULL`,
			administratorID,
		)
		return err
	}
	agentNo, err := idgen.New()
	if err != nil {
		return err
	}
	supportOnly := 0
	if otherRoles == 0 {
		supportOnly = 1
	}
	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO support_agents
			(admin_user_id,agent_no,agent_role,status,presence,max_active,support_only)
		VALUES(?,?,?,IF(?=1,1,0),0,8,?)
		ON DUPLICATE KEY UPDATE
			agent_role=VALUES(agent_role),status=VALUES(status),
			presence=IF(VALUES(status)=1,presence,0),support_only=VALUES(support_only)`,
		administratorID, agentNo, agentRole, administratorStatus, supportOnly,
	)
	if err != nil {
		return err
	}
	if supportOnly == 1 {
		_, err = tx.ExecContext(r.Context(), `
			UPDATE admin_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
			WHERE admin_user_id=? AND portal='admin' AND revoked_at IS NULL`,
			administratorID,
		)
	}
	return err
}

func allPositiveIDs(values []int64) bool {
	for _, value := range values {
		if value < 1 {
			return false
		}
	}
	return true
}

func uniquePositiveIDs(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value < 1 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func splitCSV(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func jsonOrNil(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	var result any
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.UseNumber()
	if decoder.Decode(&result) != nil {
		return nil
	}
	return stringifyAuditIdentifierValues("", result)
}

func stringifyAuditIdentifierValues(key string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, childValue := range typed {
			typed[childKey] = stringifyAuditIdentifierValues(childKey, childValue)
		}
		return typed
	case []any:
		for index, childValue := range typed {
			typed[index] = stringifyAuditIdentifierValues(key, childValue)
		}
		return typed
	case json.Number:
		if auditIdentifierKey(key) {
			return typed.String()
		}
		return typed
	default:
		return value
	}
}

func auditIdentifierKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "id" ||
		strings.HasSuffix(normalized, "_id") ||
		strings.HasSuffix(normalized, "_ids") {
		return true
	}
	switch normalized {
	case "user_id", "admin_id", "actor_id", "sender_id", "owner_user_id",
		"host_user_id", "target_user_id", "applicant_user_id", "from_user_id",
		"to_user_id", "assigned_admin_id", "requested_by", "reviewed_by",
		"confirmed_by", "verified_by", "created_by", "updated_by":
		return true
	default:
		return false
	}
}
