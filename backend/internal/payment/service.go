package payment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/paymentconfig"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

const (
	createTraceLockWaitSeconds    = 12
	createTraceCleanupTimeout     = 3 * time.Second
	providerConfigSnapshotVersion = 1
	paymentKeyVersion             = 1
	// database.Open currently caps MySQL at 60 connections. Named locks hold
	// one dedicated connection while normal order queries use the pool, so
	// keep this well below that cap to avoid connection-pool starvation.
	createTraceLockConcurrency = 16
)

var createTraceLockSlots = make(chan struct{}, createTraceLockConcurrency)

type Options struct {
	HTTPClient *http.Client
	Now        func() time.Time
}

type Service struct {
	db        *sql.DB
	wallet    *wallet.Service
	cipher    *paymentconfig.Cipher
	client    *bepusdtClient
	publicURL string
	now       func() time.Time
}

type channel struct {
	ID                 int64
	Key                string
	Provider           string
	Currency           string
	CurrencyScale      int
	MinAmountMinor     int64
	MaxAmountMinor     int64
	KeyVersion         int
	ConfigCiphertext   []byte
	ConfigVerifiedHash string
	ConfigVerifiedAt   sql.NullTime
	Status             int
	Config             paymentconfig.ChannelConfig
}

type product struct {
	ID            int64
	Name          string
	FiatCurrency  string
	CurrencyScale int
	AmountMinor   int64
	CoinAmount    int64
	BonusCoin     int64
}

func New(
	db *sql.DB,
	walletService *wallet.Service,
	masterKey string,
	publicURL string,
	options Options,
) (*Service, error) {
	if db == nil || walletService == nil {
		return nil, errors.New("payment database and wallet service are required")
	}
	configCipher, err := paymentconfig.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	parsedPublicURL, err := url.ParseRequestURI(publicURL)
	if err != nil || parsedPublicURL.Host == "" ||
		(parsedPublicURL.Scheme != "http" && parsedPublicURL.Scheme != "https") {
		return nil, errors.New("payment public URL is invalid")
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 8 * time.Second}
	} else {
		copyClient := *httpClient
		httpClient = &copyClient
		if httpClient.Timeout <= 0 {
			httpClient.Timeout = 8 * time.Second
		}
	}
	// A redirect could replay the signed request body to another origin.
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		db: db, wallet: walletService, cipher: configCipher,
		client:    &bepusdtClient{httpClient: httpClient},
		publicURL: publicURL, now: now,
	}, nil
}

func (s *Service) CreateRecharge(ctx context.Context, request CreateRequest) (Order, error) {
	request.ClientTraceID = strings.TrimSpace(request.ClientTraceID)
	request.ClientIP = strings.TrimSpace(request.ClientIP)
	if request.UserID < 1 || request.ProductID < 1 ||
		!validClientTraceID(request.ClientTraceID) || len(request.ClientIP) > 45 {
		return Order{}, ErrInvalidRequest
	}
	unlock, err := s.lockCreateTrace(ctx, request.UserID, request.ClientTraceID)
	if err != nil {
		return Order{}, fmt.Errorf("lock recharge idempotency key: %w", err)
	}
	defer unlock()
	existing, found, err := s.orderByTrace(ctx, request.UserID, request.ClientTraceID)
	if err != nil {
		return Order{}, fmt.Errorf("find recharge idempotency key: %w", err)
	}
	if found {
		if existing.ProductID != request.ProductID {
			return Order{}, ErrIdempotencyReuse
		}
		if existing.PaymentURL != "" || existing.Status >= OrderStatusPaid {
			return existing, nil
		}
		return s.createProviderOrder(ctx, existing)
	}

	channel, err := s.loadChannel(ctx, USDTChannelKey, true)
	if err != nil {
		return Order{}, err
	}
	product, err := s.loadProduct(ctx, request.ProductID)
	if err != nil {
		return Order{}, err
	}
	if err = validateProductForChannel(product, channel); err != nil {
		return Order{}, err
	}
	orderNo, err := idgen.New()
	if err != nil {
		return Order{}, fmt.Errorf("generate recharge order: %w", err)
	}
	configSnapshot, err := s.cipher.Encrypt(providerSnapshotScope(orderNo), channel.Config)
	if err != nil {
		return Order{}, ErrChannelNotReady
	}
	productNameSnapshot := strings.TrimSpace(product.Name)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO recharge_orders
			(order_no,user_id,product_id,channel_id,client_trace_id,
			 provider_config_ciphertext,provider_config_key_version,
			 product_name_snapshot,fiat_currency,currency_scale,amount_minor,
			 coin_amount,bonus_coin,status,client_ip,provider_payload)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,0,?,JSON_OBJECT('client','uniapp'))`,
		orderNo, request.UserID, product.ID, channel.ID, request.ClientTraceID,
		configSnapshot, providerConfigSnapshotVersion, productNameSnapshot,
		product.FiatCurrency, product.CurrencyScale, product.AmountMinor,
		product.CoinAmount, product.BonusCoin, request.ClientIP,
	)
	if err != nil {
		if !isDuplicateKey(err) {
			return Order{}, fmt.Errorf("insert recharge order: %w", err)
		}
		existing, found, err = s.orderByTrace(ctx, request.UserID, request.ClientTraceID)
		if err != nil {
			return Order{}, fmt.Errorf("recover concurrent recharge order: %w", err)
		}
		if !found {
			return Order{}, fmt.Errorf("recover concurrent recharge order: %w", ErrOrderNotFound)
		}
		if existing.ProductID != request.ProductID {
			return Order{}, ErrIdempotencyReuse
		}
		if existing.PaymentURL != "" || existing.Status >= OrderStatusPaid {
			return existing, nil
		}
		return s.createProviderOrder(ctx, existing)
	}
	created, found, err := s.orderByNumber(ctx, request.UserID, orderNo)
	if err != nil || !found {
		if err == nil {
			err = ErrOrderNotFound
		}
		return Order{}, fmt.Errorf("load created recharge order: %w", err)
	}
	return s.createProviderOrder(ctx, created)
}

func (s *Service) createProviderOrder(ctx context.Context, order Order) (Order, error) {
	config, err := s.providerConfigSnapshot(order)
	if err != nil {
		return Order{}, err
	}
	amount, err := formatMinorCanonical(order.AmountMinor, order.CurrencyScale)
	if err != nil {
		return Order{}, ErrInvalidRequest
	}
	providerResult, err := s.client.createTransaction(ctx, config, providerCreateRequest{
		OrderNo: order.OrderNo, Amount: amount, Fiat: order.FiatCurrency,
		TradeType: config.TradeType, Name: order.productNameSnapshot,
		NotifyURL: s.publicURL + "/api/v2/payments/bepusdt/notify",
		RedirectURL: s.publicURL + "/h5/#/pages/wallet/detail?type=charge&order_no=" +
			url.QueryEscape(order.OrderNo),
		TimeoutSeconds: config.TimeoutSeconds,
	})
	if err != nil {
		return Order{}, err
	}
	if providerResult.ExpirationTime > int64(config.TimeoutSeconds)+30 {
		return Order{}, fmt.Errorf("%w: provider expiration exceeds channel timeout", ErrProvider)
	}
	expiresAt := s.now().Add(time.Duration(providerResult.ExpirationTime) * time.Second)
	createPayload, err := json.Marshal(map[string]any{
		"trade_id": providerResult.TradeID, "order_id": providerResult.OrderID,
		"fiat": providerResult.Fiat, "amount": providerResult.Amount,
		"trade_type":    config.TradeType,
		"actual_amount": providerResult.ActualAmount, "token": providerResult.Address,
		"payment_url": providerResult.PaymentURL, "status": providerResult.Status,
		"expiration_time": providerResult.ExpirationTime,
	})
	if err != nil {
		return Order{}, fmt.Errorf("encode provider order: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Order{}, fmt.Errorf("begin provider order persistence: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	locked, found, err := orderByNumberTx(ctx, tx, order.UserID, order.OrderNo, true)
	if err != nil || !found {
		if err == nil {
			err = ErrOrderNotFound
		}
		return Order{}, err
	}
	if locked.ProductID != order.ProductID || locked.ChannelID != order.ChannelID ||
		locked.providerConfigKeyVersion != order.providerConfigKeyVersion ||
		locked.productNameSnapshot != order.productNameSnapshot ||
		!bytes.Equal(locked.providerConfigCiphertext, order.providerConfigCiphertext) {
		return Order{}, ErrIdempotencyReuse
	}
	if locked.ProviderOrderNo != "" && locked.ProviderOrderNo != providerResult.TradeID {
		return Order{}, ErrCallbackConflict
	}
	if locked.ActualAmount != "" && locked.ActualAmount != providerResult.ActualAmount {
		return Order{}, ErrCallbackConflict
	}
	if locked.PaymentAddress != "" && locked.PaymentAddress != providerResult.Address {
		return Order{}, ErrCallbackConflict
	}
	targetStatus := locked.Status
	if targetStatus == OrderStatusCreated {
		targetStatus = OrderStatusPaying
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE recharge_orders
		SET provider_order_no=?,payment_url=?,actual_amount=?,payment_address=?,
		    expires_at=COALESCE(expires_at,?),status=?,
		    provider_payload=JSON_SET(COALESCE(provider_payload,JSON_OBJECT()),
		                              '$.create',CAST(? AS JSON)),
		    failure_reason=IF(status IN (0,1),'',failure_reason)
		WHERE id=?`,
		providerResult.TradeID, providerResult.PaymentURL, providerResult.ActualAmount,
		providerResult.Address, expiresAt, targetStatus, string(createPayload), locked.ID,
	)
	if err != nil {
		return Order{}, fmt.Errorf("persist provider order: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return Order{}, fmt.Errorf("commit provider order: %w", err)
	}
	result, found, err := s.orderByNumber(ctx, order.UserID, order.OrderNo)
	if err != nil || !found {
		if err == nil {
			err = ErrOrderNotFound
		}
		return Order{}, err
	}
	return result, nil
}

func (s *Service) OrderStatus(ctx context.Context, userID int64, reference string) (Order, error) {
	reference = strings.TrimSpace(reference)
	if userID < 1 || reference == "" || len(reference) > 100 {
		return Order{}, ErrInvalidRequest
	}
	order, found, err := s.orderByNumber(ctx, userID, reference)
	if err != nil {
		return Order{}, err
	}
	if !found {
		order, found, err = s.orderByTrace(ctx, userID, reference)
	}
	if err != nil {
		return Order{}, err
	}
	if !found {
		return Order{}, ErrOrderNotFound
	}
	return order, nil
}

func (s *Service) loadChannel(
	ctx context.Context,
	channelKey string,
	requireEnabled bool,
) (channel, error) {
	var result channel
	err := s.db.QueryRowContext(ctx, `
		SELECT id,channel_key,provider,currency,currency_scale,min_amount_minor,
		       max_amount_minor,config_ciphertext,key_version,
		       config_verified_hash,config_verified_at,status
		FROM payment_channels WHERE channel_key=?`,
		channelKey,
	).Scan(
		&result.ID, &result.Key, &result.Provider, &result.Currency,
		&result.CurrencyScale, &result.MinAmountMinor, &result.MaxAmountMinor,
		&result.ConfigCiphertext, &result.KeyVersion, &result.ConfigVerifiedHash,
		&result.ConfigVerifiedAt, &result.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return channel{}, ErrChannelNotReady
	}
	if err != nil {
		return channel{}, fmt.Errorf("load payment channel: %w", err)
	}
	if requireEnabled && result.Status != 1 {
		return channel{}, ErrChannelDisabled
	}
	if result.Provider != "bepusdt" || result.KeyVersion != paymentKeyVersion ||
		len(result.ConfigCiphertext) == 0 {
		return channel{}, ErrChannelNotReady
	}
	if requireEnabled {
		configHash := sha256.Sum256(result.ConfigCiphertext)
		if !result.ConfigVerifiedAt.Valid ||
			!strings.EqualFold(
				strings.TrimSpace(result.ConfigVerifiedHash),
				hex.EncodeToString(configHash[:]),
			) {
			return channel{}, ErrChannelNotReady
		}
	}
	result.Config, err = s.cipher.Decrypt(result.Key, result.ConfigCiphertext)
	if err != nil {
		return channel{}, ErrChannelNotReady
	}
	if result.Config.Fiat != result.Currency ||
		fiatScale(result.Config.Fiat) != result.CurrencyScale ||
		(result.MaxAmountMinor > 0 && result.MinAmountMinor > result.MaxAmountMinor) {
		return channel{}, ErrChannelNotReady
	}
	return result, nil
}

func (s *Service) providerConfigSnapshot(order Order) (paymentconfig.ChannelConfig, error) {
	if order.providerConfigKeyVersion != providerConfigSnapshotVersion ||
		len(order.providerConfigCiphertext) == 0 ||
		strings.TrimSpace(order.productNameSnapshot) == "" {
		return paymentconfig.ChannelConfig{}, ErrChannelNotReady
	}
	config, err := s.cipher.Decrypt(
		providerSnapshotScope(order.OrderNo),
		order.providerConfigCiphertext,
	)
	if err != nil || config.Fiat != order.FiatCurrency ||
		fiatScale(config.Fiat) != order.CurrencyScale {
		return paymentconfig.ChannelConfig{}, ErrChannelNotReady
	}
	return config, nil
}

func providerSnapshotScope(orderNo string) string {
	// idgen.New returns a 26-character ULID. Lower-casing it keeps the scope
	// within paymentconfig's authenticated-key grammar while binding a
	// ciphertext to one immutable order number.
	return USDTChannelKey + "-" + strings.ToLower(strings.TrimSpace(orderNo))
}

func (s *Service) loadProduct(ctx context.Context, productID int64) (product, error) {
	var result product
	err := s.db.QueryRowContext(ctx, `
		SELECT id,name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin
		FROM recharge_products WHERE id=? AND status=1`,
		productID,
	).Scan(
		&result.ID, &result.Name, &result.FiatCurrency, &result.CurrencyScale,
		&result.AmountMinor, &result.CoinAmount, &result.BonusCoin,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return product{}, ErrProductNotFound
	}
	if err != nil {
		return product{}, fmt.Errorf("load recharge product: %w", err)
	}
	result.Name = strings.TrimSpace(result.Name)
	return result, nil
}

func validateProductForChannel(product product, channel channel) error {
	if product.ID < 1 || product.Name == "" || len(product.Name) > 100 ||
		product.AmountMinor < 1 || product.CoinAmount < 1 ||
		product.BonusCoin < 0 || product.FiatCurrency != channel.Currency ||
		product.CurrencyScale != channel.CurrencyScale {
		return ErrInvalidRequest
	}
	if channel.MinAmountMinor > 0 && product.AmountMinor < channel.MinAmountMinor {
		return ErrInvalidRequest
	}
	if channel.MaxAmountMinor > 0 && product.AmountMinor > channel.MaxAmountMinor {
		return ErrInvalidRequest
	}
	return nil
}

func (s *Service) orderByTrace(
	ctx context.Context,
	userID int64,
	traceID string,
) (Order, bool, error) {
	return scanOrder(s.db.QueryRowContext(ctx, orderSelect+`
		WHERE recharge.user_id=? AND recharge.client_trace_id=?
		  AND channel.channel_key='usdt'`, userID, traceID))
}

func (s *Service) orderByNumber(
	ctx context.Context,
	userID int64,
	orderNo string,
) (Order, bool, error) {
	return scanOrder(s.db.QueryRowContext(ctx, orderSelect+`
		WHERE recharge.user_id=? AND recharge.order_no=?
		  AND channel.channel_key='usdt'`, userID, orderNo))
}

func orderByNumberTx(
	ctx context.Context,
	tx *sql.Tx,
	userID int64,
	orderNo string,
	forUpdate bool,
) (Order, bool, error) {
	query := orderSelect + `
		WHERE recharge.user_id=? AND recharge.order_no=?
		  AND channel.channel_key='usdt'`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanOrder(tx.QueryRowContext(ctx, query, userID, orderNo))
}

const orderSelect = `
	SELECT recharge.id,recharge.order_no,recharge.user_id,recharge.product_id,
	       recharge.channel_id,COALESCE(recharge.client_trace_id,''),
	       recharge.provider_config_ciphertext,
	       recharge.provider_config_key_version,recharge.product_name_snapshot,
	       COALESCE(recharge.provider_order_no,''),
	       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(
	           recharge.provider_payload,'$.create.trade_type')),''),
	       recharge.payment_url,
	       recharge.actual_amount,recharge.payment_address,
	       recharge.block_transaction_id,recharge.fiat_currency,
	       recharge.currency_scale,recharge.amount_minor,recharge.coin_amount,
	       recharge.bonus_coin,recharge.status,recharge.expires_at,
	       recharge.callback_count,recharge.last_callback_status,
	       recharge.last_callback_at,recharge.callback_payload_hash,
	       recharge.failure_reason,recharge.paid_at,recharge.closed_at,
	       recharge.created_at,recharge.updated_at
	FROM recharge_orders recharge
	JOIN payment_channels channel ON channel.id=recharge.channel_id
`

type rowScanner interface {
	Scan(...any) error
}

func scanOrder(row rowScanner) (Order, bool, error) {
	var result Order
	var expiresAt, lastCallbackAt, paidAt, closedAt sql.NullTime
	err := row.Scan(
		&result.ID, &result.OrderNo, &result.UserID, &result.ProductID,
		&result.ChannelID, &result.ClientTraceID,
		&result.providerConfigCiphertext, &result.providerConfigKeyVersion,
		&result.productNameSnapshot, &result.ProviderOrderNo,
		&result.TradeType, &result.PaymentURL, &result.ActualAmount, &result.PaymentAddress,
		&result.BlockTransactionID, &result.FiatCurrency, &result.CurrencyScale,
		&result.AmountMinor, &result.CoinAmount, &result.BonusCoin, &result.Status,
		&expiresAt, &result.CallbackCount, &result.LastCallbackStatus,
		&lastCallbackAt, &result.CallbackPayloadHash, &result.FailureReason,
		&paidAt, &closedAt, &result.CreatedAt, &result.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, false, nil
	}
	if err != nil {
		return Order{}, false, err
	}
	result.ExpiresAt = nullTimePointer(expiresAt)
	result.LastCallbackAt = nullTimePointer(lastCallbackAt)
	result.PaidAt = nullTimePointer(paidAt)
	result.ClosedAt = nullTimePointer(closedAt)
	return result, true, nil
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func validClientTraceID(value string) bool {
	if len(value) < 8 || len(value) > 100 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func isDuplicateKey(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

// lockCreateTrace serializes the full local-create/provider-create/persist
// sequence across every API instance. BEpusdt is itself idempotent by
// order_id, but this lock also prevents concurrent requests with the same
// client trace from making duplicate upstream HTTP calls.
func (s *Service) lockCreateTrace(
	ctx context.Context,
	userID int64,
	traceID string,
) (func(), error) {
	select {
	case createTraceLockSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	releaseSlot := true
	defer func() {
		if releaseSlot {
			<-createTraceLockSlots
		}
	}()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(strconv.FormatInt(userID, 10) + "\x00" + traceID))
	lockName := fmt.Sprintf("payment-create:%x", sum[:22])
	var acquired sql.NullInt64
	if err = conn.QueryRowContext(
		ctx, "SELECT GET_LOCK(?,?)", lockName, createTraceLockWaitSeconds,
	).Scan(&acquired); err != nil {
		discardPaymentSQLConn(conn)
		_ = conn.Close()
		return nil, err
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		_ = conn.Close()
		return nil, errors.New("recharge order is already being created")
	}
	releaseSlot = false
	return func() {
		defer func() {
			_ = conn.Close()
			<-createTraceLockSlots
		}()
		releaseContext, cancel := context.WithTimeout(
			context.Background(), createTraceCleanupTimeout,
		)
		defer cancel()
		var released sql.NullInt64
		releaseErr := conn.QueryRowContext(
			releaseContext, "SELECT RELEASE_LOCK(?)", lockName,
		).Scan(&released)
		if releaseErr != nil || !released.Valid || released.Int64 != 1 {
			// Never return a physical connection that may still own the named
			// lock to database/sql's pool.
			discardPaymentSQLConn(conn)
		}
	}, nil
}

func discardPaymentSQLConn(conn *sql.Conn) {
	_ = conn.Raw(func(any) error {
		return driver.ErrBadConn
	})
}
