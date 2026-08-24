package admin

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

type adminUserListItem struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	CountryCode string `json:"country_code"`
	Mobile      string `json:"mobile"`
	Email       string `json:"email"`
	Nickname    string `json:"nickname"`
	Status      int    `json:"status"`
	IsVirtual   bool   `json:"is_virtual"`
	TeamCode    string `json:"team_code"`
	TeamName    string `json:"team_name"`
	Available   int64  `json:"available"`
	Frozen      int64  `json:"frozen"`
	LastLoginAt any    `json:"last_login_at"`
	CreatedAt   int64  `json:"created_at"`
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	status, _ := strconv.Atoi(r.URL.Query().Get("status"))
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		keyword, like, like, like, like, like, like, like,
		status, status,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM users app_user
		LEFT JOIN teams team ON team.id=app_user.team_id
		WHERE (?='' OR CAST(app_user.id AS CHAR) LIKE ?
		       OR app_user.username LIKE ? OR app_user.mobile LIKE ?
		       OR app_user.email LIKE ? OR app_user.nickname LIKE ?
		       OR team.code LIKE ? OR team.name LIKE ?)
		  AND (?=0 OR app_user.status=?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取用户失败")
		return
	}
	query := `
		SELECT app_user.id,app_user.username,app_user.country_code,COALESCE(app_user.mobile,''),
		       COALESCE(app_user.email,''),app_user.nickname,app_user.status,app_user.is_virtual,
		       COALESCE(team.code,''),COALESCE(team.name,''),
		       COALESCE(wallet.available,0),COALESCE(wallet.frozen,0),
		       app_user.last_login_at,app_user.created_at
		FROM users app_user
		LEFT JOIN teams team ON team.id=app_user.team_id
		LEFT JOIN wallet_accounts wallet ON wallet.user_id=app_user.id AND wallet.currency='COIN'
		WHERE (?='' OR CAST(app_user.id AS CHAR) LIKE ?
		       OR app_user.username LIKE ? OR app_user.mobile LIKE ?
		       OR app_user.email LIKE ? OR app_user.nickname LIKE ?
		       OR team.code LIKE ? OR team.name LIKE ?)
		  AND (?=0 OR app_user.status=?)
		ORDER BY app_user.created_at DESC,app_user.id DESC
		LIMIT ? OFFSET ?`
	rows, err := h.db.QueryContext(r.Context(), query,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取用户失败")
		return
	}
	defer rows.Close()
	items := make([]adminUserListItem, 0, pageSize)
	for rows.Next() {
		var id, available, frozen int64
		var username, countryCode, mobile, email, nickname, teamCode, teamName string
		var userStatus, virtual int
		var lastLogin sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &username, &countryCode, &mobile, &email, &nickname, &userStatus, &virtual,
			&teamCode, &teamName, &available, &frozen, &lastLogin, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取用户失败")
			return
		}
		items = append(items, adminUserListItem{
			ID: apiDecimalID(id), Username: username, CountryCode: countryCode, Mobile: mobile,
			Email: email, Nickname: nickname, Status: userStatus, IsVirtual: virtual == 1,
			TeamCode: teamCode, TeamName: teamName, Available: available, Frozen: frozen,
			LastLoginAt: nullTime(lastLogin), CreatedAt: createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取用户失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

type directWalletAdjustmentRequest struct {
	Direction string `json:"direction"`
	Amount    int64  `json:"amount"`
	Reason    string `json:"reason"`
	RequestID string `json:"request_id"`
}

func (request *directWalletAdjustmentRequest) normalize() (int64, error) {
	request.Direction = strings.ToLower(strings.TrimSpace(request.Direction))
	request.Reason = strings.TrimSpace(request.Reason)
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.Direction != "credit" && request.Direction != "debit" {
		return 0, errors.New("invalid adjustment direction")
	}
	if request.Amount < 1 {
		return 0, errors.New("adjustment amount must be positive")
	}
	if request.Reason == "" || len(request.Reason) > 500 {
		return 0, errors.New("invalid adjustment reason")
	}
	if request.RequestID == "" || len(request.RequestID) > 100 {
		return 0, errors.New("invalid adjustment request id")
	}
	if request.Direction == "debit" {
		return -request.Amount, nil
	}
	return request.Amount, nil
}

func directWalletAdjustmentNo(adminID int64, requestID string) string {
	sum := sha256.Sum256([]byte(strconv.FormatInt(adminID, 10) + ":" + requestID))
	return "Q" + hex.EncodeToString(sum[:])[:25]
}

func (h *Handler) adjustUserWallet(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户编号无效")
		return
	}
	var request directWalletAdjustmentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	signedAmount, err := request.normalize()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "调账参数无效")
		return
	}
	adminUser, _ := adminFromRequest(r)
	adjustmentNo := directWalletAdjustmentNo(adminUser.ID, request.RequestID)

	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调账失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var lockedUserID int64
	err = tx.QueryRowContext(r.Context(), "SELECT id FROM users WHERE id=? FOR UPDATE", userID).Scan(&lockedUserID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "用户不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调账失败")
		return
	}

	var adjustmentID, existingUserID, existingAmount, requestedBy int64
	var existingReason string
	var existingStatus int
	idempotent := false
	err = tx.QueryRowContext(r.Context(), `
		SELECT id,user_id,amount,reason,status,requested_by
		FROM wallet_adjustments
		WHERE adjustment_no=?
		FOR UPDATE`,
		adjustmentNo,
	).Scan(
		&adjustmentID, &existingUserID, &existingAmount, &existingReason,
		&existingStatus, &requestedBy,
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		result, insertErr := tx.ExecContext(r.Context(), `
			INSERT INTO wallet_adjustments
				(adjustment_no,user_id,amount,reason,status,requested_by,reviewed_by,reviewed_at)
			VALUES(?,?,?,?,3,?,?,CURRENT_TIMESTAMP(3))`,
			adjustmentNo, userID, signedAmount, request.Reason, adminUser.ID, adminUser.ID,
		)
		if insertErr != nil {
			var mysqlErr *mysqlDriver.MySQLError
			if errors.As(insertErr, &mysqlErr) && mysqlErr.Number == 1062 {
				httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "request_id 已用于其他调账")
				return
			}
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调账失败")
			return
		}
		adjustmentID, _ = result.LastInsertId()
	case err != nil:
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调账失败")
		return
	case existingUserID != userID || existingAmount != signedAmount ||
		existingReason != request.Reason || requestedBy != adminUser.ID || existingStatus != 3:
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "request_id 已用于其他调账")
		return
	default:
		idempotent = true
	}

	entry, err := h.wallet.ApplyTx(r.Context(), tx, wallet.ApplyRequest{
		UserID:       userID,
		Amount:       signedAmount,
		BusinessType: "admin_adjustment",
		BusinessID:   adjustmentNo,
		Description:  request.Reason,
		Metadata: map[string]any{
			"adjustment_id": apiDecimalID(adjustmentID),
			"admin_id":      apiDecimalID(adminUser.ID),
			"direction":     request.Direction,
			"request_id":    request.RequestID,
		},
	})
	if errors.Is(err, wallet.ErrInsufficientFunds) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "可用余额不足，无法扣款")
		return
	}
	if errors.Is(err, wallet.ErrAccountDisabled) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "用户钱包不可用")
		return
	}
	if errors.Is(err, wallet.ErrIdempotencyReuse) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "request_id 已用于其他调账")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调账失败")
		return
	}
	balanceAvailable, balanceFrozen := entry.Available, entry.Frozen
	if idempotent {
		err = tx.QueryRowContext(r.Context(), `
			SELECT available,frozen
			FROM wallet_accounts
			WHERE user_id=? AND currency='COIN'`,
			userID,
		).Scan(&balanceAvailable, &balanceFrozen)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取用户余额失败")
			return
		}
	}

	if !idempotent {
		if err = auditAdmin(
			r.Context(), tx, r, "wallet.adjustment.direct", "wallet_adjustment", adjustmentID,
			map[string]any{
				"user_id":   apiDecimalID(userID),
				"currency":  "COIN",
				"available": entry.Available - signedAmount,
				"frozen":    entry.Frozen,
			},
			map[string]any{
				"adjustment_no":   adjustmentNo,
				"request_id":      request.RequestID,
				"user_id":         apiDecimalID(userID),
				"direction":       request.Direction,
				"amount":          request.Amount,
				"reason":          request.Reason,
				"currency":        "COIN",
				"available":       entry.Available,
				"frozen":          entry.Frozen,
				"ledger_entry_no": entry.EntryNo,
			},
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录资金审计失败")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调账失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"adjustment": map[string]any{
			"id": apiDecimalID(adjustmentID), "adjustment_no": adjustmentNo, "request_id": request.RequestID,
			"user_id": apiDecimalID(userID), "direction": request.Direction, "amount": request.Amount,
			"reason": request.Reason, "status": "applied",
		},
		"balance": map[string]any{
			"currency": "COIN", "available": balanceAvailable, "frozen": balanceFrozen,
		},
		"ledger_entry_no": entry.EntryNo,
		"idempotent":      idempotent,
	})
}

func (h *Handler) setUserStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户编号无效")
		return
	}
	var request struct {
		Status int    `json:"status"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Status < 1 || request.Status > 3 || strings.TrimSpace(request.Reason) == "" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "状态或原因无效")
		return
	}
	adminUser, _ := adminFromRequest(r)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var previous int
	if err = tx.QueryRowContext(r.Context(), "SELECT status FROM users WHERE id=? FOR UPDATE", userID).Scan(&previous); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "用户不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE users SET status=?,closed_at=IF(?=3,CURRENT_TIMESTAMP(3),NULL) WHERE id=?`,
		request.Status, request.Status, userID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户失败")
		return
	}
	if request.Status != 1 {
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE user_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
			WHERE user_id=? AND revoked_at IS NULL`,
			userID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户失败")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE team_console_sessions SET revoked_at=CURRENT_TIMESTAMP(3)
			WHERE user_id=? AND revoked_at IS NULL`,
			userID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户失败")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO audit_logs
			(request_id,actor_type,actor_id,action,resource_type,resource_id,before_data,after_data,ip,user_agent)
		VALUES(?,1,?,'user.status.change','user',?,
		       JSON_OBJECT('status',?),JSON_OBJECT('status',?,'reason',?),?,?)`,
		httpx.RequestID(r.Context()), adminUser.ID, userID, previous, request.Status,
		request.Reason, clientIP(r), r.UserAgent(),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新用户失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"user_id": apiDecimalID(userID), "status": request.Status,
	})
}

func (h *Handler) listWalletLedger(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	gameCode := strings.TrimSpace(r.URL.Query().Get("game_code"))
	roundNo := strings.TrimSpace(r.URL.Query().Get("round_no"))
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		userID, userID,
		gameCode, gameCode,
		roundNo, roundNo,
		keyword, like, like, like, like, like, like, like, like,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM wallet_ledger_entries entry
		WHERE (?=0 OR entry.user_id=?)
		  AND (?='' OR entry.game_code=?)
		  AND (?='' OR entry.round_no=?)
		  AND (?='' OR entry.entry_no LIKE ? OR entry.business_type LIKE ?
		       OR entry.business_id LIKE ? OR entry.description LIKE ?
		       OR entry.game_code LIKE ? OR entry.venue_code LIKE ?
		       OR entry.round_no LIKE ? OR CAST(entry.user_id AS CHAR) LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金流水失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT entry.id,entry.entry_no,entry.user_id,entry.delta_available,entry.delta_frozen,
		       entry.balance_available,entry.balance_frozen,entry.business_type,entry.business_id,
		       entry.direction,entry.game_code,entry.venue_code,entry.table_no,entry.round_no,
		       entry.description,entry.created_at
		FROM wallet_ledger_entries entry
		WHERE (?=0 OR entry.user_id=?)
		  AND (?='' OR entry.game_code=?)
		  AND (?='' OR entry.round_no=?)
		  AND (?='' OR entry.entry_no LIKE ? OR entry.business_type LIKE ?
		       OR entry.business_id LIKE ? OR entry.description LIKE ?
		       OR entry.game_code LIKE ? OR entry.venue_code LIKE ?
		       OR entry.round_no LIKE ? OR CAST(entry.user_id AS CHAR) LIKE ?)
		ORDER BY entry.created_at DESC,entry.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金流水失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, ledgerUserID, deltaAvailable, deltaFrozen, available, frozen int64
		var tableNo, direction int
		var entryNo, businessType, businessID, game, venue, round, description string
		var createdAt time.Time
		if err = rows.Scan(
			&id, &entryNo, &ledgerUserID, &deltaAvailable, &deltaFrozen, &available, &frozen,
			&businessType, &businessID, &direction, &game, &venue, &tableNo, &round,
			&description, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金流水失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "entry_no": entryNo, "user_id": apiDecimalID(ledgerUserID),
			"delta_available": deltaAvailable, "delta_frozen": deltaFrozen,
			"balance_available": available, "balance_frozen": frozen,
			"business_type": businessType, "business_id": businessID, "direction": direction,
			"game_code": game, "venue_code": venue, "table_no": tableNo, "round_no": round,
			"description": description, "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金流水失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) createWalletAdjustment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		UserID          decimalIDInput `json:"user_id"`
		Amount          int64          `json:"amount"`
		Reason          string         `json:"reason"`
		EvidenceAssetID decimalIDInput `json:"evidence_asset_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	userID := request.UserID.Int64()
	request.Reason = strings.TrimSpace(request.Reason)
	if userID < 1 || request.Amount == 0 || request.Reason == "" || len(request.Reason) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "调账参数无效")
		return
	}
	adjustmentNo, err := idgen.New()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建调账失败")
		return
	}
	adminUser, _ := adminFromRequest(r)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建调账失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO wallet_adjustments
			(adjustment_no,user_id,amount,reason,evidence_asset_id,status,requested_by)
		SELECT ?,?,?,?,?,0,? FROM users WHERE id=?`,
		adjustmentNo, userID, request.Amount, request.Reason,
		request.EvidenceAssetID.Int64(), adminUser.ID, userID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建调账失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "用户不存在")
		return
	}
	adjustmentID, _ := result.LastInsertId()
	if err = auditAdmin(
		r.Context(), tx, r, "wallet.adjustment.create", "wallet_adjustment", adjustmentID,
		nil, map[string]any{
			"adjustment_no": adjustmentNo, "user_id": apiDecimalID(userID),
			"amount": request.Amount, "reason": request.Reason,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录资金审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建调账失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(adjustmentID), "adjustment_no": adjustmentNo, "status": "pending",
	})
}

func (h *Handler) approveWalletAdjustment(w http.ResponseWriter, r *http.Request) {
	adjustmentID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || adjustmentID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "调账编号无效")
		return
	}
	adminUser, _ := adminFromRequest(r)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "审核调账失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var adjustmentNo, reason string
	var userID, amount, requestedBy int64
	var status int
	err = tx.QueryRowContext(r.Context(), `
		SELECT adjustment_no,user_id,amount,reason,status,requested_by
		FROM wallet_adjustments WHERE id=? FOR UPDATE`,
		adjustmentID,
	).Scan(&adjustmentNo, &userID, &amount, &reason, &status, &requestedBy)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "调账申请不存在")
		return
	}
	if err != nil || status != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "调账申请状态已变化")
		return
	}
	if requestedBy == adminUser.ID {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "申请人不能审核自己的调账")
		return
	}
	entry, err := h.wallet.ApplyTx(r.Context(), tx, wallet.ApplyRequest{
		UserID: userID, Amount: amount, BusinessType: "admin_adjustment",
		BusinessID: adjustmentNo, Description: reason,
		Metadata: map[string]any{
			"adjustment_id": apiDecimalID(adjustmentID),
			"reviewed_by":   apiDecimalID(adminUser.ID),
		},
	})
	if errors.Is(err, wallet.ErrInsufficientFunds) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "可用余额不足，调账无法执行")
		return
	}
	if errors.Is(err, wallet.ErrAccountDisabled) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "用户钱包不可用")
		return
	}
	if errors.Is(err, wallet.ErrIdempotencyReuse) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "调账业务号已被其他金额占用")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "调账执行失败")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		UPDATE wallet_adjustments
		SET status=3,reviewed_by=?,reviewed_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND status=0`,
		adminUser.ID, adjustmentID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新调账状态失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "调账申请状态已变化")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "wallet.adjustment.approve", "wallet_adjustment", adjustmentID,
		map[string]any{"status": status, "requested_by": apiDecimalID(requestedBy)},
		map[string]any{
			"status": 3, "ledger_entry_no": entry.EntryNo,
			"reviewed_by": apiDecimalID(adminUser.ID),
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录资金审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新调账状态失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"adjustment_id": apiDecimalID(adjustmentID), "status": "applied",
		"ledger": walletEntryForAPI(entry),
	})
}

func (h *Handler) rejectWalletAdjustment(w http.ResponseWriter, r *http.Request) {
	adjustmentID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || adjustmentID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "调账编号无效")
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" || len(request.Reason) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "驳回原因无效")
		return
	}
	adminUser, _ := adminFromRequest(r)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "驳回调账失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		UPDATE wallet_adjustments
		SET status=2,reviewed_by=?,reviewed_at=CURRENT_TIMESTAMP(3),
		    reason=CONCAT(reason,'；驳回：',?)
		WHERE id=? AND status=0`,
		adminUser.ID, request.Reason, adjustmentID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "驳回调账失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "调账申请状态已变化")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "wallet.adjustment.reject", "wallet_adjustment", adjustmentID,
		map[string]int{"status": 0},
		map[string]any{
			"status": 2, "reason": request.Reason,
			"reviewed_by": apiDecimalID(adminUser.ID),
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录资金审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "驳回调账失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"adjustment_id": apiDecimalID(adjustmentID), "status": "rejected",
	})
}

func (h *Handler) listRechargeOrders(w http.ResponseWriter, r *http.Request) {
	h.listSimpleOrders(w, r, "recharge")
}

func (h *Handler) listWithdrawOrders(w http.ResponseWriter, r *http.Request) {
	h.listSimpleOrders(w, r, "withdraw")
}

func (h *Handler) listSimpleOrders(w http.ResponseWriter, r *http.Request, orderType string) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		if parsedStatus, parseErr := strconv.Atoi(rawStatus); parseErr == nil {
			status = parsedStatus
		}
	}
	filterArguments := []any{keyword, like, like, status, status}
	var total int64
	var rows *sql.Rows
	var err error
	if orderType == "recharge" {
		err = h.db.QueryRowContext(r.Context(), `
			SELECT COUNT(*)
			FROM recharge_orders recharge_order
			WHERE (?='' OR recharge_order.order_no LIKE ?
			       OR CAST(recharge_order.user_id AS CHAR) LIKE ?)
			  AND (? < 0 OR recharge_order.status=?)`,
			filterArguments...,
		).Scan(&total)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金订单失败")
			return
		}
		rows, err = h.db.QueryContext(r.Context(), `
			SELECT id,order_no,user_id,amount_minor,coin_amount,bonus_coin,status,created_at
			FROM recharge_orders recharge_order
			WHERE (?='' OR recharge_order.order_no LIKE ?
			       OR CAST(recharge_order.user_id AS CHAR) LIKE ?)
			  AND (? < 0 OR recharge_order.status=?)
			ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`,
			append(filterArguments, pageSize, (page-1)*pageSize)...,
		)
	} else {
		err = h.db.QueryRowContext(r.Context(), `
			SELECT COUNT(*)
			FROM withdraw_orders withdraw_order
			WHERE (?='' OR withdraw_order.order_no LIKE ?
			       OR CAST(withdraw_order.user_id AS CHAR) LIKE ?)
			  AND (? < 0 OR withdraw_order.status=?)`,
			filterArguments...,
		).Scan(&total)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金订单失败")
			return
		}
		rows, err = h.db.QueryContext(r.Context(), `
			SELECT id,order_no,user_id,payout_amount_minor,coin_amount,fee_coin,status,created_at
			FROM withdraw_orders withdraw_order
			WHERE (?='' OR withdraw_order.order_no LIKE ?
			       OR CAST(withdraw_order.user_id AS CHAR) LIKE ?)
			  AND (? < 0 OR withdraw_order.status=?)
			ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`,
			append(filterArguments, pageSize, (page-1)*pageSize)...,
		)
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金订单失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, userID, amountMinor, coinAmount, extra int64
		var orderNo string
		var status int
		var createdAt time.Time
		if err = rows.Scan(&id, &orderNo, &userID, &amountMinor, &coinAmount, &extra, &status, &createdAt); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金订单失败")
			return
		}
		item := map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(userID), "amount_minor": amountMinor,
			"coin_amount": coinAmount, "status": status, "created_at": createdAt.Unix(),
		}
		if orderType == "recharge" {
			item["bonus_coin"] = extra
		} else {
			item["fee_coin"] = extra
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取资金订单失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
