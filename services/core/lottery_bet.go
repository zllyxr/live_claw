package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type LotteryBetRequest struct {
	UID           int64
	GameID        int64
	IssueID       int64
	ClientTraceID string
	ItemsJSON     string
}

type lotteryBetInput struct {
	OptionID betInteger `json:"option_id"`
	Amount   betInteger `json:"amount"`
}

type lotteryBetOption struct {
	OptionID   int64
	OptionCode string
	OptionName string
	Odds       string
	PlayID     int64
	PlayCode   string
	Amount     int64
}

func (s *LotteryService) PlaceBet(ctx context.Context, request LotteryBetRequest) (map[string]any, error) {
	request.ClientTraceID = strings.TrimSpace(request.ClientTraceID)
	if request.ClientTraceID == "" || len(request.ClientTraceID) > 80 {
		return nil, appError(1001, "客户端订单号无效")
	}
	inputs, err := decodeBetItems[lotteryBetInput](request.ItemsJSON)
	if err != nil || len(inputs) == 0 || len(inputs) > 50 {
		return nil, appError(1002, "下注内容错误")
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if existingID, err := existingLotteryOrder(ctx, tx, request.UID, request.ClientTraceID); err == nil {
		return s.formatLotteryOrderTx(ctx, tx, existingID, true)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var game lotteryGame
	err = tx.QueryRowContext(ctx, `
		SELECT id,category_id,game_code,game_name,game_name_en,icon,COALESCE(description,''),COALESCE(rule_desc,''),
		       interval_sec,seal_advance_sec,min_bet,max_bet,max_issue_bet,status
		FROM cmf_lottery_game WHERE id=? AND status=1 FOR UPDATE`, request.GameID).Scan(
		&game.ID, &game.CategoryID, &game.Code, &game.Name, &game.NameEN, &game.Icon, &game.Description, &game.RuleDesc,
		&game.IntervalSec, &game.SealAdvanceSec, &game.MinBet, &game.MaxBet, &game.MaxIssueBet, &game.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appError(1003, "游戏不存在或维护中")
	}
	if err != nil {
		return nil, err
	}

	var issue lotteryIssue
	err = tx.QueryRowContext(ctx, `
		SELECT id,game_id,issue_num,open_code,open_time,seal_time,next_open_time,status
		FROM cmf_lottery_issue WHERE id=? AND game_id=? FOR UPDATE`, request.IssueID, request.GameID).Scan(
		&issue.ID, &issue.GameID, &issue.IssueNum, &issue.OpenCode, &issue.OpenTime, &issue.SealTime, &issue.NextOpenTime, &issue.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appError(1004, "期号不存在")
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	if issue.Status != lotteryIssueOpen || issue.SealTime <= now {
		return nil, appError(1005, "本期已封盘")
	}

	options := make([]lotteryBetOption, 0, len(inputs))
	var totalBet int64
	seen := map[int64]bool{}
	for _, input := range inputs {
		optionID, amount := int64(input.OptionID), int64(input.Amount)
		if optionID < 1 || amount < 1 || seen[optionID] {
			return nil, appError(1006, "下注项错误")
		}
		seen[optionID] = true
		var option lotteryBetOption
		err := tx.QueryRowContext(ctx, `
			SELECT o.id,o.option_code,o.option_name,CAST(o.odds AS CHAR),p.id,p.play_code
			FROM cmf_lottery_option o JOIN cmf_lottery_play p ON p.id=o.play_id
			WHERE o.id=? AND o.status=1 AND p.game_id=? AND p.status=1`, optionID, request.GameID).Scan(
			&option.OptionID, &option.OptionCode, &option.OptionName, &option.Odds, &option.PlayID, &option.PlayCode,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appError(1007, "玩法选项不存在")
		}
		if err != nil {
			return nil, err
		}
		option.Amount = amount
		if totalBet > game.MaxBet-amount {
			return nil, appError(1010, "下注金额超过单次限制")
		}
		totalBet += amount
		options = append(options, option)
	}
	if totalBet < game.MinBet {
		return nil, appError(1009, "下注金额低于最低限制")
	}
	if totalBet > game.MaxBet {
		return nil, appError(1010, "下注金额超过单次限制")
	}

	var issueBet int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_bet),0) FROM cmf_lottery_bet_order WHERE uid=? AND game_id=? AND issue_id=? AND status IN (0,1,2)`, request.UID, request.GameID, request.IssueID).Scan(&issueBet); err != nil {
		return nil, err
	}
	if issueBet+totalBet > game.MaxIssueBet {
		return nil, appError(1011, "下注金额超过本期限制")
	}

	result, err := tx.ExecContext(ctx, "UPDATE cmf_user SET coin=coin-?,consumption=consumption+? WHERE id=? AND user_status=1 AND coin>=?", totalBet, totalBet, request.UID, totalBet)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, appError(1012, "余额不足")
	}

	orderNo, err := newOrderNo("L", request.UID)
	if err != nil {
		return nil, err
	}
	orderResult, err := tx.ExecContext(ctx, `
		INSERT INTO cmf_lottery_bet_order
		(order_no,client_trace_id,uid,game_id,issue_id,issue_num,total_bet,status,bet_time,create_time,update_time)
		VALUES (?,?,?,?,?,?,?,0,?,?,?)`, orderNo, request.ClientTraceID, request.UID, request.GameID, request.IssueID, issue.IssueNum, totalBet, now, now, now)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			_ = tx.Rollback()
			return s.lotteryOrderByTrace(ctx, request.UID, request.ClientTraceID)
		}
		return nil, err
	}
	orderID, _ := orderResult.LastInsertId()
	for _, option := range options {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO cmf_lottery_bet_item
			(order_id,uid,game_id,issue_id,play_id,play_code,option_id,option_code,option_name,odds,bet_amount,win_status,create_time,update_time)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,0,?,?)`,
			orderID, request.UID, request.GameID, request.IssueID, option.PlayID, option.PlayCode,
			option.OptionID, option.OptionCode, option.OptionName, option.Odds, option.Amount, now, now,
		)
		if err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO cmf_user_coinrecord(type,action,uid,touid,giftid,giftcount,totalcoin,showid,addtime)
		VALUES(0,19,?,?,?,1,?,0,?)`, request.UID, request.UID, orderID, totalBet, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.lotteryOrderByID(ctx, orderID, true)
}

func existingLotteryOrder(ctx context.Context, tx *sql.Tx, uid int64, traceID string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM cmf_lottery_bet_order WHERE uid=? AND client_trace_id=? LIMIT 1", uid, traceID).Scan(&id)
	return id, err
}

func newOrderNo(prefix string, uid int64) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s%06d%s", prefix, time.Now().UTC().Format("20060102150405"), uid%1000000, strings.ToUpper(hex.EncodeToString(random))), nil
}

func (s *LotteryService) lotteryOrderByTrace(ctx context.Context, uid int64, traceID string) (map[string]any, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, "SELECT id FROM cmf_lottery_bet_order WHERE uid=? AND client_trace_id=?", uid, traceID).Scan(&id); err != nil {
		return nil, err
	}
	return s.lotteryOrderByID(ctx, id, true)
}

func (s *LotteryService) lotteryOrderByID(ctx context.Context, orderID int64, withItems bool) (map[string]any, error) {
	return s.formatLotteryOrderRow(ctx, s.db, orderID, withItems)
}

func (s *LotteryService) formatLotteryOrderTx(ctx context.Context, tx *sql.Tx, orderID int64, withItems bool) (map[string]any, error) {
	return s.formatLotteryOrderRow(ctx, tx, orderID, withItems)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *LotteryService) formatLotteryOrderRow(ctx context.Context, db queryRower, orderID int64, withItems bool) (map[string]any, error) {
	var id, uid, gameID, issueID, totalBet, totalPayout, netAmount, betTime, settleTime int64
	var orderNo, traceID, issueNum, gameName, gameCode, remark string
	var status int
	err := db.QueryRowContext(ctx, `
		SELECT o.id,o.order_no,o.client_trace_id,o.uid,o.game_id,o.issue_id,o.issue_num,o.total_bet,o.total_payout,o.net_amount,
		       o.status,o.bet_time,o.settle_time,o.settle_remark,g.game_name,g.game_code
		FROM cmf_lottery_bet_order o JOIN cmf_lottery_game g ON g.id=o.game_id WHERE o.id=?`, orderID).Scan(
		&id, &orderNo, &traceID, &uid, &gameID, &issueID, &issueNum, &totalBet, &totalPayout, &netAmount,
		&status, &betTime, &settleTime, &remark, &gameName, &gameCode,
	)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"id": strconv.FormatInt(id, 10), "order_no": orderNo, "client_trace_id": traceID, "uid": strconv.FormatInt(uid, 10),
		"game_id": strconv.FormatInt(gameID, 10), "game_name": gameName, "game_code": gameCode,
		"issue_id": strconv.FormatInt(issueID, 10), "issue_num": issueNum, "total_bet": strconv.FormatInt(totalBet, 10),
		"total_payout": strconv.FormatInt(totalPayout, 10), "net_amount": strconv.FormatInt(netAmount, 10),
		"status": strconv.Itoa(status), "status_text": lotteryOrderStatusText(status), "bet_time": strconv.FormatInt(betTime, 10),
		"settle_time": strconv.FormatInt(settleTime, 10), "settle_remark": remark,
	}
	if withItems {
		rows, err := db.QueryContext(ctx, `
			SELECT id,play_id,play_code,option_id,option_code,option_name,CAST(odds AS CHAR),bet_amount,payout_amount,win_status
			FROM cmf_lottery_bet_item WHERE order_id=? ORDER BY id`, orderID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var itemID, playID, optionID, amount, payout int64
			var playCode, optionCode, optionName, odds string
			var winStatus int
			if err := rows.Scan(&itemID, &playID, &playCode, &optionID, &optionCode, &optionName, &odds, &amount, &payout, &winStatus); err != nil {
				return nil, err
			}
			items = append(items, map[string]any{
				"id": strconv.FormatInt(itemID, 10), "play_id": strconv.FormatInt(playID, 10), "play_code": playCode,
				"option_id": strconv.FormatInt(optionID, 10), "option_code": optionCode, "option_name": optionName, "odds": odds,
				"bet_amount": strconv.FormatInt(amount, 10), "payout_amount": strconv.FormatInt(payout, 10), "win_status": strconv.Itoa(winStatus),
			})
		}
		result["items"] = items
	}
	return result, nil
}

func lotteryOrderStatusText(status int) string {
	switch status {
	case lotteryOrderWin:
		return "已中奖"
	case lotteryOrderLose:
		return "未中奖"
	case lotteryOrderRefund:
		return "已退款"
	default:
		return "待开奖"
	}
}

func (s *LotteryService) OrderList(ctx context.Context, uid, gameID int64, gameCode string, page int) (map[string]any, error) {
	if gameID == 0 && strings.TrimSpace(gameCode) != "" {
		game, err := s.getGame(ctx, 0, gameCode)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			gameID = game.ID
		}
	}
	page = max(page, 1)
	limit := 20
	offset := (page - 1) * limit
	query := "SELECT id FROM cmf_lottery_bet_order WHERE uid=?"
	args := []any{uid}
	if gameID > 0 {
		query += " AND game_id=?"
		args = append(args, gameID)
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		item, err := s.lotteryOrderByID(ctx, id, true)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	summaryQuery := "SELECT COUNT(*),COALESCE(SUM(total_bet),0),COALESCE(SUM(total_payout),0),COALESCE(SUM(net_amount),0) FROM cmf_lottery_bet_order WHERE uid=?"
	summaryArgs := []any{uid}
	if gameID > 0 {
		summaryQuery += " AND game_id=?"
		summaryArgs = append(summaryArgs, gameID)
	}
	var count, totalBet, totalPayout, net int64
	if err := s.db.QueryRowContext(ctx, summaryQuery, summaryArgs...).Scan(&count, &totalBet, &totalPayout, &net); err != nil {
		return nil, err
	}
	coin, err := s.userCoin(ctx, uid)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"list": items, "page": strconv.Itoa(page), "total_count": strconv.FormatInt(count, 10),
		"total_bet": strconv.FormatInt(totalBet, 10), "total_payout": strconv.FormatInt(totalPayout, 10),
		"profit_loss": strconv.FormatInt(net, 10), "coin": strconv.FormatInt(coin, 10),
	}, nil
}
