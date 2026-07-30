package payment

import (
	"errors"
	"time"
)

const (
	USDTChannelKey = "usdt"

	OrderStatusCreated  = 0
	OrderStatusPaying   = 1
	OrderStatusPaid     = 2
	OrderStatusFailed   = 3
	OrderStatusClosed   = 4
	OrderStatusRefunded = 5
)

var (
	ErrChannelDisabled   = errors.New("payment channel is disabled")
	ErrChannelNotReady   = errors.New("payment channel is not configured")
	ErrProductNotFound   = errors.New("recharge product was not found")
	ErrOrderNotFound     = errors.New("recharge order was not found")
	ErrInvalidRequest    = errors.New("invalid payment request")
	ErrIdempotencyReuse  = errors.New("payment idempotency key was reused")
	ErrProvider          = errors.New("payment provider request failed")
	ErrInvalidCallback   = errors.New("invalid payment callback")
	ErrInvalidSignature  = errors.New("invalid payment callback signature")
	ErrCallbackConflict  = errors.New("payment callback conflicts with the order")
	ErrUnsupportedStatus = errors.New("unsupported payment provider status")
)

type CreateRequest struct {
	UserID        int64
	ProductID     int64
	ClientTraceID string
	ClientIP      string
}

type Order struct {
	ID                       int64
	OrderNo                  string
	UserID                   int64
	ProductID                int64
	ChannelID                int64
	ClientTraceID            string
	ProviderOrderNo          string
	TradeType                string
	PaymentURL               string
	ActualAmount             string
	PaymentAddress           string
	BlockTransactionID       string
	FiatCurrency             string
	CurrencyScale            int
	AmountMinor              int64
	CoinAmount               int64
	BonusCoin                int64
	Status                   int
	ExpiresAt                *time.Time
	CallbackCount            uint64
	LastCallbackStatus       int
	LastCallbackAt           *time.Time
	CallbackPayloadHash      string
	FailureReason            string
	PaidAt                   *time.Time
	ClosedAt                 *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
	providerConfigCiphertext []byte
	providerConfigKeyVersion int
	productNameSnapshot      string
}

type CallbackResult struct {
	OrderNo   string
	Status    int
	Duplicate bool
}

type providerCreateRequest struct {
	OrderNo        string
	Amount         string
	Fiat           string
	TradeType      string
	Name           string
	NotifyURL      string
	RedirectURL    string
	TimeoutSeconds int
}

type providerOrder struct {
	TradeID        string
	OrderID        string
	Fiat           string
	TradeType      string
	Amount         string
	ActualAmount   string
	Address        string
	PaymentURL     string
	Status         int
	ExpirationTime int64
}

type callbackData struct {
	Fields             map[string]any
	SanitizedJSON      []byte
	PayloadHash        string
	TradeID            string
	OrderID            string
	Amount             string
	ActualAmount       string
	PaymentAddress     string
	BlockTransactionID string
	Status             int
}
