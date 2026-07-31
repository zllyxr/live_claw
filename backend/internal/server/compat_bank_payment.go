package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/bankpayment"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/storage"
)

const (
	bankChannelKey             = "bank"
	manualBankProvider         = "manual_bank"
	bankAssignmentWait         = 10 * time.Minute
	maximumBankProofUpload     = 10 << 20
	bankProofMultipartOverhead = 2 << 20
)

func (s *Server) compatCreateBankRechargeOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	traceID := strings.TrimSpace(r.FormValue("client_trace_id"))
	productID := compatInt64(r.FormValue("changeid"))
	if productID < 1 || !validBankClientTraceID(traceID) {
		writeCompat(w, 400, "充值参数无效", nil)
		return
	}
	if existing, found, err := s.compatBankRechargeByReference(r.Context(), userID, traceID); err != nil {
		writeCompat(w, 500, "读取银行卡充值订单失败", nil)
		return
	} else if found {
		if existingProductID, _ := strconv.ParseInt(stringValue(existing["product_id"]), 10, 64); existingProductID != productID {
			writeCompat(w, 409, "充值请求标识已用于其他档位", nil)
			return
		}
		writeCompat(w, 0, "充值订单已创建", existing)
		return
	}

	var channelID int64
	var channelCurrency string
	var channelScale, channelStatus int
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id,currency,currency_scale,status
		FROM payment_channels WHERE channel_key='bank' AND provider='manual_bank'`,
	).Scan(&channelID, &channelCurrency, &channelScale, &channelStatus)
	if errors.Is(err, sql.ErrNoRows) || channelStatus != 1 {
		writeCompat(w, 503, "银行卡支付通道未启用", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "读取银行卡支付通道失败", nil)
		return
	}
	var activeAccounts int
	if err = s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_bank_accounts WHERE status=1`).Scan(&activeAccounts); err != nil {
		writeCompat(w, 500, "读取收款银行卡失败", nil)
		return
	}
	if activeAccounts == 0 {
		writeCompat(w, 503, "当前没有可用的收款银行卡", nil)
		return
	}
	var productName, fiatCurrency string
	var currencyScale int
	var amountMinor, coinAmount, bonusCoin int64
	err = s.db.QueryRowContext(r.Context(), `
		SELECT name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin
		FROM recharge_products WHERE id=? AND status=1`, productID).Scan(
		&productName, &fiatCurrency, &currencyScale, &amountMinor, &coinAmount, &bonusCoin,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 404, "充值档位不存在", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "读取充值档位失败", nil)
		return
	}
	if fiatCurrency != channelCurrency || currencyScale != channelScale || amountMinor < 1 {
		writeCompat(w, 409, "充值档位与银行卡通道币种不匹配", nil)
		return
	}
	orderNo, err := idgen.New()
	if err != nil {
		writeCompat(w, 500, "创建充值订单失败", nil)
		return
	}
	expiresAt := time.Now().Add(bankAssignmentWait)
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeCompat(w, 500, "创建充值订单失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO recharge_orders
			(order_no,user_id,product_id,channel_id,client_trace_id,product_name_snapshot,
			 fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,
			 client_ip,provider_payload,expires_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,0,?,JSON_OBJECT('client','uniapp','payment_method','bank_transfer'),?)`,
		orderNo, userID, productID, channelID, traceID, strings.TrimSpace(productName),
		fiatCurrency, currencyScale, amountMinor, coinAmount, bonusCoin, requestIP(r), expiresAt,
	)
	if err != nil {
		if compatIsDuplicate(err) {
			if existing, found, findErr := s.compatBankRechargeByReference(r.Context(), userID, traceID); findErr == nil && found {
				writeCompat(w, 0, "充值订单已创建", existing)
				return
			}
		}
		writeCompat(w, 500, "创建充值订单失败", nil)
		return
	}
	orderID, _ := result.LastInsertId()
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO payment_bank_order_details(recharge_order_id) VALUES(?)`, orderID); err != nil {
		writeCompat(w, 500, "创建银行卡订单失败", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		writeCompat(w, 500, "创建充值订单失败", nil)
		return
	}
	order, found, err := s.compatBankRechargeByReference(r.Context(), userID, orderNo)
	if err != nil || !found {
		writeCompat(w, 500, "读取充值订单失败", nil)
		return
	}
	writeCompat(w, 0, "充值订单已创建", order)
}

func (s *Server) compatBankRechargeByReference(
	ctx context.Context,
	userID int64,
	reference string,
) (map[string]any, bool, error) {
	reference = strings.TrimSpace(reference)
	if userID < 1 || reference == "" || len(reference) > 100 {
		return nil, false, nil
	}
	if err := s.expireBankRecharges(ctx, userID); err != nil {
		return nil, false, err
	}
	var (
		id, productID, amountMinor, coinAmount, bonusCoin, accountID int64
		orderNo, traceID, fiat, channelName, provider                string
		status, scale, snapshotVersion                               int
		snapshotCiphertext                                           []byte
		assignedAt, expiresAt, paidAt, closedAt                      sql.NullTime
		proofID, proofStatus                                         sql.NullInt64
		proofReason, failureReason, closeReason                      string
		createdAt, updatedAt                                         time.Time
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT recharge.id,recharge.order_no,recharge.product_id,
		       COALESCE(recharge.client_trace_id,''),recharge.fiat_currency,
		       recharge.currency_scale,recharge.amount_minor,recharge.coin_amount,
		       recharge.bonus_coin,recharge.status,recharge.expires_at,recharge.paid_at,
		       recharge.closed_at,recharge.failure_reason,recharge.created_at,recharge.updated_at,
		       channel.name,channel.provider,detail.bank_account_id,
		       detail.account_snapshot_ciphertext,detail.snapshot_key_version,
		       detail.assigned_at,detail.close_reason,proof.id,proof.status,
		       COALESCE(proof.review_reason,'')
		FROM recharge_orders recharge
		JOIN payment_channels channel ON channel.id=recharge.channel_id AND channel.channel_key='bank'
		JOIN payment_bank_order_details detail ON detail.recharge_order_id=recharge.id
		LEFT JOIN payment_bank_proofs proof ON proof.recharge_order_id=recharge.id
		WHERE recharge.user_id=? AND (recharge.order_no=? OR recharge.client_trace_id=?)`,
		userID, reference, reference,
	).Scan(
		&id, &orderNo, &productID, &traceID, &fiat, &scale, &amountMinor, &coinAmount,
		&bonusCoin, &status, &expiresAt, &paidAt, &closedAt, &failureReason, &createdAt,
		&updatedAt, &channelName, &provider, &accountID, &snapshotCiphertext,
		&snapshotVersion, &assignedAt, &closeReason, &proofID, &proofStatus, &proofReason,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	hasPendingProof := proofID.Valid && proofStatus.Int64 == 0
	stage := bankpayment.Stage(status, accountID > 0, hasPendingProof)
	statusText := map[string]string{
		bankpayment.StageWaitingAssignment: "等待分配银行卡",
		bankpayment.StageAwaitingPayment:   "等待付款",
		bankpayment.StageReviewPending:     "等待后台审核",
		bankpayment.StagePaid:              "已到账",
		bankpayment.StageClosed:            "已关闭",
	}[stage]
	result := map[string]any{
		"id": strconv.FormatInt(id, 10), "orderid": orderNo, "order_no": orderNo,
		"product_id": strconv.FormatInt(productID, 10), "client_trace_id": traceID,
		"channel": bankChannelKey, "channel_name": channelName, "provider": provider,
		"payment_method": "bank_transfer", "fiat_currency": fiat, "currency": fiat,
		"money": formatMinorAmount(amountMinor, scale), "amount": formatMinorAmount(amountMinor, scale),
		"amount_minor": strconv.FormatInt(amountMinor, 10), "currency_scale": strconv.Itoa(scale),
		"coin": strconv.FormatInt(coinAmount, 10), "coin_amount": strconv.FormatInt(coinAmount, 10),
		"give": strconv.FormatInt(bonusCoin, 10), "bonus_coin": strconv.FormatInt(bonusCoin, 10),
		"status": strconv.Itoa(status), "status_text": statusText, "bank_stage": stage,
		"bank_account_id": strconv.FormatInt(accountID, 10),
		"proof_id":        nullableInt64String(proofID), "proof_status": nullableInt64String(proofStatus),
		"proof_review_reason": proofReason, "failure_reason": failureReason,
		"close_reason": closeReason, "assigned_at": compatNullableUnix(assignedAt),
		"expires_at": compatNullableUnix(expiresAt), "expiration_time": compatNullableUnix(expiresAt),
		"paid_at": compatNullableUnix(paidAt), "closed_at": compatNullableUnix(closedAt),
		"addtime": strconv.FormatInt(createdAt.Unix(), 10), "created_at": strconv.FormatInt(createdAt.Unix(), 10),
		"updated_at": strconv.FormatInt(updatedAt.Unix(), 10), "payment_url": "", "payurl": "", "url": "",
	}
	if accountID > 0 {
		if s.bankCipher == nil || snapshotVersion != bankpayment.KeyVersion {
			return nil, false, errors.New("bank account snapshot is unavailable")
		}
		snapshot, decryptErr := s.bankCipher.DecryptSnapshot(orderNo, snapshotCiphertext)
		if decryptErr != nil {
			return nil, false, decryptErr
		}
		result["bank_account"] = map[string]any{
			"display_name": snapshot.DisplayName, "bank_name": snapshot.BankName,
			"branch_name": snapshot.BranchName, "holder_name": snapshot.HolderName,
			"card_number": snapshot.CardNumber, "card_number_masked": bankpayment.MaskCardNumber(snapshot.CardNumber),
			"instructions": snapshot.Instructions,
		}
	}
	return result, true, nil
}

func (s *Server) expireBankRecharges(ctx context.Context, userID int64) error {
	arguments := []any{}
	userFilter := ""
	if userID > 0 {
		userFilter = " AND recharge.user_id=?"
		arguments = append(arguments, userID)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE recharge_orders recharge
		JOIN payment_channels channel ON channel.id=recharge.channel_id AND channel.provider='manual_bank'
		LEFT JOIN payment_bank_proofs proof ON proof.recharge_order_id=recharge.id
		SET recharge.status=4,recharge.closed_at=COALESCE(recharge.closed_at,CURRENT_TIMESTAMP(3)),
		    recharge.failure_reason='银行卡订单已超时'
		WHERE recharge.status IN (0,1) AND recharge.expires_at<=CURRENT_TIMESTAMP(3)
		  AND proof.id IS NULL`+userFilter, arguments...)
	return err
}

func (s *Server) compatSubmitBankPaymentProof(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.storage == nil {
		writeCompat(w, 405, "上传方式不受支持", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximumBankProofUpload+bankProofMultipartOverhead)
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeCompat(w, 400, "付款凭证无效或超过10MB", nil)
		return
	}
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	orderNo := strings.TrimSpace(r.FormValue("order_no"))
	if orderNo == "" || len(orderNo) > 100 {
		writeCompat(w, 400, "充值订单号无效", nil)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeCompat(w, 400, "请选择付款凭证图片", nil)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximumBankProofUpload+1))
	if err != nil || len(data) == 0 || len(data) > maximumBankProofUpload {
		writeCompat(w, 400, "付款凭证无效或超过10MB", nil)
		return
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data[:min(len(data), 512)]), ";")[0]))
	extension := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mimeType]
	if extension == "" {
		writeCompat(w, 400, "付款凭证仅支持 JPEG、PNG 或 WebP 图片", nil)
		return
	}
	if err = s.expireBankRecharges(r.Context(), userID); err != nil {
		writeCompat(w, 500, "检查银行卡订单失败", nil)
		return
	}
	var orderID int64
	var status int
	var expiresAt sql.NullTime
	var accountID int64
	err = s.db.QueryRowContext(r.Context(), `
		SELECT recharge.id,recharge.status,recharge.expires_at,detail.bank_account_id
		FROM recharge_orders recharge
		JOIN payment_channels channel ON channel.id=recharge.channel_id AND channel.provider='manual_bank'
		JOIN payment_bank_order_details detail ON detail.recharge_order_id=recharge.id
		WHERE recharge.user_id=? AND recharge.order_no=?`, userID, orderNo).Scan(
		&orderID, &status, &expiresAt, &accountID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 404, "银行卡充值订单不存在", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "读取银行卡订单失败", nil)
		return
	}
	if status != 1 || accountID < 1 || !expiresAt.Valid || !expiresAt.Time.After(time.Now()) {
		writeCompat(w, 409, "订单未分配、已超时或不能提交凭证", nil)
		return
	}
	objectID, err := idgen.New()
	if err != nil {
		writeCompat(w, 500, "生成付款凭证编号失败", nil)
		return
	}
	now := time.Now()
	objectKey := "bank-proofs/" + strconv.FormatInt(userID, 10) + "/" + now.Format("2006/01") + "/" + strings.ToLower(objectID) + extension
	if err = s.storage.PutObject(r.Context(), storage.PrivateBucket, objectKey,
		bytes.NewReader(data), int64(len(data)), mimeType); err != nil {
		writeCompat(w, 500, "保存付款凭证失败", nil)
		return
	}
	removeObject := true
	defer func() {
		if removeObject {
			_ = s.storage.RemoveObject(context.Background(), storage.PrivateBucket, objectKey)
		}
	}()
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeCompat(w, 500, "提交付款凭证失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	err = tx.QueryRowContext(r.Context(), `
		SELECT recharge.status,recharge.expires_at,detail.bank_account_id
		FROM recharge_orders recharge
		JOIN payment_bank_order_details detail ON detail.recharge_order_id=recharge.id
		WHERE recharge.id=? AND recharge.user_id=? FOR UPDATE`, orderID, userID).Scan(&status, &expiresAt, &accountID)
	if err != nil || status != 1 || accountID < 1 || !expiresAt.Valid || !expiresAt.Time.After(time.Now()) {
		writeCompat(w, 409, "订单状态已变化，请刷新后重试", nil)
		return
	}
	var proofCount int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payment_bank_proofs WHERE recharge_order_id=?`, orderID).Scan(&proofCount); err != nil {
		writeCompat(w, 500, "读取付款凭证失败", nil)
		return
	}
	if proofCount != 0 {
		writeCompat(w, 409, "该订单已经提交过付款凭证", nil)
		return
	}
	digest := sha256.Sum256(data)
	assetResult, err := tx.ExecContext(r.Context(), `
		INSERT INTO media_assets
			(owner_user_id,bucket,object_key,media_type,mime_type,size_bytes,sha256,status)
		VALUES(?,?,?,?,?,?,?,1)`, userID, storage.PrivateBucket, objectKey, "image", mimeType,
		len(data), hex.EncodeToString(digest[:]))
	if err != nil {
		writeCompat(w, 500, "记录付款凭证失败", nil)
		return
	}
	assetID, _ := assetResult.LastInsertId()
	proofResult, err := tx.ExecContext(r.Context(), `
		INSERT INTO payment_bank_proofs(recharge_order_id,user_id,asset_id,status)
		VALUES(?,?,?,0)`, orderID, userID, assetID)
	if err != nil {
		writeCompat(w, 409, "该订单已经提交过付款凭证", nil)
		return
	}
	proofID, _ := proofResult.LastInsertId()
	if err = tx.Commit(); err != nil {
		writeCompat(w, 500, "提交付款凭证失败", nil)
		return
	}
	removeObject = false
	writeCompat(w, 0, "付款凭证已提交，请等待后台审核", map[string]any{
		"id": strconv.FormatInt(proofID, 10), "order_no": orderNo,
		"status": "1", "bank_stage": bankpayment.StageReviewPending,
	})
}

func validBankClientTraceID(value string) bool {
	if len(value) < 8 || len(value) > 100 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func nullableInt64String(value sql.NullInt64) string {
	if !value.Valid {
		return ""
	}
	return strconv.FormatInt(value.Int64, 10)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return ""
	}
}
