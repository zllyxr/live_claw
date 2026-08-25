package admin

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/invite"
)

const (
	agentConsolePath   = "/agent-console"
	agentSessionCookie = "claw_agent_session"
	agentCSRFCookie    = "claw_agent_csrf"
)

func (h *Handler) registerAgentConsole(mux *http.ServeMux) {
	mux.HandleFunc("GET "+agentConsolePath, h.agentRoot)
	mux.HandleFunc("GET "+agentConsolePath+"/", h.agentRoot)
	mux.HandleFunc("GET "+agentConsolePath+"/login", h.agentLoginPage)
	mux.HandleFunc("GET "+agentConsolePath+"/app", h.requireAgentPage(h.agentAppPage))
	mux.HandleFunc("POST "+agentConsolePath+"/api/login", h.agentLogin)
	mux.HandleFunc("POST "+agentConsolePath+"/api/logout", h.requireAgentAPI("", true, h.agentLogout))
	mux.HandleFunc("POST "+agentConsolePath+"/api/csrf", h.requireAgentAPI("", false, h.agentRefreshCSRF))
	mux.HandleFunc("GET "+agentConsolePath+"/api/me", h.requireAgentAPI("", false, h.agentMe))
	mux.HandleFunc("GET "+agentConsolePath+"/api/team-prefixes", h.requireAgentAPI("", false, h.listOwnTeamPrefixes))
	mux.HandleFunc("POST "+agentConsolePath+"/api/team-prefixes", h.requireAgentAPI("", true, h.createOwnTeamPrefix))
	mux.HandleFunc("GET "+agentConsolePath+"/api/team-prefixes/{code}/members", h.requireAgentAPI("", false, h.listAgentTeamMembers))
	mux.HandleFunc("POST "+agentConsolePath+"/api/team-prefixes/{code}/account", h.requireAgentAPI("", true, h.createAgentTeamAccount))
	mux.HandleFunc("POST "+agentConsolePath+"/api/team-prefixes/{code}/account/password", h.requireAgentAPI("", true, h.resetAgentTeamAccountPassword))

	// Business APIs are deliberately mounted one by one. Sensitive admin APIs
	// do not exist below /agent-console even when a request is forged manually.
	mux.HandleFunc("GET "+agentConsolePath+"/api/games", h.requireAgentAPI("games.read", false, h.listGames))
	mux.HandleFunc("POST "+agentConsolePath+"/api/games/{id}", h.requireAgentAPI("games.write", true, h.updateGame))
	mux.HandleFunc("POST "+agentConsolePath+"/api/games/venues/{id}", h.requireAgentAPI("games.write", true, h.updateGameVenue))
	mux.HandleFunc("GET "+agentConsolePath+"/api/games/settlements", h.requireAgentAPI("games.read", false, h.listGameSettlements))

	mux.HandleFunc("GET "+agentConsolePath+"/api/live/rooms", h.requireAgentAPI("live.read", false, h.listLiveRooms))
	mux.HandleFunc("GET "+agentConsolePath+"/api/live/hosts", h.requireAgentAPI("live.write", false, h.listLiveHosts))
	mux.HandleFunc("POST "+agentConsolePath+"/api/live/rooms", h.requireAgentAPI("live.write", true, h.createLiveRoom))
	mux.HandleFunc("POST "+agentConsolePath+"/api/live/rooms/{id}", h.requireAgentAPI("live.write", true, h.updateLiveRoom))

	mux.HandleFunc("GET "+agentConsolePath+"/api/lottery/catalog", h.requireAgentAPI("lottery.read", false, h.listLotteryCatalog))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/categories", h.requireAgentAPI("lottery.write", true, h.createLotteryCategory))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/categories/{id}", h.requireAgentAPI("lottery.write", true, h.updateLotteryCategory))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/games", h.requireAgentAPI("lottery.write", true, h.createLotteryGame))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/games/{id}", h.requireAgentAPI("lottery.write", true, h.updateLotteryGame))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/games/{id}/status", h.requireAgentAPI("lottery.write", true, h.setLotteryGameStatus))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/plays", h.requireAgentAPI("lottery.write", true, h.createLotteryPlay))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/plays/{id}", h.requireAgentAPI("lottery.write", true, h.updateLotteryPlay))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/options", h.requireAgentAPI("lottery.write", true, h.createLotteryOption))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/options/{id}", h.requireAgentAPI("lottery.write", true, h.updateLotteryOption))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/issues", h.requireAgentAPI("lottery.write", true, h.createLotteryIssue))
	mux.HandleFunc("GET "+agentConsolePath+"/api/lottery/issues", h.requireAgentAPI("lottery.read", false, h.listLotteryIssues))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/issues/{id}/close", h.requireAgentAPI("lottery.write", true, h.closeLotteryIssue))
	mux.HandleFunc("POST "+agentConsolePath+"/api/lottery/issues/{id}/draw", h.requireAgentAPI("lottery.write", true, h.drawLotteryIssue))
	mux.HandleFunc("GET "+agentConsolePath+"/api/lottery/orders", h.requireAgentAPI("lottery.read", false, h.listLotteryOrders))

	mux.HandleFunc("GET "+agentConsolePath+"/api/sports/matches", h.requireAgentAPI("sports.read", false, h.listSportsMatches))
	mux.HandleFunc("GET "+agentConsolePath+"/api/sports/sync", h.requireAgentAPI("sports.read", false, h.sportsSyncStatus))
	mux.HandleFunc("POST "+agentConsolePath+"/api/sports/matches", h.requireAgentAPI("sports.write", true, h.createSportsMatch))
	mux.HandleFunc("POST "+agentConsolePath+"/api/sports/matches/{id}", h.requireAgentAPI("sports.write", true, h.updateSportsMatch))
	mux.HandleFunc("POST "+agentConsolePath+"/api/sports/matches/{id}/settle", h.requireAgentAPI("sports.write", true, h.markSportsSettlementReady))
	mux.HandleFunc("GET "+agentConsolePath+"/api/sports/matches/{id}/markets", h.requireAgentAPI("sports.read", false, h.listSportsMarkets))
	mux.HandleFunc("POST "+agentConsolePath+"/api/sports/markets", h.requireAgentAPI("sports.write", true, h.createSportsMarket))
	mux.HandleFunc("POST "+agentConsolePath+"/api/sports/markets/{id}", h.requireAgentAPI("sports.write", true, h.updateSportsMarket))
	mux.HandleFunc("POST "+agentConsolePath+"/api/sports/options/{id}", h.requireAgentAPI("sports.write", true, h.updateSportsOption))

	mux.HandleFunc("GET "+agentConsolePath+"/api/bets/dashboard", h.requireAgentAPI("bets.read", false, h.bettingDashboard))
	mux.HandleFunc("GET "+agentConsolePath+"/api/bets/lottery", h.requireAgentAPI("bets.read", false, h.listBetLotteryOrders))
	mux.HandleFunc("GET "+agentConsolePath+"/api/bets/sports", h.requireAgentAPI("bets.read", false, h.listBetSportsOrders))
	mux.HandleFunc("GET "+agentConsolePath+"/api/bets/game", h.requireAgentAPI("bets.read", false, h.listBetGameOrders))

	mux.HandleFunc("GET "+agentConsolePath+"/api/app/releases", h.requireAgentAPI("app.read", false, h.listAppReleases))
	mux.HandleFunc("POST "+agentConsolePath+"/api/app/uploads/prepare", h.requireAgentAPI("app.write", true, h.prepareAppUpload))
	mux.HandleFunc("POST "+agentConsolePath+"/api/app/uploads/finalize", h.requireAgentAPI("app.write", true, h.finalizeAppUpload))
	mux.HandleFunc("POST "+agentConsolePath+"/api/app/releases", h.requireAgentAPI("app.write", true, h.createAppRelease))
	mux.HandleFunc("PATCH "+agentConsolePath+"/api/app/releases/{id}", h.requireAgentAPI("app.write", true, h.updateAppRelease))
	mux.HandleFunc("POST "+agentConsolePath+"/api/app/releases/{id}/publish", h.requireAgentAPI("app.write", true, h.publishAppRelease))
	mux.HandleFunc("POST "+agentConsolePath+"/api/app/releases/{id}/pause", h.requireAgentAPI("app.write", true, h.pauseAppRelease))
	mux.HandleFunc("POST "+agentConsolePath+"/api/app/releases/{id}/resume", h.requireAgentAPI("app.write", true, h.resumeAppRelease))
	mux.HandleFunc("POST "+agentConsolePath+"/api/app/releases/{id}/archive", h.requireAgentAPI("app.write", true, h.archiveAppRelease))
}

func (h *Handler) agentRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != agentConsolePath && r.URL.Path != agentConsolePath+"/" {
		http.NotFound(w, r)
		return
	}
	if _, err := h.currentPlatformAgent(r); err != nil {
		http.Redirect(w, r, agentConsolePath+"/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, agentConsolePath+"/app", http.StatusSeeOther)
}

func (h *Handler) agentLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentPlatformAgent(r); err == nil {
		http.Redirect(w, r, agentConsolePath+"/app", http.StatusSeeOther)
		return
	}
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.agentLoginTemplate.Execute(w, nil)
}

func (h *Handler) agentAppPage(w http.ResponseWriter, r *http.Request) {
	agent, _ := adminFromRequest(r)
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.agentAppTemplate.Execute(w, agent)
}

func (h *Handler) agentLogin(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	session, err := h.auth.LoginForPortal(
		r.Context(), adminauth.PortalAgent, request.Username, request.Password,
		clientIP(r), r.UserAgent(),
	)
	if errors.Is(err, adminauth.ErrInvalidCredentials) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "代理账号或密码错误")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "代理登录暂不可用")
		return
	}
	agent, err := h.loadPlatformAgent(r.Context(), session.Admin)
	if err != nil {
		_ = h.auth.LogoutForPortal(r.Context(), adminauth.PortalAgent, session.Token)
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "该账号未开通或已停用代理权限")
		return
	}
	h.setAgentSessionCookies(w, session)
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"agent": adminIdentityForAPI(agent), "csrf_token": session.CSRFToken,
	})
}

func (h *Handler) agentLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.LogoutForPortal(r.Context(), adminauth.PortalAgent, agentSessionToken(r)); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "退出失败")
		return
	}
	h.clearAgentSessionCookies(w)
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"logged_out": true})
}

func (h *Handler) agentRefreshCSRF(w http.ResponseWriter, r *http.Request) {
	csrf, err := h.auth.RefreshCSRFForPortal(r.Context(), adminauth.PortalAgent, agentSessionToken(r))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "请重新登录")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: agentCSRFCookie, Value: csrf, Path: agentConsolePath,
		HttpOnly: false, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		MaxAge: int((12 * time.Hour).Seconds()),
	})
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]string{"csrf_token": csrf})
}

func (h *Handler) agentMe(w http.ResponseWriter, r *http.Request) {
	agent, _ := adminFromRequest(r)
	httpx.OK(w, httpx.RequestID(r.Context()), adminIdentityForAPI(agent))
}

func (h *Handler) listOwnTeamPrefixes(w http.ResponseWriter, r *http.Request) {
	agent, _ := adminFromRequest(r)
	page, pageSize := pageParams(r)
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM platform_agent_teams WHERE admin_user_id=?`, agent.ID,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队前缀失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT team.code,
		       COALESCE((SELECT COUNT(*) FROM team_members member WHERE member.team_id=team.id AND member.status=1),0),
		       account.username,account.status
		FROM platform_agent_teams owned
		JOIN teams team ON team.id=owned.team_id
		LEFT JOIN team_console_accounts account ON account.team_id=team.id
		WHERE owned.admin_user_id=?
		ORDER BY team.code LIMIT ? OFFSET ?`, agent.ID, pageSize, (page-1)*pageSize)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队前缀失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var code string
		var count int64
		var accountUsername sql.NullString
		var accountStatus sql.NullInt64
		if err = rows.Scan(&code, &count, &accountUsername, &accountStatus); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队前缀失败")
			return
		}
		items = append(items, map[string]any{
			"code": code, "member_count": count,
			"has_team_account":      accountUsername.Valid,
			"team_account_username": accountUsername.String,
			"team_account_status":   nullableTeamAccountStatus(accountStatus),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队前缀失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

type createAgentTeamAccountRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (h *Handler) createAgentTeamAccount(w http.ResponseWriter, r *http.Request) {
	agent, _ := adminFromRequest(r)
	code := normalizedTeamCode(r.PathValue("code"))
	if code == "" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队前缀无效")
		return
	}
	var request createAgentTeamAccountRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Username = strings.ToLower(strings.TrimSpace(request.Username))
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	if request.DisplayName == "" {
		request.DisplayName = "团队 " + code
	}
	if !adminNamePattern.MatchString(request.Username) || !validManagedPassword(request.Password) ||
		len(request.DisplayName) > 100 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队账号参数无效；账号需为 3 至 80 位，密码需为 12 至 128 个字符")
		return
	}
	passwordHash, err := adminauth.HashPassword(request.Password)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队账号密码不符合安全要求")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建团队账号失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var teamID int64
	if err = tx.QueryRowContext(r.Context(), `
		SELECT team.id
		FROM platform_agent_teams owned
		JOIN teams team ON team.id=owned.team_id
		WHERE owned.admin_user_id=? AND team.code=? AND team.code<>'sys'
		FOR UPDATE`, agent.ID, code).Scan(&teamID); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "团队前缀不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建团队账号失败")
		return
	}
	var existingCount int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT (SELECT COUNT(*) FROM team_console_accounts WHERE team_id=?) +
		       (SELECT COUNT(*) FROM users WHERE username=? OR mobile=? OR email=?)`,
		teamID, request.Username, request.Username, request.Username,
	).Scan(&existingCount); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建团队账号失败")
		return
	}
	if existingCount != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该团队已有后台账号，或登录账号已被占用")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO team_console_accounts
			(team_id,username,password_hash,display_name,status,created_by)
		VALUES(?,?,?,?,1,?)`, teamID, request.Username, passwordHash, request.DisplayName, agent.ID)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该团队已有后台账号，或登录账号已被占用")
			return
		}
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建团队账号失败")
		return
	}
	accountID, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), tx, r, "agent.team.account.create", "team_console_account", accountID, nil, map[string]any{
		"team_id": apiDecimalID(teamID), "team_code": code,
		"username": request.Username, "display_name": request.DisplayName,
	}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录团队账号审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建团队账号失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(accountID), "team_code": code,
		"username": request.Username, "display_name": request.DisplayName,
	})
}

type resetAgentTeamAccountPasswordRequest struct {
	Password string `json:"password"`
}

func (h *Handler) resetAgentTeamAccountPassword(w http.ResponseWriter, r *http.Request) {
	agent, _ := adminFromRequest(r)
	code := normalizedTeamCode(r.PathValue("code"))
	if code == "" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队前缀无效")
		return
	}
	var request resetAgentTeamAccountPasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if !validManagedPassword(request.Password) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队账号密码需为 12 至 128 个字符")
		return
	}
	passwordHash, err := adminauth.HashPassword(request.Password)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队账号密码不符合安全要求")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置团队账号密码失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var accountID, teamID int64
	var username, previousHash string
	if err = tx.QueryRowContext(r.Context(), `
		SELECT account.id,team.id,account.username,account.password_hash
		FROM platform_agent_teams owned
		JOIN teams team ON team.id=owned.team_id
		JOIN team_console_accounts account ON account.team_id=team.id
		WHERE owned.admin_user_id=? AND team.code=? AND team.code<>'sys'
		FOR UPDATE`, agent.ID, code).Scan(&accountID, &teamID, &username, &previousHash); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "团队后台账号不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置团队账号密码失败")
		return
	}
	if adminauth.VerifyPassword(previousHash, request.Password) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "新密码不能与当前密码相同")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE team_console_accounts
		SET password_hash=?,password_changed_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`, passwordHash, accountID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置团队账号密码失败")
		return
	}
	revokeResult, err := tx.ExecContext(r.Context(), `
		UPDATE team_console_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
		WHERE account_id=? AND revoked_at IS NULL`, accountID)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "撤销团队后台会话失败")
		return
	}
	revokedSessions, _ := revokeResult.RowsAffected()
	if err = auditAdmin(r.Context(), tx, r, "agent.team.account.password.reset", "team_console_account", accountID, nil, map[string]any{
		"team_id": apiDecimalID(teamID), "team_code": code, "username": username,
		"password_changed": true, "revoked_sessions": revokedSessions,
	}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录团队账号审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置团队账号密码失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"team_code": code, "username": username, "password_reset": true,
		"revoked_sessions": revokedSessions,
	})
}

func normalizedTeamCode(value string) string {
	code := strings.ToLower(strings.TrimSpace(value))
	if !teamCodePattern.MatchString(code) || code == "sys" {
		return ""
	}
	return code
}

func nullableTeamAccountStatus(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func (h *Handler) createOwnTeamPrefix(w http.ResponseWriter, r *http.Request) {
	agent, _ := adminFromRequest(r)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成团队前缀失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	for attempt := 0; attempt < 64; attempt++ {
		code, generateErr := invite.GeneratePart(rand.Reader, 3)
		if generateErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成团队前缀失败")
			return
		}
		if code == "sys" {
			continue
		}
		result, insertErr := tx.ExecContext(r.Context(), `
			INSERT INTO teams(code,name,owner_user_id,status,created_by)
			VALUES(?,CONCAT('团队 ',?),0,1,?)`, code, code, agent.ID)
		if insertErr != nil {
			var mysqlErr *mysqlDriver.MySQLError
			if errors.As(insertErr, &mysqlErr) && mysqlErr.Number == 1062 {
				continue
			}
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成团队前缀失败")
			return
		}
		teamID, _ := result.LastInsertId()
		if _, err = tx.ExecContext(r.Context(), `
			INSERT INTO platform_agent_teams(team_id,admin_user_id,assigned_by)
			VALUES(?,?,?)`, teamID, agent.ID, agent.ID); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成团队前缀失败")
			return
		}
		if err = auditAdmin(r.Context(), tx, r, "agent.team.generate", "team", teamID, nil, map[string]any{
			"code": code, "agent_id": apiDecimalID(agent.ID),
		}); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录团队前缀审计失败")
			return
		}
		if err = tx.Commit(); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成团队前缀失败")
			return
		}
		httpx.OK(w, httpx.RequestID(r.Context()), map[string]string{"code": code})
		return
	}
	httpx.Error(w, httpx.RequestID(r.Context()), http.StatusServiceUnavailable, 503, "团队前缀空间暂时不可用")
}

func (h *Handler) listAgentTeamMembers(w http.ResponseWriter, r *http.Request) {
	agent, _ := adminFromRequest(r)
	code := strings.ToLower(strings.TrimSpace(r.PathValue("code")))
	if !teamCodePattern.MatchString(code) || code == "sys" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "团队前缀无效")
		return
	}
	var teamID, ownerUserID int64
	err := h.db.QueryRowContext(r.Context(), `
		SELECT team.id,team.owner_user_id
		FROM platform_agent_teams owned
		JOIN teams team ON team.id=owned.team_id
		WHERE owned.admin_user_id=? AND team.code=?`, agent.ID, code,
	).Scan(&teamID, &ownerUserID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "团队前缀不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队成员失败")
		return
	}
	h.listScopedTeamMembers(w, r, teamID, ownerUserID)
}

func (h *Handler) requireAgentPage(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent, err := h.currentPlatformAgent(r)
		if err != nil {
			http.Redirect(w, r, agentConsolePath+"/login", http.StatusSeeOther)
			return
		}
		next(w, r.WithContext(withAdmin(r, agent)))
	}
}

func (h *Handler) requireAgentAPI(permission string, requireCSRF bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent, err := h.currentPlatformAgent(r)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "代理登录已失效")
			return
		}
		if permission != "" && !hasPermission(agent, permission) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "无权执行此代理操作")
			return
		}
		if requireCSRF && !h.auth.VerifyCSRFForPortal(
			r.Context(), adminauth.PortalAgent, agentSessionToken(r), r.Header.Get("X-CSRF-Token"),
		) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "请求校验失败")
			return
		}
		next(w, r.WithContext(withAdmin(r, agent)))
	}
}

func (h *Handler) currentPlatformAgent(r *http.Request) (adminauth.Admin, error) {
	admin, err := h.auth.AuthenticateForPortal(r.Context(), adminauth.PortalAgent, agentSessionToken(r))
	if err != nil {
		return adminauth.Admin{}, err
	}
	return h.loadPlatformAgent(r.Context(), admin)
}

func (h *Handler) loadPlatformAgent(ctx context.Context, admin adminauth.Admin) (adminauth.Admin, error) {
	var status int
	if err := h.db.QueryRowContext(ctx, `
		SELECT status FROM platform_agents WHERE admin_user_id=?`, admin.ID,
	).Scan(&status); err != nil || status != 1 {
		if err == nil {
			err = adminauth.ErrInvalidCredentials
		}
		return adminauth.Admin{}, err
	}
	rows, err := h.db.QueryContext(ctx, `
		SELECT permission.permission_key
		FROM platform_agent_permissions grant_row
		JOIN admin_permissions permission ON permission.id=grant_row.permission_id
		WHERE grant_row.admin_user_id=? ORDER BY permission.permission_key`, admin.ID)
	if err != nil {
		return adminauth.Admin{}, err
	}
	defer rows.Close()
	permissions := make([]string, 0, 12)
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return adminauth.Admin{}, err
		}
		if _, allowed := platformAgentPermissionAllowlist[key]; allowed {
			permissions = append(permissions, key)
		}
	}
	if err = rows.Err(); err != nil {
		return adminauth.Admin{}, err
	}
	admin.Permissions = permissions
	return admin, nil
}

func agentSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(agentSessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handler) setAgentSessionCookies(w http.ResponseWriter, session adminauth.Session) {
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	http.SetCookie(w, &http.Cookie{
		Name: agentSessionCookie, Value: session.Token, Path: agentConsolePath,
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: maxAge,
	})
	http.SetCookie(w, &http.Cookie{
		Name: agentCSRFCookie, Value: session.CSRFToken, Path: agentConsolePath,
		HttpOnly: false, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: maxAge,
	})
}

func (h *Handler) clearAgentSessionCookies(w http.ResponseWriter) {
	for _, cookie := range []struct {
		name     string
		httpOnly bool
	}{{agentSessionCookie, true}, {agentCSRFCookie, false}} {
		http.SetCookie(w, &http.Cookie{
			Name: cookie.name, Value: "", Path: agentConsolePath, HttpOnly: cookie.httpOnly,
			Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
			MaxAge: -1, Expires: time.Unix(1, 0),
		})
	}
}
