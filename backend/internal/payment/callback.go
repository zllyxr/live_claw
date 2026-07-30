package payment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

const callbackOldTokenGrace = 10 * time.Minute

type verifiedChannelToken struct {
	value      string
	verifiedAt time.Time
}

func (s *Service) HandleBEpusdtCallback(
	ctx context.Context,
	raw []byte,
) (CallbackResult, error) {
	// Only extract the local order reference before selecting an API token.
	// Every order retains the encrypted provider config that created it, so a
	// later channel/token rotation cannot invalidate an in-flight callback.
	fields, err := decodeCallbackFields(raw)
	if err != nil {
		return CallbackResult{}, err
	}
	orderID, err := requiredCallbackString(fields, "order_id", 100)
	if err != nil {
		return CallbackResult{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return CallbackResult{}, fmt.Errorf("begin payment callback: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	order, found, err := orderByCallbackTx(ctx, tx, orderID)
	if err != nil {
		return CallbackResult{}, fmt.Errorf("load callback recharge order: %w", err)
	}
	if !found {
		return CallbackResult{}, ErrOrderNotFound
	}
	config, err := s.providerConfigSnapshot(order)
	if err != nil {
		return CallbackResult{}, err
	}
	currentToken, tokenErr := s.currentVerifiedChannelToken(
		ctx, tx, order.ChannelID,
	)
	if tokenErr != nil {
		return CallbackResult{}, tokenErr
	}
	var callback callbackData
	if currentToken.value != "" && currentToken.value != config.APIToken {
		callback, err = decodeBEpusdtCallbackFields(fields, currentToken.value)
		if errors.Is(err, ErrInvalidSignature) &&
			!s.now().After(currentToken.verifiedAt.Add(callbackOldTokenGrace)) {
			callback, err = decodeBEpusdtCallbackFields(fields, config.APIToken)
		}
	} else {
		callback, err = decodeBEpusdtCallbackFields(fields, config.APIToken)
	}
	if err != nil {
		return CallbackResult{}, err
	}
	if order.ProviderOrderNo != "" && order.ProviderOrderNo != callback.TradeID {
		return CallbackResult{}, ErrCallbackConflict
	}
	if order.BlockTransactionID != "" && callback.BlockTransactionID != "" &&
		order.BlockTransactionID != callback.BlockTransactionID {
		return CallbackResult{}, ErrCallbackConflict
	}
	if order.ActualAmount != "" && callback.ActualAmount != "" &&
		order.ActualAmount != callback.ActualAmount {
		return CallbackResult{}, ErrCallbackConflict
	}
	if order.PaymentAddress != "" && callback.PaymentAddress != "" &&
		order.PaymentAddress != callback.PaymentAddress {
		return CallbackResult{}, ErrCallbackConflict
	}
	callbackMinor, err := decimalToMinor(callback.Amount, order.CurrencyScale)
	if err != nil || callbackMinor != order.AmountMinor {
		return CallbackResult{}, ErrCallbackConflict
	}
	if err = claimCallbackBlockTx(ctx, tx, order, callback); err != nil {
		return CallbackResult{}, err
	}

	eventID, err := idgen.New()
	if err != nil {
		return CallbackResult{}, fmt.Errorf("generate payment callback event: %w", err)
	}
	var blockTransaction any
	if callback.BlockTransactionID == "" {
		blockTransaction = nil
	} else {
		blockTransaction = callback.BlockTransactionID
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO payment_callback_events
			(event_id,channel_id,provider,order_no,provider_order_no,
			 block_transaction_id,provider_status,payload_hash,signature_valid,
			 process_status,payload)
		VALUES(?,?,?,?,?,?,?,?,1,0,CAST(? AS JSON))`,
		eventID, order.ChannelID, "bepusdt", order.OrderNo, callback.TradeID,
		blockTransaction, callback.Status, callback.PayloadHash,
		string(callback.SanitizedJSON),
	)
	if err != nil {
		if !isDuplicateKey(err) {
			return CallbackResult{}, fmt.Errorf("record payment callback: %w", err)
		}
		duplicate, duplicateErr := callbackDuplicate(
			ctx, tx, order.ChannelID, callback, order.OrderNo,
		)
		if duplicateErr != nil {
			return CallbackResult{}, duplicateErr
		}
		if !duplicate {
			return CallbackResult{}, ErrCallbackConflict
		}
		if err = tx.Commit(); err != nil {
			return CallbackResult{}, fmt.Errorf("commit duplicate payment callback: %w", err)
		}
		current, currentFound, loadErr := s.orderByNumber(ctx, order.UserID, order.OrderNo)
		if loadErr != nil || !currentFound {
			if loadErr == nil {
				loadErr = ErrOrderNotFound
			}
			return CallbackResult{}, loadErr
		}
		return CallbackResult{
			OrderNo: current.OrderNo, Status: current.Status, Duplicate: true,
		}, nil
	}

	targetStatus := order.Status
	failureReason := order.FailureReason
	closedAt := order.ClosedAt
	switch callback.Status {
	case 1:
		if order.Status == OrderStatusCreated {
			targetStatus = OrderStatusPaying
			failureReason = ""
		}
	case 2:
		if order.CoinAmount < 0 || order.BonusCoin < 0 ||
			order.CoinAmount > math.MaxInt64-order.BonusCoin {
			return CallbackResult{}, ErrCallbackConflict
		}
		_, err = s.wallet.ApplyTx(ctx, tx, wallet.ApplyRequest{
			UserID: order.UserID, Amount: order.CoinAmount + order.BonusCoin,
			BusinessType: "recharge", BusinessID: order.OrderNo,
			Description: "BEpusdt 充值到账",
			Metadata: map[string]any{
				"recharge_order_id":      strconv.FormatInt(order.ID, 10),
				"provider":               "bepusdt",
				"provider_order_no":      callback.TradeID,
				"block_transaction_id":   callback.BlockTransactionID,
				"actual_amount":          callback.ActualAmount,
				"payment_callback_event": eventID,
			},
		})
		if err != nil {
			if errors.Is(err, wallet.ErrIdempotencyReuse) {
				return CallbackResult{}, ErrCallbackConflict
			}
			return CallbackResult{}, fmt.Errorf("credit recharge wallet: %w", err)
		}
		if order.Status != OrderStatusRefunded {
			targetStatus = OrderStatusPaid
		}
		failureReason = ""
	case 3:
		if order.Status == OrderStatusCreated || order.Status == OrderStatusPaying {
			targetStatus = OrderStatusClosed
			failureReason = "provider payment expired"
			now := s.now()
			closedAt = &now
		}
	case 6:
		if order.Status == OrderStatusCreated || order.Status == OrderStatusPaying {
			targetStatus = OrderStatusFailed
			failureReason = "provider payment failed"
			now := s.now()
			closedAt = &now
		}
	default:
		return CallbackResult{}, ErrUnsupportedStatus
	}

	var closedAtValue any
	if closedAt != nil {
		closedAtValue = *closedAt
	}
	paidExpression := "paid_at"
	if callback.Status == 2 && order.Status != OrderStatusRefunded {
		paidExpression = "COALESCE(paid_at,CURRENT_TIMESTAMP(3))"
	}
	updateQuery := `
		UPDATE recharge_orders
		SET provider_order_no=?,actual_amount=IF(?<>'',?,actual_amount),
		    payment_address=IF(?<>'',?,payment_address),
		    block_transaction_id=IF(?<>'',?,block_transaction_id),
		    status=?,paid_at=` + paidExpression + `,
		    closed_at=COALESCE(closed_at,?),
		    callback_count=callback_count+1,last_callback_status=?,
		    last_callback_at=CURRENT_TIMESTAMP(3),callback_payload_hash=?,
		    failure_reason=?,
		    provider_payload=JSON_SET(COALESCE(provider_payload,JSON_OBJECT()),
		                              '$.callback',CAST(? AS JSON))
		WHERE id=?`
	_, err = tx.ExecContext(ctx, updateQuery,
		callback.TradeID,
		callback.ActualAmount, callback.ActualAmount,
		callback.PaymentAddress, callback.PaymentAddress,
		callback.BlockTransactionID, callback.BlockTransactionID,
		targetStatus, closedAtValue, callback.Status, callback.PayloadHash,
		failureReason, string(callback.SanitizedJSON), order.ID,
	)
	if err != nil {
		return CallbackResult{}, fmt.Errorf("update recharge from callback: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE payment_callback_events
		SET process_status=1,processed_at=CURRENT_TIMESTAMP(3)
		WHERE event_id=?`,
		eventID,
	)
	if err != nil {
		return CallbackResult{}, fmt.Errorf("complete payment callback event: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return CallbackResult{}, fmt.Errorf("commit payment callback: %w", err)
	}
	return CallbackResult{OrderNo: order.OrderNo, Status: targetStatus}, nil
}

func (s *Service) currentVerifiedChannelToken(
	ctx context.Context,
	tx *sql.Tx,
	channelID int64,
) (verifiedChannelToken, error) {
	var (
		channelKey, provider, verifiedHash string
		ciphertext                         []byte
		keyVersion                         int
		verifiedAt                         sql.NullTime
	)
	err := tx.QueryRowContext(ctx, `
		SELECT channel_key,provider,config_ciphertext,key_version,
		       config_verified_hash,config_verified_at
		FROM payment_channels
		WHERE id=?
		FOR SHARE`,
		channelID,
	).Scan(
		&channelKey, &provider, &ciphertext, &keyVersion,
		&verifiedHash, &verifiedAt,
	)
	if err != nil {
		return verifiedChannelToken{}, fmt.Errorf("load current payment callback token: %w", err)
	}
	sum := sha256.Sum256(ciphertext)
	if channelKey != USDTChannelKey || provider != "bepusdt" ||
		keyVersion != paymentKeyVersion || len(ciphertext) == 0 ||
		!verifiedAt.Valid ||
		!strings.EqualFold(
			strings.TrimSpace(verifiedHash),
			hex.EncodeToString(sum[:]),
		) {
		return verifiedChannelToken{}, nil
	}
	config, err := s.cipher.Decrypt(channelKey, ciphertext)
	if err != nil {
		return verifiedChannelToken{}, nil
	}
	return verifiedChannelToken{
		value: strings.TrimSpace(config.APIToken), verifiedAt: verifiedAt.Time,
	}, nil
}

func decodeBEpusdtCallback(raw []byte, apiToken string) (callbackData, error) {
	fields, err := decodeCallbackFields(raw)
	if err != nil {
		return callbackData{}, err
	}
	return decodeBEpusdtCallbackFields(fields, apiToken)
}

func decodeBEpusdtCallbackFields(
	fields map[string]any,
	apiToken string,
) (callbackData, error) {
	_, payloadHash, err := verifyFields(fields, apiToken)
	if err != nil {
		return callbackData{}, err
	}
	tradeID, err := requiredCallbackString(fields, "trade_id", 190)
	if err != nil {
		return callbackData{}, err
	}
	orderID, err := requiredCallbackString(fields, "order_id", 100)
	if err != nil {
		return callbackData{}, err
	}
	amount, err := callbackDecimal(fields, "amount", 64, true)
	if err != nil {
		return callbackData{}, err
	}
	actualAmount, err := callbackDecimal(fields, "actual_amount", 64, false)
	if err != nil {
		return callbackData{}, err
	}
	paymentAddress, err := optionalCallbackString(fields, "token", 190)
	if err != nil {
		return callbackData{}, err
	}
	blockTransactionID, err := optionalCallbackString(fields, "block_transaction_id", 190)
	if err != nil {
		return callbackData{}, err
	}
	statusText, include, err := providerString(fields["status"])
	if err != nil || !include {
		return callbackData{}, ErrInvalidCallback
	}
	status, err := strconv.Atoi(statusText)
	if err != nil || (status != 1 && status != 2 && status != 3 && status != 6) {
		return callbackData{}, ErrUnsupportedStatus
	}
	if status == 2 {
		if actualAmount == "" || paymentAddress == "" || blockTransactionID == "" {
			return callbackData{}, ErrInvalidCallback
		}
	}
	sanitized := make(map[string]any, len(fields)-1)
	for key, value := range fields {
		if key != "signature" {
			sanitized[key] = value
		}
	}
	sanitizedJSON, err := json.Marshal(sanitized)
	if err != nil {
		return callbackData{}, ErrInvalidCallback
	}
	return callbackData{
		Fields: fields, SanitizedJSON: sanitizedJSON, PayloadHash: payloadHash,
		TradeID: tradeID, OrderID: orderID, Amount: amount,
		ActualAmount: actualAmount, PaymentAddress: paymentAddress,
		BlockTransactionID: blockTransactionID, Status: status,
	}, nil
}

func callbackDecimal(
	fields map[string]any,
	key string,
	maximum int,
	required bool,
) (string, error) {
	value, ok := fields[key]
	if !ok || value == nil {
		if required {
			return "", ErrInvalidCallback
		}
		return "", nil
	}
	var raw string
	switch typed := value.(type) {
	case string:
		raw = typed
	case json.Number:
		raw = typed.String()
	case float64:
		raw = strconv.FormatFloat(typed, 'g', -1, 64)
	default:
		return "", ErrInvalidCallback
	}
	raw = strings.TrimSpace(raw)
	if raw == "" && !required {
		return "", nil
	}
	normalized, err := normalizePositiveDecimal(raw, maximum)
	if err != nil {
		return "", ErrInvalidCallback
	}
	return normalized, nil
}

func requiredCallbackString(fields map[string]any, key string, maximum int) (string, error) {
	value, err := optionalCallbackString(fields, key, maximum)
	if err != nil || value == "" {
		return "", ErrInvalidCallback
	}
	return value, nil
}

func optionalCallbackString(fields map[string]any, key string, maximum int) (string, error) {
	value, ok := fields[key]
	if !ok || value == nil {
		return "", nil
	}
	typed, ok := value.(string)
	typed = strings.TrimSpace(typed)
	if !ok || len(typed) > maximum {
		return "", ErrInvalidCallback
	}
	return typed, nil
}

func orderByCallbackTx(
	ctx context.Context,
	tx *sql.Tx,
	orderNo string,
) (Order, bool, error) {
	return scanOrder(tx.QueryRowContext(ctx, orderSelect+`
		WHERE recharge.order_no=? AND channel.channel_key='usdt' FOR UPDATE`,
		orderNo,
	))
}

func claimCallbackBlockTx(
	ctx context.Context,
	tx *sql.Tx,
	order Order,
	callback callbackData,
) error {
	if callback.BlockTransactionID == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO payment_callback_block_bindings
			(channel_id,block_transaction_id,order_no,provider_order_no)
		VALUES(?,?,?,?)
		ON DUPLICATE KEY UPDATE
			block_transaction_id=VALUES(block_transaction_id)`,
		order.ChannelID, callback.BlockTransactionID, order.OrderNo, callback.TradeID,
	)
	if err != nil {
		return fmt.Errorf("bind payment block transaction: %w", err)
	}
	var existingOrderNo, existingProviderOrderNo string
	err = tx.QueryRowContext(ctx, `
		SELECT order_no,provider_order_no
		FROM payment_callback_block_bindings
		WHERE channel_id=? AND block_transaction_id=?
		FOR UPDATE`,
		order.ChannelID, callback.BlockTransactionID,
	).Scan(&existingOrderNo, &existingProviderOrderNo)
	if err != nil {
		return fmt.Errorf("load payment block transaction binding: %w", err)
	}
	if existingOrderNo != order.OrderNo || existingProviderOrderNo != callback.TradeID {
		return ErrCallbackConflict
	}
	return nil
}

func callbackDuplicate(
	ctx context.Context,
	tx *sql.Tx,
	channelID int64,
	callback callbackData,
	orderNo string,
) (bool, error) {
	query := `
		SELECT order_no,provider_order_no,provider_status,process_status
		FROM payment_callback_events
		WHERE channel_id=? AND (payload_hash=?`
	arguments := []any{channelID, callback.PayloadHash}
	if callback.BlockTransactionID != "" {
		query += " OR (block_transaction_id=? AND provider_status=?))"
		arguments = append(arguments, callback.BlockTransactionID, callback.Status)
	} else {
		query += ")"
	}
	query += " ORDER BY (payload_hash=?) DESC,id LIMIT 1"
	arguments = append(arguments, callback.PayloadHash)
	var existingOrderNo, providerOrderNo string
	var providerStatus, processStatus int
	err := tx.QueryRowContext(ctx, query, arguments...).Scan(
		&existingOrderNo, &providerOrderNo, &providerStatus, &processStatus,
	)
	if err != nil {
		return false, fmt.Errorf("inspect duplicate payment callback: %w", err)
	}
	return existingOrderNo == orderNo &&
		providerOrderNo == callback.TradeID &&
		providerStatus == callback.Status &&
		processStatus == 1, nil
}

func pointerTimeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
