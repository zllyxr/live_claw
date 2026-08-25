package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

const (
	teamConsolePath   = "/team-console"
	teamSessionCookie = "claw_team_session"
	teamCSRFCookie    = "claw_team_csrf"
	teamSessionTTL    = 12 * time.Hour
)

type teamConsolePrincipal struct {
	UserID      int64
	AccountID   int64
	OwnerUserID int64
	Username    string
	Nickname    string
	TeamID      int64
	TeamCode    string
	TeamName    string
	MemberCount int64
}

type teamConsoleSession struct {
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

type contextTeamConsoleKey struct{}

func (h *Handler) registerTeamConsole(mux *http.ServeMux) {
	mux.HandleFunc("GET "+teamConsolePath, h.teamRoot)
	mux.HandleFunc("GET "+teamConsolePath+"/", h.teamRoot)
	mux.HandleFunc("GET "+teamConsolePath+"/login", h.teamLoginPage)
	mux.HandleFunc("GET "+teamConsolePath+"/app", h.requireTeamPage(h.teamAppPage))
	mux.HandleFunc("POST "+teamConsolePath+"/api/login", h.teamLogin)
	mux.HandleFunc("POST "+teamConsolePath+"/api/logout", h.requireTeamAPI(true, h.teamLogout))
	mux.HandleFunc("POST "+teamConsolePath+"/api/csrf", h.requireTeamAPI(false, h.teamRefreshCSRF))
	mux.HandleFunc("GET "+teamConsolePath+"/api/me", h.requireTeamAPI(false, h.teamMe))
	mux.HandleFunc("GET "+teamConsolePath+"/api/members", h.requireTeamAPI(false, h.listOwnTeamMembers))
}

func (h *Handler) teamRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != teamConsolePath && r.URL.Path != teamConsolePath+"/" {
		http.NotFound(w, r)
		return
	}
	if _, err := h.currentTeamPrincipal(r); err != nil {
		http.Redirect(w, r, teamConsolePath+"/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, teamConsolePath+"/app", http.StatusSeeOther)
}

func (h *Handler) teamLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentTeamPrincipal(r); err == nil {
		http.Redirect(w, r, teamConsolePath+"/app", http.StatusSeeOther)
		return
	}
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.teamLoginTemplate.Execute(w, nil)
}

func (h *Handler) teamAppPage(w http.ResponseWriter, r *http.Request) {
	principal, _ := teamPrincipalFromRequest(r)
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.teamAppTemplate.Execute(w, principal)
}

func (h *Handler) teamLogin(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CountryCode string `json:"country_code"`
		Login       string `json:"login"`
		Password    string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Login = strings.TrimSpace(request.Login)
	var principal teamConsolePrincipal
	accountID, accountFound, err := h.verifyTeamAccountCredentials(
		r.Context(), request.Login, request.Password,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "团队登录暂不可用")
		return
	}
	if accountFound {
		if accountID == 0 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "团队账号或密码错误")
			return
		}
		principal, err = h.loadTeamAccountPrincipal(r.Context(), accountID)
	} else {
		if h.userAuth == nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "团队登录暂不可用")
			return
		}
		var user auth.User
		user, err = h.userAuth.VerifyCredentials(r.Context(), request.CountryCode, request.Login, request.Password)
		if err == nil {
			principal, err = h.loadTeamPrincipal(r.Context(), user.ID)
		}
	}
	if errors.Is(err, auth.ErrInvalidCredentials) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "团队账号或密码错误")
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "该账号没有可登录的有效团队")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "团队登录暂不可用")
		return
	}
	session, err := h.createTeamSession(r, principal)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "团队登录暂不可用")
		return
	}
	h.setTeamSessionCookies(w, session)
	if principal.AccountID != 0 {
		_, _ = h.db.ExecContext(r.Context(), `
			UPDATE team_console_accounts
			SET last_login_at=CURRENT_TIMESTAMP(3),last_login_ip=? WHERE id=?`,
			boundedTeamValue(clientIP(r), 45), principal.AccountID)
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"manager": teamPrincipalForAPI(principal), "csrf_token": session.CSRFToken,
	})
}

func (h *Handler) verifyTeamAccountCredentials(ctx context.Context, login, password string) (int64, bool, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" || password == "" {
		return 0, true, nil
	}
	var accountID int64
	var passwordHash string
	var status int
	err := h.db.QueryRowContext(ctx, `
		SELECT id,password_hash,status FROM team_console_accounts WHERE username=? LIMIT 1`, login,
	).Scan(&accountID, &passwordHash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if status != 1 || !adminauth.VerifyPassword(passwordHash, password) {
		return 0, true, nil
	}
	return accountID, true, nil
}

func (h *Handler) createTeamSession(r *http.Request, principal teamConsolePrincipal) (teamConsoleSession, error) {
	sessionID, err := idgen.New()
	if err != nil {
		return teamConsoleSession{}, err
	}
	token, tokenHash, err := teamOpaqueSecret(32)
	if err != nil {
		return teamConsoleSession{}, err
	}
	csrf, csrfHash, err := teamOpaqueSecret(24)
	if err != nil {
		return teamConsoleSession{}, err
	}
	expiresAt := time.Now().Add(teamSessionTTL)
	ip := boundedTeamValue(clientIP(r), 45)
	userAgent := boundedTeamValue(r.UserAgent(), 500)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return teamConsoleSession{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO team_console_sessions
			(id,user_id,account_id,token_hash,csrf_hash,ip,user_agent,expires_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		sessionID, nullableTeamPrincipalID(principal.UserID), nullableTeamPrincipalID(principal.AccountID),
		tokenHash, csrfHash, ip, userAgent, expiresAt,
	); err != nil {
		return teamConsoleSession{}, err
	}
	actorType, actorID := teamPrincipalActor(principal)
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO audit_logs
			(request_id,actor_type,actor_id,action,resource_type,resource_id,ip,user_agent,after_data)
		VALUES(?,?,?,'team.login','team',?,?,?,JSON_OBJECT('team_code',?,'session_id',?))`,
		httpx.RequestID(r.Context()), actorType, actorID, principal.TeamID, ip, userAgent,
		principal.TeamCode, sessionID,
	); err != nil {
		return teamConsoleSession{}, err
	}
	if err = tx.Commit(); err != nil {
		return teamConsoleSession{}, err
	}
	return teamConsoleSession{Token: token, CSRFToken: csrf, ExpiresAt: expiresAt}, nil
}

func (h *Handler) teamLogout(w http.ResponseWriter, r *http.Request) {
	principal, _ := teamPrincipalFromRequest(r)
	tokenHash := teamSecretHash(teamSessionToken(r))
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "退出失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE team_console_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
		WHERE token_hash=? AND revoked_at IS NULL`, tokenHash,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "退出失败")
		return
	}
	actorType, actorID := teamPrincipalActor(principal)
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO audit_logs
			(request_id,actor_type,actor_id,action,resource_type,resource_id,ip,user_agent)
		VALUES(?,?,?,'team.logout','team',?,?,?)`,
		httpx.RequestID(r.Context()), actorType, actorID, principal.TeamID,
		boundedTeamValue(clientIP(r), 45), boundedTeamValue(r.UserAgent(), 500),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "退出失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "退出失败")
		return
	}
	h.clearTeamSessionCookies(w)
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"logged_out": true})
}

func (h *Handler) teamRefreshCSRF(w http.ResponseWriter, r *http.Request) {
	csrf, csrfHash, err := teamOpaqueSecret(24)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "刷新请求校验失败")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE team_console_sessions SET csrf_hash=?,last_seen_at=CURRENT_TIMESTAMP(3)
		WHERE token_hash=? AND revoked_at IS NULL AND expires_at>CURRENT_TIMESTAMP(3)`,
		csrfHash, teamSecretHash(teamSessionToken(r)),
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "刷新请求校验失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "团队登录已失效")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: teamCSRFCookie, Value: csrf, Path: teamConsolePath,
		HttpOnly: false, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		MaxAge: int(teamSessionTTL.Seconds()),
	})
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]string{"csrf_token": csrf})
}

func (h *Handler) teamMe(w http.ResponseWriter, r *http.Request) {
	principal, _ := teamPrincipalFromRequest(r)
	httpx.OK(w, httpx.RequestID(r.Context()), teamPrincipalForAPI(principal))
}

func (h *Handler) listOwnTeamMembers(w http.ResponseWriter, r *http.Request) {
	principal, _ := teamPrincipalFromRequest(r)
	h.listScopedTeamMembers(w, r, principal.TeamID, principal.OwnerUserID)
}

func (h *Handler) listScopedTeamMembers(w http.ResponseWriter, r *http.Request, teamID, ownerUserID int64) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	status := -1
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 1 && parsed <= 3 {
			status = parsed
		}
	}
	arguments := []any{teamID, teamID, keyword, like, like, status, status}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM team_members member
		JOIN users user ON user.id=member.user_id AND user.team_id=member.team_id
		WHERE member.team_id=? AND user.team_id=? AND member.status=1
		  AND (?='' OR CAST(user.id AS CHAR) LIKE ? OR user.nickname LIKE ?)
		  AND (? < 0 OR user.status=?)`, arguments...).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队成员失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT user.id,COALESCE(NULLIF(user.nickname,''),CONCAT('用户 ',user.id)),
		       user.status,member.joined_at
		FROM team_members member
		JOIN users user ON user.id=member.user_id AND user.team_id=member.team_id
		WHERE member.team_id=? AND user.team_id=? AND member.status=1
		  AND (?='' OR CAST(user.id AS CHAR) LIKE ? OR user.nickname LIKE ?)
		  AND (? < 0 OR user.status=?)
		ORDER BY member.joined_at DESC,user.id DESC LIMIT ? OFFSET ?`,
		append(arguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队成员失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var userID int64
		var nickname string
		var userStatus int
		var joinedAt time.Time
		if err = rows.Scan(&userID, &nickname, &userStatus, &joinedAt); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队成员失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(userID), "nickname": nickname, "status": userStatus,
			"joined_at": joinedAt.Unix(), "is_owner": userID == ownerUserID,
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取团队成员失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) requireTeamPage(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := h.currentTeamPrincipal(r)
		if err != nil {
			http.Redirect(w, r, teamConsolePath+"/login", http.StatusSeeOther)
			return
		}
		next(w, r.WithContext(withTeamPrincipal(r, principal)))
	}
}

func (h *Handler) requireTeamAPI(requireCSRF bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := h.currentTeamPrincipal(r)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "团队登录已失效")
			return
		}
		if requireCSRF && !h.verifyTeamCSRF(r, r.Header.Get("X-CSRF-Token")) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "请求校验失败")
			return
		}
		next(w, r.WithContext(withTeamPrincipal(r, principal)))
	}
}

func (h *Handler) currentTeamPrincipal(r *http.Request) (teamConsolePrincipal, error) {
	token := teamSessionToken(r)
	if token == "" {
		return teamConsolePrincipal{}, auth.ErrInvalidSession
	}
	var sessionID string
	var userID, accountID sql.NullInt64
	err := h.db.QueryRowContext(r.Context(), `
		SELECT session.id,session.user_id,session.account_id
		FROM team_console_sessions session
		LEFT JOIN users user ON user.id=session.user_id AND user.status=1
		LEFT JOIN team_console_accounts account ON account.id=session.account_id AND account.status=1
		WHERE session.token_hash=? AND session.revoked_at IS NULL
		  AND session.expires_at>CURRENT_TIMESTAMP(3)
		  AND ((session.user_id IS NOT NULL AND user.id IS NOT NULL)
		    OR (session.account_id IS NOT NULL AND account.id IS NOT NULL))
		LIMIT 1`,
		teamSecretHash(token),
	).Scan(&sessionID, &userID, &accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return teamConsolePrincipal{}, auth.ErrInvalidSession
	}
	if err != nil {
		return teamConsolePrincipal{}, err
	}
	var principal teamConsolePrincipal
	if accountID.Valid {
		principal, err = h.loadTeamAccountPrincipal(r.Context(), accountID.Int64)
	} else if userID.Valid {
		principal, err = h.loadTeamPrincipal(r.Context(), userID.Int64)
	} else {
		return teamConsolePrincipal{}, auth.ErrInvalidSession
	}
	if err != nil {
		return teamConsolePrincipal{}, err
	}
	_, _ = h.db.ExecContext(r.Context(), `
		UPDATE team_console_sessions SET last_seen_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND last_seen_at<CURRENT_TIMESTAMP(3)-INTERVAL 5 MINUTE`, sessionID)
	return principal, nil
}

func (h *Handler) loadTeamPrincipal(ctx context.Context, userID int64) (teamConsolePrincipal, error) {
	var principal teamConsolePrincipal
	err := h.db.QueryRowContext(ctx, `
		SELECT user.id,COALESCE(NULLIF(user.nickname,''),CONCAT('用户 ',user.id)),
		       user.username,team.owner_user_id,team.id,team.code,team.name,
		       COALESCE((SELECT COUNT(*) FROM team_members current_member
		                 WHERE current_member.team_id=team.id AND current_member.status=1),0)
		FROM users user
		JOIN teams team ON team.owner_user_id=user.id AND team.id=user.team_id
		JOIN team_members membership ON membership.user_id=user.id
		  AND membership.team_id=team.id AND membership.status=1
		WHERE user.id=? AND user.status=1 AND team.status=1 AND team.code<>'sys'
		LIMIT 1`, userID,
	).Scan(
		&principal.UserID, &principal.Nickname, &principal.Username, &principal.OwnerUserID, &principal.TeamID,
		&principal.TeamCode, &principal.TeamName, &principal.MemberCount,
	)
	return principal, err
}

func (h *Handler) loadTeamAccountPrincipal(ctx context.Context, accountID int64) (teamConsolePrincipal, error) {
	var principal teamConsolePrincipal
	err := h.db.QueryRowContext(ctx, `
		SELECT account.id,account.username,
		       COALESCE(NULLIF(account.display_name,''),account.username),
		       team.owner_user_id,team.id,team.code,team.name,
		       COALESCE((SELECT COUNT(*) FROM team_members current_member
		                 WHERE current_member.team_id=team.id AND current_member.status=1),0)
		FROM team_console_accounts account
		JOIN teams team ON team.id=account.team_id
		WHERE account.id=? AND account.status=1 AND team.status=1 AND team.code<>'sys'
		LIMIT 1`, accountID,
	).Scan(
		&principal.AccountID, &principal.Username, &principal.Nickname,
		&principal.OwnerUserID, &principal.TeamID, &principal.TeamCode,
		&principal.TeamName, &principal.MemberCount,
	)
	return principal, err
}

func (h *Handler) verifyTeamCSRF(r *http.Request, csrf string) bool {
	if teamSessionToken(r) == "" || strings.TrimSpace(csrf) == "" {
		return false
	}
	var exists int
	err := h.db.QueryRowContext(r.Context(), `
		SELECT 1 FROM team_console_sessions
		WHERE token_hash=? AND csrf_hash=? AND revoked_at IS NULL
		  AND expires_at>CURRENT_TIMESTAMP(3)`,
		teamSecretHash(teamSessionToken(r)), teamSecretHash(csrf),
	).Scan(&exists)
	return err == nil && exists == 1
}

func withTeamPrincipal(r *http.Request, principal teamConsolePrincipal) context.Context {
	return context.WithValue(r.Context(), contextTeamConsoleKey{}, principal)
}

func teamPrincipalFromRequest(r *http.Request) (teamConsolePrincipal, bool) {
	principal, ok := r.Context().Value(contextTeamConsoleKey{}).(teamConsolePrincipal)
	return principal, ok
}

func teamPrincipalForAPI(principal teamConsolePrincipal) map[string]any {
	principalType := "owner_user"
	var userID any = apiDecimalID(principal.UserID)
	var accountID any
	if principal.AccountID != 0 {
		principalType = "team_account"
		userID = nil
		accountID = apiDecimalID(principal.AccountID)
	}
	return map[string]any{
		"principal_type": principalType, "user_id": userID, "account_id": accountID,
		"username": principal.Username, "nickname": principal.Nickname,
		"team": map[string]any{
			"id": apiDecimalID(principal.TeamID), "code": principal.TeamCode,
			"name": principal.TeamName, "member_count": principal.MemberCount,
		},
	}
}

func nullableTeamPrincipalID(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func teamPrincipalActor(principal teamConsolePrincipal) (int, int64) {
	if principal.AccountID != 0 {
		return 4, principal.AccountID
	}
	return 2, principal.UserID
}

func teamOpaqueSecret(size int) (string, string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	value := base64.RawURLEncoding.EncodeToString(buffer)
	return value, teamSecretHash(value), nil
}

func teamSecretHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func boundedTeamValue(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) > maximum {
		return value[:maximum]
	}
	return value
}

func teamSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(teamSessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handler) setTeamSessionCookies(w http.ResponseWriter, session teamConsoleSession) {
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	http.SetCookie(w, &http.Cookie{
		Name: teamSessionCookie, Value: session.Token, Path: teamConsolePath,
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: maxAge,
	})
	http.SetCookie(w, &http.Cookie{
		Name: teamCSRFCookie, Value: session.CSRFToken, Path: teamConsolePath,
		HttpOnly: false, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: maxAge,
	})
}

func (h *Handler) clearTeamSessionCookies(w http.ResponseWriter) {
	for _, cookie := range []struct {
		name     string
		httpOnly bool
	}{{teamSessionCookie, true}, {teamCSRFCookie, false}} {
		http.SetCookie(w, &http.Cookie{
			Name: cookie.name, Value: "", Path: teamConsolePath, HttpOnly: cookie.httpOnly,
			Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
			MaxAge: -1, Expires: time.Unix(1, 0),
		})
	}
}
