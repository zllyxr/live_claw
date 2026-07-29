package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func (h *Handler) listWalletAdjustments(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT adjustment.id,adjustment.adjustment_no,adjustment.user_id,
		       COALESCE(NULLIF(user.nickname,''),user.username),adjustment.amount,
		       adjustment.reason,adjustment.status,adjustment.requested_by,
		       adjustment.reviewed_by,adjustment.reviewed_at,adjustment.created_at
		FROM wallet_adjustments adjustment
		LEFT JOIN users user ON user.id=adjustment.user_id
		ORDER BY adjustment.created_at DESC,adjustment.id DESC
		LIMIT ? OFFSET ?`,
		pageSize, (page-1)*pageSize,
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
			"id": id, "adjustment_no": adjustmentNo, "user_id": userID, "nickname": nickname,
			"amount": amount, "reason": reason, "status": status, "requested_by": requestedBy,
			"reviewed_by": reviewedBy, "reviewed_at": nullTime(reviewedAt),
			"created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取调账申请失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "items": items,
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
	var orderNo string
	var userID, coinAmount, bonusCoin int64
	var status int
	err = h.db.QueryRowContext(r.Context(), `
		SELECT order_no,user_id,coin_amount,bonus_coin,status
		FROM recharge_orders WHERE id=?`,
		orderID,
	).Scan(&orderNo, &userID, &coinAmount, &bonusCoin, &status)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "充值订单不存在")
		return
	}
	if err != nil || (status != 0 && status != 1 && status != 2) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "充值订单状态不能确认")
		return
	}
	entry, err := h.wallet.Apply(r.Context(), wallet.ApplyRequest{
		UserID: userID, Amount: coinAmount + bonusCoin,
		BusinessType: "recharge", BusinessID: orderNo,
		Description: request.Reason,
		Metadata: map[string]any{
			"recharge_order_id": orderID, "provider_order_no": request.ProviderOrderNo,
			"confirmed_by": adminID(r),
		},
	})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "充值入账失败")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE recharge_orders
		SET status=2,provider_order_no=?,paid_at=COALESCE(paid_at,CURRENT_TIMESTAMP(3)),
		    provider_payload=JSON_SET(COALESCE(provider_payload,JSON_OBJECT()),
		                              '$.manual_reason',?,'$.confirmed_by',?)
		WHERE id=? AND status IN (0,1,2)`,
		request.ProviderOrderNo, request.Reason, adminID(r), orderID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新充值订单失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 && status != 2 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "充值订单状态已变化")
		return
	}
	if err = auditAdmin(
		r.Context(), h.db, r, "wallet.recharge.manual_paid", "recharge_order", orderID,
		map[string]int{"status": status}, map[string]any{
			"status": 2, "provider_order_no": request.ProviderOrderNo, "ledger_entry_no": entry.EntryNo,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录资金审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": orderID, "status": 2, "ledger": entry,
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
	var orderNo, holdNo string
	var status int
	err = h.db.QueryRowContext(r.Context(), `
		SELECT order_no,hold_no,status FROM withdraw_orders WHERE id=?`,
		orderID,
	).Scan(&orderNo, &holdNo, &status)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "提现订单不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取提现订单失败")
		return
	}
	targetStatus := status
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
		if _, err = h.wallet.ReleaseHold(
			r.Context(), holdNo, "提现驳回退回冻结余额",
			map[string]any{"withdraw_order_id": orderID, "reviewed_by": adminID(r)},
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "退回提现冻结余额失败")
			return
		}
		targetStatus = 4
	case "paid":
		if status != 1 && status != 2 && status != 3 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "当前提现状态不能确认打款")
			return
		}
		if _, err = h.wallet.CommitHold(r.Context(), wallet.CommitRequest{
			HoldNo: holdNo, Payout: 0, Description: "提现完成扣除冻结余额",
			Metadata: map[string]any{
				"withdraw_order_id": orderID, "provider_order_no": request.ProviderOrderNo,
				"confirmed_by": adminID(r),
			},
		}); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "扣除提现冻结余额失败")
			return
		}
		targetStatus = 3
	}
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE withdraw_orders
		SET status=?,reject_reason=IF(?=4,?,reject_reason),
		    provider_order_no=IF(?<>'',?,provider_order_no),
		    reviewed_by=IF(reviewed_by=0,?,reviewed_by),
		    reviewed_at=COALESCE(reviewed_at,CURRENT_TIMESTAMP(3)),
		    paid_at=IF(?=3,COALESCE(paid_at,CURRENT_TIMESTAMP(3)),paid_at)
		WHERE id=? AND status=?`,
		targetStatus, targetStatus, request.Reason,
		request.ProviderOrderNo, request.ProviderOrderNo,
		adminID(r), targetStatus, orderID, status,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新提现订单失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 && targetStatus != status {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "提现订单状态已变化")
		return
	}
	if err = auditAdmin(
		r.Context(), h.db, r, "wallet.withdraw."+request.Action, "withdraw_order", orderID,
		map[string]int{"status": status}, map[string]any{
			"status": targetStatus, "reason": request.Reason, "provider_order_no": request.ProviderOrderNo,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录资金审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": orderID, "order_no": orderNo, "status": targetStatus,
	})
}
