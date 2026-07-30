package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/zllyxr/live_claw/backend/internal/paymentconfig"
)

const maxProviderResponseBody = 64 << 10

type bepusdtClient struct {
	httpClient *http.Client
}

func (c *bepusdtClient) createTransaction(
	ctx context.Context,
	config paymentconfig.ChannelConfig,
	request providerCreateRequest,
) (providerOrder, error) {
	endpoint, err := providerEndpoint(config.APIBaseURL, "/api/v1/order/create-transaction")
	if err != nil {
		return providerOrder{}, ErrChannelNotReady
	}
	fields := map[string]any{
		"order_id":     request.OrderNo,
		"amount":       json.Number(request.Amount),
		"fiat":         request.Fiat,
		"trade_type":   request.TradeType,
		"name":         request.Name,
		"notify_url":   request.NotifyURL,
		"redirect_url": request.RedirectURL,
		"timeout":      request.TimeoutSeconds,
	}
	signature, err := signFields(fields, config.APIToken)
	if err != nil {
		return providerOrder{}, ErrInvalidRequest
	}
	fields["signature"] = signature
	body, err := json.Marshal(fields)
	if err != nil {
		return providerOrder{}, ErrInvalidRequest
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return providerOrder{}, ErrProvider
	}
	publicURL, err := url.Parse(config.PublicBaseURL)
	if err != nil {
		return providerOrder{}, ErrChannelNotReady
	}
	httpRequest.Host = publicURL.Host
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("User-Agent", "claw-v2/bepusdt")
	httpRequest.Header.Set("X-Forwarded-Host", publicURL.Host)
	httpRequest.Header.Set("X-Forwarded-Proto", publicURL.Scheme)

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return providerOrder{}, fmt.Errorf("%w: provider unavailable", ErrProvider)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxProviderResponseBody+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil || len(responseBody) > maxProviderResponseBody {
		return providerOrder{}, fmt.Errorf("%w: invalid provider response", ErrProvider)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return providerOrder{}, fmt.Errorf("%w: provider HTTP status %d", ErrProvider, response.StatusCode)
	}
	var payload struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			TradeID        string      `json:"trade_id"`
			OrderID        string      `json:"order_id"`
			Fiat           string      `json:"fiat"`
			TradeType      string      `json:"trade_type"`
			Amount         string      `json:"amount"`
			ActualAmount   string      `json:"actual_amount"`
			Token          string      `json:"token"`
			PaymentURL     string      `json:"payment_url"`
			Status         json.Number `json:"status"`
			ExpirationTime json.Number `json:"expiration_time"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.UseNumber()
	if err = decoder.Decode(&payload); err != nil {
		return providerOrder{}, fmt.Errorf("%w: invalid provider response", ErrProvider)
	}
	if payload.StatusCode != http.StatusOK {
		return providerOrder{}, fmt.Errorf("%w: provider rejected the order", ErrProvider)
	}
	status, err := strconv.Atoi(payload.Data.Status.String())
	if err != nil {
		return providerOrder{}, fmt.Errorf("%w: invalid provider status", ErrProvider)
	}
	expiration, err := strconv.ParseInt(payload.Data.ExpirationTime.String(), 10, 64)
	if err != nil || expiration < 1 {
		return providerOrder{}, fmt.Errorf("%w: invalid provider expiration", ErrProvider)
	}
	paymentURL, err := canonicalPaymentURL(payload.Data.PaymentURL, config.PublicBaseURL)
	if err != nil {
		return providerOrder{}, fmt.Errorf("%w: invalid provider payment URL", ErrProvider)
	}
	result := providerOrder{
		TradeID:        strings.TrimSpace(payload.Data.TradeID),
		OrderID:        strings.TrimSpace(payload.Data.OrderID),
		Fiat:           strings.ToUpper(strings.TrimSpace(payload.Data.Fiat)),
		TradeType:      strings.ToLower(strings.TrimSpace(payload.Data.TradeType)),
		Amount:         strings.TrimSpace(payload.Data.Amount),
		ActualAmount:   strings.TrimSpace(payload.Data.ActualAmount),
		Address:        strings.TrimSpace(payload.Data.Token),
		PaymentURL:     paymentURL,
		Status:         status,
		ExpirationTime: expiration,
	}
	if result.OrderID != request.OrderNo || result.Fiat != request.Fiat ||
		result.TradeType != request.TradeType ||
		result.TradeID == "" || len(result.TradeID) > 190 ||
		result.Status != 1 || result.Address == "" || len(result.Address) > 190 ||
		result.ActualAmount == "" || len(result.ActualAmount) > 64 {
		return providerOrder{}, fmt.Errorf("%w: inconsistent provider response", ErrProvider)
	}
	responseMinor, err := decimalToMinor(result.Amount, fiatScale(result.Fiat))
	if err != nil {
		return providerOrder{}, fmt.Errorf("%w: invalid provider amount", ErrProvider)
	}
	requestMinor, err := decimalToMinor(request.Amount, fiatScale(request.Fiat))
	if err != nil || responseMinor != requestMinor {
		return providerOrder{}, fmt.Errorf("%w: provider amount mismatch", ErrProvider)
	}
	if result.ActualAmount != "" {
		if result.ActualAmount, err = normalizePositiveDecimal(result.ActualAmount, 64); err != nil {
			return providerOrder{}, fmt.Errorf("%w: invalid provider crypto amount", ErrProvider)
		}
	}
	return result, nil
}

func providerEndpoint(baseURL, endpointPath string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid provider base URL")
	}
	parsed.Path = path.Join(parsed.Path, endpointPath)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func canonicalPaymentURL(raw, publicBase string) (string, error) {
	providerURL, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || providerURL.Host == "" ||
		(providerURL.Scheme != "http" && providerURL.Scheme != "https") ||
		providerURL.User != nil || !strings.HasPrefix(providerURL.EscapedPath(), "/pay/") {
		return "", errors.New("invalid payment URL")
	}
	publicURL, err := url.Parse(publicBase)
	if err != nil || publicURL.Host == "" {
		return "", errors.New("invalid public payment base URL")
	}
	providerURL.Scheme = publicURL.Scheme
	providerURL.Host = publicURL.Host
	return providerURL.String(), nil
}

func fiatScale(fiat string) int {
	if strings.EqualFold(strings.TrimSpace(fiat), "JPY") {
		return 0
	}
	return 2
}
