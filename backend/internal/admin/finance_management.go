package admin

import (
	"database/sql"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func (h *Handler) listWalletAdjustments(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	userID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("user_id")), 10, 64)
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		status, _ = strconv.Atoi(rawStatus)
	}
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		keyword, like, like, like, like, like,
		userID, userID,
		status, status,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM wallet_adjustments adjustment
		LEFT JOIN users user ON user.id=adjustment.user_id
		WHERE (?='' OR adjustment.adjustment_no LIKE ?
		       OR adjustment.reason LIKE ? OR user.username LIKE ?
		       OR user.nickname LIKE ? OR CAST(adjustment.user_id AS CHAR) LIKE ?)
		  AND (?=0 OR adjustment.user_id=?)
		  AND (? < 0 OR adjustment.status=?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取调账申请失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT adjustment.id,adjustment.adjustment_no,adjustment.user_id,
		       COALESCE(NULLIF(user.nickname,''),user.username),adjustment.amount,
		       adjustment.reason,adjustment.status,adjustment.requested_by,
		       adjustment.reviewed_by,adjustment.reviewed_at,adjustment.created_at
		FROM wallet_adjustments adjustment
		LEFT JOIN users user ON user.id=adjustment.user_id
		WHERE (?='' OR adjustment.adjustment_no LIKE ?
		       OR adjustment.reason LIKE ? OR user.username LIKE ?
		       OR user.nickname LIKE ? OR CAST(adjustment.user_id AS CHAR) LIKE ?)
		  AND (?=0 OR adjustment.user_id=?)
		  AND (? < 0 OR adjustment.status=?)
		ORDER BY adjustment.created_at DESC,adjustment.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取调账申请失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, userID, amount, requestedBy, reviewedBy int64
		var adjustmentNo, nickname, reason string
		var status int
		var reviewedAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &adjustmentNo, &userID, &nickname, &amount, &reason, &status,
			&requestedBy, &reviewedBy, &reviewedAt, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取调账申请失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "adjustment_no": adjustmentNo,
			"user_id": apiDecimalID(userID), "nickname": nickname,
			"amount": amount, "reason": reason, "status": status,
			"requested_by": apiDecimalID(requestedBy),
			"reviewed_by":  apiDecimalID(reviewedBy), "reviewed_at": nullTime(reviewedAt),
			"created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取调账申请失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) markRechargePaid(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || orderID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "充值订单编号无效")
		return
	}
	var request struct {
		ProviderOrderNo string `json:"provider_order_no"`
		Reason          string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.ProviderOrderNo = strings.TrimSpace(request.ProviderOrderNo)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.ProviderOrderNo == "" || len(request.ProviderOrderNo) > 190 ||
		request.Reason == "" || len(request.Reason) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "充值确认参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "确认充值失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var orderNo string
	var providerOrderNo sql.NullString
	var userID, coinAmount, bonusCoin int64
	var status int
	err = tx.QueryRowContext(r.Context(), `
		SELECT order_no,user_id,coin_amount,bonus_coin,status,provider_order_no
		FROM recharge_orders WHERE id=? FOR UPDATE`,
		orderID,
	).Scan(&orderNo, &userID, &coinAmount, &bonusCoin, &status, &providerOrderNo)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "充值订单不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值订单失败")
		return
	}
	if status != 0 && status != 1 && status != 2 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "充值订单状态不能确认")
		return
	}
	if status == 2 && strings.TrimSpace(providerOrderNo.String) != "" &&
		providerOrderNo.String != request.ProviderOrderNo {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "充值订单已由其他渠道订单确认")
		return
	}
	if coinAmount < 0 || bonusCoin < 0 || coinAmount > math.MaxInt64-bonusCoin {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "充值金额异常")
		return
	}
	entry, err := h.wallet.ApplyTx(r.Context(), tx, wallet.ApplyRequest{
		UserID: userID, Amount: coinAmount + bonusCoin,
		BusinessType: "recharge", BusinessID: orderNo,
		Description: request.Reason,
		Metadata: map[string]any{
			"recharge_order_id": apiDecimalID(orderID), "provider_order_no": request.ProviderOrderNo,
			"confirmed_by": apiDecimalID(adminID(r)),
		},
	})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "充值入账失败")
		return
	}
	_, err = tx.ExecContext(r.Context(), `
		UPDATE recharge_orders
		SET status=2,provider_order_no=?,paid_at=COALESCE(paid_at,CURRENT_TIMESTAMP(3)),
		    provider_payload=JSON_SET(COALESCE(provider_payload,JSON_OBJECT()),
		                              '$.manual_reason',?,'$.confirmed_by',?)
		WHERE id=?`,
		request.ProviderOrderNo, request.Reason, adminID(r), orderID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新充值订单失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "wallet.recharge.manual_paid", "recharge_order", orderID,
		map[string]any{"status": status, "provider_order_no": providerOrderNo.String}, map[string]any{
			"status": 2, "provider_order_no": request.ProviderOrderNo, "ledger_entry_no": entry.EntryNo,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录资金审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "确认充值失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(orderID), "status": 2, "ledger": walletEntryForAPI(entry),
	})
}

func (h *Handler) reviewWithdrawal(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || orderID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "提现订单编号无效")
		return
	}
	var request struct {
		Action          string `json:"action"`
		Reason          string `json:"reason"`
		ProviderOrderNo string `json:"provider_order_no"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.Reason = strings.TrimSpace(request.Reason)
	request.ProviderOrderNo = strings.TrimSpace(request.ProviderOrderNo)
	if request.Action != "approve" && request.Action != "reject" &&
		request.Action != "paying" && request.Action != "paid" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "提现审核动作无效")
		return
	}
	if request.Action == "reject" && request.Reason == "" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "驳回提现必须填写原因")
		return
	}
	if request.Action == "paid" && request.ProviderOrderNo == "" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "确认打款必须填写渠道订单号")
		return
	}
	if len(request.Reason) > 500 || len(request.ProviderOrderNo) > 190 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "提现审核参数过长")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "审核提现失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var orderNo, holdNo, existingProviderOrderNo string
	var status int
	err = tx.QueryRowContext(r.Context(), `
		SELECT order_no,hold_no,status,provider_order_no
		FROM withdraw_orders WHERE id=? FOR UPDATE`,
		orderID,
	).Scan(&orderNo, &holdNo, &status, &existingProviderOrderNo)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "提现订单不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取提现订单失败")
		return
	}
	targetStatus := status
	walletEntryNo := ""
	switch request.Action {
	case "approve":
		if status != 0 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "只有待审核订单可以通过")
			return
		}
		targetStatus = 1
	case "paying":
		if status != 1 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "只有已通过订单可以进入打款")
			return
		}
		targetStatus = 2
	case "reject":
		if status != 0 && status != 1 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "当前提现状态不能驳回")
			return
		}
		entry, releaseErr := h.wallet.ReleaseHoldTx(r.Context(), tx, wallet.ReleaseRequest{
			HoldNo: holdNo, Description: "提现驳回退回冻结余额",
			Metadata: map[string]any{
				"withdraw_order_id": apiDecimalID(orderID),
				"reviewed_by":       apiDecimalID(adminID(r)),
			},
		})
		if releaseErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "退回提现冻结余额失败")
			return
		}
		walletEntryNo = entry.EntryNo
		targetStatus = 4
	case "paid":
		if status != 1 && status != 2 && status != 3 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "当前提现状态不能确认打款")
			return
		}
		if existingProviderOrderNo != "" && existingProviderOrderNo != request.ProviderOrderNo {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "提现订单已绑定其他渠道订单")
			return
		}
		entry, commitErr := h.wallet.CommitHoldTx(r.Context(), tx, wallet.CommitRequest{
			HoldNo: holdNo, Payout: 0, Description: "提现完成扣除冻结余额",
			Metadata: map[string]any{
				"withdraw_order_id": apiDecimalID(orderID), "provider_order_no": request.ProviderOrderNo,
				"confirmed_by": apiDecimalID(adminID(r)),
			},
		})
		if commitErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "扣除提现冻结余额失败")
			return
		}
		walletEntryNo = entry.EntryNo
		targetStatus = 3
	}
	_, err = tx.ExecContext(r.Context(), `
		UPDATE withdraw_orders
		SET status=?,reject_reason=IF(?=4,?,reject_reason),
		    provider_order_no=IF(?<>'',?,provider_order_no),
		    reviewed_by=IF(reviewed_by=0,?,reviewed_by),
		    reviewed_at=COALESCE(reviewed_at,CURRENT_TIMESTAMP(3)),
		    paid_at=IF(?=3,COALESCE(paid_at,CURRENT_TIMESTAMP(3)),paid_at)
		WHERE id=?`,
		targetStatus, targetStatus, request.Reason,
		request.ProviderOrderNo, request.ProviderOrderNo,
		adminID(r), targetStatus, orderID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新提现订单失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "wallet.withdraw."+request.Action, "withdraw_order", orderID,
		map[string]any{"status": status, "provider_order_no": existingProviderOrderNo}, map[string]any{
			"status": targetStatus, "reason": request.Reason, "provider_order_no": request.ProviderOrderNo,
			"ledger_entry_no": walletEntryNo,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录资金审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "审核提现失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(orderID), "order_no": orderNo, "status": targetStatus,
	})
}
