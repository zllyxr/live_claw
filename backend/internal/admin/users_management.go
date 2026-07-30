package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

var (
	userCountryCodePattern = regexp.MustCompile(`^[0-9]{1,8}$`)
	userMobilePattern      = regexp.MustCompile(`^[0-9]{5,20}$`)
)

type updateUserProfileRequest struct {
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	CountryCode string `json:"country_code"`
	Mobile      string `json:"mobile"`
	Email       string `json:"email"`
}

type editableUserProfile struct {
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	CountryCode string `json:"country_code"`
	Mobile      string `json:"mobile"`
	Email       string `json:"email"`
}

func (request *updateUserProfileRequest) normalize() error {
	request.Username = strings.TrimSpace(request.Username)
	request.Nickname = strings.TrimSpace(request.Nickname)
	request.CountryCode = strings.TrimSpace(request.CountryCode)
	request.Mobile = strings.TrimSpace(request.Mobile)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))

	switch {
	case !validUserLoginName(request.Username):
		return errors.New("用户账号不能为空、不能包含空格或控制字符，且最多 120 个字符")
	case !validUserNickname(request.Nickname):
		return errors.New("用户昵称不能包含控制字符，且最多 100 个字符")
	case !userCountryCodePattern.MatchString(request.CountryCode):
		return errors.New("国家或地区代码必须为 1 到 8 位数字")
	case request.Mobile != "" && !userMobilePattern.MatchString(request.Mobile):
		return errors.New("手机号必须为空或为 5 到 20 位数字")
	case request.Email != "" && !validUserEmail(request.Email):
		return errors.New("邮箱格式无效或超过 190 个字符")
	default:
		return nil
	}
}

func (request updateUserProfileRequest) profile() editableUserProfile {
	return editableUserProfile{
		Username:    request.Username,
		Nickname:    request.Nickname,
		CountryCode: request.CountryCode,
		Mobile:      request.Mobile,
		Email:       request.Email,
	}
}

func validUserLoginName(value string) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 120 {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validUserNickname(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 100 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validUserEmail(value string) bool {
	if len(value) > 190 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(address.Address, value)
}

func nullableUserField(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func validManagedPassword(value string) bool {
	// Bound raw input before counting runes so malformed or adversarially large
	// payloads cannot turn password validation into avoidable CPU/memory work.
	if len(value) > 512 || !utf8.ValidString(value) {
		return false
	}
	characterCount := utf8.RuneCountInString(value)
	return characterCount >= 12 && characterCount <= 128
}

func userProfileConflictMessage(err error) (string, bool) {
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return "", false
	}
	message := strings.ToLower(mysqlErr.Message)
	switch {
	case strings.Contains(message, "uk_users_login"):
		return "该国家/地区代码下的用户账号已存在", true
	case strings.Contains(message, "uk_users_mobile"):
		return "该国家/地区代码下的手机号已存在", true
	case strings.Contains(message, "uk_users_email"):
		return "该邮箱已存在", true
	default:
		return "用户账号、手机号或邮箱已存在", true
	}
}

func parseAdminUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户编号无效")
		return 0, false
	}
	return userID, true
}

func (h *Handler) updateUserProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseAdminUserID(w, r)
	if !ok {
		return
	}
	var request updateUserProfileRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if validationErr := request.normalize(); validationErr != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, validationErr.Error())
		return
	}

	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户资料失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var before editableUserProfile
	var beforeMobile, beforeEmail sql.NullString
	err = tx.QueryRowContext(r.Context(), `
		SELECT username,nickname,country_code,mobile,email
		FROM users
		WHERE id=?
		FOR UPDATE`,
		userID,
	).Scan(
		&before.Username, &before.Nickname, &before.CountryCode, &beforeMobile, &beforeEmail,
	)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "用户不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户资料失败")
		return
	}
	before.Mobile = beforeMobile.String
	before.Email = beforeEmail.String
	after := request.profile()

	_, err = tx.ExecContext(r.Context(), `
		UPDATE users
		SET username=?,nickname=?,country_code=?,mobile=?,email=?
		WHERE id=?`,
		after.Username, after.Nickname, after.CountryCode,
		nullableUserField(after.Mobile), nullableUserField(after.Email), userID,
	)
	if err != nil {
		if message, conflict := userProfileConflictMessage(err); conflict {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, message)
			return
		}
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户资料失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "user.profile.update", "user", userID, before, after,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录用户审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户资料失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"user_id": apiDecimalID(userID),
		"user":    after,
		"updated": true,
	})
}

type resetUserPasswordRequest struct {
	Password string `json:"password"`
	Reason   string `json:"reason"`
}

func (request *resetUserPasswordRequest) normalize() error {
	request.Reason = strings.TrimSpace(request.Reason)
	if !validManagedPassword(request.Password) {
		return errors.New("密码需为 12 到 128 个字符")
	}
	if request.Reason == "" || len(request.Reason) > 500 {
		return errors.New("必须填写修改原因，且最多 500 个字符")
	}
	return nil
}

func (h *Handler) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseAdminUserID(w, r)
	if !ok {
		return
	}
	var request resetUserPasswordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if validationErr := request.normalize(); validationErr != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, validationErr.Error())
		return
	}
	passwordHash, err := adminauth.HashPassword(request.Password)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成密码凭据失败")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置用户密码失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var username, previousAlgorithm string
	err = tx.QueryRowContext(r.Context(), `
		SELECT username,password_algo
		FROM users
		WHERE id=?
		FOR UPDATE`,
		userID,
	).Scan(&username, &previousAlgorithm)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "用户不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置用户密码失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE users
		SET password_hash=?,password_algo='argon2id'
		WHERE id=?`,
		passwordHash, userID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置用户密码失败")
		return
	}
	revokeResult, err := tx.ExecContext(r.Context(), `
		UPDATE user_sessions
		SET revoked_at=CURRENT_TIMESTAMP(3)
		WHERE user_id=? AND revoked_at IS NULL`,
		userID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "撤销用户登录会话失败")
		return
	}
	revokedSessions, err := revokeResult.RowsAffected()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取会话撤销结果失败")
		return
	}
	auditAfter := map[string]any{
		"username":         username,
		"password_algo":    "argon2id",
		"reason":           request.Reason,
		"revoked_sessions": revokedSessions,
	}
	if err = auditAdmin(
		r.Context(), tx, r, "user.password.reset", "user", userID,
		map[string]any{"username": username, "password_algo": previousAlgorithm},
		auditAfter,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录用户审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "重置用户密码失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"user_id":          apiDecimalID(userID),
		"password_reset":   true,
		"revoked_sessions": revokedSessions,
	})
}
