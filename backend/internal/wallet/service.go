package wallet

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

const defaultCurrency = "COIN"

var (
	ErrInsufficientFunds = errors.New("insufficient wallet balance")
	ErrHoldNotFound      = errors.New("wallet hold not found")
	ErrInvalidHoldState  = errors.New("wallet hold state does not allow this operation")
	ErrAccountDisabled   = errors.New("wallet account is disabled")
	ErrIdempotencyReuse  = errors.New("idempotency key was reused with different parameters")
)

type Service struct {
	db  *sql.DB
	now func() time.Time
}

type Balance struct {
	UserID    int64  `json:"user_id"`
	Currency  string `json:"currency"`
	Available int64  `json:"available"`
	Frozen    int64  `json:"frozen"`
	Version   uint64 `json:"version"`
}

type Entry struct {
	EntryNo        string `json:"entry_no"`
	UserID         int64  `json:"user_id"`
	Available      int64  `json:"available"`
	Frozen         int64  `json:"frozen"`
	DeltaAvailable int64  `json:"delta_available"`
	DeltaFrozen    int64  `json:"delta_frozen"`
	BusinessType   string `json:"business_type"`
	BusinessID     string `json:"business_id"`
	GameCode       string `json:"game_code,omitempty"`
	VenueCode      string `json:"venue_code,omitempty"`
	TableNo        int    `json:"table_no,omitempty"`
	RoundNo        string `json:"round_no,omitempty"`
}

type ApplyRequest struct {
	UserID       int64
	Amount       int64
	BusinessType string
	BusinessID   string
	Description  string
	Metadata     any
	GameCode     string
	VenueCode    string
	TableNo      int
	RoundNo      string
}

type HoldRequest struct {
	UserID       int64
	Amount       int64
	BusinessType string
	BusinessID   string
	ExpiresAt    time.Time
	Description  string
	Metadata     any
	GameCode     string
	VenueCode    string
	TableNo      int
	RoundNo      string
}

type Hold struct {
	HoldNo       string    `json:"hold_no"`
	UserID       int64     `json:"user_id"`
	Amount       int64     `json:"amount"`
	Status       uint8     `json:"status"`
	BusinessType string    `json:"business_type"`
	BusinessID   string    `json:"business_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type CommitRequest struct {
	HoldNo      string
	Payout      int64
	Description string
	Metadata    any
	GameCode    string
	VenueCode   string
	TableNo     int
	RoundNo     string
}

type ReleaseRequest struct {
	HoldNo      string
	Description string
	Metadata    any
	GameCode    string
	VenueCode   string
	TableNo     int
	RoundNo     string
}

type TransferRequest struct {
	FromUserID   int64
	ToUserID     int64
	Amount       int64
	BusinessType string
	BusinessID   string
	Description  string
	Metadata     any
}

type TransferResult struct {
	Debit  Entry `json:"debit"`
	Credit Entry `json:"credit"`
}

func New(db *sql.DB) *Service {
	return &Service{db: db, now: time.Now}
}

// Transfer moves available coins between two users in one transaction. The
// business key is idempotent for both ledger sides.
func (s *Service) Transfer(ctx context.Context, request TransferRequest) (TransferResult, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return TransferResult{}, fmt.Errorf("begin wallet transfer: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := s.TransferTx(ctx, tx, request)
	if err != nil {
		return TransferResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return TransferResult{}, fmt.Errorf("commit wallet transfer: %w", err)
	}
	return result, nil
}

// TransferTx moves available coins using the caller's transaction. It never
// commits or rolls back, so business records and their wallet entries can be
// made atomic by the owning service.
func (s *Service) TransferTx(
	ctx context.Context,
	tx *sql.Tx,
	request TransferRequest,
) (TransferResult, error) {
	if tx == nil {
		return TransferResult{}, errors.New("wallet transfer transaction is required")
	}
	if request.FromUserID < 1 || request.ToUserID < 1 || request.FromUserID == request.ToUserID {
		return TransferResult{}, errors.New("invalid wallet transfer users")
	}
	if request.Amount < 1 {
		return TransferResult{}, errors.New("wallet transfer amount must be positive")
	}
	if err := validateBusiness(request.FromUserID, request.BusinessType, request.BusinessID); err != nil {
		return TransferResult{}, err
	}
	if err := validateBusiness(request.ToUserID, request.BusinessType, request.BusinessID); err != nil {
		return TransferResult{}, err
	}

	firstID, secondID := request.FromUserID, request.ToUserID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}
	first, err := lockAccount(ctx, tx, firstID)
	if err != nil {
		return TransferResult{}, err
	}
	second, err := lockAccount(ctx, tx, secondID)
	if err != nil {
		return TransferResult{}, err
	}
	fromAccount, toAccount := first, second
	if fromAccount.UserID != request.FromUserID {
		fromAccount, toAccount = second, first
	}
	existingDebit, debitFound, err := findEntry(
		ctx, tx, request.BusinessType, request.BusinessID, request.FromUserID,
	)
	if err != nil {
		return TransferResult{}, err
	}
	existingCredit, creditFound, err := findEntry(
		ctx, tx, request.BusinessType, request.BusinessID, request.ToUserID,
	)
	if err != nil {
		return TransferResult{}, err
	}
	if debitFound || creditFound {
		if !debitFound || !creditFound ||
			existingDebit.DeltaAvailable != -request.Amount ||
			existingCredit.DeltaAvailable != request.Amount {
			return TransferResult{}, ErrIdempotencyReuse
		}
		return TransferResult{Debit: existingDebit, Credit: existingCredit}, nil
	}
	if fromAccount.Available < request.Amount {
		return TransferResult{}, ErrInsufficientFunds
	}
	if toAccount.Available > math.MaxInt64-request.Amount {
		return TransferResult{}, errors.New("wallet balance would overflow")
	}
	fromAccount.Available -= request.Amount
	fromAccount.Version++
	toAccount.Available += request.Amount
	toAccount.Version++
	if _, err = tx.ExecContext(ctx, `
		UPDATE wallet_accounts SET available=?,version=? WHERE id=?`,
		fromAccount.Available, fromAccount.Version, fromAccount.ID,
	); err != nil {
		return TransferResult{}, fmt.Errorf("debit wallet transfer: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE wallet_accounts SET available=?,version=? WHERE id=?`,
		toAccount.Available, toAccount.Version, toAccount.ID,
	); err != nil {
		return TransferResult{}, fmt.Errorf("credit wallet transfer: %w", err)
	}
	debit, err := insertEntry(
		ctx, tx, fromAccount, -request.Amount, 0, request.BusinessType, request.BusinessID,
		request.Description, request.Metadata, "", "", 0, "",
	)
	if err != nil {
		return TransferResult{}, err
	}
	credit, err := insertEntry(
		ctx, tx, toAccount, request.Amount, 0, request.BusinessType, request.BusinessID,
		request.Description, request.Metadata, "", "", 0, "",
	)
	if err != nil {
		return TransferResult{}, err
	}
	return TransferResult{Debit: debit, Credit: credit}, nil
}

// Apply writes one idempotent available-balance change. Positive amounts credit
// the user and negative amounts debit the user.
func (s *Service) Apply(ctx context.Context, request ApplyRequest) (Entry, error) {
	if err := validateBusiness(request.UserID, request.BusinessType, request.BusinessID); err != nil {
		return Entry{}, err
	}
	if request.Amount == 0 {
		return Entry{}, errors.New("wallet amount must not be zero")
	}
	if request.Amount == math.MinInt64 {
		return Entry{}, errors.New("wallet amount is out of range")
	}
	return s.withTx(ctx, func(tx *sql.Tx) (Entry, error) {
		account, err := lockAccount(ctx, tx, request.UserID)
		if err != nil {
			return Entry{}, err
		}
		if existing, found, findErr := findEntry(ctx, tx, request.BusinessType, request.BusinessID, request.UserID); findErr != nil {
			return Entry{}, findErr
		} else if found {
			if existing.DeltaAvailable != request.Amount || existing.DeltaFrozen != 0 {
				return Entry{}, ErrIdempotencyReuse
			}
			return existing, nil
		}
		if request.Amount < 0 && account.Available < -request.Amount {
			return Entry{}, ErrInsufficientFunds
		}
		if request.Amount > 0 && account.Available > math.MaxInt64-request.Amount {
			return Entry{}, errors.New("wallet balance would overflow")
		}
		account.Available += request.Amount
		account.Version++
		if _, err = tx.ExecContext(ctx, `
			UPDATE wallet_accounts
			SET available=?,version=? WHERE id=?`,
			account.Available, account.Version, account.ID,
		); err != nil {
			return Entry{}, fmt.Errorf("update wallet account: %w", err)
		}
		return insertEntry(ctx, tx, account, request.Amount, 0, request.BusinessType, request.BusinessID,
			request.Description, request.Metadata, request.GameCode, request.VenueCode, request.TableNo, request.RoundNo)
	})
}

// PlaceHold atomically moves coins from available to frozen. Repeating the
// same business key returns the original hold without moving funds twice.
func (s *Service) PlaceHold(ctx context.Context, request HoldRequest) (Hold, error) {
	if err := validateBusiness(request.UserID, request.BusinessType, request.BusinessID); err != nil {
		return Hold{}, err
	}
	if request.Amount < 1 {
		return Hold{}, errors.New("hold amount must be positive")
	}
	if request.ExpiresAt.IsZero() {
		request.ExpiresAt = s.now().Add(30 * time.Minute)
	}
	var result Hold
	_, err := s.withTx(ctx, func(tx *sql.Tx) (Entry, error) {
		account, err := lockAccount(ctx, tx, request.UserID)
		if err != nil {
			return Entry{}, err
		}
		existing, found, err := findHoldByBusiness(ctx, tx, request.BusinessType, request.BusinessID, request.UserID, true)
		if err != nil {
			return Entry{}, err
		}
		if found {
			if existing.Amount != request.Amount {
				return Entry{}, ErrIdempotencyReuse
			}
			result = existing
			return Entry{}, nil
		}
		if account.Available < request.Amount {
			return Entry{}, ErrInsufficientFunds
		}
		if account.Frozen > math.MaxInt64-request.Amount {
			return Entry{}, errors.New("wallet frozen balance would overflow")
		}
		holdNo, err := idgen.New()
		if err != nil {
			return Entry{}, err
		}
		account.Available -= request.Amount
		account.Frozen += request.Amount
		account.Version++
		if _, err = tx.ExecContext(ctx, `
			UPDATE wallet_accounts
			SET available=?,frozen=?,version=? WHERE id=?`,
			account.Available, account.Frozen, account.Version, account.ID,
		); err != nil {
			return Entry{}, fmt.Errorf("freeze wallet funds: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO wallet_holds
				(hold_no,account_id,user_id,business_type,business_id,amount,status,expires_at)
			VALUES(?,?,?,?,?,?,0,?)`,
			holdNo, account.ID, request.UserID, request.BusinessType, request.BusinessID, request.Amount, request.ExpiresAt,
		); err != nil {
			return Entry{}, fmt.Errorf("insert wallet hold: %w", err)
		}
		result = Hold{
			HoldNo: holdNo, UserID: request.UserID, Amount: request.Amount, Status: 0,
			BusinessType: request.BusinessType, BusinessID: request.BusinessID, ExpiresAt: request.ExpiresAt,
		}
		return insertEntry(ctx, tx, account, -request.Amount, request.Amount,
			"hold_create/"+request.BusinessType, request.BusinessID, request.Description, request.Metadata,
			request.GameCode, request.VenueCode, request.TableNo, request.RoundNo)
	})
	return result, err
}

// CommitHold consumes all frozen coins in the hold and credits payout back to
// available balance in the same transaction. The ledger row stores game, venue,
// table and round identifiers for exact per-game win/loss reporting.
func (s *Service) CommitHold(ctx context.Context, request CommitRequest) (Entry, error) {
	if strings.TrimSpace(request.HoldNo) == "" || request.Payout < 0 {
		return Entry{}, errors.New("invalid hold commit")
	}
	preview, err := s.loadHold(ctx, request.HoldNo)
	if err != nil {
		return Entry{}, err
	}
	return s.withTx(ctx, func(tx *sql.Tx) (Entry, error) {
		account, err := lockAccount(ctx, tx, preview.UserID)
		if err != nil {
			return Entry{}, err
		}
		hold, found, err := findHoldByNo(ctx, tx, request.HoldNo, true)
		if err != nil {
			return Entry{}, err
		}
		if !found {
			return Entry{}, ErrHoldNotFound
		}
		businessType := "hold_commit/" + hold.BusinessType
		if hold.Status == 1 {
			existing, exists, findErr := findEntry(ctx, tx, businessType, hold.BusinessID, hold.UserID)
			if findErr != nil {
				return Entry{}, findErr
			}
			if !exists || existing.DeltaAvailable != request.Payout || existing.DeltaFrozen != -hold.Amount {
				return Entry{}, ErrIdempotencyReuse
			}
			return existing, nil
		}
		if hold.Status != 0 {
			return Entry{}, ErrInvalidHoldState
		}
		if account.Frozen < hold.Amount {
			return Entry{}, errors.New("wallet invariant violated: frozen balance below hold")
		}
		if account.Available > math.MaxInt64-request.Payout {
			return Entry{}, errors.New("wallet balance would overflow")
		}
		account.Available += request.Payout
		account.Frozen -= hold.Amount
		account.Version++
		if _, err = tx.ExecContext(ctx, `
			UPDATE wallet_accounts SET available=?,frozen=?,version=? WHERE id=?`,
			account.Available, account.Frozen, account.Version, account.ID,
		); err != nil {
			return Entry{}, fmt.Errorf("commit wallet account: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE wallet_holds SET status=1,committed_at=? WHERE hold_no=? AND status=0`,
			s.now(), hold.HoldNo,
		); err != nil {
			return Entry{}, fmt.Errorf("commit wallet hold: %w", err)
		}
		return insertEntry(ctx, tx, account, request.Payout, -hold.Amount,
			businessType, hold.BusinessID, request.Description, request.Metadata,
			request.GameCode, request.VenueCode, request.TableNo, request.RoundNo)
	})
}

// ReleaseHold returns an active hold to available balance.
func (s *Service) ReleaseHold(ctx context.Context, holdNo, description string, metadata any) (Entry, error) {
	return s.ReleaseHoldWithContext(ctx, ReleaseRequest{
		HoldNo: holdNo, Description: description, Metadata: metadata,
	})
}

func (s *Service) ReleaseHoldWithContext(ctx context.Context, request ReleaseRequest) (Entry, error) {
	request.HoldNo = strings.TrimSpace(request.HoldNo)
	if request.HoldNo == "" {
		return Entry{}, errors.New("hold number is required")
	}
	preview, err := s.loadHold(ctx, request.HoldNo)
	if err != nil {
		return Entry{}, err
	}
	return s.withTx(ctx, func(tx *sql.Tx) (Entry, error) {
		account, err := lockAccount(ctx, tx, preview.UserID)
		if err != nil {
			return Entry{}, err
		}
		hold, found, err := findHoldByNo(ctx, tx, request.HoldNo, true)
		if err != nil {
			return Entry{}, err
		}
		if !found {
			return Entry{}, ErrHoldNotFound
		}
		businessType := "hold_release/" + hold.BusinessType
		if hold.Status == 2 || hold.Status == 3 {
			existing, exists, findErr := findEntry(ctx, tx, businessType, hold.BusinessID, hold.UserID)
			if findErr != nil {
				return Entry{}, findErr
			}
			if !exists {
				return Entry{}, ErrInvalidHoldState
			}
			return existing, nil
		}
		if hold.Status != 0 {
			return Entry{}, ErrInvalidHoldState
		}
		if account.Frozen < hold.Amount {
			return Entry{}, errors.New("wallet invariant violated: frozen balance below hold")
		}
		if account.Available > math.MaxInt64-hold.Amount {
			return Entry{}, errors.New("wallet balance would overflow")
		}
		account.Available += hold.Amount
		account.Frozen -= hold.Amount
		account.Version++
		if _, err = tx.ExecContext(ctx, `
			UPDATE wallet_accounts SET available=?,frozen=?,version=? WHERE id=?`,
			account.Available, account.Frozen, account.Version, account.ID,
		); err != nil {
			return Entry{}, fmt.Errorf("release wallet account: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE wallet_holds SET status=2,released_at=? WHERE hold_no=? AND status=0`,
			s.now(), hold.HoldNo,
		); err != nil {
			return Entry{}, fmt.Errorf("release wallet hold: %w", err)
		}
		return insertEntry(ctx, tx, account, hold.Amount, -hold.Amount,
			businessType, hold.BusinessID, request.Description, request.Metadata,
			request.GameCode, request.VenueCode, request.TableNo, request.RoundNo)
	})
}

func (s *Service) Balance(ctx context.Context, userID int64) (Balance, error) {
	if userID < 1 {
		return Balance{}, errors.New("invalid user id")
	}
	var balance Balance
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id,currency,available,frozen,version
		FROM wallet_accounts WHERE user_id=? AND currency=?`,
		userID, defaultCurrency,
	).Scan(&balance.UserID, &balance.Currency, &balance.Available, &balance.Frozen, &balance.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return Balance{UserID: userID, Currency: defaultCurrency}, nil
	}
	return balance, err
}

type account struct {
	ID        int64
	UserID    int64
	Available int64
	Frozen    int64
	Version   uint64
	Status    uint8
}

func lockAccount(ctx context.Context, tx *sql.Tx, userID int64) (account, error) {
	if _, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO wallet_accounts(user_id,currency) VALUES(?,?)`,
		userID, defaultCurrency,
	); err != nil {
		return account{}, fmt.Errorf("ensure wallet account: %w", err)
	}
	var result account
	err := tx.QueryRowContext(ctx, `
		SELECT id,user_id,available,frozen,version,status
		FROM wallet_accounts
		WHERE user_id=? AND currency=?
		FOR UPDATE`,
		userID, defaultCurrency,
	).Scan(&result.ID, &result.UserID, &result.Available, &result.Frozen, &result.Version, &result.Status)
	if err != nil {
		return account{}, fmt.Errorf("lock wallet account: %w", err)
	}
	if result.Status != 1 {
		return account{}, ErrAccountDisabled
	}
	return result, nil
}

func findEntry(ctx context.Context, tx *sql.Tx, businessType, businessID string, userID int64) (Entry, bool, error) {
	var result Entry
	err := tx.QueryRowContext(ctx, `
		SELECT entry_no,user_id,balance_available,balance_frozen,delta_available,delta_frozen,
		       business_type,business_id,game_code,venue_code,table_no,round_no
		FROM wallet_ledger_entries
		WHERE business_type=? AND business_id=? AND user_id=?`,
		businessType, businessID, userID,
	).Scan(
		&result.EntryNo, &result.UserID, &result.Available, &result.Frozen,
		&result.DeltaAvailable, &result.DeltaFrozen, &result.BusinessType, &result.BusinessID,
		&result.GameCode, &result.VenueCode, &result.TableNo, &result.RoundNo,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, false, nil
	}
	return result, err == nil, err
}

func insertEntry(
	ctx context.Context,
	tx *sql.Tx,
	account account,
	deltaAvailable, deltaFrozen int64,
	businessType, businessID, description string,
	metadata any,
	gameCode, venueCode string,
	tableNo int,
	roundNo string,
) (Entry, error) {
	entryNo, err := idgen.New()
	if err != nil {
		return Entry{}, err
	}
	metadataJSON, err := marshalMetadata(metadata)
	if err != nil {
		return Entry{}, err
	}
	direction := uint8(3)
	net := deltaAvailable + deltaFrozen
	if net > 0 {
		direction = 1
	} else if net < 0 {
		direction = 2
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallet_ledger_entries
			(entry_no,account_id,user_id,delta_available,delta_frozen,balance_available,balance_frozen,
			 business_type,business_id,direction,game_code,venue_code,table_no,round_no,description,metadata)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		entryNo, account.ID, account.UserID, deltaAvailable, deltaFrozen, account.Available, account.Frozen,
		businessType, businessID, direction, gameCode, venueCode, tableNo, roundNo, description, metadataJSON,
	)
	if err != nil {
		return Entry{}, fmt.Errorf("insert wallet ledger entry: %w", err)
	}
	return Entry{
		EntryNo: entryNo, UserID: account.UserID, Available: account.Available, Frozen: account.Frozen,
		DeltaAvailable: deltaAvailable, DeltaFrozen: deltaFrozen, BusinessType: businessType, BusinessID: businessID,
		GameCode: gameCode, VenueCode: venueCode, TableNo: tableNo, RoundNo: roundNo,
	}, nil
}

func findHoldByBusiness(ctx context.Context, tx *sql.Tx, businessType, businessID string, userID int64, lock bool) (Hold, bool, error) {
	query := `
		SELECT hold_no,user_id,amount,status,business_type,business_id,expires_at
		FROM wallet_holds WHERE business_type=? AND business_id=? AND user_id=?`
	if lock {
		query += " FOR UPDATE"
	}
	return scanHold(tx.QueryRowContext(ctx, query, businessType, businessID, userID))
}

func findHoldByNo(ctx context.Context, tx *sql.Tx, holdNo string, lock bool) (Hold, bool, error) {
	query := `
		SELECT hold_no,user_id,amount,status,business_type,business_id,expires_at
		FROM wallet_holds WHERE hold_no=?`
	if lock {
		query += " FOR UPDATE"
	}
	return scanHold(tx.QueryRowContext(ctx, query, holdNo))
}

func (s *Service) loadHold(ctx context.Context, holdNo string) (Hold, error) {
	hold, found, err := scanHold(s.db.QueryRowContext(ctx, `
		SELECT hold_no,user_id,amount,status,business_type,business_id,expires_at
		FROM wallet_holds WHERE hold_no=?`, holdNo))
	if err != nil {
		return Hold{}, err
	}
	if !found {
		return Hold{}, ErrHoldNotFound
	}
	return hold, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHold(row rowScanner) (Hold, bool, error) {
	var result Hold
	err := row.Scan(
		&result.HoldNo, &result.UserID, &result.Amount, &result.Status,
		&result.BusinessType, &result.BusinessID, &result.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Hold{}, false, nil
	}
	return result, err == nil, err
}

func validateBusiness(userID int64, businessType, businessID string) error {
	if userID < 1 {
		return errors.New("invalid user id")
	}
	if strings.TrimSpace(businessType) != businessType || businessType == "" || len(businessType) > 36 {
		return errors.New("invalid business type")
	}
	if strings.TrimSpace(businessID) != businessID || businessID == "" || len(businessID) > 100 {
		return errors.New("invalid business id")
	}
	return nil
}

func marshalMetadata(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal wallet metadata: %w", err)
	}
	return body, nil
}

func (s *Service) withTx(ctx context.Context, operation func(*sql.Tx) (Entry, error)) (Entry, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Entry{}, fmt.Errorf("begin wallet transaction: %w", err)
	}
	result, err := operation(tx)
	if err != nil {
		_ = tx.Rollback()
		return Entry{}, err
	}
	if err = tx.Commit(); err != nil {
		return Entry{}, fmt.Errorf("commit wallet transaction: %w", err)
	}
	return result, nil
}
