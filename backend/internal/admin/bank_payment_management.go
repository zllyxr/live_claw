package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/bankpayment"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/storage"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

const manualBankProvider = "manual_bank"

func (h *Handler) adminBankRechargeDetails(ctx context.Context, orderID int64, orderStatus int) (map[string]any, error) {
	var accountID int64
	var assignedAt sql.NullTime
	var closeReason string
	var displayName, masked sql.NullString
	var proofID, proofStatus sql.NullInt64
	var proofReason sql.NullString
	err := h.db.QueryRowContext(ctx, `
		SELECT detail.bank_account_id,detail.assigned_at,detail.close_reason,
		       account.display_name,account.account_masked,
		       proof.id,proof.status,proof.review_reason
		FROM payment_bank_order_details detail
		LEFT JOIN payment_bank_accounts account ON account.id=detail.bank_account_id
		LEFT JOIN payment_bank_proofs proof ON proof.recharge_order_id=detail.recharge_order_id
		WHERE detail.recharge_order_id=?`, orderID).Scan(
		&accountID, &assignedAt, &closeReason, &displayName, &masked,
		&proofID, &proofStatus, &proofReason,
	)
	if err != nil {
		return nil, err
	}
	hasPendingProof := proofID.Valid && proofStatus.Int64 == 0
	return map[string]any{
		"payment_method":  "bank_transfer",
		"bank_stage":      bankpayment.Stage(orderStatus, accountID > 0, hasPendingProof),
		"bank_account_id": apiDecimalID(accountID), "bank_account_name": displayName.String,
		"bank_account_masked": masked.String, "bank_assigned_at": nullTime(assignedAt),
		"bank_close_reason": closeReason, "proof_id": nullableAdminInt64(proofID),
		"proof_status": nullableAdminInt64(proofStatus), "proof_review_reason": proofReason.String,
	}, nil
}

func nullableAdminInt64(value sql.NullInt64) any {
	if !value.Valid {
		return ""
	}
	return value.Int64
}

type bankAccountWriteRequest struct {
	DisplayName  string `json:"display_name"`
	BankName     string `json:"bank_name"`
	BranchName   string `json:"branch_name"`
	HolderName   string `json:"holder_name"`
	CardNumber   string `json:"card_number"`
	Instructions string `json:"instructions"`
	SortOrder    int    `json:"sort_order"`
}

func (request *bankAccountWriteRequest) normalize() {
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.BankName = strings.TrimSpace(request.BankName)
	request.BranchName = strings.TrimSpace(request.BranchName)
	request.HolderName = strings.TrimSpace(request.HolderName)
	request.CardNumber = strings.TrimSpace(request.CardNumber)
	request.Instructions = strings.TrimSpace(request.Instructions)
}

func (request bankAccountWriteRequest) validate(requireCard bool) error {
	if request.DisplayName == "" || len([]rune(request.DisplayName)) > 100 ||
		request.BankName == "" || len([]rune(request.BankName)) > 190 ||
		len([]rune(request.BranchName)) > 190 || request.HolderName == "" ||
		len([]rune(request.HolderName)) > 100 || len([]rune(request.Instructions)) > 500 ||
		request.SortOrder < -1_000_000 || request.SortOrder > 1_000_000 {
		return errors.New("名称、银行、持卡人、支行、说明或排序无效")
	}
	if requireCard || request.CardNumber != "" {
		if _, err := bankpayment.NormalizeCardNumber(request.CardNumber); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) listBankAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id,display_name,bank_name,branch_name,account_masked,
		       account_hash,account_ciphertext,key_version,instructions,
		       status,sort_order,created_at,updated_at
		FROM payment_bank_accounts
		ORDER BY sort_order DESC,id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取银行卡失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 8)
	for rows.Next() {
		var id int64
		var displayName, bankName, branchName, masked, accountHash, instructions string
		var ciphertext []byte
		var status, sortOrder, keyVersion int
		var createdAt, updatedAt time.Time
		if err = rows.Scan(&id, &displayName, &bankName, &branchName, &masked,
			&accountHash, &ciphertext, &keyVersion, &instructions, &status,
			&sortOrder, &createdAt, &updatedAt); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取银行卡失败")
			return
		}
		if keyVersion != bankpayment.KeyVersion {
			httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "银行卡密钥版本不受支持")
			return
		}
		secret, decryptErr := h.bankPaymentCipher.DecryptAccount(accountHash, ciphertext)
		if decryptErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "银行卡密文无法校验")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "display_name": displayName, "bank_name": bankName,
			"branch_name": branchName, "card_number_masked": masked,
			"instructions": instructions, "status": status, "sort_order": sortOrder,
			"holder_name": secret.HolderName,
			"created_at":  createdAt.Unix(), "updated_at": updatedAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取银行卡失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": 1, "page_size": len(items), "total": len(items), "has_more": false, "items": items,
	})
}

func (h *Handler) createBankAccount(w http.ResponseWriter, r *http.Request) {
	var request bankAccountWriteRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.normalize()
	if err := request.validate(true); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "银行卡参数无效："+err.Error())
		return
	}
	cardNumber, _ := bankpayment.NormalizeCardNumber(request.CardNumber)
	accountHash := bankpayment.CardHash(cardNumber)
	ciphertext, err := h.bankPaymentCipher.EncryptAccount(accountHash, bankpayment.AccountSecret{
		HolderName: request.HolderName, CardNumber: cardNumber,
	})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "加密银行卡失败")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "新增银行卡失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO payment_bank_accounts
			(display_name,bank_name,branch_name,account_ciphertext,account_hash,
			 account_masked,key_version,instructions,status,sort_order)
		VALUES(?,?,?,?,?,?,?, ?,0,?)`, request.DisplayName, request.BankName,
		request.BranchName, ciphertext, accountHash, bankpayment.MaskCardNumber(cardNumber),
		bankpayment.KeyVersion, request.Instructions, request.SortOrder)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "该银行卡已经存在")
			return
		}
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "新增银行卡失败")
		return
	}
	id, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), tx, r, "payment.bank_account.create", "payment_bank_account", id,
		nil, map[string]any{"display_name": request.DisplayName, "bank_name": request.BankName,
			"card_number_masked": bankpayment.MaskCardNumber(cardNumber), "status": 0}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "新增银行卡失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(id), "status": 0})
}

func (h *Handler) updateBankAccount(w http.ResponseWriter, r *http.Request) {
	id, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "银行卡编号无效")
		return
	}
	var request bankAccountWriteRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.normalize()
	if err = request.validate(false); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "银行卡参数无效："+err.Error())
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新银行卡失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var oldDisplay, oldBank, oldBranch, oldHash, oldMasked, oldInstructions string
	var oldCiphertext []byte
	var oldStatus, oldSort, keyVersion int
	err = tx.QueryRowContext(r.Context(), `
		SELECT display_name,bank_name,branch_name,account_hash,account_masked,
		       account_ciphertext,key_version,instructions,status,sort_order
		FROM payment_bank_accounts WHERE id=? FOR UPDATE`, id).Scan(
		&oldDisplay, &oldBank, &oldBranch, &oldHash, &oldMasked, &oldCiphertext,
		&keyVersion, &oldInstructions, &oldStatus, &oldSort,
	)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), 404, 404, "银行卡不存在")
		return
	}
	if err != nil || keyVersion != bankpayment.KeyVersion {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取银行卡失败")
		return
	}
	secret, err := h.bankPaymentCipher.DecryptAccount(oldHash, oldCiphertext)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "银行卡密文无法校验")
		return
	}
	cardNumber := secret.CardNumber
	if request.CardNumber != "" {
		cardNumber, _ = bankpayment.NormalizeCardNumber(request.CardNumber)
	}
	accountHash := bankpayment.CardHash(cardNumber)
	ciphertext, err := h.bankPaymentCipher.EncryptAccount(accountHash, bankpayment.AccountSecret{
		HolderName: request.HolderName, CardNumber: cardNumber,
	})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "加密银行卡失败")
		return
	}
	masked := bankpayment.MaskCardNumber(cardNumber)
	_, err = tx.ExecContext(r.Context(), `
		UPDATE payment_bank_accounts
		SET display_name=?,bank_name=?,branch_name=?,account_ciphertext=?,account_hash=?,
		    account_masked=?,key_version=?,instructions=?,sort_order=? WHERE id=?`,
		request.DisplayName, request.BankName, request.BranchName, ciphertext, accountHash,
		masked, bankpayment.KeyVersion, request.Instructions, request.SortOrder, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "该银行卡已经存在")
			return
		}
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新银行卡失败")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "payment.bank_account.update", "payment_bank_account", id,
		map[string]any{"display_name": oldDisplay, "bank_name": oldBank, "branch_name": oldBranch,
			"card_number_masked": oldMasked, "instructions": oldInstructions, "status": oldStatus, "sort_order": oldSort},
		map[string]any{"display_name": request.DisplayName, "bank_name": request.BankName,
			"branch_name": request.BranchName, "card_number_masked": masked,
			"instructions": request.Instructions, "status": oldStatus, "sort_order": request.SortOrder}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新银行卡失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(id), "status": oldStatus})
}

func (h *Handler) setBankAccountStatus(w http.ResponseWriter, r *http.Request) {
	id, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "银行卡编号无效")
		return
	}
	var request struct {
		Status int `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Status != 0 && request.Status != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "银行卡状态无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新银行卡状态失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var channelEnabled int
	if err = tx.QueryRowContext(r.Context(),
		"SELECT status FROM payment_channels WHERE channel_key='bank' FOR UPDATE",
	).Scan(&channelEnabled); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取银行卡通道失败")
		return
	}
	var previous int
	if err = tx.QueryRowContext(r.Context(), "SELECT status FROM payment_bank_accounts WHERE id=? FOR UPDATE", id).Scan(&previous); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), 404, 404, "银行卡不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取银行卡失败")
		return
	}
	if request.Status == 0 && previous == 1 {
		var activeAccounts int
		if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM payment_bank_accounts WHERE status=1 AND id<>?", id).Scan(&activeAccounts); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取银行卡失败")
			return
		}
		if channelEnabled == 1 && activeAccounts == 0 {
			httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "请先停用银行卡支付通道，再停用最后一张收款卡")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE payment_bank_accounts SET status=? WHERE id=?", request.Status, id); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新银行卡状态失败")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "payment.bank_account.status", "payment_bank_account", id,
		map[string]any{"status": previous}, map[string]any{"status": request.Status}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新银行卡状态失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(id), "status": request.Status})
}

func (h *Handler) assignRechargeBankAccount(w http.ResponseWriter, r *http.Request) {
	orderID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "充值订单编号无效")
		return
	}
	var request struct {
		BankAccountID string `json:"bank_account_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	accountID, err := strconv.ParseInt(strings.TrimSpace(request.BankAccountID), 10, 64)
	if err != nil || accountID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "请选择有效的收款银行卡")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "分配银行卡失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var orderNo, provider string
	var orderStatus int
	var expiresAt sql.NullTime
	var assignedAccountID int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT recharge.order_no,recharge.status,recharge.expires_at,channel.provider,detail.bank_account_id
		FROM recharge_orders recharge
		JOIN payment_channels channel ON channel.id=recharge.channel_id
		JOIN payment_bank_order_details detail ON detail.recharge_order_id=recharge.id
		WHERE recharge.id=? FOR UPDATE`, orderID).Scan(&orderNo, &orderStatus, &expiresAt, &provider, &assignedAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), 404, 404, "银行卡充值订单不存在")
		return
	}
	if err != nil || provider != manualBankProvider || orderStatus != 0 || assignedAccountID != 0 ||
		!expiresAt.Valid || !expiresAt.Time.After(time.Now()) {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "订单已超时、已分配或状态不允许分配")
		return
	}
	var displayName, bankName, branchName, accountHash, instructions string
	var ciphertext []byte
	var keyVersion int
	var accountStatus int
	err = tx.QueryRowContext(r.Context(), `
		SELECT display_name,bank_name,branch_name,account_hash,account_ciphertext,
		       key_version,instructions,status
		FROM payment_bank_accounts WHERE id=? FOR UPDATE`, accountID).Scan(
		&displayName, &bankName, &branchName, &accountHash, &ciphertext,
		&keyVersion, &instructions, &accountStatus,
	)
	if errors.Is(err, sql.ErrNoRows) || accountStatus != 1 || keyVersion != bankpayment.KeyVersion {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "所选银行卡未启用或配置无效")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取银行卡失败")
		return
	}
	secret, err := h.bankPaymentCipher.DecryptAccount(accountHash, ciphertext)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "银行卡密文无法校验")
		return
	}
	snapshotCiphertext, err := h.bankPaymentCipher.EncryptSnapshot(orderNo, bankpayment.AccountSnapshot{
		DisplayName: displayName, BankName: bankName, BranchName: branchName,
		HolderName: secret.HolderName, CardNumber: secret.CardNumber, Instructions: instructions,
	})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "生成订单银行卡快照失败")
		return
	}
	assignedAt := time.Now()
	newExpiry := assignedAt.Add(30 * time.Minute)
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE payment_bank_order_details
		SET bank_account_id=?,account_snapshot_ciphertext=?,snapshot_key_version=?,
		    assigned_by=?,assigned_at=? WHERE recharge_order_id=? AND bank_account_id=0`,
		accountID, snapshotCiphertext, bankpayment.KeyVersion, adminID(r), assignedAt, orderID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "分配银行卡失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE recharge_orders SET status=1,expires_at=? WHERE id=? AND status=0", newExpiry, orderID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新充值订单失败")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "payment.bank_recharge.assign", "recharge_order", orderID,
		map[string]any{"bank_account_id": "0", "status": 0},
		map[string]any{"bank_account_id": apiDecimalID(accountID), "status": 1,
			"card_number_masked": bankpayment.MaskCardNumber(secret.CardNumber), "expires_at": newExpiry.Unix()}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "分配银行卡失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(orderID), "status": 1, "bank_account_id": apiDecimalID(accountID),
		"bank_stage": bankpayment.StageAwaitingPayment, "expires_at": newExpiry.Unix(),
	})
}

func (h *Handler) closeBankRecharge(w http.ResponseWriter, r *http.Request) {
	orderID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "充值订单编号无效")
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" || len([]rune(request.Reason)) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "关闭原因必填且不能超过500字")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "关闭订单失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var status, proofCount int
	var provider string
	err = tx.QueryRowContext(r.Context(), `
		SELECT recharge.status,channel.provider,
		       (SELECT COUNT(*) FROM payment_bank_proofs proof WHERE proof.recharge_order_id=recharge.id)
		FROM recharge_orders recharge JOIN payment_channels channel ON channel.id=recharge.channel_id
		WHERE recharge.id=? FOR UPDATE`, orderID).Scan(&status, &provider, &proofCount)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), 404, 404, "充值订单不存在")
		return
	}
	if err != nil || provider != manualBankProvider || (status != 0 && status != 1) || proofCount != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "该订单不能直接关闭；已提交凭证请执行审核")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE recharge_orders SET status=4,closed_at=CURRENT_TIMESTAMP(3),failure_reason=? WHERE id=?`, request.Reason, orderID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "关闭订单失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE payment_bank_order_details SET close_reason=? WHERE recharge_order_id=?`, request.Reason, orderID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "关闭订单失败")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "payment.bank_recharge.close", "recharge_order", orderID,
		map[string]any{"status": status}, map[string]any{"status": 4, "reason": request.Reason}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "关闭订单失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(orderID), "status": 4})
}

func (h *Handler) bankRechargeProof(w http.ResponseWriter, r *http.Request) {
	orderID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "充值订单编号无效")
		return
	}
	var proofID int64
	var bucket, objectKey, mimeType string
	err = h.db.QueryRowContext(r.Context(), `
		SELECT proof.id,asset.bucket,asset.object_key,asset.mime_type
		FROM payment_bank_proofs proof
		JOIN media_assets asset ON asset.id=proof.asset_id AND asset.status=1
		WHERE proof.recharge_order_id=?`, orderID).Scan(&proofID, &bucket, &objectKey, &mimeType)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), 404, 404, "付款凭证不存在")
		return
	}
	if err != nil || bucket != storage.PrivateBucket {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "读取付款凭证失败")
		return
	}
	viewURL, err := h.storage.PresignedGet(r.Context(), bucket, objectKey, 5*time.Minute)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "生成付款凭证查看地址失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(proofID), "view_url": viewURL, "mime_type": mimeType, "expires_in": 300,
	})
}

func (h *Handler) approveBankRechargeProof(w http.ResponseWriter, r *http.Request) {
	h.reviewBankRechargeProof(w, r, true)
}

func (h *Handler) rejectBankRechargeProof(w http.ResponseWriter, r *http.Request) {
	h.reviewBankRechargeProof(w, r, false)
}

func (h *Handler) reviewBankRechargeProof(w http.ResponseWriter, r *http.Request, approve bool) {
	orderID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "充值订单编号无效")
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" || len([]rune(request.Reason)) > 500 {
		httpx.Error(w, httpx.RequestID(r.Context()), 400, 400, "审核说明必填且不能超过500字")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "审核付款凭证失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var orderNo, provider string
	var userID, coinAmount, bonusCoin, proofID int64
	var orderStatus, proofStatus int
	err = tx.QueryRowContext(r.Context(), `
		SELECT recharge.order_no,recharge.user_id,recharge.coin_amount,recharge.bonus_coin,
		       recharge.status,channel.provider,proof.id,proof.status
		FROM recharge_orders recharge
		JOIN payment_channels channel ON channel.id=recharge.channel_id
		JOIN payment_bank_proofs proof ON proof.recharge_order_id=recharge.id
		WHERE recharge.id=? FOR UPDATE`, orderID).Scan(
		&orderNo, &userID, &coinAmount, &bonusCoin, &orderStatus, &provider, &proofID, &proofStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), 404, 404, "待审核付款凭证不存在")
		return
	}
	if err != nil || provider != manualBankProvider || orderStatus != 1 || proofStatus != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "付款凭证已审核或订单状态已变化")
		return
	}
	if !approve {
		if _, err = tx.ExecContext(r.Context(), `UPDATE payment_bank_proofs SET status=2,review_reason=?,reviewed_by=?,reviewed_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status=0`, request.Reason, adminID(r), proofID); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "驳回付款凭证失败")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE recharge_orders SET status=4,closed_at=CURRENT_TIMESTAMP(3),failure_reason=? WHERE id=?`, request.Reason, orderID); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "关闭充值订单失败")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE payment_bank_order_details SET close_reason=? WHERE recharge_order_id=?`, request.Reason, orderID); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "关闭充值订单失败")
			return
		}
		if err = auditAdmin(r.Context(), tx, r, "payment.bank_recharge.reject", "recharge_order", orderID,
			map[string]any{"status": 1, "proof_status": 0},
			map[string]any{"status": 4, "proof_status": 2, "reason": request.Reason}); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录支付审计失败")
			return
		}
		if err = tx.Commit(); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "驳回付款凭证失败")
			return
		}
		httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(orderID), "status": 4, "bank_stage": bankpayment.StageClosed})
		return
	}
	if coinAmount < 0 || bonusCoin < 0 || coinAmount > math.MaxInt64-bonusCoin {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "充值金额异常")
		return
	}
	entry, err := h.wallet.ApplyTx(r.Context(), tx, wallet.ApplyRequest{
		UserID: userID, Amount: coinAmount + bonusCoin, BusinessType: "recharge",
		BusinessID: orderNo, Description: request.Reason,
		Metadata: map[string]any{"recharge_order_id": apiDecimalID(orderID),
			"payment_method": "bank_transfer", "proof_id": apiDecimalID(proofID),
			"confirmed_by": apiDecimalID(adminID(r))},
	})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 409, 409, "充值入账失败："+err.Error())
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE payment_bank_proofs SET status=1,review_reason=?,reviewed_by=?,reviewed_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status=0`, request.Reason, adminID(r), proofID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新付款凭证失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE recharge_orders SET status=2,paid_at=COALESCE(paid_at,CURRENT_TIMESTAMP(3)),failure_reason='' WHERE id=?`, orderID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "更新充值订单失败")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "payment.bank_recharge.approve", "recharge_order", orderID,
		map[string]any{"status": 1, "proof_status": 0},
		map[string]any{"status": 2, "proof_status": 1, "ledger_entry_no": entry.EntryNo,
			"credited_coin": coinAmount + bonusCoin, "reason": request.Reason}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), 500, 500, "确认充值失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(orderID), "status": 2, "bank_stage": bankpayment.StagePaid,
		"ledger": walletEntryForAPI(entry), "message": fmt.Sprintf("已入账 %d 星币", coinAmount+bonusCoin),
	})
}
