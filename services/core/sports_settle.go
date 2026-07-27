package main

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

func (s *SportsService) settleDueMatches(ctx context.Context, limit int) (int, error) {
	rows, err := s.db.QueryContext(ctx, sportsMatchSelect+" WHERE settle_status=0 AND (status IN ('FT','AET','PEN','AWD','WO','Fin','Final','Res','CANC','ABD','SUSP','PST')) ORDER BY kickoff_time,id LIMIT ?", limit)
	if err != nil {
		return 0, err
	}
	matches := make([]sportsMatch, 0, limit)
	for rows.Next() {
		match, err := scanSportsMatch(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		matches = append(matches, match)
	}
	rows.Close()
	count := 0
	for _, match := range matches {
		var settled bool
		if isCanceledSportsStatus(match.Status) {
			settled, err = s.refundSportsMatch(ctx, match.ID, "比赛取消或延期")
		} else if match.HomeScore >= 0 && match.AwayScore >= 0 && isFinishedSportsStatus(match.Status) {
			settled, err = s.settleSportsMatch(ctx, match.ID)
		}
		if err != nil {
			return count, err
		}
		if settled {
			count++
		}
	}
	return count, nil
}

func (s *SportsService) settleSportsMatch(ctx context.Context, matchID int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	match, err := scanSportsMatch(tx.QueryRowContext(ctx, sportsMatchSelect+" WHERE id=? FOR UPDATE", matchID))
	if err != nil {
		return false, err
	}
	if match.SettleStatus != 0 || match.HomeScore < 0 || match.AwayScore < 0 || !isFinishedSportsStatus(match.Status) {
		return false, nil
	}
	orders, err := lockedSportsOrders(ctx, tx, match.ID)
	if err != nil {
		return false, err
	}
	now := time.Now().Unix()
	var winCount, loseCount, payoutTotal int64
	for _, order := range orders {
		rows, err := tx.QueryContext(ctx, `SELECT id,market_code,option_code,CAST(odds AS CHAR),bet_amount FROM cmf_sports_bet_item WHERE order_id=? FOR UPDATE`, order.id)
		if err != nil {
			return false, err
		}
		type settleItem struct {
			id, amount           int64
			market, option, odds string
		}
		items := make([]settleItem, 0)
		for rows.Next() {
			var item settleItem
			if err := rows.Scan(&item.id, &item.market, &item.option, &item.odds, &item.amount); err != nil {
				rows.Close()
				return false, err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return false, err
		}
		rows.Close()
		var payoutTotalForOrder int64
		for _, item := range items {
			win := isSportsWin(match.HomeScore, match.AwayScore, item.market, item.option)
			payout, itemStatus := int64(0), sportsOrderLose
			if win {
				scaled, err := parseOddsScaled(item.odds)
				if err != nil {
					return false, err
				}
				payout = item.amount * scaled / 10000
				itemStatus = sportsOrderWin
			}
			payoutTotalForOrder += payout
			if _, err := tx.ExecContext(ctx, "UPDATE cmf_sports_bet_item SET payout_amount=?,win_status=?,update_time=? WHERE id=?", payout, itemStatus, now, item.id); err != nil {
				return false, err
			}
		}
		orderStatus := sportsOrderLose
		if payoutTotalForOrder > 0 {
			orderStatus = sportsOrderWin
			winCount++
			payoutTotal += payoutTotalForOrder
			if _, err := tx.ExecContext(ctx, "UPDATE cmf_user SET coin=coin+? WHERE id=?", payoutTotalForOrder, order.uid); err != nil {
				return false, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO cmf_user_coinrecord(type,action,uid,touid,giftid,giftcount,totalcoin,showid,addtime) VALUES(1,31,?,?,?,1,?,0,?)`, order.uid, order.uid, order.id, payoutTotalForOrder, now); err != nil {
				return false, err
			}
		} else {
			loseCount++
		}
		if _, err := tx.ExecContext(ctx, `UPDATE cmf_sports_bet_order SET total_payout=?,net_amount=?,status=?,settle_time=?,update_time=? WHERE id=? AND status=0`, payoutTotalForOrder, payoutTotalForOrder-order.totalBet, orderStatus, now, now, order.id); err != nil {
			return false, err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE cmf_sports_match SET bet_status=2,settle_status=1,settle_time=?,settle_remark='ok',update_time=? WHERE id=?", now, now, match.ID); err != nil {
		return false, err
	}
	settleKey := "match_" + strconv.FormatInt(match.ID, 10) + "_" + strconv.Itoa(match.HomeScore) + "_" + strconv.Itoa(match.AwayScore)
	_, err = tx.ExecContext(ctx, `INSERT INTO cmf_sports_settle_log(match_id,settle_key,home_score,away_score,orders_total,orders_win,orders_lose,orders_refund,payout_total,success,message,create_time) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, match.ID, settleKey, match.HomeScore, match.AwayScore, len(orders), winCount, loseCount, 0, payoutTotal, 1, "ok", now)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

type sportsLockedOrder struct{ id, uid, totalBet int64 }

func lockedSportsOrders(ctx context.Context, tx *sql.Tx, matchID int64) ([]sportsLockedOrder, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id,uid,total_bet FROM cmf_sports_bet_order WHERE match_id=? AND status=0 ORDER BY id FOR UPDATE", matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := []sportsLockedOrder{}
	for rows.Next() {
		var order sportsLockedOrder
		if err := rows.Scan(&order.id, &order.uid, &order.totalBet); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *SportsService) refundSportsMatch(ctx context.Context, matchID int64, reason string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	match, err := scanSportsMatch(tx.QueryRowContext(ctx, sportsMatchSelect+" WHERE id=? FOR UPDATE", matchID))
	if err != nil {
		return false, err
	}
	if match.SettleStatus != 0 {
		return false, nil
	}
	orders, err := lockedSportsOrders(ctx, tx, match.ID)
	if err != nil {
		return false, err
	}
	now, refundTotal := time.Now().Unix(), int64(0)
	for _, order := range orders {
		refundTotal += order.totalBet
		if _, err := tx.ExecContext(ctx, "UPDATE cmf_user SET coin=coin+? WHERE id=?", order.totalBet, order.uid); err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE cmf_sports_bet_item SET payout_amount=bet_amount,win_status=3,update_time=? WHERE order_id=?", now, order.id); err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE cmf_sports_bet_order SET total_payout=total_bet,net_amount=0,status=3,settle_time=?,settle_remark=?,update_time=? WHERE id=? AND status=0", now, reason, now, order.id); err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cmf_user_coinrecord(type,action,uid,touid,giftid,giftcount,totalcoin,showid,addtime) VALUES(1,30,?,?,?,1,?,0,?)`, order.uid, order.uid, order.id, order.totalBet, now); err != nil {
			return false, err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE cmf_sports_match SET bet_status=2,settle_status=2,settle_time=?,settle_remark=?,update_time=? WHERE id=?", now, reason, now, match.ID); err != nil {
		return false, err
	}
	settleKey := "refund_" + strconv.FormatInt(match.ID, 10)
	_, err = tx.ExecContext(ctx, `INSERT INTO cmf_sports_settle_log(match_id,settle_key,home_score,away_score,orders_total,orders_win,orders_lose,orders_refund,payout_total,success,message,create_time) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, match.ID, settleKey, match.HomeScore, match.AwayScore, len(orders), 0, 0, len(orders), refundTotal, 1, reason, now)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func isSportsWin(home, away int, market, option string) bool {
	market, option = strings.ToUpper(market), strings.ToUpper(option)
	switch market {
	case "MATCH_RESULT":
		return option == "HOME_WIN" && home > away || option == "DRAW" && home == away || option == "AWAY_WIN" && home < away
	case "TOTAL_GOALS":
		total := home + away
		if option == "OTHER" {
			return total >= 16
		}
		value, ok := prefixedNumber(option, "TG_")
		return ok && value == total
	case "HOME_GOALS":
		value, ok := prefixedNumber(option, "HG_")
		return ok && value == home
	case "AWAY_GOALS":
		value, ok := prefixedNumber(option, "AG_")
		return ok && value == away
	case "CORRECT_SCORE":
		if option == "OTHER" {
			return home > 7 || away > 7
		}
		return option == "CS_"+strconv.Itoa(home)+"_"+strconv.Itoa(away)
	default:
		return false
	}
}

var _ = errors.Is
