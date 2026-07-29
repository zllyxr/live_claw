package scheduler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

type Runner struct {
	db     *sql.DB
	redis  *redis.Client
	wallet *wallet.Service
	logger *slog.Logger
	now    func() time.Time
	sports sportsSyncState
}

func New(
	db *sql.DB,
	redisClient *redis.Client,
	walletService *wallet.Service,
	logger *slog.Logger,
) *Runner {
	return &Runner{
		db: db, redis: redisClient, wallet: walletService, logger: logger, now: time.Now,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	r.runSlow(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			r.runSlow(ctx)
		}
	}
}

func (r *Runner) runSlow(ctx context.Context) {
	lock, err := r.redis.SetNX(ctx, "scheduler:v2:minute:lock", strconv.FormatInt(r.now().UnixNano(), 10), 55*time.Second).Result()
	if err != nil || !lock {
		return
	}
	defer r.redis.Del(context.Background(), "scheduler:v2:minute:lock") //nolint:errcheck
	if err = r.ensureLotteryIssues(ctx); err != nil {
		r.logger.Error("ensure lottery issues", "error", err)
	}
	if err = r.closeLotteryIssues(ctx); err != nil {
		r.logger.Error("close lottery issues", "error", err)
	}
	if err = r.drawLotteryIssues(ctx); err != nil {
		r.logger.Error("draw lottery issues", "error", err)
	}
	if err = r.settleLotteryIssues(ctx); err != nil {
		r.logger.Error("settle lottery issues", "error", err)
	}
	if err = r.runSportsSync(ctx); err != nil {
		r.logger.Error("sync sports upstream", "error", err)
	}
	if err = r.settleSportsMatches(ctx); err != nil {
		r.logger.Error("settle sports matches", "error", err)
	}
	if err = r.releaseExpiredHolds(ctx); err != nil {
		r.logger.Error("release expired holds", "error", err)
	}
	if err = r.aggregateDailyMetrics(ctx); err != nil {
		r.logger.Error("aggregate metrics", "error", err)
	}
}

func (r *Runner) closeLotteryIssues(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE lottery_issues
		SET status=2
		WHERE status IN (0,1) AND sale_close_at<=CURRENT_TIMESTAMP(3)`)
	return err
}

func (r *Runner) settleLotteryIssues(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id FROM lottery_issues
		WHERE status=3 ORDER BY draw_at,id LIMIT 500`)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, 500)
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, issueID := range ids {
		if err = r.settleLotteryIssue(ctx, issueID); err != nil {
			r.logger.Error("settle lottery issue", "issue_id", issueID, "error", err)
		}
	}
	return nil
}

func (r *Runner) settleLotteryIssue(ctx context.Context, issueID int64) error {
	var gameCode, issueNo string
	var resultJSON []byte
	var status int
	err := r.db.QueryRowContext(ctx, `
		SELECT game.game_code,issue.issue_no,issue.draw_result,issue.status
		FROM lottery_issues issue
		JOIN lottery_games game ON game.id=issue.game_id
		WHERE issue.id=?`,
		issueID,
	).Scan(&gameCode, &issueNo, &resultJSON, &status)
	if errors.Is(err, sql.ErrNoRows) || status != 3 {
		return nil
	}
	if err != nil {
		return err
	}
	winners, err := winnerOptionIDs(resultJSON)
	if err != nil {
		return r.recordSettlementFailure(ctx, issueID, err)
	}
	runNo, err := idgen.New()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO lottery_settlement_runs(run_no,issue_id,status,started_at)
		VALUES(?,?,1,CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			status=IF(status=3,status,1),started_at=COALESCE(started_at,CURRENT_TIMESTAMP(3)),
			error_message=''`,
		runNo, issueID,
	)
	if err != nil {
		return err
	}
	orderRows, err := r.db.QueryContext(ctx, `
		SELECT id,order_no,user_id,hold_no,total_bet
		FROM lottery_bet_orders
		WHERE issue_id=? AND status=0 ORDER BY id`,
		issueID,
	)
	if err != nil {
		return err
	}
	type orderRecord struct {
		ID       int64
		OrderNo  string
		UserID   int64
		HoldNo   string
		TotalBet int64
	}
	orders := make([]orderRecord, 0, 64)
	for orderRows.Next() {
		var order orderRecord
		if err = orderRows.Scan(&order.ID, &order.OrderNo, &order.UserID, &order.HoldNo, &order.TotalBet); err != nil {
			orderRows.Close()
			return err
		}
		orders = append(orders, order)
	}
	if err = orderRows.Close(); err != nil {
		return err
	}
	var totalBet, totalPayout int64
	for _, order := range orders {
		payout, settleErr := r.settleLotteryOrder(
			ctx, issueID, gameCode, issueNo, order, winners,
		)
		if settleErr != nil {
			return r.recordSettlementFailure(ctx, issueID, settleErr)
		}
		totalBet += order.TotalBet
		totalPayout += payout
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(ctx, `
		UPDATE lottery_issues SET status=4 WHERE id=? AND status=3`,
		issueID,
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE lottery_settlement_runs
		SET status=3,order_count=?,total_bet=?,total_payout=?,
		    finished_at=CURRENT_TIMESTAMP(3),error_message=''
		WHERE issue_id=?`,
		len(orders), totalBet, totalPayout, issueID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Runner) settleLotteryOrder(
	ctx context.Context,
	issueID int64,
	gameCode string,
	issueNo string,
	order struct {
		ID       int64
		OrderNo  string
		UserID   int64
		HoldNo   string
		TotalBet int64
	},
	winners map[int64]struct{},
) (int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,option_id,bet_amount,odds_scaled
		FROM lottery_bet_items WHERE order_id=? ORDER BY id`,
		order.ID,
	)
	if err != nil {
		return 0, err
	}
	type itemRecord struct {
		ID, OptionID, Amount, Odds int64
		Won                        bool
		Payout                     int64
	}
	items := make([]itemRecord, 0, 8)
	var totalPayout int64
	for rows.Next() {
		var item itemRecord
		if err = rows.Scan(&item.ID, &item.OptionID, &item.Amount, &item.Odds); err != nil {
			rows.Close()
			return 0, err
		}
		_, item.Won = winners[item.OptionID]
		if item.Won {
			item.Payout, err = scaledPayout(item.Amount, item.Odds)
			if err != nil {
				rows.Close()
				return 0, err
			}
			if totalPayout > math.MaxInt64-item.Payout {
				rows.Close()
				return 0, errors.New("lottery payout overflow")
			}
			totalPayout += item.Payout
		}
		items = append(items, item)
	}
	if err = rows.Close(); err != nil {
		return 0, err
	}
	_, err = r.wallet.CommitHold(ctx, wallet.CommitRequest{
		HoldNo: order.HoldNo, Payout: totalPayout,
		Description: "彩票结算",
		Metadata: map[string]any{
			"issue_id": issueID, "order_id": order.ID, "order_no": order.OrderNo,
		},
		GameCode: gameCode, RoundNo: issueNo,
	})
	if err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, item := range items {
		result := 2
		if item.Won {
			result = 1
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE lottery_bet_items SET payout_amount=?,result=? WHERE id=?`,
			item.Payout, result, item.ID,
		); err != nil {
			return 0, err
		}
	}
	orderStatus := 2
	if totalPayout > 0 {
		orderStatus = 1
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE lottery_bet_orders
		SET total_payout=?,status=?,settled_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND status=0`,
		totalPayout, orderStatus, order.ID,
	); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return totalPayout, nil
}

func (r *Runner) recordSettlementFailure(ctx context.Context, issueID int64, failure error) error {
	runNo, _ := idgen.New()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO lottery_settlement_runs
			(run_no,issue_id,status,error_message,started_at,finished_at)
		VALUES(?,?,2,?,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			status=2,error_message=VALUES(error_message),finished_at=CURRENT_TIMESTAMP(3)`,
		runNo, issueID, truncate(failure.Error(), 1000),
	)
	if err != nil {
		return err
	}
	return failure
}

func winnerOptionIDs(raw []byte) (map[int64]struct{}, error) {
	var payload struct {
		WinnerOptionIDs []json.Number `json:"winner_option_ids"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, errors.New("draw result must be an object with winner_option_ids")
	}
	if len(payload.WinnerOptionIDs) == 0 {
		return nil, errors.New("draw result has no winner_option_ids")
	}
	result := make(map[int64]struct{}, len(payload.WinnerOptionIDs))
	for _, rawID := range payload.WinnerOptionIDs {
		id, err := rawID.Int64()
		if err != nil || id < 1 {
			return nil, errors.New("draw result contains an invalid winner option id")
		}
		result[id] = struct{}{}
	}
	return result, nil
}

func scaledPayout(amount, oddsScaled int64) (int64, error) {
	if amount < 0 || oddsScaled < 0 {
		return 0, errors.New("invalid lottery payout input")
	}
	product := new(big.Int).Mul(big.NewInt(amount), big.NewInt(oddsScaled))
	product.Quo(product, big.NewInt(1_000_000))
	if !product.IsInt64() {
		return 0, errors.New("lottery payout overflow")
	}
	return product.Int64(), nil
}

func (r *Runner) releaseExpiredHolds(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT hold_no FROM wallet_holds
		WHERE status=0 AND expires_at<=CURRENT_TIMESTAMP(3)
		ORDER BY expires_at,id LIMIT 100`)
	if err != nil {
		return err
	}
	holds := make([]string, 0, 100)
	for rows.Next() {
		var holdNo string
		if err = rows.Scan(&holdNo); err != nil {
			rows.Close()
			return err
		}
		holds = append(holds, holdNo)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, holdNo := range holds {
		if _, err = r.wallet.ReleaseHold(ctx, holdNo, "冻结超时自动退回", map[string]any{
			"expired_by": "worker",
		}); err != nil && !errors.Is(err, wallet.ErrInvalidHoldState) {
			r.logger.Error("release expired hold", "hold_no", holdNo, "error", err)
			continue
		}
		_, _ = r.db.ExecContext(ctx, `
			UPDATE wallet_holds SET status=3
			WHERE hold_no=? AND status=2 AND expires_at<=CURRENT_TIMESTAMP(3)`,
			holdNo,
		)
	}
	return nil
}

func (r *Runner) aggregateDailyMetrics(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO metric_daily(metric_date,metric_key,metric_value)
		VALUES
			(CURRENT_DATE,'users.total',(SELECT COUNT(*) FROM users)),
			(CURRENT_DATE,'users.new_today',(SELECT COUNT(*) FROM users WHERE created_at>=CURRENT_DATE)),
			(CURRENT_DATE,'wallet.available',(SELECT COALESCE(SUM(available),0) FROM wallet_accounts)),
			(CURRENT_DATE,'wallet.frozen',(SELECT COALESCE(SUM(frozen),0) FROM wallet_accounts)),
			(CURRENT_DATE,'games.settlements_today',
			 (SELECT COUNT(*) FROM game_settlements WHERE created_at>=CURRENT_DATE)),
			(CURRENT_DATE,'lottery.bet_today',
			 (SELECT COALESCE(SUM(total_bet),0) FROM lottery_bet_orders WHERE created_at>=CURRENT_DATE)),
			(CURRENT_DATE,'sports.bet_today',
			 (SELECT COALESCE(SUM(total_bet),0) FROM sports_bet_orders WHERE created_at>=CURRENT_DATE))
		ON DUPLICATE KEY UPDATE metric_value=VALUES(metric_value)`)
	return err
}

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
