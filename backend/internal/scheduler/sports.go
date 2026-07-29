package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"math"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

func (r *Runner) settleSportsMatches(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id FROM sports_matches
		WHERE settle_status=1 AND match_status IN ('FT','CANCELLED')
		ORDER BY kickoff_at,id LIMIT 10`)
	if err != nil {
		return err
	}
	matchIDs := make([]int64, 0, 10)
	for rows.Next() {
		var matchID int64
		if err = rows.Scan(&matchID); err != nil {
			rows.Close()
			return err
		}
		matchIDs = append(matchIDs, matchID)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, matchID := range matchIDs {
		if err = r.settleSportsMatch(ctx, matchID); err != nil {
			r.logger.Error("settle sports match", "match_id", matchID, "error", err)
		}
	}
	return nil
}

func (r *Runner) settleSportsMatch(ctx context.Context, matchID int64) error {
	var publicID, matchStatus string
	var settleStatus int
	err := r.db.QueryRowContext(ctx, `
		SELECT public_match_id,match_status,settle_status
		FROM sports_matches WHERE id=?`,
		matchID,
	).Scan(&publicID, &matchStatus, &settleStatus)
	if errors.Is(err, sql.ErrNoRows) || settleStatus != 1 {
		return nil
	}
	if err != nil {
		return err
	}
	runNo, err := idgen.New()
	if err != nil {
		return err
	}
	if _, err = r.db.ExecContext(ctx, `
		INSERT INTO sports_settlement_runs(run_no,match_id,status,started_at)
		VALUES(?,?,1,CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			status=IF(status=3,status,1),
			started_at=COALESCE(started_at,CURRENT_TIMESTAMP(3)),
			error_message=''`,
		runNo, matchID,
	); err != nil {
		return err
	}
	orderRows, err := r.db.QueryContext(ctx, `
		SELECT id,order_no,user_id,hold_no,total_bet
		FROM sports_bet_orders WHERE match_id=? AND status=0 ORDER BY id`,
		matchID,
	)
	if err != nil {
		return err
	}
	type sportsOrder struct {
		ID       int64
		OrderNo  string
		UserID   int64
		HoldNo   string
		TotalBet int64
	}
	orders := make([]sportsOrder, 0, 64)
	for orderRows.Next() {
		var order sportsOrder
		if err = orderRows.Scan(
			&order.ID, &order.OrderNo, &order.UserID, &order.HoldNo, &order.TotalBet,
		); err != nil {
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
		var payout int64
		if matchStatus == "CANCELLED" {
			payout, err = r.refundSportsOrder(ctx, matchID, publicID, order)
		} else {
			payout, err = r.settleSportsOrder(ctx, matchID, publicID, order)
		}
		if err != nil {
			return r.recordSportsSettlementFailure(ctx, matchID, err)
		}
		if totalBet > math.MaxInt64-order.TotalBet || totalPayout > math.MaxInt64-payout {
			return r.recordSportsSettlementFailure(ctx, matchID, errors.New("sports settlement total overflow"))
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
		UPDATE sports_matches SET settle_status=2,bet_status=0
		WHERE id=? AND settle_status=1`,
		matchID,
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE sports_settlement_runs
		SET status=3,order_count=?,total_bet=?,total_payout=?,
		    finished_at=CURRENT_TIMESTAMP(3),error_message=''
		WHERE match_id=?`,
		len(orders), totalBet, totalPayout, matchID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Runner) settleSportsOrder(
	ctx context.Context,
	matchID int64,
	publicID string,
	order struct {
		ID       int64
		OrderNo  string
		UserID   int64
		HoldNo   string
		TotalBet int64
	},
) (int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT item.id,item.bet_amount,item.odds_scaled,market_option.result
		FROM sports_bet_items item
		JOIN sports_market_options market_option ON market_option.id=item.option_id
		WHERE item.order_id=? ORDER BY item.id`,
		order.ID,
	)
	if err != nil {
		return 0, err
	}
	type sportsItem struct {
		ID, Amount, Odds int64
		Result           int
		Payout           int64
	}
	items := make([]sportsItem, 0, 8)
	var totalPayout int64
	for rows.Next() {
		var item sportsItem
		if err = rows.Scan(&item.ID, &item.Amount, &item.Odds, &item.Result); err != nil {
			rows.Close()
			return 0, err
		}
		if item.Result != 1 && item.Result != 2 {
			rows.Close()
			return 0, errors.New("sports market option has no final result")
		}
		if item.Result == 1 {
			item.Payout, err = scaledPayout(item.Amount, item.Odds)
			if err != nil {
				rows.Close()
				return 0, err
			}
			if totalPayout > math.MaxInt64-item.Payout {
				rows.Close()
				return 0, errors.New("sports payout overflow")
			}
			totalPayout += item.Payout
		}
		items = append(items, item)
	}
	if err = rows.Close(); err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, errors.New("sports order has no bet items")
	}
	if _, err = r.wallet.CommitHold(ctx, wallet.CommitRequest{
		HoldNo: order.HoldNo, Payout: totalPayout, Description: "体育投注结算",
		Metadata: map[string]any{
			"match_id": matchID, "order_id": order.ID, "order_no": order.OrderNo,
		},
		GameCode: "sports", RoundNo: publicID,
	}); err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, item := range items {
		if _, err = tx.ExecContext(ctx, `
			UPDATE sports_bet_items SET payout_amount=?,result=? WHERE id=?`,
			item.Payout, item.Result, item.ID,
		); err != nil {
			return 0, err
		}
	}
	orderStatus := 2
	if totalPayout > 0 {
		orderStatus = 1
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE sports_bet_orders
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

func (r *Runner) refundSportsOrder(
	ctx context.Context,
	matchID int64,
	publicID string,
	order struct {
		ID       int64
		OrderNo  string
		UserID   int64
		HoldNo   string
		TotalBet int64
	},
) (int64, error) {
	if _, err := r.wallet.ReleaseHoldWithContext(ctx, wallet.ReleaseRequest{
		HoldNo: order.HoldNo, Description: "赛事取消退回",
		Metadata: map[string]any{
			"match_id": matchID, "order_id": order.ID, "order_no": order.OrderNo,
		},
		GameCode: "sports", RoundNo: publicID,
	}); err != nil && !errors.Is(err, wallet.ErrInvalidHoldState) {
		return 0, err
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE sports_bet_orders
		SET total_payout=total_bet,status=3,settled_at=CURRENT_TIMESTAMP(3)
		WHERE id=? AND status=0`,
		order.ID,
	)
	return order.TotalBet, err
}

func (r *Runner) recordSportsSettlementFailure(
	ctx context.Context,
	matchID int64,
	failure error,
) error {
	runNo, _ := idgen.New()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sports_settlement_runs
			(run_no,match_id,status,error_message,started_at,finished_at)
		VALUES(?,?,2,?,CURRENT_TIMESTAMP(3),CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			status=2,error_message=VALUES(error_message),finished_at=CURRENT_TIMESTAMP(3)`,
		runNo, matchID, truncate(failure.Error(), 1000),
	)
	if err != nil {
		return err
	}
	return failure
}
