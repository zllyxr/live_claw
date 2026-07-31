package game

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

var (
	ErrVenueNotFound        = errors.New("game venue not found")
	ErrSessionNotFound      = errors.New("game session not found")
	ErrInvalidFishingCannon = errors.New("invalid fishing cannon value")
)

type Service struct {
	db         *sql.DB
	matchmaker *Matchmaker
	wallet     *wallet.Service
	now        func() time.Time
}

type FishingLaunch struct {
	SessionID     string `json:"session_id"`
	GameCode      string `json:"game_code"`
	GameName      string `json:"game_name"`
	EntryPath     string `json:"entry_path"`
	VenueCode     string `json:"venue_code"`
	VenueName     string `json:"venue_name"`
	Multiplier    int    `json:"multiplier"`
	Table         int    `json:"table"`
	Seat          int    `json:"seat"`
	EscrowAmount  int64  `json:"escrow_amount"` // Deprecated: direct-wallet sessions always return zero.
	WalletBalance int64  `json:"wallet_balance"`
	ResumeToken   string `json:"resume_token"`
	Resumed       bool   `json:"resumed"`
	venueID       int64
}

type fishingVenue struct {
	GameID       int64
	GameCode     string
	GameName     string
	EntryPath    string
	VenueID      int64
	VenueCode    string
	VenueName    string
	Multiplier   int
	MinBalance   int64
	EscrowAmount int64
}

type CatalogItem struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	EntryPath   string `json:"entry_path"`
	PlayersMin  int    `json:"players_min"`
	PlayersMax  int    `json:"players_max"`
	Orientation string `json:"orientation"`
	UseWallet   bool   `json:"use_wallet"`
}

type FishingVenueItem struct {
	ID            int64   `json:"venue_id"`
	Code          string  `json:"venue_code"`
	Name          string  `json:"venue_name"`
	Multiplier    int     `json:"multiplier"`
	TableCount    int     `json:"table_count"`
	SeatsPerTable int     `json:"seats_per_table"`
	MinBalance    int64   `json:"min_balance"`
	EscrowAmount  int64   `json:"escrow_amount"`
	BetLevels     []int64 `json:"bet_levels"`
}

func NewService(db *sql.DB, matchmaker *Matchmaker, walletService *wallet.Service) *Service {
	return &Service{db: db, matchmaker: matchmaker, wallet: walletService, now: time.Now}
}

func (s *Service) Catalog(ctx context.Context, category string) ([]CatalogItem, error) {
	category = strings.TrimSpace(category)
	query := `
		SELECT id,game_code,name,category,entry_path,min_players,max_players,orientation,wallet_enabled
		FROM games WHERE status=1`
	args := make([]any, 0, 1)
	if category != "" {
		query += " AND category=?"
		args = append(args, category)
	}
	query += " ORDER BY sort_order DESC,id"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CatalogItem, 0, 16)
	for rows.Next() {
		var item CatalogItem
		if err = rows.Scan(
			&item.ID, &item.Code, &item.Name, &item.Category, &item.EntryPath,
			&item.PlayersMin, &item.PlayersMax, &item.Orientation, &item.UseWallet,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) FishingVenues(ctx context.Context) ([]FishingVenueItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT venue.id,venue.venue_code,venue.name,venue.multiplier,
		       venue.table_count,venue.seats_per_table,venue.min_balance,
		       venue.escrow_amount,venue.bet_levels
		FROM game_venues venue
		JOIN games game ON game.id=venue.game_id
		WHERE game.game_code='deepsea_hunter' AND game.status=1 AND venue.status=1
		ORDER BY venue.multiplier ASC,venue.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]FishingVenueItem, 0, 3)
	for rows.Next() {
		var item FishingVenueItem
		var raw []byte
		if err = rows.Scan(
			&item.ID, &item.Code, &item.Name, &item.Multiplier,
			&item.TableCount, &item.SeatsPerTable, &item.MinBalance,
			&item.EscrowAmount, &raw,
		); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &item.BetLevels); err != nil {
			return nil, fmt.Errorf("decode fishing venue %s bet levels: %w", item.Code, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) EnterFishing(ctx context.Context, userID int64, venueCode string) (FishingLaunch, error) {
	if userID < 1 {
		return FishingLaunch{}, errors.New("invalid user id")
	}
	venueCode = strings.ToLower(strings.TrimSpace(venueCode))
	if venueCode == "" {
		venueCode = "novice"
	}
	venue, err := s.loadFishingVenue(ctx, venueCode)
	if err != nil {
		return FishingLaunch{}, err
	}
	balance, err := s.wallet.Balance(ctx, userID)
	if err != nil {
		return FishingLaunch{}, err
	}
	if balance.Available < venue.MinBalance {
		return FishingLaunch{}, wallet.ErrInsufficientFunds
	}
	now := s.now()
	// Direct-wallet sessions never revive an expired or legacy escrow session.
	// Closing it here also releases the generated active-user key so a fresh
	// direct-wallet session can be inserted safely.
	if _, err = s.db.ExecContext(ctx, `
		UPDATE game_sessions
		SET status=4,disconnected_at=COALESCE(disconnected_at,?)
		WHERE user_id=? AND game_id=? AND status IN (1,2)
		  AND (wallet_mode<>1 OR escrow_hold_no<>'' OR expires_at<=?)`,
		now, userID, venue.GameID, now,
	); err != nil {
		return FishingLaunch{}, err
	}
	existing, found, err := s.activeFishingSession(ctx, userID, venue.GameID)
	if err != nil {
		return FishingLaunch{}, err
	}
	if found {
		return s.resumeFishingSession(ctx, existing, userID, balance.Available)
	}
	sessionID, err := idgen.New()
	if err != nil {
		return FishingLaunch{}, err
	}
	resumeToken, resumeHash, err := gameSecret(32)
	if err != nil {
		return FishingLaunch{}, err
	}
	for attempt := 0; attempt < 12; attempt++ {
		assignment, assignErr := s.matchmaker.AssignFishing(ctx, venue.VenueID, userID)
		if assignErr != nil {
			return FishingLaunch{}, assignErr
		}
		_, insertErr := s.db.ExecContext(ctx, `
			INSERT INTO game_sessions
				(id,user_id,game_id,venue_id,table_no,seat_no,resume_token_hash,
				 escrow_hold_no,wallet_mode,escrow_balance,event_seq,status,connected_at,expires_at)
			VALUES(?,?,?,?,?,?,?,'',1,?,0,1,?,?)`,
			sessionID, userID, venue.GameID, venue.VenueID, assignment.Table, assignment.Seat,
			resumeHash, balance.Available, now, now.Add(30*time.Minute),
		)
		if insertErr == nil {
			return FishingLaunch{
				SessionID: sessionID, GameCode: venue.GameCode, GameName: venue.GameName,
				EntryPath: venue.EntryPath, VenueCode: venue.VenueCode, VenueName: venue.VenueName,
				Multiplier: venue.Multiplier, Table: assignment.Table, Seat: assignment.Seat,
				EscrowAmount: 0, WalletBalance: balance.Available, ResumeToken: resumeToken,
			}, nil
		}
		_ = s.matchmaker.ReleaseFishing(ctx, venue.VenueID, userID)
		var mysqlErr *mysqlDriver.MySQLError
		if !errors.As(insertErr, &mysqlErr) || mysqlErr.Number != 1062 {
			return FishingLaunch{}, insertErr
		}
		if concurrent, concurrentFound, findErr := s.activeFishingSession(ctx, userID, venue.GameID); findErr != nil {
			return FishingLaunch{}, findErr
		} else if concurrentFound {
			return s.resumeFishingSession(ctx, concurrent, userID, balance.Available)
		}
	}
	return FishingLaunch{}, errors.New("unable to persist a free fishing seat")
}

func (s *Service) resumeFishingSession(
	ctx context.Context,
	existing FishingLaunch,
	userID int64,
	walletBalance int64,
) (FishingLaunch, error) {
	if err := s.matchmaker.ReserveFishing(
		ctx, existing.venueID, userID, existing.Table, existing.Seat,
	); err != nil {
		return FishingLaunch{}, err
	}
	resumeToken, resumeHash, err := gameSecret(32)
	if err != nil {
		return FishingLaunch{}, err
	}
	now := s.now()
	update, err := s.db.ExecContext(ctx, `
		UPDATE game_sessions
		SET resume_token_hash=?,escrow_balance=?,status=1,disconnected_at=NULL,expires_at=?
		WHERE id=? AND user_id=? AND status IN (1,2)
		  AND wallet_mode=1 AND escrow_hold_no='' AND expires_at>?`,
		resumeHash, walletBalance, now.Add(30*time.Minute), existing.SessionID, userID, now,
	)
	if err != nil {
		return FishingLaunch{}, err
	}
	if affected, affectedErr := update.RowsAffected(); affectedErr != nil || affected != 1 {
		if affectedErr != nil {
			return FishingLaunch{}, affectedErr
		}
		return FishingLaunch{}, ErrSessionNotFound
	}
	existing.EscrowAmount = 0
	existing.WalletBalance = walletBalance
	existing.ResumeToken = resumeToken
	existing.Resumed = true
	return existing, nil
}

func (s *Service) LeaveFishing(ctx context.Context, userID int64, sessionID string) (wallet.Balance, error) {
	sessionID = strings.TrimSpace(sessionID)
	if userID < 1 || sessionID == "" {
		return wallet.Balance{}, errors.New("invalid leave request")
	}
	var venueID int64
	var status, walletMode uint8
	err := s.db.QueryRowContext(ctx, `
		SELECT session.venue_id,session.status,session.wallet_mode
		FROM game_sessions session
		WHERE session.id=? AND session.user_id=?`,
		sessionID, userID,
	).Scan(&venueID, &status, &walletMode)
	if errors.Is(err, sql.ErrNoRows) {
		return wallet.Balance{}, ErrSessionNotFound
	}
	if err != nil {
		return wallet.Balance{}, err
	}
	if walletMode != 1 || (status != 1 && status != 2 && status != 3) {
		return wallet.Balance{}, errors.New("game session cannot be closed")
	}
	balance, err := s.wallet.Balance(ctx, userID)
	if err != nil {
		return wallet.Balance{}, err
	}
	if _, err = s.db.ExecContext(ctx, `
		UPDATE game_sessions SET status=3,escrow_balance=?,settled_at=?
		WHERE id=? AND user_id=? AND status IN (1,2)`,
		balance.Available, s.now(), sessionID, userID,
	); err != nil {
		return wallet.Balance{}, err
	}
	_ = s.matchmaker.ReleaseFishing(ctx, venueID, userID)
	return balance, nil
}

func (s *Service) activeFishingSession(ctx context.Context, userID, gameID int64) (FishingLaunch, bool, error) {
	var result FishingLaunch
	var venueID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT session.id,session.venue_id,session.table_no,session.seat_no,session.escrow_balance,
		       game.game_code,game.name,game.entry_path,venue.venue_code,venue.name,venue.multiplier
		FROM game_sessions session
		JOIN games game ON game.id=session.game_id
		JOIN game_venues venue ON venue.id=session.venue_id
		WHERE session.user_id=? AND session.game_id=? AND session.status IN (1,2)
		  AND session.wallet_mode=1 AND session.escrow_hold_no='' AND session.expires_at>?
		LIMIT 1`,
		userID, gameID, s.now(),
	).Scan(
		&result.SessionID, &venueID, &result.Table, &result.Seat, &result.EscrowAmount,
		&result.GameCode, &result.GameName, &result.EntryPath,
		&result.VenueCode, &result.VenueName, &result.Multiplier,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return FishingLaunch{}, false, nil
	}
	if err != nil {
		return FishingLaunch{}, false, err
	}
	result.venueID = venueID
	result.EscrowAmount = 0
	return result, true, nil
}

func (s *Service) loadFishingVenue(ctx context.Context, venueCode string) (fishingVenue, error) {
	var result fishingVenue
	err := s.db.QueryRowContext(ctx, `
		SELECT game.id,game.game_code,game.name,game.entry_path,
		       venue.id,venue.venue_code,venue.name,venue.multiplier,venue.min_balance,venue.escrow_amount
		FROM games game
		JOIN game_venues venue ON venue.game_id=game.id
		WHERE game.game_code='deepsea_hunter' AND game.status=1
		  AND venue.venue_code=? AND venue.status=1`,
		venueCode,
	).Scan(
		&result.GameID, &result.GameCode, &result.GameName, &result.EntryPath,
		&result.VenueID, &result.VenueCode, &result.VenueName, &result.Multiplier,
		&result.MinBalance, &result.EscrowAmount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return fishingVenue{}, ErrVenueNotFound
	}
	return result, err
}

func gameSecret(size int) (string, string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(value))
	return value, hex.EncodeToString(sum[:]), nil
}
