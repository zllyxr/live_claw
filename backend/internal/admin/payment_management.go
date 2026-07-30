package admin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/payment"
	"github.com/zllyxr/live_claw/backend/internal/paymentconfig"
)

const (
	paymentProviderBEpusdt = "bepusdt"
	paymentKeyVersion      = 1
)

var paymentCurrencyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{2,7}$`)

type paymentChannelUpdateRequest struct {
	Name           string `json:"name"`
	APIBaseURL     string `json:"api_base_url"`
	PublicBaseURL  string `json:"public_base_url"`
	APIToken       string `json:"api_token"`
	TradeType      string `json:"trade_type"`
	Fiat           string `json:"fiat"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	CurrencyScale  int    `json:"currency_scale"`
	MinAmountMinor int64  `json:"min_amount_minor"`
	MaxAmountMinor int64  `json:"max_amount_minor"`
	SortOrder      int    `json:"sort_order"`
	Status         int    `json:"status"`
}

type rechargeProductWriteRequest struct {
	Name          string `json:"name"`
	FiatCurrency  string `json:"fiat_currency"`
	CurrencyScale int    `json:"currency_scale"`
	AmountMinor   int64  `json:"amount_minor"`
	CoinAmount    int64  `json:"coin_amount"`
	BonusCoin     int64  `json:"bonus_coin"`
	Status        int    `json:"status"`
	SortOrder     int    `json:"sort_order"`
}

type paymentInfoProtocolResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

type bepusdtPaidOrderExpectation struct {
	TradeID        string
	OrderID        string
	Fiat           string
	TradeType      string
	AmountMinor    int64
	CurrencyScale  int
	ActualAmount   string
	PaymentAddress string
}

var (
	errBEpusdtVerificationUnavailable = errors.New("BEpusdt verification unavailable")
	errBEpusdtPaymentNotConfirmed     = errors.New("BEpusdt payment is not confirmed")
)

func (h *Handler) listPaymentChannels(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		if parsedStatus, err := strconv.Atoi(rawStatus); err == nil {
			status = parsedStatus
		}
	}
	filterArguments := []any{keyword, like, like, like, like, status, status}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM payment_channels channel
		WHERE (?='' OR channel.channel_key LIKE ? OR channel.name LIKE ?
		       OR channel.provider LIKE ? OR channel.currency LIKE ?)
		  AND (? < 0 OR channel.status=?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取支付通道失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT channel.id,channel.channel_key,channel.name,channel.provider,
		       channel.currency,channel.currency_scale,channel.min_amount_minor,
		       channel.max_amount_minor,channel.config_ciphertext,channel.key_version,
		       channel.config_verified_hash,channel.config_verified_at,
		       channel.status,channel.sort_order,channel.created_at,channel.updated_at
		FROM payment_channels channel
		WHERE (?='' OR channel.channel_key LIKE ? OR channel.name LIKE ?
		       OR channel.provider LIKE ? OR channel.currency LIKE ?)
		  AND (? < 0 OR channel.status=?)
		ORDER BY channel.sort_order DESC,channel.id
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取支付通道失败")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var (
			id, minAmountMinor, maxAmountMinor int64
			channelKey, name, provider, fiat   string
			currencyScale, keyVersion          int
			status, sortOrder                  int
			ciphertext                         []byte
			verifiedHash                       string
			verifiedAt                         sql.NullTime
			createdAt, updatedAt               time.Time
		)
		if err = rows.Scan(
			&id, &channelKey, &name, &provider, &fiat, &currencyScale,
			&minAmountMinor, &maxAmountMinor, &ciphertext, &keyVersion,
			&verifiedHash, &verifiedAt, &status, &sortOrder, &createdAt, &updatedAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取支付通道失败")
			return
		}
		item := map[string]any{
			"id": apiDecimalID(id), "channel_key": channelKey, "name": name,
			"provider": provider, "fiat": fiat, "currency_scale": currencyScale,
			"min_amount_minor": minAmountMinor, "max_amount_minor": maxAmountMinor,
			"key_version": keyVersion, "status": status, "sort_order": sortOrder,
			"created_at": createdAt.Unix(), "updated_at": updatedAt.Unix(),
			"config_verified":    paymentConfigVerified(ciphertext, verifiedHash, verifiedAt),
			"config_verified_at": nullTime(verifiedAt),
		}
		for key, value := range paymentConfigAPIData(h.paymentCipher, channelKey, ciphertext) {
			item[key] = value
		}
		if item["config_valid"] == true {
			configFiat, _ := item["fiat"].(string)
			expectedScale, supported := paymentFiatScale(configFiat)
			if !supported || configFiat != fiat || currencyScale != expectedScale {
				item["config_valid"] = false
				item["config_error"] = "法币或法币精度与通道配置不一致"
			}
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取支付通道失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) updatePaymentChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "支付通道编号无效")
		return
	}
	var request paymentChannelUpdateRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.normalize()
	if err = request.validate(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "支付通道参数无效："+err.Error())
		return
	}
	if h.paymentCipher == nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "支付配置加密服务不可用")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新支付通道失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var (
		channelKey, previousName, provider, previousFiat string
		previousScale, previousStatus, previousSort      int
		previousMin, previousMax                         int64
		previousCiphertext                               []byte
		previousVerifiedHash                             string
		previousVerifiedAt                               sql.NullTime
	)
	err = tx.QueryRowContext(r.Context(), `
		SELECT channel_key,name,provider,currency,currency_scale,min_amount_minor,
		       max_amount_minor,config_ciphertext,config_verified_hash,
		       config_verified_at,status,sort_order
		FROM payment_channels WHERE id=? FOR UPDATE`,
		channelID,
	).Scan(
		&channelKey, &previousName, &provider, &previousFiat, &previousScale,
		&previousMin, &previousMax, &previousCiphertext, &previousVerifiedHash,
		&previousVerifiedAt, &previousStatus, &previousSort,
	)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "支付通道不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取支付通道失败")
		return
	}
	if provider != paymentProviderBEpusdt {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "当前仅支持配置 BEpusdt 通道")
		return
	}

	config := paymentconfig.ChannelConfig{
		APIBaseURL: request.APIBaseURL, PublicBaseURL: request.PublicBaseURL,
		APIToken: request.APIToken, TradeType: request.TradeType,
		Fiat: request.Fiat, TimeoutSeconds: request.TimeoutSeconds,
	}
	previousConfig, previousConfigErr := h.paymentCipher.Decrypt(channelKey, previousCiphertext)
	if config.APIToken == "" {
		if len(previousCiphertext) == 0 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "首次配置必须填写 API Token")
			return
		}
		if previousConfigErr != nil || strings.TrimSpace(previousConfig.APIToken) == "" {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "原 Token 无法读取，请重新填写")
			return
		}
		config.APIToken = previousConfig.APIToken
	}
	if err = paymentconfig.Validate(config); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "BEpusdt 配置无效："+err.Error())
		return
	}
	if h.environment == "production" {
		switch {
		case config.APIBaseURL != "http://bepusdt:8080":
			httpx.Error(
				w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400,
				"生产环境 BEpusdt API 地址必须使用容器内网 http://bepusdt:8080",
			)
			return
		case h.publicURL == "" || !strings.HasPrefix(h.publicURL, "https://") ||
			config.PublicBaseURL != h.publicURL:
			httpx.Error(
				w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400,
				"生产环境支付公开地址必须与平台 HTTPS 公网地址一致",
			)
			return
		}
	}
	ciphertext, err := h.paymentCipher.Encrypt(channelKey, config)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "加密支付配置失败")
		return
	}
	// Every configuration save invalidates the credential check and stops new
	// orders until the administrator performs the signed protocol check again.
	request.Status = 0
	_, err = tx.ExecContext(r.Context(), `
		UPDATE payment_channels
		SET name=?,currency=?,currency_scale=?,min_amount_minor=?,max_amount_minor=?,
		    config_ciphertext=?,key_version=?,config_verified_hash='',
		    config_verified_at=NULL,status=0,sort_order=?
		WHERE id=?`,
		request.Name, request.Fiat, request.CurrencyScale, request.MinAmountMinor,
		request.MaxAmountMinor, ciphertext, paymentKeyVersion, request.SortOrder, channelID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新支付通道失败")
		return
	}
	before := map[string]any{
		"name": previousName, "provider": provider, "fiat": previousFiat,
		"currency_scale": previousScale, "min_amount_minor": previousMin,
		"max_amount_minor": previousMax, "status": previousStatus,
		"sort_order":       previousSort,
		"api_base_url":     previousConfig.APIBaseURL,
		"public_base_url":  previousConfig.PublicBaseURL,
		"trade_type":       previousConfig.TradeType,
		"timeout_seconds":  previousConfig.TimeoutSeconds,
		"token_configured": paymentTokenConfigured(h.paymentCipher, channelKey, previousCiphertext),
		"config_verified": paymentConfigVerified(
			previousCiphertext, previousVerifiedHash, previousVerifiedAt,
		),
	}
	after := paymentChannelAuditData(channelKey, provider, request, true)
	after["token_changed"] = previousConfigErr != nil ||
		previousConfig.APIToken != config.APIToken
	after["config_verified"] = false
	if err = auditAdmin(
		r.Context(), tx, r, "payment.channel.update", "payment_channel", channelID,
		before, after,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新支付通道失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(channelID), "status": 0,
		"token_configured": true, "config_verified": false,
		"__formMessage": "配置已保存并停用，请先执行签名检查再启用",
	})
}

func (h *Handler) setPaymentChannelStatus(w http.ResponseWriter, r *http.Request) {
	channelID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "支付通道编号无效")
		return
	}
	var request struct {
		Status int `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Status != 0 && request.Status != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "支付通道状态无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新支付通道状态失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var channelKey, provider string
	var previousStatus int
	var ciphertext []byte
	var verifiedHash string
	var verifiedAt sql.NullTime
	err = tx.QueryRowContext(r.Context(), `
		SELECT channel_key,provider,status,config_ciphertext,
		       config_verified_hash,config_verified_at
		FROM payment_channels WHERE id=? FOR UPDATE`,
		channelID,
	).Scan(
		&channelKey, &provider, &previousStatus, &ciphertext,
		&verifiedHash, &verifiedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "支付通道不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取支付通道失败")
		return
	}
	if request.Status == 1 {
		if provider != paymentProviderBEpusdt || h.paymentCipher == nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "只有配置完整的 BEpusdt 通道可以启用")
			return
		}
		config, decryptErr := h.paymentCipher.Decrypt(channelKey, ciphertext)
		if decryptErr != nil || strings.TrimSpace(config.APIToken) == "" {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "请先完成并校验 BEpusdt 配置")
			return
		}
		if !paymentConfigVerified(ciphertext, verifiedHash, verifiedAt) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "请先通过 BEpusdt 签名与 Token 检查")
			return
		}
	}
	if _, err = tx.ExecContext(
		r.Context(), "UPDATE payment_channels SET status=? WHERE id=?",
		request.Status, channelID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新支付通道状态失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "payment.channel.status", "payment_channel", channelID,
		map[string]any{"status": previousStatus},
		map[string]any{"status": request.Status},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新支付通道状态失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(channelID), "status": request.Status,
	})
}

func (h *Handler) checkPaymentChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "支付通道编号无效")
		return
	}
	var channelKey, provider string
	var ciphertext []byte
	err = h.db.QueryRowContext(r.Context(), `
		SELECT channel_key,provider,config_ciphertext
		FROM payment_channels WHERE id=?`,
		channelID,
	).Scan(&channelKey, &provider, &ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "支付通道不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取支付通道失败")
		return
	}
	if provider != paymentProviderBEpusdt || h.paymentCipher == nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "当前通道不是可检查的 BEpusdt 通道")
		return
	}
	config, err := h.paymentCipher.Decrypt(channelKey, ciphertext)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "BEpusdt 配置无法解密或校验")
		return
	}
	if h.environment == "production" {
		if config.APIBaseURL != "http://bepusdt:8080" ||
			h.publicURL == "" || !strings.HasPrefix(h.publicURL, "https://") ||
			config.PublicBaseURL != h.publicURL {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "生产环境 BEpusdt 地址不受信任")
			return
		}
	}
	protocol, checkErr := checkBEpusdtCredentials(r.Context(), h.paymentHTTPClient, config)
	auditAfter := map[string]any{
		"method": "POST", "path": "/api/v1/order/cancel-transaction",
		"token_configured": strings.TrimSpace(config.APIToken) != "",
		"ok":               checkErr == nil,
	}
	if checkErr == nil {
		auditAfter["provider_status_code"] = protocol.StatusCode
	}
	if checkErr != nil {
		if auditErr := auditAdmin(
			r.Context(), h.db, r, "payment.channel.check", "payment_channel",
			channelID, nil, auditAfter,
		); auditErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录支付审计失败")
			return
		}
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadGateway, 502, "BEpusdt 协议检查失败："+checkErr.Error())
		return
	}
	verifiedSum := sha256.Sum256(ciphertext)
	verifiedHash := hex.EncodeToString(verifiedSum[:])
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "保存支付检查结果失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var currentCiphertext []byte
	if err = tx.QueryRowContext(
		r.Context(), "SELECT config_ciphertext FROM payment_channels WHERE id=? FOR UPDATE",
		channelID,
	).Scan(&currentCiphertext); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "检查期间支付配置已变化，请重新检查")
		return
	}
	if !bytes.Equal(currentCiphertext, ciphertext) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "检查期间支付配置已变化，请重新检查")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE payment_channels
		SET config_verified_hash=?,config_verified_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		verifiedHash, channelID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "保存支付检查结果失败")
		return
	}
	auditAfter["config_verified_hash"] = verifiedHash
	if err = auditAdmin(
		r.Context(), tx, r, "payment.channel.check", "payment_channel",
		channelID, nil, auditAfter,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "保存支付检查结果失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(channelID), "ok": true,
		"method": "POST", "path": "/api/v1/order/cancel-transaction",
		"provider_status_code": protocol.StatusCode,
		"provider_message":     protocol.Message,
		"config_verified":      true,
	})
}

func checkBEpusdtCredentials(
	ctx context.Context,
	client *http.Client,
	config paymentconfig.ChannelConfig,
) (paymentInfoProtocolResponse, error) {
	if err := paymentconfig.Validate(config); err != nil {
		return paymentInfoProtocolResponse{}, fmt.Errorf("invalid configuration: %w", err)
	}
	endpoint, err := paymentProtocolEndpoint(
		config.APIBaseURL, "/api/v1/order/cancel-transaction",
	)
	if err != nil {
		return paymentInfoProtocolResponse{}, err
	}
	tradeID, err := nonexistentPaymentTradeID()
	if err != nil {
		return paymentInfoProtocolResponse{}, errors.New("generate check identifier")
	}
	fields := map[string]any{"trade_id": tradeID}
	signature, err := payment.SignBEpusdtRequest(fields, config.APIToken)
	if err != nil {
		return paymentInfoProtocolResponse{}, errors.New("sign protocol check")
	}
	fields["signature"] = signature
	body, err := json.Marshal(fields)
	if err != nil {
		return paymentInfoProtocolResponse{}, errors.New("encode protocol check")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return paymentInfoProtocolResponse{}, errors.New("build protocol check")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if client == nil {
		client = newPaymentHTTPClient()
	}
	response, err := client.Do(request)
	if err != nil {
		return paymentInfoProtocolResponse{}, errors.New("request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return paymentInfoProtocolResponse{}, fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	var protocol paymentInfoProtocolResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if err = decoder.Decode(&protocol); err != nil {
		return paymentInfoProtocolResponse{}, errors.New("invalid JSON response")
	}
	if protocol.StatusCode != http.StatusBadRequest ||
		!strings.Contains(strings.TrimSpace(protocol.Message), "订单不存在") {
		return paymentInfoProtocolResponse{}, errors.New("unexpected protocol response")
	}
	return protocol, nil
}

func verifyBEpusdtPaidOrder(
	ctx context.Context,
	client *http.Client,
	config paymentconfig.ChannelConfig,
	expected bepusdtPaidOrderExpectation,
) error {
	if err := paymentconfig.Validate(config); err != nil {
		return errBEpusdtVerificationUnavailable
	}
	endpoint, err := paymentProtocolEndpoint(config.APIBaseURL, "/api/v1/pay/info")
	if err != nil {
		return errBEpusdtVerificationUnavailable
	}
	body, err := json.Marshal(map[string]string{"trade_id": expected.TradeID})
	if err != nil {
		return errBEpusdtVerificationUnavailable
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(body),
	)
	if err != nil {
		return errBEpusdtVerificationUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if client == nil {
		client = newPaymentHTTPClient()
	}
	response, err := client.Do(request)
	if err != nil {
		return errBEpusdtVerificationUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return errBEpusdtVerificationUnavailable
	}
	var protocol struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			TradeID      string      `json:"trade_id"`
			OrderID      string      `json:"order_id"`
			TradeType    string      `json:"trade_type"`
			Status       json.Number `json:"status"`
			Money        string      `json:"money"`
			ActualAmount string      `json:"actual_amount"`
			Token        string      `json:"token"`
			Fiat         string      `json:"fiat"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.UseNumber()
	if err = decoder.Decode(&protocol); err != nil {
		return errBEpusdtVerificationUnavailable
	}
	status, err := strconv.Atoi(protocol.Data.Status.String())
	if err != nil {
		return errBEpusdtPaymentNotConfirmed
	}
	if protocol.StatusCode != http.StatusOK || status != 2 ||
		strings.TrimSpace(protocol.Data.TradeID) != expected.TradeID ||
		strings.TrimSpace(protocol.Data.OrderID) != expected.OrderID ||
		strings.ToUpper(strings.TrimSpace(protocol.Data.Fiat)) != expected.Fiat ||
		!payment.AmountMatchesMinor(
			protocol.Data.Money, expected.AmountMinor, expected.CurrencyScale,
		) {
		return errBEpusdtPaymentNotConfirmed
	}
	if expected.TradeType != "" &&
		strings.ToLower(strings.TrimSpace(protocol.Data.TradeType)) != expected.TradeType {
		return errBEpusdtPaymentNotConfirmed
	}
	if expected.ActualAmount != "" &&
		strings.TrimSpace(protocol.Data.ActualAmount) != expected.ActualAmount {
		return errBEpusdtPaymentNotConfirmed
	}
	if expected.PaymentAddress != "" &&
		strings.TrimSpace(protocol.Data.Token) != expected.PaymentAddress {
		return errBEpusdtPaymentNotConfirmed
	}
	return nil
}

func paymentProtocolEndpoint(rawBaseURL string, endpointPath string) (string, error) {
	baseURL, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") ||
		baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return "", errors.New("invalid API base URL")
	}
	if endpointPath == "" || !strings.HasPrefix(endpointPath, "/") {
		return "", errors.New("invalid API endpoint path")
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + endpointPath
	baseURL.RawPath = ""
	return baseURL.String(), nil
}

func nonexistentPaymentTradeID() (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return "claw_config_check_" + hex.EncodeToString(randomBytes), nil
}

func newPaymentHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("redirect refused")
		},
	}
}

func (h *Handler) listDetailedRechargeOrders(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		if parsedStatus, err := strconv.Atoi(rawStatus); err == nil {
			status = parsedStatus
		}
	}
	filterArguments := []any{
		keyword, like, like, like, like, like, like, like, like, like, like,
		status, status,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM recharge_orders recharge
		LEFT JOIN payment_channels channel ON channel.id=recharge.channel_id
		WHERE (?='' OR recharge.order_no LIKE ? OR recharge.client_trace_id LIKE ?
		       OR recharge.provider_order_no LIKE ?
		       OR recharge.block_transaction_id LIKE ? OR recharge.payment_address LIKE ?
		       OR recharge.failure_reason LIKE ? OR channel.channel_key LIKE ?
		       OR channel.name LIKE ? OR channel.provider LIKE ?
		       OR CAST(recharge.user_id AS CHAR) LIKE ?)
		  AND (? < 0 OR recharge.status=?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值订单失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT recharge.id,recharge.order_no,recharge.user_id,recharge.product_id,
		       recharge.channel_id,COALESCE(channel.channel_key,''),
		       COALESCE(channel.name,''),COALESCE(channel.provider,''),
		       COALESCE(recharge.provider_order_no,''),recharge.client_trace_id,
		       recharge.fiat_currency,recharge.currency_scale,recharge.amount_minor,
		       recharge.coin_amount,recharge.bonus_coin,recharge.actual_amount,
		       recharge.payment_address,recharge.block_transaction_id,
		       recharge.payment_url,recharge.expires_at,recharge.status,
		       recharge.callback_count,recharge.last_callback_status,
		       recharge.last_callback_at,recharge.callback_payload_hash,
		       recharge.failure_reason,recharge.paid_at,recharge.closed_at,
		       recharge.created_at,recharge.updated_at
		FROM recharge_orders recharge
		LEFT JOIN payment_channels channel ON channel.id=recharge.channel_id
		WHERE (?='' OR recharge.order_no LIKE ? OR recharge.client_trace_id LIKE ?
		       OR recharge.provider_order_no LIKE ?
		       OR recharge.block_transaction_id LIKE ? OR recharge.payment_address LIKE ?
		       OR recharge.failure_reason LIKE ? OR channel.channel_key LIKE ?
		       OR channel.name LIKE ? OR channel.provider LIKE ?
		       OR CAST(recharge.user_id AS CHAR) LIKE ?)
		  AND (? < 0 OR recharge.status=?)
		ORDER BY recharge.created_at DESC,recharge.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值订单失败")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var (
			id, userID, productID, channelID                     int64
			amountMinor, coinAmount, bonusCoin                   int64
			orderNo, channelKey, channelName, provider           string
			providerTradeID, fiat, actualAmount                  string
			paymentAddress, blockTransactionID, paymentURL       string
			callbackPayloadHash, failureReason                   string
			clientTraceID                                        sql.NullString
			currencyScale, status, callbackCount, callbackStatus int
			expiresAt, lastCallbackAt, paidAt, closedAt          sql.NullTime
			createdAt, updatedAt                                 time.Time
		)
		if err = rows.Scan(
			&id, &orderNo, &userID, &productID, &channelID, &channelKey,
			&channelName, &provider, &providerTradeID, &clientTraceID,
			&fiat, &currencyScale, &amountMinor, &coinAmount, &bonusCoin,
			&actualAmount, &paymentAddress, &blockTransactionID, &paymentURL,
			&expiresAt, &status, &callbackCount, &callbackStatus,
			&lastCallbackAt, &callbackPayloadHash, &failureReason,
			&paidAt, &closedAt, &createdAt, &updatedAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值订单失败")
			return
		}
		channelLabel := strings.TrimSpace(channelName)
		if channelLabel == "" {
			channelLabel = channelKey
		} else if channelKey != "" {
			channelLabel += " (" + channelKey + ")"
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(userID), "product_id": apiDecimalID(productID),
			"channel_id": apiDecimalID(channelID), "channel": channelLabel,
			"channel_key": channelKey, "channel_name": channelName, "provider": provider,
			"provider_trade_id": providerTradeID, "provider_order_no": providerTradeID,
			"client_trace_id": clientTraceID.String, "fiat_currency": fiat,
			"currency_scale": currencyScale, "amount_minor": amountMinor,
			"coin_amount": coinAmount, "bonus_coin": bonusCoin,
			"actual_amount": actualAmount, "payment_address": paymentAddress,
			"block_transaction_id": blockTransactionID, "block_hash": blockTransactionID,
			"payment_url": paymentURL, "expires_at": nullTime(expiresAt),
			"status": status, "callback_count": callbackCount,
			"last_callback_status":  callbackStatus,
			"last_callback_at":      nullTime(lastCallbackAt),
			"callback_payload_hash": callbackPayloadHash,
			"failure_reason":        failureReason, "paid_at": nullTime(paidAt),
			"closed_at": nullTime(closedAt), "created_at": createdAt.Unix(),
			"updated_at": updatedAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值订单失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) listRechargeProducts(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		if parsedStatus, err := strconv.Atoi(rawStatus); err == nil {
			status = parsedStatus
		}
	}
	filterArguments := []any{keyword, like, like, status, status}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM recharge_products product
		WHERE (?='' OR product.name LIKE ? OR product.fiat_currency LIKE ?)
		  AND (? < 0 OR product.status=?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值商品失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT product.id,product.name,product.fiat_currency,product.currency_scale,
		       product.amount_minor,product.coin_amount,product.bonus_coin,
		       product.status,product.sort_order,product.created_at,product.updated_at
		FROM recharge_products product
		WHERE (?='' OR product.name LIKE ? OR product.fiat_currency LIKE ?)
		  AND (? < 0 OR product.status=?)
		ORDER BY product.sort_order DESC,product.id
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值商品失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, amountMinor, coinAmount, bonusCoin int64
		var name, fiat string
		var currencyScale, status, sortOrder int
		var createdAt, updatedAt time.Time
		if err = rows.Scan(
			&id, &name, &fiat, &currencyScale, &amountMinor, &coinAmount,
			&bonusCoin, &status, &sortOrder, &createdAt, &updatedAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值商品失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "name": name, "fiat_currency": fiat,
			"currency_scale": currencyScale, "amount_minor": amountMinor,
			"coin_amount": coinAmount, "bonus_coin": bonusCoin,
			"status": status, "sort_order": sortOrder,
			"created_at": createdAt.Unix(), "updated_at": updatedAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值商品失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) createRechargeProduct(w http.ResponseWriter, r *http.Request) {
	h.writeRechargeProduct(w, r, true)
}

func (h *Handler) updateRechargeProduct(w http.ResponseWriter, r *http.Request) {
	h.writeRechargeProduct(w, r, false)
}

func (h *Handler) writeRechargeProduct(w http.ResponseWriter, r *http.Request, create bool) {
	var productID int64
	var err error
	if !create {
		productID, err = positivePathID(r)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "充值商品编号无效")
			return
		}
	}
	var request rechargeProductWriteRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.normalize()
	if err = request.validate(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "充值商品参数无效："+err.Error())
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "保存充值商品失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	var before any
	if create {
		result, insertErr := tx.ExecContext(r.Context(), `
			INSERT INTO recharge_products
			    (name,fiat_currency,currency_scale,amount_minor,coin_amount,
			     bonus_coin,status,sort_order)
			VALUES(?,?,?,?,?,?,?,?)`,
			request.Name, request.FiatCurrency, request.CurrencyScale,
			request.AmountMinor, request.CoinAmount, request.BonusCoin,
			request.Status, request.SortOrder,
		)
		if insertErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建充值商品失败")
			return
		}
		productID, _ = result.LastInsertId()
	} else {
		var previous rechargeProductWriteRequest
		err = tx.QueryRowContext(r.Context(), `
			SELECT name,fiat_currency,currency_scale,amount_minor,coin_amount,
			       bonus_coin,status,sort_order
			FROM recharge_products WHERE id=? FOR UPDATE`,
			productID,
		).Scan(
			&previous.Name, &previous.FiatCurrency, &previous.CurrencyScale,
			&previous.AmountMinor, &previous.CoinAmount, &previous.BonusCoin,
			&previous.Status, &previous.SortOrder,
		)
		if errors.Is(err, sql.ErrNoRows) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "充值商品不存在")
			return
		}
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值商品失败")
			return
		}
		before = previous
		if _, err = tx.ExecContext(r.Context(), `
			UPDATE recharge_products
			SET name=?,fiat_currency=?,currency_scale=?,amount_minor=?,
			    coin_amount=?,bonus_coin=?,status=?,sort_order=?
			WHERE id=?`,
			request.Name, request.FiatCurrency, request.CurrencyScale,
			request.AmountMinor, request.CoinAmount, request.BonusCoin,
			request.Status, request.SortOrder, productID,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新充值商品失败")
			return
		}
	}
	action := "payment.product.update"
	if create {
		action = "payment.product.create"
	}
	if err = auditAdmin(
		r.Context(), tx, r, action, "recharge_product", productID,
		before, request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "保存充值商品失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(productID), "status": request.Status,
	})
}

func (h *Handler) setRechargeProductStatus(w http.ResponseWriter, r *http.Request) {
	productID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "充值商品编号无效")
		return
	}
	var request struct {
		Status int `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Status != 0 && request.Status != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "充值商品状态无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新充值商品状态失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var previousStatus int
	err = tx.QueryRowContext(
		r.Context(), "SELECT status FROM recharge_products WHERE id=? FOR UPDATE", productID,
	).Scan(&previousStatus)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "充值商品不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取充值商品失败")
		return
	}
	if _, err = tx.ExecContext(
		r.Context(), "UPDATE recharge_products SET status=? WHERE id=?",
		request.Status, productID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新充值商品状态失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "payment.product.status", "recharge_product", productID,
		map[string]any{"status": previousStatus}, map[string]any{"status": request.Status},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录支付审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新充值商品状态失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(productID), "status": request.Status,
	})
}

func (request *paymentChannelUpdateRequest) normalize() {
	request.Name = strings.TrimSpace(request.Name)
	request.APIBaseURL = strings.TrimRight(strings.TrimSpace(request.APIBaseURL), "/")
	request.PublicBaseURL = strings.TrimRight(strings.TrimSpace(request.PublicBaseURL), "/")
	request.APIToken = strings.TrimSpace(request.APIToken)
	request.TradeType = strings.TrimSpace(request.TradeType)
	request.Fiat = strings.ToUpper(strings.TrimSpace(request.Fiat))
}

func (request paymentChannelUpdateRequest) validate() error {
	if request.Name == "" || len([]rune(request.Name)) > 100 {
		return errors.New("名称必填且最多 100 个字")
	}
	if request.CurrencyScale < 0 || request.CurrencyScale > 8 {
		return errors.New("币种精度必须在 0 到 8 之间")
	}
	expectedScale, supported := paymentFiatScale(request.Fiat)
	if !supported || request.CurrencyScale != expectedScale {
		return errors.New("法币不受支持或法币精度不匹配")
	}
	if request.MinAmountMinor < 1 || request.MaxAmountMinor < request.MinAmountMinor {
		return errors.New("最小金额必须大于 0，最大金额不能小于最小金额")
	}
	if request.Status != 0 && request.Status != 1 {
		return errors.New("状态只能为启用或停用")
	}
	if request.SortOrder < -1_000_000 || request.SortOrder > 1_000_000 {
		return errors.New("排序超出允许范围")
	}
	return nil
}

func (request *rechargeProductWriteRequest) normalize() {
	request.Name = strings.TrimSpace(request.Name)
	request.FiatCurrency = strings.ToUpper(strings.TrimSpace(request.FiatCurrency))
}

func (request rechargeProductWriteRequest) validate() error {
	if request.Name == "" || len([]rune(request.Name)) > 100 {
		return errors.New("名称必填且最多 100 个字")
	}
	if !paymentCurrencyPattern.MatchString(request.FiatCurrency) {
		return errors.New("法币代码格式无效")
	}
	expectedScale, supported := paymentFiatScale(request.FiatCurrency)
	if !supported || request.CurrencyScale != expectedScale {
		return errors.New("法币不受支持或法币精度不匹配")
	}
	if request.AmountMinor < 1 || request.CoinAmount < 1 || request.BonusCoin < 0 {
		return errors.New("金额和星币数量必须为正数，赠送星币不能为负数")
	}
	if request.Status != 0 && request.Status != 1 {
		return errors.New("状态只能为启用或停用")
	}
	if request.SortOrder < -1_000_000 || request.SortOrder > 1_000_000 {
		return errors.New("排序超出允许范围")
	}
	return nil
}

func paymentFiatScale(fiat string) (int, bool) {
	switch strings.ToUpper(strings.TrimSpace(fiat)) {
	case "CNY", "USD", "EUR", "GBP":
		return 2, true
	case "JPY":
		return 0, true
	default:
		return 0, false
	}
}

func paymentTokenConfigured(
	cipher *paymentconfig.Cipher,
	channelKey string,
	ciphertext []byte,
) bool {
	if cipher == nil || len(ciphertext) == 0 {
		return false
	}
	config, err := cipher.Decrypt(channelKey, ciphertext)
	return err == nil && strings.TrimSpace(config.APIToken) != ""
}

func paymentConfigVerified(
	ciphertext []byte,
	verifiedHash string,
	verifiedAt sql.NullTime,
) bool {
	verifiedHash = strings.ToLower(strings.TrimSpace(verifiedHash))
	if len(ciphertext) == 0 || !verifiedAt.Valid || len(verifiedHash) != sha256.Size*2 {
		return false
	}
	sum := sha256.Sum256(ciphertext)
	return hex.EncodeToString(sum[:]) == verifiedHash
}

func paymentConfigAPIData(
	cipher *paymentconfig.Cipher,
	channelKey string,
	ciphertext []byte,
) map[string]any {
	result := map[string]any{
		"token_configured": false,
		"config_valid":     false,
	}
	switch {
	case len(ciphertext) == 0:
		result["config_error"] = "尚未配置"
	case cipher == nil:
		result["config_error"] = "支付配置服务不可用"
	default:
		config, err := cipher.Decrypt(channelKey, ciphertext)
		if err != nil {
			result["config_error"] = "配置无法解密或校验失败"
			return result
		}
		result["api_base_url"] = config.APIBaseURL
		result["public_base_url"] = config.PublicBaseURL
		result["trade_type"] = config.TradeType
		result["fiat"] = config.Fiat
		result["timeout_seconds"] = config.TimeoutSeconds
		result["token_configured"] = strings.TrimSpace(config.APIToken) != ""
		result["config_valid"] = true
	}
	return result
}

func paymentChannelAuditData(
	channelKey string,
	provider string,
	request paymentChannelUpdateRequest,
	tokenConfigured bool,
) map[string]any {
	return map[string]any{
		"channel_key": channelKey, "name": request.Name, "provider": provider,
		"api_base_url": request.APIBaseURL, "public_base_url": request.PublicBaseURL,
		"trade_type": request.TradeType, "fiat": request.Fiat,
		"timeout_seconds":  request.TimeoutSeconds,
		"currency_scale":   request.CurrencyScale,
		"min_amount_minor": request.MinAmountMinor,
		"max_amount_minor": request.MaxAmountMinor,
		"sort_order":       request.SortOrder, "status": request.Status,
		"token_configured": tokenConfigured,
	}
}
