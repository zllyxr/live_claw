package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

type SportsBetRequest struct {
	UID           int64
	MatchID       string
	ClientTraceID string
	ItemsJSON     string
}

type sportsBetInput struct {
	OptionID betInteger `json:"option_id"`
	Amount   betInteger `json:"amount"`
}

func (s *SportsService) MatchMarkets(ctx context.Context, requestID string, uid int64) (map[string]any, error) {
	now := time.Now().Unix()
	match, err := s.findMatch(ctx, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		coin, _ := s.userCoin(ctx, uid)
		return map[string]any{"match": nil, "bet_open": "0", "bet_enabled": "0", "bet_status_text": "比赛暂未采集", "close_countdown": "0", "markets": []any{}, "coin": strconv.FormatInt(coin, 10), "server_time": now, "timezone": sportsTimezoneName}, nil
	}
	if err != nil {
		return nil, err
	}
	markets, err := s.activeMarkets(ctx, s.db, match.ID)
	if err != nil {
		return nil, err
	}
	windowOpen := sportsBetWindowOpen(match, now)
	enabled := windowOpen && len(markets) > 0
	message := "可投注"
	if !enabled {
		if !windowOpen {
			message = "比赛已封盘"
		} else {
			message = "赔率同步中"
		}
	}
	coin, err := s.userCoin(ctx, uid)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"match": formatSportsMatchAt(match, now), "bet_open": map[bool]string{true: "1", false: "0"}[windowOpen],
		"bet_enabled":     map[bool]string{true: "1", false: "0"}[enabled],
		"bet_status_text": message, "close_countdown": strconv.FormatInt(max(match.BetCloseTime-now, 0), 10),
		"kickoff_ts": match.KickoffTime, "bet_close_ts": match.BetCloseTime,
		"server_time": now, "timezone": sportsTimezoneName, "timezone_offset": sportsTimezoneOffset(now),
		"markets": markets, "coin": strconv.FormatInt(coin, 10),
	}, nil
}

func (s *SportsService) activeMarkets(ctx context.Context, db queryRower, matchID int64) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.id,m.market_code,m.market_name,m.market_rule,o.id,o.option_code,o.option_name,CAST(o.odds AS CHAR)
		FROM cmf_sports_market m JOIN cmf_sports_option o ON o.market_id=m.id AND o.status=1
		WHERE m.match_id=? AND m.status=1 ORDER BY m.sort DESC,m.id,o.sort DESC,o.id`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	indexes := map[int64]int{}
	for rows.Next() {
		var marketID, optionID int64
		var marketCode, marketName, marketRule, optionCode, optionName, odds string
		if err := rows.Scan(&marketID, &marketCode, &marketName, &marketRule, &optionID, &optionCode, &optionName, &odds); err != nil {
			return nil, err
		}
		index, exists := indexes[marketID]
		if !exists {
			index = len(items)
			indexes[marketID] = index
			items = append(items, map[string]any{
				"id": strconv.FormatInt(marketID, 10), "market_code": marketCode, "market_name": marketName,
				"market_rule": marketRule, "options": []map[string]any{},
			})
		}
		options := items[index]["options"].([]map[string]any)
		items[index]["options"] = append(options, map[string]any{
			"id": strconv.FormatInt(optionID, 10), "option_code": optionCode, "option_name": optionName, "odds": odds,
		})
	}
	return items, rows.Err()
}

func (s *SportsService) PlaceBet(ctx context.Context, request SportsBetRequest) (map[string]any, error) {
	request.ClientTraceID = strings.TrimSpace(request.ClientTraceID)
	if request.ClientTraceID == "" || len(request.ClientTraceID) > 80 {
		return nil, appError(3001, "客户端订单号无效")
	}
	inputs, err := decodeBetItems[sportsBetInput](request.ItemsJSON)
	if err != nil || len(inputs) == 0 || len(inputs) > 50 {
		return nil, appError(3002, "下注内容错误")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var existingID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM cmf_sports_bet_order WHERE uid=? AND client_trace_id=?", request.UID, request.ClientTraceID).Scan(&existingID); err == nil {
		return s.formatSportsOrder(ctx, tx, existingID, true)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	match, err := findSportsMatchTx(ctx, tx, request.MatchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appError(3003, "比赛不存在或暂未采集")
	}
	if err != nil {
		return nil, err
	}
	if !sportsBetWindowOpen(match, time.Now().Unix()) {
		return nil, appError(3004, "比赛已封盘")
	}

	type optionRow struct {
		optionID, marketID                                   int64
		optionCode, optionName, odds, marketCode, marketName string
		amount                                               int64
	}
	options := make([]optionRow, 0, len(inputs))
	seen, totalBet := map[int64]bool{}, int64(0)
	for _, input := range inputs {
		optionID, amount := int64(input.OptionID), int64(input.Amount)
		if optionID < 1 || amount < 1 || seen[optionID] {
			return nil, appError(3005, "下注项错误")
		}
		seen[optionID] = true
		var option optionRow
		err := tx.QueryRowContext(ctx, `
			SELECT o.id,o.option_code,o.option_name,CAST(o.odds AS CHAR),m.id,m.market_code,m.market_name
			FROM cmf_sports_option o JOIN cmf_sports_market m ON m.id=o.market_id
			WHERE o.id=? AND o.status=1 AND m.match_id=? AND m.status=1`, optionID, match.ID).Scan(
			&option.optionID, &option.optionCode, &option.optionName, &option.odds, &option.marketID, &option.marketCode, &option.marketName)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appError(3006, "投注项不存在或已下架")
		}
		if err != nil {
			return nil, err
		}
		if !validSportsOption(option.marketCode, option.optionCode) {
			return nil, appError(3008, "投注项规则错误")
		}
		option.amount = amount
		totalBet += amount
		options = append(options, option)
	}
	if totalBet < match.MinBet {
		return nil, appError(3009, "下注金额低于最低限制")
	}
	if totalBet > match.MaxBet {
		return nil, appError(3010, "下注金额超过单次限制")
	}
	var alreadyBet int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(total_bet),0) FROM cmf_sports_bet_order WHERE uid=? AND match_id=? AND status IN (0,1,2)", request.UID, match.ID).Scan(&alreadyBet); err != nil {
		return nil, err
	}
	if alreadyBet+totalBet > match.MaxMatchBet {
		return nil, appError(3011, "下注金额超过本场限制")
	}
	result, err := tx.ExecContext(ctx, "UPDATE cmf_user SET coin=coin-?,consumption=consumption+? WHERE id=? AND user_status=1 AND coin>=?", totalBet, totalBet, request.UID, totalBet)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, appError(3012, "余额不足")
	}
	orderNo, err := newOrderNo("S", request.UID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	orderResult, err := tx.ExecContext(ctx, `
		INSERT INTO cmf_sports_bet_order(order_no,client_trace_id,uid,match_id,source_match_id,match_title,kickoff_time,total_bet,status,bet_time,create_time,update_time)
		VALUES(?,?,?,?,?,?,?, ?,0,?,?,?)`, orderNo, request.ClientTraceID, request.UID, match.ID, match.SourceMatchID, match.HomeName+" vs "+match.AwayName, match.KickoffTime, totalBet, now, now, now)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			_ = tx.Rollback()
			return s.sportsOrderByTrace(ctx, request.UID, request.ClientTraceID)
		}
		return nil, err
	}
	orderID, _ := orderResult.LastInsertId()
	for _, option := range options {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO cmf_sports_bet_item(order_id,uid,match_id,market_id,market_code,market_name,option_id,option_code,option_name,odds,bet_amount,win_status,create_time,update_time)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,0,?,?)`, orderID, request.UID, match.ID, option.marketID, option.marketCode, option.marketName, option.optionID, option.optionCode, option.optionName, option.odds, option.amount, now, now)
		if err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cmf_user_coinrecord(type,action,uid,touid,giftid,giftcount,totalcoin,showid,addtime) VALUES(0,29,?,?,?,1,?,0,?)`, request.UID, request.UID, orderID, totalBet, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.sportsOrderByID(ctx, orderID, true)
}

func findSportsMatchTx(ctx context.Context, tx *sql.Tx, requestID string) (sportsMatch, error) {
	return scanSportsMatch(tx.QueryRowContext(ctx, sportsMatchSelect+" WHERE CAST(id AS CHAR)=? OR source_match_id=? ORDER BY source_match_id=? DESC LIMIT 1 FOR UPDATE", requestID, requestID, requestID))
}

func validSportsOption(market, option string) bool {
	market, option = strings.ToUpper(market), strings.ToUpper(option)
	switch market {
	case "MATCH_RESULT":
		return option == "HOME_WIN" || option == "DRAW" || option == "AWAY_WIN"
	case "TOTAL_GOALS":
		value, ok := prefixedNumber(option, "TG_")
		return option == "OTHER" || ok && value >= 0 && value <= 15
	case "HOME_GOALS":
		value, ok := prefixedNumber(option, "HG_")
		return ok && value >= 0 && value <= 7
	case "AWAY_GOALS":
		value, ok := prefixedNumber(option, "AG_")
		return ok && value >= 0 && value <= 7
	case "CORRECT_SCORE":
		if option == "OTHER" {
			return true
		}
		parts := strings.Split(strings.TrimPrefix(option, "CS_"), "_")
		if len(parts) != 2 {
			return false
		}
		home, err1 := strconv.Atoi(parts[0])
		away, err2 := strconv.Atoi(parts[1])
		return err1 == nil && err2 == nil && home >= 0 && home <= 7 && away >= 0 && away <= 7
	default:
		return false
	}
}

func prefixedNumber(value, prefix string) (int, bool) {
	if !strings.HasPrefix(value, prefix) {
		return 0, false
	}
	number, err := strconv.Atoi(strings.TrimPrefix(value, prefix))
	return number, err == nil
}

func (s *SportsService) userCoin(ctx context.Context, uid int64) (int64, error) {
	if uid < 1 {
		return 0, nil
	}
	var coin int64
	err := s.db.QueryRowContext(ctx, "SELECT coin FROM cmf_user WHERE id=?", uid).Scan(&coin)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return coin, err
}

func (s *SportsService) sportsOrderByTrace(ctx context.Context, uid int64, traceID string) (map[string]any, error) {
	var id int64
	if err := s.db.QueryRowContext(ctx, "SELECT id FROM cmf_sports_bet_order WHERE uid=? AND client_trace_id=?", uid, traceID).Scan(&id); err != nil {
		return nil, err
	}
	return s.sportsOrderByID(ctx, id, true)
}

func (s *SportsService) sportsOrderByID(ctx context.Context, id int64, withItems bool) (map[string]any, error) {
	return s.formatSportsOrder(ctx, s.db, id, withItems)
}

func (s *SportsService) formatSportsOrder(ctx context.Context, db queryRower, orderID int64, withItems bool) (map[string]any, error) {
	var id, uid, matchID, kickoff, totalBet, totalPayout, net, betTime, settleTime int64
	var orderNo, traceID, sourceMatchID, title, remark string
	var status int
	err := db.QueryRowContext(ctx, `SELECT id,order_no,client_trace_id,uid,match_id,source_match_id,match_title,kickoff_time,total_bet,total_payout,net_amount,status,bet_time,settle_time,settle_remark FROM cmf_sports_bet_order WHERE id=?`, orderID).Scan(
		&id, &orderNo, &traceID, &uid, &matchID, &sourceMatchID, &title, &kickoff, &totalBet, &totalPayout, &net, &status, &betTime, &settleTime, &remark)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"id": strconv.FormatInt(id, 10), "order_no": orderNo, "client_trace_id": traceID, "uid": strconv.FormatInt(uid, 10),
		"match_id": strconv.FormatInt(matchID, 10), "source_match_id": sourceMatchID, "match_title": title,
		"kickoff_time": strconv.FormatInt(kickoff, 10), "kickoff_ts": kickoff, "kickoff_text": sportsTimestampText(kickoff, "01-02 15:04"),
		"bet_time_text": sportsTimestampText(betTime, "01-02 15:04:05"), "settle_time_text": sportsTimestampText(settleTime, "01-02 15:04:05"),
		"total_bet":    strconv.FormatInt(totalBet, 10),
		"total_payout": strconv.FormatInt(totalPayout, 10), "net_amount": strconv.FormatInt(net, 10),
		"status": strconv.Itoa(status), "status_text": sportsOrderStatusText(status), "bet_time": strconv.FormatInt(betTime, 10),
		"settle_time": strconv.FormatInt(settleTime, 10), "settle_remark": remark,
	}
	if withItems {
		rows, err := db.QueryContext(ctx, `SELECT id,market_id,market_code,market_name,option_id,option_code,option_name,CAST(odds AS CHAR),bet_amount,payout_amount,win_status FROM cmf_sports_bet_item WHERE order_id=? ORDER BY id`, orderID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var itemID, marketID, optionID, amount, payout int64
			var marketCode, marketName, optionCode, optionName, odds string
			var winStatus int
			if err := rows.Scan(&itemID, &marketID, &marketCode, &marketName, &optionID, &optionCode, &optionName, &odds, &amount, &payout, &winStatus); err != nil {
				return nil, err
			}
			items = append(items, map[string]any{
				"id": strconv.FormatInt(itemID, 10), "market_id": strconv.FormatInt(marketID, 10), "market_code": marketCode,
				"market_name": marketName, "option_id": strconv.FormatInt(optionID, 10), "option_code": optionCode,
				"option_name": optionName, "odds": odds, "bet_amount": strconv.FormatInt(amount, 10),
				"payout_amount": strconv.FormatInt(payout, 10), "win_status": strconv.Itoa(winStatus),
			})
		}
		result["items"] = items
	}
	return result, nil
}

func sportsOrderStatusText(status int) string {
	switch status {
	case sportsOrderWin:
		return "已中奖"
	case sportsOrderLose:
		return "未中奖"
	case sportsOrderRefund:
		return "已退款"
	case sportsOrderCanceled:
		return "已取消"
	default:
		return "待结算"
	}
}

func (s *SportsService) OrderList(ctx context.Context, uid int64, requestMatchID string, page int) (map[string]any, error) {
	var match *sportsMatch
	if strings.TrimSpace(requestMatchID) != "" {
		found, err := s.findMatch(ctx, requestMatchID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			match = &found
		}
	}
	page, limit := max(page, 1), 20
	query, args := "SELECT id FROM cmf_sports_bet_order WHERE uid=?", []any{uid}
	if match != nil {
		query += " AND match_id=?"
		args = append(args, match.ID)
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, (page-1)*limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		item, err := s.sportsOrderByID(ctx, id, true)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	summary, summaryArgs := "SELECT COUNT(*),COALESCE(SUM(total_bet),0),COALESCE(SUM(total_payout),0),COALESCE(SUM(net_amount),0) FROM cmf_sports_bet_order WHERE uid=?", []any{uid}
	if match != nil {
		summary += " AND match_id=?"
		summaryArgs = append(summaryArgs, match.ID)
	}
	var count, totalBet, totalPayout, net int64
	if err := s.db.QueryRowContext(ctx, summary, summaryArgs...).Scan(&count, &totalBet, &totalPayout, &net); err != nil {
		return nil, err
	}
	return map[string]any{
		"list": items, "page": strconv.Itoa(page), "total_count": strconv.FormatInt(count, 10),
		"total_bet": strconv.FormatInt(totalBet, 10), "total_payout": strconv.FormatInt(totalPayout, 10),
		"profit_loss": strconv.FormatInt(net, 10), "match": func() any {
			if match != nil {
				return formatSportsMatch(*match)
			}
			return nil
		}(),
	}, nil
}

var _ = fmt.Sprintf
