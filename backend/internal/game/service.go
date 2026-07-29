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
	ErrVenueNotFound   = errors.New("game venue not found")
	ErrSessionNotFound = errors.New("game session not found")
)

type Service struct {
	db         *sql.DB
	matchmaker *Matchmaker
	wallet     *wallet.Service
	now        func() time.Time
}

type FishingLaunch struct {
	SessionID    string `json:"session_id"`
	GameCode     string `json:"game_code"`
	GameName     string `json:"game_name"`
	EntryPath    string `json:"entry_path"`
	VenueCode    string `json:"venue_code"`
	VenueName    string `json:"venue_name"`
	Multiplier   int    `json:"multiplier"`
	Table        int    `json:"table"`
	Seat         int    `json:"seat"`
	EscrowAmount int64  `json:"escrow_amount"`
	ResumeToken  string `json:"resume_token"`
	Resumed      bool   `json:"resumed"`
	venueID      int64
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
	existing, found, err := s.activeFishingSession(ctx, userID, venue.GameID)
	if err != nil {
		return FishingLaunch{}, err
	}
	if found {
		if err = s.matchmaker.ReserveFishing(
			ctx, existing.venueID, userID, existing.Table, existing.Seat,
		); err != nil {
			return FishingLaunch{}, err
		}
		resumeToken, resumeHash, err := gameSecret(32)
		if err != nil {
			return FishingLaunch{}, err
		}
		if _, err = s.db.ExecContext(ctx, `
			UPDATE game_sessions
			SET resume_token_hash=?,status=1,disconnected_at=NULL,expires_at=?
			WHERE id=? AND user_id=? AND status IN (1,2)`,
			resumeHash, s.now().Add(30*time.Minute), existing.SessionID, userID,
		); err != nil {
			return FishingLaunch{}, err
		}
		existing.ResumeToken = resumeToken
		existing.Resumed = true
		return existing, nil
	}

	balance, err := s.wallet.Balance(ctx, userID)
	if err != nil {
		return FishingLaunch{}, err
	}
	if balance.Available < venue.MinBalance || balance.Available < venue.EscrowAmount {
		return FishingLaunch{}, wallet.ErrInsufficientFunds
	}
	sessionID, err := idgen.New()
	if err != nil {
		return FishingLaunch{}, err
	}
	resumeToken, resumeHash, err := gameSecret(32)
	if err != nil {
		return FishingLaunch{}, err
	}
	hold, err := s.wallet.PlaceHold(ctx, wallet.HoldRequest{
		UserID: userID, Amount: venue.EscrowAmount,
		BusinessType: "game_session", BusinessID: sessionID,
		ExpiresAt:   s.now().Add(45 * time.Minute),
		Description: "捕鱼场次托管",
		GameCode:    venue.GameCode, VenueCode: venue.VenueCode,
	})
	if err != nil {
		return FishingLaunch{}, err
	}
	compensateHold := true
	defer func() {
		if compensateHold {
			_, _ = s.wallet.ReleaseHold(context.Background(), hold.HoldNo, "进入捕鱼失败，退回托管", nil)
		}
	}()

	for attempt := 0; attempt < 12; attempt++ {
		assignment, assignErr := s.matchmaker.AssignFishing(ctx, venue.VenueID, userID)
		if assignErr != nil {
			return FishingLaunch{}, assignErr
		}
		_, insertErr := s.db.ExecContext(ctx, `
			INSERT INTO game_sessions
				(id,user_id,game_id,venue_id,table_no,seat_no,resume_token_hash,
				 escrow_hold_no,escrow_balance,event_seq,status,connected_at,expires_at)
			VALUES(?,?,?,?,?,?,?,?,?,0,1,?,?)`,
			sessionID, userID, venue.GameID, venue.VenueID, assignment.Table, assignment.Seat,
			resumeHash, hold.HoldNo, venue.EscrowAmount, s.now(), s.now().Add(30*time.Minute),
		)
		if insertErr == nil {
			compensateHold = false
			return FishingLaunch{
				SessionID: sessionID, GameCode: venue.GameCode, GameName: venue.GameName,
				EntryPath: venue.EntryPath, VenueCode: venue.VenueCode, VenueName: venue.VenueName,
				Multiplier: venue.Multiplier, Table: assignment.Table, Seat: assignment.Seat,
				EscrowAmount: venue.EscrowAmount, ResumeToken: resumeToken,
			}, nil
		}
		_ = s.matchmaker.ReleaseFishing(ctx, venue.VenueID, userID)
		var mysqlErr *mysqlDriver.MySQLError
		if !errors.As(insertErr, &mysqlErr) || mysqlErr.Number != 1062 {
			return FishingLaunch{}, insertErr
		}
	}
	return FishingLaunch{}, errors.New("unable to persist a free fishing seat")
}

func (s *Service) LeaveFishing(ctx context.Context, userID int64, sessionID string) (wallet.Entry, error) {
	sessionID = strings.TrimSpace(sessionID)
	if userID < 1 || sessionID == "" {
		return wallet.Entry{}, errors.New("invalid leave request")
	}
	var holdNo, gameCode, venueCode string
	var venueID int64
	var tableNo int
	var escrowBalance int64
	var status uint8
	err := s.db.QueryRowContext(ctx, `
		SELECT session.escrow_hold_no,session.venue_id,session.table_no,session.escrow_balance,
		       session.status,game.game_code,venue.venue_code
		FROM game_sessions session
		JOIN games game ON game.id=session.game_id
		JOIN game_venues venue ON venue.id=session.venue_id
		WHERE session.id=? AND session.user_id=?`,
		sessionID, userID,
	).Scan(&holdNo, &venueID, &tableNo, &escrowBalance, &status, &gameCode, &venueCode)
	if errors.Is(err, sql.ErrNoRows) {
		return wallet.Entry{}, ErrSessionNotFound
	}
	if err != nil {
		return wallet.Entry{}, err
	}
	if status != 1 && status != 2 && status != 3 {
		return wallet.Entry{}, errors.New("game session cannot be settled")
	}
	if status != 3 {
		var checkpointBalance int64
		checkpointErr := s.db.QueryRowContext(ctx, `
			SELECT escrow_balance FROM fishing_checkpoints
			WHERE session_id=? ORDER BY event_seq DESC LIMIT 1`,
			sessionID,
		).Scan(&checkpointBalance)
		if checkpointErr == nil {
			escrowBalance = checkpointBalance
		} else if !errors.Is(checkpointErr, sql.ErrNoRows) {
			return wallet.Entry{}, checkpointErr
		}
	}
	entry, err := s.wallet.CommitHold(ctx, wallet.CommitRequest{
		HoldNo: holdNo, Payout: escrowBalance, Description: "捕鱼场次结算",
		GameCode: gameCode, VenueCode: venueCode, TableNo: tableNo, RoundNo: sessionID,
		Metadata: map[string]any{"session_id": sessionID},
	})
	if err != nil {
		return wallet.Entry{}, err
	}
	if _, err = s.db.ExecContext(ctx, `
		UPDATE game_sessions SET status=3,escrow_balance=?,settled_at=?
		WHERE id=? AND user_id=? AND status IN (1,2)`,
		escrowBalance, s.now(), sessionID, userID,
	); err != nil {
		return wallet.Entry{}, err
	}
	_ = s.matchmaker.ReleaseFishing(ctx, venueID, userID)
	return entry, nil
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
		LIMIT 1`,
		userID, gameID,
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
