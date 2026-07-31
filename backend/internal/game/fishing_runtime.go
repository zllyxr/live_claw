package game

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

type FishingPlayer struct {
	SessionID     string
	UserID        int64
	Name          string
	VenueID       int64
	VenueCode     string
	VenueName     string
	VenueFactor   int
	Table         int
	Seat          int
	WalletBalance int64
	WalletVersion uint64
	EventSeq      int64
	BetLevels     []int64
	TargetRTPPPM  int
}

type FishingFireResult struct {
	CommandID     string `json:"command_id"`
	EventSeq      int64  `json:"event_seq"`
	Bet           int64  `json:"bet"`
	Reward        int64  `json:"reward"`
	Balance       int64  `json:"balance"`
	WalletVersion uint64 `json:"-"`
	Multiplier    int    `json:"multiplier"`
	Captured      bool   `json:"captured"`
	Replayed      bool   `json:"-"`
}

type fishingCheckpointPayload struct {
	CommandID       string `json:"command_id"`
	ParentCommandID string `json:"parent_command_id,omitempty"`
	Kind            string `json:"kind,omitempty"`
	Bet             int64  `json:"bet"`
	Reward          int64  `json:"reward"`
	Balance         int64  `json:"balance"`
	Multiplier      int    `json:"multiplier"`
	Captured        bool   `json:"captured"`
}

func (s *Service) AuthenticateFishingSession(
	ctx context.Context,
	sessionID string,
	resumeToken string,
) (FishingPlayer, error) {
	sessionID = strings.TrimSpace(sessionID)
	resumeToken = strings.TrimSpace(resumeToken)
	if len(sessionID) != 26 || len(resumeToken) < 32 {
		return FishingPlayer{}, ErrSessionNotFound
	}
	var result FishingPlayer
	var resumeHash string
	var betLevels []byte
	var status int
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT session.id,session.user_id,
		       COALESCE(NULLIF(user_row.nickname,''),user_row.username),
		       session.venue_id,venue.venue_code,venue.name,venue.multiplier,
		       session.table_no,session.seat_no,account.available,account.version,
		       session.event_seq,venue.bet_levels,venue.target_rtp_ppm,
		       session.resume_token_hash,session.status,session.expires_at
		FROM game_sessions session
		JOIN users user_row ON user_row.id=session.user_id
		JOIN game_venues venue ON venue.id=session.venue_id
		JOIN wallet_accounts account ON account.user_id=session.user_id AND account.currency='COIN'
		WHERE session.id=? AND session.wallet_mode=1 AND session.escrow_hold_no=''`,
		sessionID,
	).Scan(
		&result.SessionID, &result.UserID, &result.Name,
		&result.VenueID, &result.VenueCode, &result.VenueName, &result.VenueFactor,
		&result.Table, &result.Seat, &result.WalletBalance, &result.WalletVersion,
		&result.EventSeq,
		&betLevels, &result.TargetRTPPPM, &resumeHash, &status, &expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return FishingPlayer{}, ErrSessionNotFound
	}
	if err != nil {
		return FishingPlayer{}, err
	}
	sum := sha256.Sum256([]byte(resumeToken))
	suppliedHash := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(suppliedHash), []byte(resumeHash)) != 1 ||
		(status != 1 && status != 2) || !expiresAt.After(s.now()) {
		return FishingPlayer{}, ErrSessionNotFound
	}
	if err = json.Unmarshal(betLevels, &result.BetLevels); err != nil || len(result.BetLevels) == 0 {
		return FishingPlayer{}, errors.New("invalid fishing bet levels")
	}
	if err = s.MarkFishingConnected(ctx, result.SessionID, result.UserID); err != nil {
		return FishingPlayer{}, err
	}
	return result, nil
}

func (s *Service) MarkFishingConnected(ctx context.Context, sessionID string, userID int64) error {
	now := s.now()
	update, err := s.db.ExecContext(ctx, `
		UPDATE game_sessions
		SET status=1,connected_at=?,disconnected_at=NULL,expires_at=?
		WHERE id=? AND user_id=? AND status IN (1,2) AND expires_at>?
		  AND wallet_mode=1 AND escrow_hold_no=''`,
		now, now.Add(30*time.Minute), sessionID, userID, now,
	)
	if err != nil {
		return err
	}
	affected, err := update.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	var status int
	var expiresAt time.Time
	err = s.db.QueryRowContext(ctx, `
		SELECT status,expires_at FROM game_sessions WHERE id=? AND user_id=?`,
		sessionID, userID,
	).Scan(&status, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	if (status != 1 && status != 2) || !expiresAt.After(now) {
		return ErrSessionNotFound
	}
	return nil
}

func (s *Service) FireFishing(
	ctx context.Context,
	sessionID string,
	userID int64,
	commandID string,
	bet int64,
	fishMultiplier int,
) (FishingFireResult, error) {
	commandID = strings.TrimSpace(commandID)
	if len(sessionID) != 26 || userID < 1 || len(commandID) < 8 || len(commandID) > 100 ||
		bet < 1 || fishMultiplier < 0 || fishMultiplier > 200 {
		return FishingFireResult{}, errors.New("invalid fishing fire request")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return FishingFireResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var eventSeq int64
	var venueID int64
	var tableNo, status int
	var betLevelsJSON []byte
	var targetRTPPPM int
	var venueCode string
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT event_seq,status,venue_id,table_no,expires_at
		FROM game_sessions
		WHERE id=? AND user_id=? AND wallet_mode=1 AND escrow_hold_no='' FOR UPDATE`,
		sessionID, userID,
	).Scan(&eventSeq, &status, &venueID, &tableNo, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return FishingFireResult{}, ErrSessionNotFound
	}
	if err != nil {
		return FishingFireResult{}, err
	}
	now := s.now()
	if (status != 1 && status != 2) || !expiresAt.After(now) {
		return FishingFireResult{}, ErrSessionNotFound
	}
	if existing, found, existingErr := fishingCheckpointResult(ctx, tx, sessionID, commandID); existingErr != nil {
		return FishingFireResult{}, existingErr
	} else if found {
		if existing.Bet != bet || existing.CommandID != commandID || existing.Multiplier != fishMultiplier {
			return FishingFireResult{}, wallet.ErrIdempotencyReuse
		}
		current, balanceErr := s.wallet.BalanceTx(ctx, tx, userID)
		if balanceErr != nil {
			return FishingFireResult{}, balanceErr
		}
		existing.Balance = current.Available
		existing.WalletVersion = current.Version
		existing.Replayed = true
		return existing, nil
	}
	// Keep the locking read scoped to the player's mutable session row. The
	// runtime role intentionally has read-only access to venue configuration,
	// so joining game_venues into SELECT ... FOR UPDATE is rejected by MySQL.
	err = tx.QueryRowContext(ctx, `
		SELECT venue_code,bet_levels,target_rtp_ppm
		FROM game_venues
		WHERE id=?`,
		venueID,
	).Scan(&venueCode, &betLevelsJSON, &targetRTPPPM)
	if errors.Is(err, sql.ErrNoRows) {
		return FishingFireResult{}, ErrVenueNotFound
	}
	if err != nil {
		return FishingFireResult{}, err
	}
	var betLevels []int64
	if err = json.Unmarshal(betLevelsJSON, &betLevels); err != nil || !containsFishingBet(betLevels, bet) {
		return FishingFireResult{}, ErrInvalidFishingCannon
	}
	walletBusinessID := fishingWalletBusinessID(sessionID, commandID)
	if _, err = s.wallet.ApplyTx(ctx, tx, wallet.ApplyRequest{
		UserID: userID, Amount: -bet,
		BusinessType: "fishing_shot", BusinessID: walletBusinessID,
		Description: "捕鱼开炮",
		Metadata:    map[string]any{"session_id": sessionID, "command_id": commandID, "bet": bet},
		GameCode:    "deepsea_hunter", VenueCode: venueCode, TableNo: tableNo, RoundNo: sessionID,
	}); err != nil {
		return FishingFireResult{}, err
	}
	captured, err := fishingShotCaptured(targetRTPPPM, fishMultiplier)
	if err != nil {
		return FishingFireResult{}, err
	}
	reward := int64(0)
	if captured {
		if bet > math.MaxInt64/int64(fishMultiplier) {
			return FishingFireResult{}, errors.New("fishing reward overflow")
		}
		reward = bet * int64(fishMultiplier)
		if _, err = s.wallet.ApplyTx(ctx, tx, wallet.ApplyRequest{
			UserID: userID, Amount: reward,
			BusinessType: "fishing_reward", BusinessID: walletBusinessID,
			Description: "捕鱼命中奖励",
			Metadata: map[string]any{
				"session_id": sessionID, "command_id": commandID,
				"bet": bet, "multiplier": fishMultiplier,
			},
			GameCode: "deepsea_hunter", VenueCode: venueCode, TableNo: tableNo, RoundNo: sessionID,
		}); err != nil {
			return FishingFireResult{}, err
		}
	}
	current, err := s.wallet.BalanceTx(ctx, tx, userID)
	if err != nil {
		return FishingFireResult{}, err
	}
	nextBalance := current.Available
	nextSeq := eventSeq + 1
	totalCost, totalReward := int64(0), int64(0)
	previousHash := strings.Repeat("0", 64)
	err = tx.QueryRowContext(ctx, `
		SELECT total_cost,total_reward,state_hash
		FROM fishing_checkpoints
		WHERE session_id=? ORDER BY event_seq DESC LIMIT 1`,
		sessionID,
	).Scan(&totalCost, &totalReward, &previousHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return FishingFireResult{}, err
	}
	payload := fishingCheckpointPayload{
		CommandID: commandID, Kind: "shot", Bet: bet, Reward: reward, Balance: nextBalance,
		Multiplier: fishMultiplier, Captured: captured,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return FishingFireResult{}, err
	}
	hash := sha256.Sum256(append([]byte(previousHash), payloadJSON...))
	update, err := tx.ExecContext(ctx, `
		UPDATE game_sessions
		SET escrow_balance=?,event_seq=?,
		    connected_at=IF(status=2,?,connected_at),status=1,disconnected_at=NULL,expires_at=?
		WHERE id=? AND user_id=? AND status IN (1,2) AND expires_at>?
		  AND wallet_mode=1 AND escrow_hold_no=''`,
		nextBalance, nextSeq, now, now.Add(30*time.Minute), sessionID, userID, now,
	)
	if err != nil {
		return FishingFireResult{}, err
	}
	affected, err := update.RowsAffected()
	if err != nil {
		return FishingFireResult{}, err
	}
	if affected != 1 {
		return FishingFireResult{}, ErrSessionNotFound
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO fishing_checkpoints
			(session_id,event_seq,client_command_id,escrow_balance,total_cost,total_reward,
			 state_payload,state_hash)
		VALUES(?,?,?,?,?,?,?,?)`,
		sessionID, nextSeq, commandID, nextBalance, totalCost+bet, totalReward+reward,
		payloadJSON, hex.EncodeToString(hash[:]),
	); err != nil {
		return FishingFireResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return FishingFireResult{}, err
	}
	return FishingFireResult{
		CommandID: commandID, EventSeq: nextSeq, Bet: bet, Reward: reward,
		Balance: nextBalance, WalletVersion: current.Version,
		Multiplier: fishMultiplier, Captured: captured,
	}, nil
}

func fishingCheckpointResult(
	ctx context.Context,
	tx *sql.Tx,
	sessionID string,
	commandID string,
) (FishingFireResult, bool, error) {
	var payloadJSON []byte
	var eventSeq int64
	err := tx.QueryRowContext(ctx, `
		SELECT event_seq,state_payload
		FROM fishing_checkpoints
		WHERE session_id=? AND client_command_id=?`,
		sessionID, commandID,
	).Scan(&eventSeq, &payloadJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return FishingFireResult{}, false, nil
	}
	if err != nil {
		return FishingFireResult{}, false, err
	}
	var payload fishingCheckpointPayload
	if err = json.Unmarshal(payloadJSON, &payload); err != nil {
		return FishingFireResult{}, false, err
	}
	return FishingFireResult{
		CommandID: payload.CommandID, EventSeq: eventSeq,
		Bet: payload.Bet, Reward: payload.Reward, Balance: payload.Balance,
		Multiplier: payload.Multiplier, Captured: payload.Captured,
	}, true, nil
}

func fishingWalletBusinessID(sessionID string, commandID string) string {
	digest := sha256.Sum256([]byte(commandID))
	return sessionID + ":" + hex.EncodeToString(digest[:16])
}

// ResolveFishingHit settles the capture chance after an already-paid projectile
// physically touches a fish. FireFishing records the launch and deducts the
// cannon cost; this method only credits a possible reward, so a projectile can
// remain in the arena and bounce until its first real collision.
func (s *Service) ResolveFishingHit(
	ctx context.Context,
	sessionID string,
	userID int64,
	commandID string,
	fishMultiplier int,
) (FishingFireResult, error) {
	commandID = strings.TrimSpace(commandID)
	if len(sessionID) != 26 || userID < 1 || len(commandID) < 8 || len(commandID) > 100 ||
		fishMultiplier < 0 || fishMultiplier > 200 {
		return FishingFireResult{}, errors.New("invalid fishing hit request")
	}
	resolveID := fishingHitCommandID(commandID)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return FishingFireResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var eventSeq int64
	var venueID int64
	var tableNo, status int
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT event_seq,status,venue_id,table_no,expires_at
		FROM game_sessions
		WHERE id=? AND user_id=? AND wallet_mode=1 AND escrow_hold_no='' FOR UPDATE`,
		sessionID, userID,
	).Scan(&eventSeq, &status, &venueID, &tableNo, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return FishingFireResult{}, ErrSessionNotFound
	}
	if err != nil {
		return FishingFireResult{}, err
	}
	now := s.now()
	if (status != 1 && status != 2) || !expiresAt.After(now) {
		return FishingFireResult{}, ErrSessionNotFound
	}
	if existing, found, existingErr := fishingCheckpointResult(ctx, tx, sessionID, resolveID); existingErr != nil {
		return FishingFireResult{}, existingErr
	} else if found {
		current, balanceErr := s.wallet.BalanceTx(ctx, tx, userID)
		if balanceErr != nil {
			return FishingFireResult{}, balanceErr
		}
		existing.Balance = current.Available
		existing.WalletVersion = current.Version
		existing.Replayed = true
		return existing, nil
	}

	var launchPayloadJSON []byte
	err = tx.QueryRowContext(ctx, `
		SELECT state_payload
		FROM fishing_checkpoints
		WHERE session_id=? AND client_command_id=?`,
		sessionID, commandID,
	).Scan(&launchPayloadJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return FishingFireResult{}, errors.New("fishing projectile launch not found")
	}
	if err != nil {
		return FishingFireResult{}, err
	}
	var launch fishingCheckpointPayload
	if err = json.Unmarshal(launchPayloadJSON, &launch); err != nil {
		return FishingFireResult{}, err
	}
	if launch.Bet < 1 || launch.Kind != "shot" || launch.Multiplier != 0 || launch.Captured {
		return FishingFireResult{}, errors.New("invalid fishing projectile launch")
	}

	var targetRTPPPM int
	var venueCode string
	err = tx.QueryRowContext(ctx, `
		SELECT venue_code,target_rtp_ppm
		FROM game_venues
		WHERE id=?`,
		venueID,
	).Scan(&venueCode, &targetRTPPPM)
	if errors.Is(err, sql.ErrNoRows) {
		return FishingFireResult{}, ErrVenueNotFound
	}
	if err != nil {
		return FishingFireResult{}, err
	}
	captured, err := fishingShotCaptured(targetRTPPPM, fishMultiplier)
	if err != nil {
		return FishingFireResult{}, err
	}
	reward := int64(0)
	if captured {
		if launch.Bet > math.MaxInt64/int64(fishMultiplier) {
			return FishingFireResult{}, errors.New("fishing reward overflow")
		}
		reward = launch.Bet * int64(fishMultiplier)
		if _, err = s.wallet.ApplyTx(ctx, tx, wallet.ApplyRequest{
			UserID: userID, Amount: reward,
			BusinessType: "fishing_reward", BusinessID: fishingWalletBusinessID(sessionID, commandID),
			Description: "捕鱼命中奖励",
			Metadata: map[string]any{
				"session_id": sessionID, "command_id": commandID,
				"bet": launch.Bet, "multiplier": fishMultiplier,
			},
			GameCode: "deepsea_hunter", VenueCode: venueCode, TableNo: tableNo, RoundNo: sessionID,
		}); err != nil {
			return FishingFireResult{}, err
		}
	}
	current, err := s.wallet.BalanceTx(ctx, tx, userID)
	if err != nil {
		return FishingFireResult{}, err
	}
	nextBalance := current.Available
	nextSeq := eventSeq + 1

	totalCost, totalReward := int64(0), int64(0)
	previousHash := strings.Repeat("0", 64)
	err = tx.QueryRowContext(ctx, `
		SELECT total_cost,total_reward,state_hash
		FROM fishing_checkpoints
		WHERE session_id=? ORDER BY event_seq DESC LIMIT 1`,
		sessionID,
	).Scan(&totalCost, &totalReward, &previousHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return FishingFireResult{}, err
	}
	payload := fishingCheckpointPayload{
		CommandID: commandID, ParentCommandID: commandID, Kind: "hit",
		Bet: launch.Bet, Reward: reward, Balance: nextBalance,
		Multiplier: fishMultiplier, Captured: captured,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return FishingFireResult{}, err
	}
	hash := sha256.Sum256(append([]byte(previousHash), payloadJSON...))
	update, err := tx.ExecContext(ctx, `
		UPDATE game_sessions
		SET escrow_balance=?,event_seq=?,
		    connected_at=IF(status=2,?,connected_at),status=1,disconnected_at=NULL,expires_at=?
		WHERE id=? AND user_id=? AND status IN (1,2) AND expires_at>?
		  AND wallet_mode=1 AND escrow_hold_no=''`,
		nextBalance, nextSeq, now, now.Add(30*time.Minute), sessionID, userID, now,
	)
	if err != nil {
		return FishingFireResult{}, err
	}
	affected, err := update.RowsAffected()
	if err != nil {
		return FishingFireResult{}, err
	}
	if affected != 1 {
		return FishingFireResult{}, ErrSessionNotFound
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO fishing_checkpoints
			(session_id,event_seq,client_command_id,escrow_balance,total_cost,total_reward,
			 state_payload,state_hash)
		VALUES(?,?,?,?,?,?,?,?)`,
		sessionID, nextSeq, resolveID, nextBalance, totalCost, totalReward+reward,
		payloadJSON, hex.EncodeToString(hash[:]),
	); err != nil {
		return FishingFireResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return FishingFireResult{}, err
	}
	return FishingFireResult{
		CommandID: commandID, EventSeq: nextSeq, Bet: launch.Bet, Reward: reward,
		Balance: nextBalance, WalletVersion: current.Version,
		Multiplier: fishMultiplier, Captured: captured,
	}, nil
}

func fishingHitCommandID(commandID string) string {
	hash := sha256.Sum256([]byte(commandID))
	return "hit-" + hex.EncodeToString(hash[:16])
}

func (s *Service) MarkFishingDisconnected(
	ctx context.Context,
	sessionID string,
	userID int64,
) {
	_, _ = s.db.ExecContext(ctx, `
		UPDATE game_sessions
		SET status=2,disconnected_at=?,expires_at=?
		WHERE id=? AND user_id=? AND status=1 AND wallet_mode=1 AND escrow_hold_no=''`,
		s.now(), s.now().Add(30*time.Minute), sessionID, userID,
	)
}

func (s *Service) FishingWalletBalances(
	ctx context.Context,
	userIDs []int64,
) (map[int64]wallet.Balance, error) {
	return s.wallet.Balances(ctx, userIDs)
}

func containsFishingBet(levels []int64, bet int64) bool {
	for _, level := range levels {
		if level == bet {
			return true
		}
	}
	return false
}

func fishingCapture(targetRTPPPM int, multiplier int) (bool, error) {
	if targetRTPPPM < 0 {
		targetRTPPPM = 0
	}
	if targetRTPPPM > 1_000_000 {
		targetRTPPPM = 1_000_000
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return false, err
	}
	unit := float64(binary.BigEndian.Uint64(raw[:])) / float64(math.MaxUint64)
	probability := float64(targetRTPPPM) / 1_000_000 / float64(multiplier)
	return unit < probability, nil
}

func fishingShotCaptured(targetRTPPPM int, multiplier int) (bool, error) {
	if multiplier == 0 {
		return false, nil
	}
	return fishingCapture(targetRTPPPM, multiplier)
}

func (p FishingPlayer) RoomID() string {
	return fmt.Sprintf("%s:%03d", p.VenueCode, p.Table)
}
