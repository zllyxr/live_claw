package server

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/payment"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func (s *Server) compatFinance(w http.ResponseWriter, r *http.Request, service string) bool {
	switch service {
	case "Charge.getAliOrder", "Charge.getWxOrder", "Charge.getBraintreePaypalOrder", "Charge.getUsdtOrder":
		s.compatCreateRechargeOrder(w, r, service)
	case "Charge.orderStatus", "Charge.getOrderStatus":
		s.compatRechargeOrderStatus(w, r)
	case "User.getProfit":
		s.compatGetProfit(w, r)
	case "User.getUserAccountList":
		s.compatWithdrawAccounts(w, r)
	case "User.setUserAccount":
		s.compatCreateWithdrawAccount(w, r)
	case "User.delUserAccount":
		s.compatDeleteWithdrawAccount(w, r)
	case "User.setCash":
		s.compatCreateWithdrawal(w, r)
	case "User.seeDailyTasks":
		s.compatDailyTasks(w, r)
	case "User.receiveTaskReward":
		s.compatClaimDailyTask(w, r)
	case "Wallet.ledger", "User.getCoinRecord":
		s.compatWalletLedger(w, r)
	case "Charge.orderList", "Charge.getOrderList":
		s.compatRechargeOrders(w, r)
	case "User.cashOrderList", "User.getCashRecord":
		s.compatWithdrawalOrders(w, r)
	case "Auth.getStatus":
		s.compatVerificationStatus(w, r)
	default:
		return false
	}
	return true
}

func formatMinorAmount(amount int64, scale int) string {
	if scale <= 0 {
		return strconv.FormatInt(amount, 10)
	}
	divisor := int64(1)
	for index := 0; index < scale; index++ {
		divisor *= 10
	}
	return fmt.Sprintf("%d.%0*d", amount/divisor, scale, amount%divisor)
}

func (s *Server) compatCreateRechargeOrder(w http.ResponseWriter, r *http.Request, service string) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	if service == "Charge.getUsdtOrder" {
		s.compatCreateBEpusdtOrder(w, r, userID)
		return
	}
	channelKey := map[string]string{
		"Charge.getAliOrder":             "ali",
		"Charge.getWxOrder":              "wx",
		"Charge.getBraintreePaypalOrder": "paypal",
		"Charge.getUsdtOrder":            "usdt",
	}[service]
	productID := compatInt64(r.FormValue("changeid"))
	var channelID int64
	var channelCurrency string
	var channelScale int
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id,currency,currency_scale FROM payment_channels
		WHERE channel_key=? AND status=1`, channelKey,
	).Scan(&channelID, &channelCurrency, &channelScale)
	if errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 503, "支付通道未启用，请在后台完成配置", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "读取支付通道失败", nil)
		return
	}
	var fiatCurrency string
	var currencyScale int
	var amountMinor, coinAmount, bonusCoin int64
	err = s.db.QueryRowContext(r.Context(), `
		SELECT fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin
		FROM recharge_products WHERE id=? AND status=1`, productID,
	).Scan(&fiatCurrency, &currencyScale, &amountMinor, &coinAmount, &bonusCoin)
	if errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 404, "充值档位不存在", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "读取充值档位失败", nil)
		return
	}
	if fiatCurrency != channelCurrency || currencyScale != channelScale {
		writeCompat(w, 409, "充值档位与支付通道币种不匹配", nil)
		return
	}
	orderNo, err := idgen.New()
	if err != nil {
		writeCompat(w, 500, "创建充值订单失败", nil)
		return
	}
	_, err = s.db.ExecContext(r.Context(), `
		INSERT INTO recharge_orders
			(order_no,user_id,product_id,channel_id,fiat_currency,currency_scale,
			 amount_minor,coin_amount,bonus_coin,status,client_ip,provider_payload)
		VALUES(?,?,?,?,?,?,?,?,?,0,?,JSON_OBJECT('client','uniapp'))`,
		orderNo, userID, productID, channelID, fiatCurrency, currencyScale,
		amountMinor, coinAmount, bonusCoin, requestIP(r),
	)
	if err != nil {
		writeCompat(w, 500, "创建充值订单失败", nil)
		return
	}
	writeCompat(w, 0, "充值订单已创建", map[string]any{
		"orderid": orderNo, "order_no": orderNo, "status": "0",
		"channel": channelKey, "money": formatMinorAmount(amountMinor, currencyScale),
		"coin": coinAmount, "give": bonusCoin,
	})
}

func (s *Server) compatCreateBEpusdtOrder(
	w http.ResponseWriter,
	r *http.Request,
	userID int64,
) {
	if s.payments == nil {
		writeCompat(w, 503, "支付服务未配置", nil)
		return
	}
	traceID := strings.TrimSpace(r.FormValue("client_trace_id"))
	if traceID == "" {
		traceID = strings.TrimSpace(r.FormValue("trace_id"))
	}
	order, err := s.payments.CreateRecharge(r.Context(), payment.CreateRequest{
		UserID: userID, ProductID: compatInt64(r.FormValue("changeid")),
		ClientTraceID: traceID, ClientIP: requestIP(r),
	})
	if err != nil {
		s.writeCompatPaymentError(w, r, "创建 BEpusdt 充值订单", err)
		return
	}
	writeCompat(w, 0, "充值订单已创建", compatPaymentOrder(order))
}

func (s *Server) compatRechargeOrderStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	if s.payments == nil {
		writeCompat(w, 503, "支付服务未配置", nil)
		return
	}
	reference := strings.TrimSpace(r.FormValue("order_no"))
	if reference == "" {
		reference = strings.TrimSpace(r.FormValue("orderid"))
	}
	if reference == "" {
		reference = strings.TrimSpace(r.FormValue("client_trace_id"))
	}
	order, err := s.payments.OrderStatus(r.Context(), userID, reference)
	if err != nil {
		s.writeCompatPaymentError(w, r, "读取 BEpusdt 充值订单", err)
		return
	}
	writeCompat(w, 0, "", compatPaymentOrder(order))
}

func (s *Server) writeCompatPaymentError(
	w http.ResponseWriter,
	r *http.Request,
	operation string,
	err error,
) {
	switch {
	case errors.Is(err, payment.ErrInvalidRequest):
		writeCompat(w, 400, "充值参数无效", nil)
	case errors.Is(err, payment.ErrIdempotencyReuse):
		writeCompat(w, 409, "充值请求标识已用于其他档位", nil)
	case errors.Is(err, payment.ErrProductNotFound):
		writeCompat(w, 404, "充值档位不存在", nil)
	case errors.Is(err, payment.ErrOrderNotFound):
		writeCompat(w, 404, "充值订单不存在", nil)
	case errors.Is(err, payment.ErrChannelDisabled),
		errors.Is(err, payment.ErrChannelNotReady):
		writeCompat(w, 503, "支付通道未启用，请在后台完成配置", nil)
	case errors.Is(err, payment.ErrProvider):
		writeCompat(w, 502, "支付服务暂不可用，请稍后重试", nil)
	default:
		if s.logger != nil {
			s.logger.Error(
				operation,
				"request_id", r.Header.Get("X-Request-ID"),
				"error", err,
			)
		}
		writeCompat(w, 500, "支付服务暂不可用", nil)
	}
}

func compatPaymentOrder(order payment.Order) map[string]any {
	expiresAt := ""
	expiresAtUnix := ""
	if order.ExpiresAt != nil {
		expiresAt = order.ExpiresAt.Format(time.RFC3339)
		expiresAtUnix = strconv.FormatInt(order.ExpiresAt.Unix(), 10)
	}
	paidAt := ""
	if order.PaidAt != nil {
		paidAt = strconv.FormatInt(order.PaidAt.Unix(), 10)
	}
	createdAt := ""
	createdAtUnix := ""
	if !order.CreatedAt.IsZero() {
		createdAt = order.CreatedAt.Format("2006-01-02 15:04:05")
		createdAtUnix = strconv.FormatInt(order.CreatedAt.Unix(), 10)
	}
	lastCallbackAt := ""
	if order.LastCallbackAt != nil {
		lastCallbackAt = strconv.FormatInt(order.LastCallbackAt.Unix(), 10)
	}
	cryptoCurrency, network := paymentAssetMeta(order.TradeType)
	return map[string]any{
		"id":                   strconv.FormatInt(order.ID, 10),
		"orderid":              order.OrderNo,
		"order_no":             order.OrderNo,
		"product_id":           strconv.FormatInt(order.ProductID, 10),
		"client_trace_id":      order.ClientTraceID,
		"provider_order_no":    order.ProviderOrderNo,
		"provider_trade_id":    order.ProviderOrderNo,
		"status":               strconv.Itoa(order.Status),
		"status_text":          rechargeStatusText(order.Status),
		"channel":              payment.USDTChannelKey,
		"currency":             order.FiatCurrency,
		"fiat_currency":        order.FiatCurrency,
		"money":                formatMinorAmount(order.AmountMinor, order.CurrencyScale),
		"amount":               formatMinorAmount(order.AmountMinor, order.CurrencyScale),
		"coin":                 strconv.FormatInt(order.CoinAmount, 10),
		"coin_amount":          strconv.FormatInt(order.CoinAmount, 10),
		"give":                 strconv.FormatInt(order.BonusCoin, 10),
		"bonus_coin":           strconv.FormatInt(order.BonusCoin, 10),
		"payment_url":          order.PaymentURL,
		"payurl":               order.PaymentURL,
		"url":                  order.PaymentURL,
		"actual_amount":        order.ActualAmount,
		"trade_type":           order.TradeType,
		"crypto_currency":      cryptoCurrency,
		"network":              network,
		"payment_address":      order.PaymentAddress,
		"token_address":        order.PaymentAddress,
		"token":                order.PaymentAddress,
		"block_transaction_id": order.BlockTransactionID,
		"expires_at":           expiresAt,
		"expires_at_unix":      expiresAtUnix,
		"expiration_time":      expiresAtUnix,
		"callback_count":       strconv.FormatUint(order.CallbackCount, 10),
		"last_callback_status": strconv.Itoa(order.LastCallbackStatus),
		"last_callback_at":     lastCallbackAt,
		"failure_reason":       order.FailureReason,
		"paid_at":              paidAt,
		"addtime":              createdAtUnix,
		"datetime":             createdAt,
	}
}

func paymentAssetMeta(tradeType string) (string, string) {
	tradeType = strings.ToLower(strings.TrimSpace(tradeType))
	parts := strings.Split(tradeType, ".")
	cryptoCurrency := ""
	if len(parts) > 0 {
		cryptoCurrency = strings.ToUpper(parts[0])
	}
	switch {
	case strings.HasSuffix(tradeType, ".trc20"):
		return cryptoCurrency, "tron"
	case strings.HasSuffix(tradeType, ".erc20"):
		return cryptoCurrency, "ethereum"
	case strings.HasSuffix(tradeType, ".bep20"):
		return cryptoCurrency, "bsc"
	default:
		return cryptoCurrency, ""
	}
}

func (s *Server) compatGetProfit(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	var available, frozen int64
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT COALESCE(available,0),COALESCE(frozen,0)
		FROM wallet_accounts WHERE user_id=? AND currency='COIN'`, userID,
	).Scan(&available, &frozen); err != nil {
		writeCompat(w, 500, "读取提现余额失败", nil)
		return
	}
	var income, expense int64
	_ = s.db.QueryRowContext(r.Context(), `
		SELECT COALESCE(SUM(CASE WHEN delta_available>0 THEN delta_available ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN delta_available<0 THEN -delta_available ELSE 0 END),0)
		FROM wallet_ledger_entries WHERE user_id=?`, userID,
	).Scan(&income, &expense)
	writeCompat(w, 0, "", map[string]any{
		"total": strconv.FormatInt(available, 10), "votes": strconv.FormatInt(available, 10),
		"frozen": strconv.FormatInt(frozen, 10), "income": strconv.FormatInt(income, 10),
		"expense": strconv.FormatInt(expense, 10),
		"tips":    "提现申请提交后由后台审核，审核期间相应星币将被冻结。",
	})
}

func withdrawAccountType(value string) string {
	switch compatInt(value) {
	case 1:
		return "alipay"
	case 2:
		return "wechat"
	case 3:
		return "bank"
	case 4:
		return "usdt"
	default:
		return ""
	}
}

func withdrawAccountTypeNumber(value string) int {
	switch value {
	case "alipay":
		return 1
	case "wechat":
		return 2
	case "bank":
		return 3
	case "usdt":
		return 4
	default:
		return 0
	}
}

func maskAccount(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	if len(runes) <= 8 {
		return string(runes[:2]) + strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-2:])
	}
	return string(runes[:4]) + strings.Repeat("*", len(runes)-8) + string(runes[len(runes)-4:])
}

func (s *Server) compatWithdrawAccounts(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id,account_type,account_masked,bank_name,created_at
		FROM withdraw_accounts
		WHERE user_id=? AND status=1 AND deleted_at IS NULL ORDER BY id DESC`, userID)
	if err != nil {
		writeCompat(w, 500, "提现账户加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]any, 0, 4)
	for rows.Next() {
		var id int64
		var accountType, accountMasked, bankName string
		var createdAt time.Time
		if err = rows.Scan(&id, &accountType, &accountMasked, &bankName, &createdAt); err != nil {
			break
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "type": strconv.Itoa(withdrawAccountTypeNumber(accountType)),
			"account": accountMasked, "account_bank": bankName, "name": "",
			"addtime": createdAt.Unix(),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "提现账户加载失败", nil)
		return
	}
	writeCompatList(w, items)
}

func (s *Server) compatCreateWithdrawAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	accountType := withdrawAccountType(r.FormValue("type"))
	account := boundedCompat(r.FormValue("account"), 500)
	holderName := boundedCompat(r.FormValue("name"), 100)
	bankName := boundedCompat(r.FormValue("account_bank"), 190)
	if accountType == "" || account == "" {
		writeCompat(w, 400, "提现账户资料不完整", nil)
		return
	}
	accountCipher, err := s.encryptSensitive(account)
	if err != nil {
		writeCompat(w, 500, "提现账户加密失败", nil)
		return
	}
	var holderCipher []byte
	if holderName != "" {
		holderCipher, err = s.encryptSensitive(holderName)
		if err != nil {
			writeCompat(w, 500, "提现账户加密失败", nil)
			return
		}
	}
	sum := sha256.Sum256([]byte(strings.ToLower(accountType + ":" + account)))
	result, err := s.db.ExecContext(r.Context(), `
		INSERT INTO withdraw_accounts
			(user_id,account_type,account_ciphertext,account_hash,account_masked,
			 holder_name_ciphertext,bank_name,status)
		VALUES(?,?,?,?,?,?,?,1)`,
		userID, accountType, accountCipher, hex.EncodeToString(sum[:]), maskAccount(account),
		holderCipher, bankName,
	)
	if err != nil {
		if compatIsDuplicate(err) {
			writeCompat(w, 409, "该提现账户已存在", nil)
			return
		}
		writeCompat(w, 500, "保存提现账户失败", nil)
		return
	}
	id, _ := result.LastInsertId()
	writeCompat(w, 0, "账户已保存", map[string]any{
		"id": strconv.FormatInt(id, 10), "type": r.FormValue("type"),
		"account": maskAccount(account), "account_bank": bankName, "name": holderName,
		"addtime": time.Now().Unix(),
	})
}

func (s *Server) compatDeleteWithdrawAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	accountID := compatInt64(r.FormValue("id"))
	var pending int
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM withdraw_orders
		WHERE user_id=? AND account_id=? AND status IN (0,1,2)`, userID, accountID,
	).Scan(&pending); err != nil {
		writeCompat(w, 500, "删除提现账户失败", nil)
		return
	}
	if pending > 0 {
		writeCompat(w, 409, "该账户存在处理中提现，暂不能删除", nil)
		return
	}
	result, err := s.db.ExecContext(r.Context(), `
		UPDATE withdraw_accounts SET status=0,deleted_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND user_id=? AND status=1`, accountID, userID)
	affected, _ := result.RowsAffected()
	if err != nil || affected != 1 {
		writeCompat(w, 404, "提现账户不存在", nil)
		return
	}
	writeCompat(w, 0, "已删除", map[string]string{"deleted": "1"})
}

func (s *Server) compatCreateWithdrawal(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	accountID := compatInt64(r.FormValue("accountid"))
	amount := compatInt64(r.FormValue("cashvote"))
	if accountID < 1 || amount < 1 {
		writeCompat(w, 400, "提现账户或金额无效", nil)
		return
	}
	var masked string
	var accountCipher []byte
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT account_masked,account_ciphertext FROM withdraw_accounts
		WHERE id=? AND user_id=? AND status=1 AND deleted_at IS NULL`, accountID, userID,
	).Scan(&masked, &accountCipher); errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 404, "提现账户不存在", nil)
		return
	} else if err != nil {
		writeCompat(w, 500, "读取提现账户失败", nil)
		return
	}
	orderNo, err := idgen.New()
	if err != nil {
		writeCompat(w, 500, "创建提现申请失败", nil)
		return
	}
	hold, err := s.wallet.PlaceHold(r.Context(), wallet.HoldRequest{
		UserID: userID, Amount: amount, BusinessType: "withdraw",
		BusinessID: orderNo, ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Description: "提现申请冻结", Metadata: map[string]any{"account_id": accountID},
	})
	if errors.Is(err, wallet.ErrInsufficientFunds) {
		writeCompat(w, 400, "可提现余额不足", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "冻结提现金额失败", nil)
		return
	}
	_, err = s.db.ExecContext(r.Context(), `
		INSERT INTO withdraw_orders
			(order_no,user_id,account_id,hold_no,coin_amount,fee_coin,payout_currency,
			 currency_scale,payout_amount_minor,account_snapshot_ciphertext,
			 account_masked,status,requested_ip)
		VALUES(?,?,?,?,?,0,'CNY',2,?,?,?,0,?)`,
		orderNo, userID, accountID, hold.HoldNo, amount, amount, accountCipher, masked, requestIP(r),
	)
	if err != nil {
		_, _ = s.wallet.ReleaseHoldWithContext(r.Context(), wallet.ReleaseRequest{
			HoldNo: hold.HoldNo, Description: "提现申请创建失败，释放冻结金额",
		})
		writeCompat(w, 500, "创建提现申请失败", nil)
		return
	}
	writeCompat(w, 0, "提现申请已提交", map[string]any{
		"orderid": orderNo, "order_no": orderNo, "status": "0",
		"coin": amount, "account": masked,
	})
}

func (s *Server) compatDailyTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	_, _ = s.db.ExecContext(r.Context(), `
		INSERT INTO user_task_progress(user_id,task_id,task_date,progress,completed_at)
		SELECT ?,id,CURRENT_DATE,target_count,CURRENT_TIMESTAMP(3)
		FROM daily_tasks WHERE task_key='daily_login' AND status=1
		ON DUPLICATE KEY UPDATE progress=GREATEST(progress,VALUES(progress)),
			completed_at=COALESCE(completed_at,VALUES(completed_at))`, userID)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT task.id,task.name,task.description,task.reward_coin,task.target_count,
		       COALESCE(progress.progress,0),progress.completed_at,progress.claimed_at
		FROM daily_tasks task
		LEFT JOIN user_task_progress progress
		  ON progress.task_id=task.id AND progress.user_id=? AND progress.task_date=CURRENT_DATE
		WHERE task.status=1 ORDER BY task.id`, userID)
	if err != nil {
		writeCompat(w, 500, "每日任务加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 8)
	for rows.Next() {
		var id, reward, target, progress int64
		var name, description string
		var completedAt, claimedAt sql.NullTime
		if err = rows.Scan(
			&id, &name, &description, &reward, &target, &progress, &completedAt, &claimedAt,
		); err != nil {
			break
		}
		status := 0
		if completedAt.Valid || progress >= target {
			status = 1
		}
		if claimedAt.Valid {
			status = 2
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "title": name, "tip": description,
			"tip_m": fmt.Sprintf("奖励 %d 星币", reward), "reward": reward,
			"progress": progress, "target": target, "status": status, "state": status,
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "每日任务加载失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]any{
		"tip_m": "完成任务后领取奖励，奖励会进入星币余额。",
		"list":  items,
	})
}

func (s *Server) compatClaimDailyTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	taskID := compatInt64(r.FormValue("taskid"))
	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeCompat(w, 500, "领取任务奖励失败", nil)
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var reward, target, progress int64
	var claimedAt sql.NullTime
	err = tx.QueryRowContext(r.Context(), `
		SELECT task.reward_coin,task.target_count,progress.progress,progress.claimed_at
		FROM daily_tasks task
		JOIN user_task_progress progress
		  ON progress.task_id=task.id AND progress.user_id=? AND progress.task_date=CURRENT_DATE
		WHERE task.id=? AND task.status=1 FOR UPDATE`,
		userID, taskID,
	).Scan(&reward, &target, &progress, &claimedAt)
	if errors.Is(err, sql.ErrNoRows) || progress < target {
		writeCompat(w, 409, "任务尚未完成", nil)
		return
	}
	if err != nil {
		writeCompat(w, 500, "领取任务奖励失败", nil)
		return
	}
	if claimedAt.Valid {
		writeCompat(w, 409, "今日奖励已领取", nil)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE user_task_progress SET claimed_at=CURRENT_TIMESTAMP(3)
		WHERE user_id=? AND task_id=? AND task_date=CURRENT_DATE AND claimed_at IS NULL`,
		userID, taskID,
	); err != nil || tx.Commit() != nil {
		writeCompat(w, 500, "领取任务奖励失败", nil)
		return
	}
	entry, err := s.wallet.Apply(r.Context(), wallet.ApplyRequest{
		UserID: userID, Amount: reward, BusinessType: "daily_task",
		BusinessID:  fmt.Sprintf("%d:%d:%s", userID, taskID, time.Now().Format("2006-01-02")),
		Description: "每日任务奖励", Metadata: map[string]any{"task_id": taskID},
	})
	if err != nil {
		// Make the claim retryable if the wallet write failed.
		_, _ = s.db.ExecContext(r.Context(), `
			UPDATE user_task_progress SET claimed_at=NULL
			WHERE user_id=? AND task_id=? AND task_date=CURRENT_DATE`, userID, taskID)
		writeCompat(w, 500, "发放任务奖励失败", nil)
		return
	}
	writeCompat(w, 0, "已领取奖励", map[string]any{
		"reward": reward, "coin": entry.Available, "status": "2",
	})
}

func (s *Server) compatWalletLedger(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	limit, offset := compatPage(r.FormValue("p"))
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id,entry_no,delta_available,delta_frozen,balance_available,balance_frozen,
		       business_type,business_id,direction,game_code,venue_code,table_no,round_no,
		       description,created_at
		FROM wallet_ledger_entries
		WHERE user_id=?
		ORDER BY created_at DESC,id DESC
		LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		writeCompat(w, 500, "资金流水加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var (
			id, deltaAvailable, deltaFrozen, balanceAvailable, balanceFrozen int64
			tableNo, direction                                               int
			entryNo, businessType, businessID, gameCode, venueCode           string
			roundNo, description                                             string
			createdAt                                                        time.Time
		)
		if err = rows.Scan(
			&id, &entryNo, &deltaAvailable, &deltaFrozen, &balanceAvailable, &balanceFrozen,
			&businessType, &businessID, &direction, &gameCode, &venueCode, &tableNo,
			&roundNo, &description, &createdAt,
		); err != nil {
			break
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "entry_no": entryNo,
			"amount":            strconv.FormatInt(deltaAvailable, 10),
			"delta_available":   strconv.FormatInt(deltaAvailable, 10),
			"delta_frozen":      strconv.FormatInt(deltaFrozen, 10),
			"balance":           strconv.FormatInt(balanceAvailable, 10),
			"balance_available": strconv.FormatInt(balanceAvailable, 10),
			"balance_frozen":    strconv.FormatInt(balanceFrozen, 10),
			"business_type":     businessType, "business_id": businessID,
			"direction": strconv.Itoa(direction), "game_code": gameCode,
			"venue_code": venueCode, "table_no": strconv.Itoa(tableNo), "round_no": roundNo,
			"description": description, "title": description,
			"addtime":  strconv.FormatInt(createdAt.Unix(), 10),
			"datetime": createdAt.Format("2006-01-02 15:04:05"),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "资金流水加载失败", nil)
		return
	}
	var total int64
	_ = s.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM wallet_ledger_entries WHERE user_id=?`, userID,
	).Scan(&total)
	writeCompat(w, 0, "", map[string]any{
		"list": items, "items": items, "total": strconv.FormatInt(total, 10),
		"page": r.FormValue("p"),
	})
}

func rechargeStatusText(status int) string {
	switch status {
	case 0:
		return "待支付"
	case 1:
		return "支付中"
	case 2:
		return "已到账"
	case 3:
		return "支付失败"
	case 4:
		return "已关闭"
	case 5:
		return "已退款"
	default:
		return "未知状态"
	}
}

func (s *Server) compatRechargeOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	limit, offset := compatPage(r.FormValue("p"))
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT recharge.id,recharge.order_no,recharge.fiat_currency,recharge.currency_scale,
		       recharge.amount_minor,recharge.coin_amount,recharge.bonus_coin,recharge.status,
		       channel.channel_key,channel.name,recharge.client_trace_id,
		       recharge.provider_order_no,
		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(
		           recharge.provider_payload,'$.create.trade_type')),''),
		       recharge.payment_url,recharge.actual_amount,
		       recharge.payment_address,recharge.block_transaction_id,recharge.expires_at,
		       recharge.callback_count,recharge.last_callback_status,recharge.last_callback_at,
		       recharge.failure_reason,recharge.paid_at,recharge.created_at
		FROM recharge_orders recharge
		LEFT JOIN payment_channels channel ON channel.id=recharge.channel_id
		WHERE recharge.user_id=?
		ORDER BY recharge.created_at DESC,recharge.id DESC
		LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		writeCompat(w, 500, "充值记录加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, amountMinor, coinAmount, bonusCoin int64
		var orderNo, currency string
		var currencyScale, status, lastCallbackStatus int
		var callbackCount uint64
		var channelKey, channelName, clientTraceID, providerOrderNo sql.NullString
		var tradeType, paymentURL, actualAmount, paymentAddress, blockTransactionID string
		var failureReason string
		var expiresAt, lastCallbackAt, paidAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &currency, &currencyScale, &amountMinor, &coinAmount,
			&bonusCoin, &status, &channelKey, &channelName, &clientTraceID,
			&providerOrderNo, &tradeType, &paymentURL, &actualAmount, &paymentAddress,
			&blockTransactionID, &expiresAt, &callbackCount, &lastCallbackStatus,
			&lastCallbackAt, &failureReason, &paidAt, &createdAt,
		); err != nil {
			break
		}
		cryptoCurrency, network := paymentAssetMeta(tradeType)
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "orderid": orderNo, "order_no": orderNo,
			"client_trace_id":   scanNullableString(clientTraceID),
			"provider_order_no": scanNullableString(providerOrderNo),
			"provider_trade_id": scanNullableString(providerOrderNo),
			"currency":          currency, "money": formatMinorAmount(amountMinor, currencyScale),
			"fiat_currency": currency, "amount": formatMinorAmount(amountMinor, currencyScale),
			"coin":        strconv.FormatInt(coinAmount, 10),
			"coin_amount": strconv.FormatInt(coinAmount, 10),
			"give":        strconv.FormatInt(bonusCoin, 10),
			"bonus_coin":  strconv.FormatInt(bonusCoin, 10),
			"status":      strconv.Itoa(status), "status_text": rechargeStatusText(status),
			"channel":      scanNullableString(channelKey),
			"channel_name": scanNullableString(channelName),
			"payment_url":  paymentURL, "payurl": paymentURL, "url": paymentURL,
			"actual_amount": actualAmount, "payment_address": paymentAddress,
			"token_address": paymentAddress, "token": paymentAddress,
			"trade_type": tradeType, "crypto_currency": cryptoCurrency, "network": network,
			"block_transaction_id": blockTransactionID,
			"expires_at":           compatNullableUnix(expiresAt),
			"expiration_time":      compatNullableUnix(expiresAt),
			"callback_count":       strconv.FormatUint(callbackCount, 10),
			"last_callback_status": strconv.Itoa(lastCallbackStatus),
			"last_callback_at":     compatNullableUnix(lastCallbackAt),
			"failure_reason":       failureReason,
			"addtime":              strconv.FormatInt(createdAt.Unix(), 10),
			"datetime":             createdAt.Format("2006-01-02 15:04:05"),
			"paid_at":              compatNullableUnix(paidAt),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "充值记录加载失败", nil)
		return
	}
	var total int64
	_ = s.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM recharge_orders WHERE user_id=?`, userID,
	).Scan(&total)
	writeCompat(w, 0, "", map[string]any{
		"list": items, "items": items, "total": strconv.FormatInt(total, 10),
	})
}

func withdrawalStatusText(status int) string {
	switch status {
	case 0:
		return "待审核"
	case 1:
		return "审核通过"
	case 2:
		return "打款中"
	case 3:
		return "已到账"
	case 4:
		return "已拒绝"
	case 5:
		return "已取消"
	case 6:
		return "打款失败"
	default:
		return "未知状态"
	}
}

func (s *Server) compatWithdrawalOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	limit, offset := compatPage(r.FormValue("p"))
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id,order_no,coin_amount,fee_coin,payout_currency,currency_scale,
		       payout_amount_minor,account_masked,status,reject_reason,reviewed_at,paid_at,created_at
		FROM withdraw_orders
		WHERE user_id=?
		ORDER BY created_at DESC,id DESC
		LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		writeCompat(w, 500, "提现记录加载失败", nil)
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, coinAmount, feeCoin, payoutMinor int64
		var orderNo, currency, accountMasked, rejectReason string
		var currencyScale, status int
		var reviewedAt, paidAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &coinAmount, &feeCoin, &currency, &currencyScale,
			&payoutMinor, &accountMasked, &status, &rejectReason,
			&reviewedAt, &paidAt, &createdAt,
		); err != nil {
			break
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "orderid": orderNo, "order_no": orderNo,
			"coin": strconv.FormatInt(coinAmount, 10),
			"fee":  strconv.FormatInt(feeCoin, 10), "currency": currency,
			"money":   formatMinorAmount(payoutMinor, currencyScale),
			"account": accountMasked, "status": strconv.Itoa(status),
			"status_text": withdrawalStatusText(status), "reject_reason": rejectReason,
			"addtime":     strconv.FormatInt(createdAt.Unix(), 10),
			"datetime":    createdAt.Format("2006-01-02 15:04:05"),
			"reviewed_at": compatNullableUnix(reviewedAt), "paid_at": compatNullableUnix(paidAt),
		})
	}
	if err != nil || rows.Err() != nil {
		writeCompat(w, 500, "提现记录加载失败", nil)
		return
	}
	var total int64
	_ = s.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM withdraw_orders WHERE user_id=?`, userID,
	).Scan(&total)
	writeCompat(w, 0, "", map[string]any{
		"list": items, "items": items, "total": strconv.FormatInt(total, 10),
	})
}

func verificationStatusText(status int) string {
	switch status {
	case 0:
		return "审核中"
	case 1:
		return "已认证"
	case 2:
		return "认证未通过"
	default:
		return "未认证"
	}
}

func (s *Server) compatVerificationStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireCompatUser(w, r)
	if !ok {
		return
	}
	var id int64
	var status int
	var rejectReason string
	var reviewedAt sql.NullTime
	var createdAt time.Time
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id,status,reject_reason,reviewed_at,created_at
		FROM user_verifications WHERE user_id=?`, userID,
	).Scan(&id, &status, &rejectReason, &reviewedAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeCompat(w, 0, "", map[string]any{
			"status": "-1", "status_text": "未认证", "verified": "0",
		})
		return
	}
	if err != nil {
		writeCompat(w, 500, "认证状态加载失败", nil)
		return
	}
	writeCompat(w, 0, "", map[string]any{
		"id": strconv.FormatInt(id, 10), "status": strconv.Itoa(status),
		"status_text": verificationStatusText(status),
		"verified":    boolString(status == 1), "reject_reason": rejectReason,
		"addtime":     strconv.FormatInt(createdAt.Unix(), 10),
		"datetime":    createdAt.Format("2006-01-02 15:04:05"),
		"reviewed_at": compatNullableUnix(reviewedAt),
	})
}

func compatNullableUnix(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return strconv.FormatInt(value.Time.Unix(), 10)
}
