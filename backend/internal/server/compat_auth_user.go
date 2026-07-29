package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/auth"
)

func (s *Server) compatAuthUser(w http.ResponseWriter, r *http.Request, service string) bool {
	switch service {
	case "Login.getCode":
		s.compatSendCode(w, r, "register")
	case "Login.getForgetCode":
		s.compatSendCode(w, r, "reset")
	case "Login.userReg":
		s.compatRegister(w, r)
	case "Login.userFindPass":
		s.compatResetPassword(w, r)
	case "Home.search":
		s.compatSearchUsers(w, r)
	case "User.getUserHome":
		s.compatGetUserHome(w, r)
	case "User.getFollowsList", "User.getFansList":
		s.compatUserRelationList(w, r, service == "User.getFansList")
	case "User.setAttent":
		s.compatToggleFollow(w, r)
	case "User.setBlack":
		s.compatToggleBlock(w, r)
	case "User.updateAvatar":
		s.compatUpdateAsset(w, r, "avatar_asset_id", r.FormValue("avatar"))
	case "User.updateBgImg":
		s.compatUpdateAsset(w, r, "background_asset_id", r.FormValue("img"))
	case "User.updateFields":
		s.compatUpdateFields(w, r)
	case "User.updatePass":
		s.compatUpdatePassword(w, r)
	case "Auth.setAuth":
		s.compatSubmitVerification(w, r)
	case "Login.getCancelCondition":
		s.compatCancelCondition(w, r)
	case "Login.cancelAccount":
		s.compatCancelAccount(w, r)
	default:
		return false
	}
	return true
}

func verificationTarget(r *http.Request) string {
	country := strings.TrimSpace(r.FormValue("country_code"))
	if country == "" {
		country = "86"
	}
	return country + ":" + strings.TrimSpace(r.FormValue("mobile")) + "|" +
		strings.ToLower(strings.TrimSpace(r.FormValue("email")))
}

func (s *Server) compatSendCode(w http.ResponseWriter, r *http.Request, purpose string) {
	target := verificationTarget(r)
	if strings.HasSuffix(target, ":|") || !strings.Contains(target, "@") {
		writeCompat(w, 400, "请输入有效手机号和邮箱", nil)
		return
	}
	if purpose == "reset" {
		var exists int
		err := s.db.QueryRowContext(r.Context(), `
			SELECT EXISTS(
				SELECT 1 FROM users
			 WHERE country_code=? AND (mobile=? OR username=?) AND LOWER(COALESCE(email,''))=?
			)`,
			strings.SplitN(target, ":", 2)[0], r.FormValue("mobile"), r.FormValue("mobile"),
			strings.ToLower(strings.TrimSpace(r.FormValue("email"))),
		).Scan(&exists)
		if err != nil || exists == 0 {
			writeCompat(w, 404, "未找到匹配的账号", nil)
			return
		}
	}
	code := "123456"
	sum := sha256.Sum256([]byte(code))
	if _, err := s.db.ExecContext(r.Context(), `
		INSERT INTO auth_verification_codes(purpose,target,code_hash,expires_at)
		VALUES(?,?,?,DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 10 MINUTE))`,
		purpose, target, hex.EncodeToString(sum[:]),
	); err != nil {
		s.logger.Error("create verification code", "error", err)
		writeCompat(w, 500, "验证码服务暂不可用", nil)
		return
	}
	if s.environment == "local" || s.environment == "development" || s.environment == "test" {
		writeCompat(w, 0, "本地验证码：123456", map[string]string{"debug_code": code, "expires_in": "600"})
		return
	}
	writeCompat(w, 503, "验证码发送通道尚未配置", nil)
}

func (s *Server) consumeVerificationCode(
	ctx context.Context, tx *sql.Tx, purpose, target, code string,
) error {
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	var id int64
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM auth_verification_codes
		WHERE purpose=? AND target=? AND code_hash=? AND consumed_at IS NULL
		  AND expires_at>CURRENT_TIMESTAMP(3) AND attempts<5
		ORDER BY id DESC LIMIT 1 FOR UPDATE`,
		purpose, target, hex.EncodeToString(sum[:]),
	).Scan(&id)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE auth_verification_codes SET consumed_at=CURRENT_TIMESTAMP(3) WHERE id=?`, id)
	return err
}

func (s *Server) compatRegister(w http.ResponseWriter, r *http.Request) {
	login := boundedCompat(r.FormValue("user_login"), 120)
	email := strings.ToLower(boundedCompat(r.FormValue("email"), 190))
	country := boundedCompat(r.FormValue("country_code"), 8)
	if country == "" {
		country = "86"
	}
	password := r.FormValue("user_pass")
	if login == "" || email == "" || password != r.FormValue("user_pass2") {
		writeCompat(w, 400, "注册资料不完整或两次密码不一致", nil)
		return
	}
	passwordHash, err := adminauth.HashPassword(password)
	if err != nil {
		writeCompat(w, 400, "密码至少需要 12 个字符", nil)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeCompat(w, 500, "注册暂不可用", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	if err = s.consumeVerificationCode(r.Context(), tx, "register", verificationTarget(r), r.FormValue("code")); err != nil {
		writeCompat(w, 400, "验证码错误或已过期", nil)
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO users
			(username,country_code,mobile,email,password_hash,password_algo,nickname,registered_ip,status)
		VALUES(?,?,?,?,?,'argon2id',?,?,1)`,
		login, country, login, email, passwordHash, login, requestIP(r),
	)
	if err != nil {
		if compatIsDuplicate(err) {
			writeCompat(w, 409, "手机号或邮箱已注册", nil)
			return
		}
		s.logger.Error("register user", "error", err)
		writeCompat(w, 500, "注册暂不可用", nil)
		return
	}
	userID, _ := result.LastInsertId()
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO wallet_accounts(user_id,currency,available,frozen,status)
		VALUES(?,'COIN',0,0,1)`, userID,
	); err != nil {
		writeCompat(w, 500, "初始化用户钱包失败", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		writeCompat(w, 500, "注册暂不可用", nil)
		return
	}
	session, err := s.auth.Login(
		r.Context(), country, login, password, r.FormValue("device_id"),
		r.FormValue("source"), requestIP(r), r.UserAgent(),
	)
	if err != nil {
		writeCompat(w, 500, "注册成功，请重新登录", nil)
		return
	}
	profile, err := s.compatUserProfile(r.Context(), session.User.ID, session.Token)
	if err != nil {
		writeCompat(w, 500, "读取用户资料失败", nil)
		return
	}
	writeCompat(w, 0, "注册成功", profile)
}

func (s *Server) compatResetPassword(w http.ResponseWriter, r *http.Request) {
	password := r.FormValue("user_pass")
	if password == "" || password != r.FormValue("user_pass2") {
		writeCompat(w, 400, "两次密码不一致", nil)
		return
	}
	passwordHash, err := adminauth.HashPassword(password)
	if err != nil {
		writeCompat(w, 400, "密码至少需要 12 个字符", nil)
		return
	}
	country := boundedCompat(r.FormValue("country_code"), 8)
	if country == "" {
		country = "86"
	}
	login := boundedCompat(r.FormValue("user_login"), 120)
	email := strings.ToLower(boundedCompat(r.FormValue("email"), 190))
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeCompat(w, 500, "重置密码失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	if err = s.consumeVerificationCode(r.Context(), tx, "reset", verificationTarget(r), r.FormValue("code")); err != nil {
		writeCompat(w, 400, "验证码错误或已过期", nil)
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		UPDATE users SET password_hash=?,password_algo='argon2id'
		WHERE country_code=? AND (mobile=? OR username=?) AND LOWER(COALESCE(email,''))=? AND status=1`,
		passwordHash, country, login, login, email,
	)
	if err != nil {
		writeCompat(w, 500, "重置密码失败", nil)
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		writeCompat(w, 404, "未找到匹配的账号", nil)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE user_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
		WHERE user_id=(SELECT id FROM users
		 WHERE country_code=? AND (mobile=? OR username=?) AND LOWER(COALESCE(email,''))=? LIMIT 1)
		  AND revoked_at IS NULL`, country, login, login, email,
	); err != nil {
		writeCompat(w, 500, "重置密码失败", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		writeCompat(w, 500, "重置密码失败", nil)
		return
	}
	writeCompat(w, 0, "密码已重置", map[string]string{"reset": "1"})
}

func (s *Server) compatSearchUsers(w http.ResponseWriter, r *http.Request) {
	currentUserID := s.optionalCompatUser(r)
	limit, offset := compatPage(r.FormValue("p"))
	key := "%" + strings.TrimSpace(r.FormValue("key")) + "%"
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id FROM users
		WHERE status=1 AND (username LIKE ? OR nickname LIKE ? OR CAST(id AS CHAR) LIKE ?)
		ORDER BY id DESC LIMIT ? OFFSET ?`, key, key, key, limit, offset)
	if err != nil {
		writeCompat(w, 500, "搜索用户失败", nil)
		return
	}
	defer rows.Close()
	items := make([]any, 0, limit)
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			break
		}
		profile, profileErr := s.compatRelationshipProfile(r.Context(), currentUserID, id)
		if profileErr != nil {
			err = profileErr
			break
		}
		items = append(items, profile)
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "搜索用户失败", nil)
		return
	}
	writeCompatList(w, items)
}

func (s *Server) compatGetUserHome(w http.ResponseWriter, r *http.Request) {
	currentUserID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	targetUserID := compatInt64(r.FormValue("touid"))
	if targetUserID < 1 {
		targetUserID = currentUserID
	}
	profile, err := s.compatRelationshipProfile(r.Context(), currentUserID, targetUserID)
	if err != nil {
		writeCompat(w, 404, "用户不存在", nil)
		return
	}
	writeCompat(w, 0, "", profile)
}

func (s *Server) compatRelationshipProfile(
	ctx context.Context, currentUserID, targetUserID int64,
) (map[string]any, error) {
	profile, err := s.compatUserProfile(ctx, targetUserID, "")
	if err != nil {
		return nil, err
	}
	var backgroundKey string
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(asset.object_key,'')
		FROM users user
		LEFT JOIN media_assets asset ON asset.id=user.background_asset_id AND asset.status=1
		WHERE user.id=?`, targetUserID,
	).Scan(&backgroundKey)
	var following, blocked int
	if currentUserID > 0 {
		_ = s.db.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM user_follows WHERE user_id=? AND target_user_id=?),
			       EXISTS(SELECT 1 FROM user_blocks WHERE user_id=? AND target_user_id=?)`,
			currentUserID, targetUserID, currentUserID, targetUserID,
		).Scan(&following, &blocked)
	}
	profile["bg_img"] = s.mediaURL(backgroundKey)
	profile["isattention"] = strconv.Itoa(following)
	profile["isattent"] = strconv.Itoa(following)
	profile["isblack"] = strconv.Itoa(blocked)
	return profile, nil
}

func (s *Server) compatUserRelationList(w http.ResponseWriter, r *http.Request, fans bool) {
	currentUserID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	targetUserID := compatInt64(r.FormValue("touid"))
	if targetUserID < 1 {
		targetUserID = currentUserID
	}
	limit, offset := compatPage(r.FormValue("p"))
	query := `SELECT target_user_id FROM user_follows WHERE user_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`
	if fans {
		query = `SELECT user_id FROM user_follows WHERE target_user_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`
	}
	rows, err := s.db.QueryContext(r.Context(), query, targetUserID, limit, offset)
	if err != nil {
		writeCompat(w, 500, "用户列表加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]any, 0, limit)
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			break
		}
		profile, profileErr := s.compatRelationshipProfile(r.Context(), currentUserID, id)
		if profileErr != nil {
			err = profileErr
			break
		}
		items = append(items, profile)
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "用户列表加载失败", nil)
		return
	}
	writeCompatList(w, items)
}

func (s *Server) compatToggleFollow(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	targetUserID := compatInt64(r.FormValue("touid"))
	if targetUserID < 1 || targetUserID == userID {
		writeCompat(w, 400, "不能关注自己", nil)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeCompat(w, 500, "关注操作失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var exists int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM user_follows WHERE user_id=? AND target_user_id=?)`,
		userID, targetUserID,
	).Scan(&exists); err != nil {
		writeCompat(w, 500, "关注操作失败", nil)
		return
	}
	if exists == 1 {
		_, err = tx.ExecContext(r.Context(),
			`DELETE FROM user_follows WHERE user_id=? AND target_user_id=?`, userID, targetUserID)
		exists = 0
	} else {
		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO user_follows(user_id,target_user_id) VALUES(?,?)`, userID, targetUserID)
		if err == nil {
			_, err = tx.ExecContext(r.Context(), `
				INSERT INTO notifications(user_id,notification_type,actor_user_id,title,content,payload)
				VALUES(?,'follow',?,'新增关注','有用户关注了你',JSON_OBJECT('user_id',?))`,
				targetUserID, userID, userID,
			)
		}
		exists = 1
	}
	if err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "关注操作失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]string{"isattent": strconv.Itoa(exists), "isattention": strconv.Itoa(exists)})
}

func (s *Server) compatToggleBlock(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	targetUserID := compatInt64(r.FormValue("touid"))
	if targetUserID < 1 || targetUserID == userID {
		writeCompat(w, 400, "不能拉黑自己", nil)
		return
	}
	var exists int
	err := s.db.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM user_blocks WHERE user_id=? AND target_user_id=?)`,
		userID, targetUserID,
	).Scan(&exists)
	if err != nil {
		writeCompat(w, 500, "黑名单操作失败", nil)
		return
	}
	if exists == 1 {
		_, err = s.db.ExecContext(r.Context(),
			`DELETE FROM user_blocks WHERE user_id=? AND target_user_id=?`, userID, targetUserID)
		exists = 0
	} else {
		_, err = s.db.ExecContext(r.Context(), `
			INSERT INTO user_blocks(user_id,target_user_id) VALUES(?,?)
			ON DUPLICATE KEY UPDATE reason=VALUES(reason)`, userID, targetUserID)
		exists = 1
	}
	if err != nil {
		writeCompat(w, 500, "黑名单操作失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]string{"isblack": strconv.Itoa(exists)})
}

func (s *Server) compatUpdateAsset(w http.ResponseWriter, r *http.Request, column, value string) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	assetID, err := s.assetIDForValue(r.Context(), userID, value)
	if err != nil || assetID < 1 {
		writeCompat(w, 400, "请选择已上传的图片", nil)
		return
	}
	if column != "avatar_asset_id" && column != "background_asset_id" {
		writeCompat(w, 400, "资料字段不受支持", nil)
		return
	}
	_, err = s.db.ExecContext(r.Context(), "UPDATE users SET "+column+"=? WHERE id=?", assetID, userID)
	if err != nil {
		writeCompat(w, 500, "更新图片失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]string{"updated": "1"})
}

func (s *Server) compatUpdateFields(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(r.FormValue("fields")), &fields); err != nil {
		writeCompat(w, 400, "资料格式错误", nil)
		return
	}
	nickname := boundedCompat(fmt.Sprint(fields["user_nickname"]), 100)
	if nickname == "" {
		nickname = boundedCompat(fmt.Sprint(fields["user_nicename"]), 100)
	}
	signature := boundedCompat(fmt.Sprint(fields["signature"]), 500)
	gender, _ := strconv.Atoi(fmt.Sprint(fields["sex"]))
	if nickname == "" || gender < 0 || gender > 2 {
		writeCompat(w, 400, "昵称或性别无效", nil)
		return
	}
	_, err := s.db.ExecContext(r.Context(),
		`UPDATE users SET nickname=?,gender=?,signature=? WHERE id=?`,
		nickname, gender, signature, userID)
	if err != nil {
		writeCompat(w, 500, "保存资料失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]string{"updated": "1"})
}

func (s *Server) compatUpdatePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	if r.FormValue("pass") == "" || r.FormValue("pass") != r.FormValue("pass2") {
		writeCompat(w, 400, "两次密码不一致", nil)
		return
	}
	var encoded, algorithm string
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT password_hash,password_algo FROM users WHERE id=?`, userID,
	).Scan(&encoded, &algorithm); err != nil {
		writeCompat(w, 500, "修改密码失败", nil)
		return
	}
	if algorithm != "argon2id" || !adminauth.VerifyPassword(encoded, r.FormValue("oldpass")) {
		writeCompat(w, 400, "原密码错误", nil)
		return
	}
	nextHash, err := adminauth.HashPassword(r.FormValue("pass"))
	if err != nil {
		writeCompat(w, 400, "密码至少需要 12 个字符", nil)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeCompat(w, 500, "修改密码失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(r.Context(),
		`UPDATE users SET password_hash=?,password_algo='argon2id' WHERE id=?`, nextHash, userID,
	); err == nil {
		tokenHash := sha256.Sum256([]byte(strings.TrimSpace(r.FormValue("token"))))
		_, err = tx.ExecContext(r.Context(), `
			UPDATE user_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
			WHERE user_id=? AND token_hash<>? AND revoked_at IS NULL`,
			userID, hex.EncodeToString(tokenHash[:]))
	}
	if err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "修改密码失败", nil)
		return
	}
	writeCompat(w, 0, "密码已修改", map[string]string{"updated": "1"})
}

func (s *Server) compatSubmitVerification(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	realName := boundedCompat(r.FormValue("real_name"), 100)
	document := boundedCompat(r.FormValue("cer_no"), 100)
	if realName == "" || document == "" {
		writeCompat(w, 400, "认证资料不完整", nil)
		return
	}
	frontID, frontErr := s.assetIDForValue(r.Context(), userID, r.FormValue("front_view"))
	backID, backErr := s.assetIDForValue(r.Context(), userID, r.FormValue("back_view"))
	if frontErr != nil || backErr != nil || frontID < 1 || backID < 1 {
		writeCompat(w, 400, "请上传有效证件图片", nil)
		return
	}
	realCipher, err := s.encryptSensitive(realName)
	if err != nil {
		writeCompat(w, 500, "认证资料加密失败", nil)
		return
	}
	documentCipher, err := s.encryptSensitive(document)
	if err != nil {
		writeCompat(w, 500, "认证资料加密失败", nil)
		return
	}
	documentHash := sha256.Sum256([]byte(strings.ToUpper(document)))
	_, err = s.db.ExecContext(r.Context(), `
		INSERT INTO user_verifications
			(user_id,real_name_ciphertext,document_no_ciphertext,document_hash,front_asset_id,back_asset_id,status)
		VALUES(?,?,?,?,?,?,0)
		ON DUPLICATE KEY UPDATE
			real_name_ciphertext=VALUES(real_name_ciphertext),
			document_no_ciphertext=VALUES(document_no_ciphertext),
			document_hash=VALUES(document_hash),
			front_asset_id=VALUES(front_asset_id),back_asset_id=VALUES(back_asset_id),
			status=0,reject_reason='',reviewed_by=0,reviewed_at=NULL`,
		userID, realCipher, documentCipher, hex.EncodeToString(documentHash[:]), frontID, backID,
	)
	if err != nil {
		if compatIsDuplicate(err) {
			writeCompat(w, 409, "该证件已绑定其他账号", nil)
			return
		}
		writeCompat(w, 500, "提交认证失败", nil)
		return
	}
	writeCompat(w, 0, "认证资料已提交", map[string]string{"status": "0"})
}

func (s *Server) compatCancelCondition(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	var available, frozen int64
	var pendingWithdrawals int
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT COALESCE(available,0),COALESCE(frozen,0)
		FROM wallet_accounts WHERE user_id=? AND currency='COIN'`, userID,
	).Scan(&available, &frozen)
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM withdraw_orders WHERE user_id=? AND status IN (0,1,2)`, userID,
	).Scan(&pendingWithdrawals)
	balanceOK := available == 0 && frozen == 0
	withdrawOK := pendingWithdrawals == 0
	writeCompat(w, 0, "", map[string]any{
		"can_cancel": boolString(balanceOK && withdrawOK),
		"list": []map[string]string{
			{"title": "账户余额已清零", "content": fmt.Sprintf("可用 %d，冻结 %d", available, frozen), "is_ok": boolString(balanceOK)},
			{"title": "没有处理中提现", "content": fmt.Sprintf("处理中提现 %d 笔", pendingWithdrawals), "is_ok": boolString(withdrawOK)},
		},
	})
}

func (s *Server) compatCancelAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	var unavailable int
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT
			EXISTS(SELECT 1 FROM wallet_accounts WHERE user_id=? AND (available<>0 OR frozen<>0)) OR
			EXISTS(SELECT 1 FROM withdraw_orders WHERE user_id=? AND status IN (0,1,2))`,
		userID, userID,
	).Scan(&unavailable); err != nil {
		writeCompat(w, 500, "注销检查失败", nil)
		return
	}
	if unavailable == 1 {
		writeCompat(w, 409, "当前账号尚不满足注销条件", nil)
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeCompat(w, 500, "注销账号失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE users SET status=3,closed_at=CURRENT_TIMESTAMP(3),
			nickname='已注销用户',signature='',mobile=NULL,email=NULL
		WHERE id=? AND status=1`, userID,
	); err == nil {
		_, err = tx.ExecContext(r.Context(), `
			UPDATE user_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
			WHERE user_id=? AND revoked_at IS NULL`, userID)
	}
	if err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "注销账号失败", nil)
		return
	}
	writeCompat(w, 0, "账号已注销", map[string]string{"closed": "1"})
}

func writeCompatList(w http.ResponseWriter, items []any) {
	httpPayload := make([]any, 0, len(items))
	httpPayload = append(httpPayload, items...)
	writeCompatInfo(w, 0, "", httpPayload)
}

func writeCompatInfo(w http.ResponseWriter, code int, message string, info []any) {
	// Kept separate from writeCompat because list endpoints expose every row
	// directly in data.info, matching the existing uni-app infoList helper.
	writeCompatRaw(w, map[string]any{"data": map[string]any{"code": code, "msg": message, "info": info}})
}

func writeCompatRaw(w http.ResponseWriter, payload map[string]any) {
	// This indirection keeps response framing in one package without exposing
	// httpx to every compatibility module.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payload)
}

var _ = errors.Is
var _ = auth.ErrInvalidSession
var _ = time.Now
