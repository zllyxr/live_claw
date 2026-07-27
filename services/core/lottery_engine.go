package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (s *LotteryService) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	nextEnsure := time.Time{}
	for {
		now := time.Now()
		if now.After(nextEnsure) {
			if err := s.ensureFutureIssues(ctx, now.Unix()); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("lottery ensure issues", "error", err)
				s.setStatus(map[string]any{"state": "degraded", "last_error": err.Error()})
			}
			nextEnsure = now.Add(10 * time.Second)
		}
		opened, drawErr := s.drawDueIssues(ctx, now.Unix(), 200)
		settled, settleErr := s.settleDueIssues(ctx, 200)
		if drawErr != nil && !errors.Is(drawErr, context.Canceled) {
			s.logger.Error("lottery draw", "error", drawErr)
		}
		if settleErr != nil && !errors.Is(settleErr, context.Canceled) {
			s.logger.Error("lottery settle", "error", settleErr)
		}
		state := "running"
		lastError := ""
		if drawErr != nil || settleErr != nil {
			state = "degraded"
			if drawErr != nil {
				lastError = drawErr.Error()
			} else {
				lastError = settleErr.Error()
			}
		}
		s.setStatus(map[string]any{
			"state": state, "last_tick": now.Format(time.RFC3339), "last_error": lastError,
			"opened": opened, "settled": settled,
		})
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *LotteryService) ensureFutureIssues(ctx context.Context, now int64) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id,g.interval_sec,g.seal_advance_sec,c.template_code,c.draw_count,c.number_min,c.number_max,c.number_unique,c.number_pad,c.sum_big_threshold,c.status
		FROM cmf_lottery_game g JOIN cmf_lottery_draw_config c ON c.game_id=g.id
		WHERE g.status=1 AND c.status=1`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type gameConfig struct {
		gameID, interval, seal int64
		config                 drawConfig
	}
	configs := make([]gameConfig, 0)
	for rows.Next() {
		var item gameConfig
		var unique int
		if err := rows.Scan(&item.gameID, &item.interval, &item.seal, &item.config.Template, &item.config.Count, &item.config.Min, &item.config.Max, &unique, &item.config.Pad, &item.config.SumBig, &item.config.ConfigState); err != nil {
			return err
		}
		item.config.GameID = item.gameID
		item.config.Unique = unique == 1
		configs = append(configs, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range configs {
		interval := max(item.interval, 1)
		firstOpen := ((now+item.seal)/interval + 1) * interval
		for index := int64(0); index < 8; index++ {
			openTime := firstOpen + index*interval
			sealTime := openTime - item.seal
			issueNum := localIssueNumber(openTime, interval)
			_, err := s.db.ExecContext(ctx, `
				INSERT IGNORE INTO cmf_lottery_issue
				(game_id,issue_num,open_time,seal_time,source_time,next_open_time,status,sync_time,create_time,update_time)
				VALUES(?,?,?,?,'local',?,0,?,?,?)`, item.gameID, issueNum, openTime, sealTime, openTime+interval, now, now, now)
			if err != nil {
				return err
			}
		}
	}
	_, err = s.db.ExecContext(ctx, `UPDATE cmf_lottery_issue SET status=4,update_time=? WHERE status IN (0,1) AND open_code='' AND open_time<?`, now, now-86400)
	return err
}

func localIssueNumber(openTime, interval int64) string {
	t := time.Unix(openTime, 0)
	location, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		t = t.In(location)
	}
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
	sequence := (openTime-dayStart)/max(interval, 1) + 1
	return fmt.Sprintf("%s%05d", t.Format("20060102"), sequence)
}

func (s *LotteryService) drawDueIssues(ctx context.Context, now int64, limit int) (int, error) {
	if _, err := s.db.ExecContext(ctx, "UPDATE cmf_lottery_issue SET status=1,update_time=? WHERE status=0 AND seal_time<=? AND open_code=''", now, now); err != nil {
		return 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cmf_lottery_issue WHERE status IN (0,1) AND open_time<=? AND open_code='' ORDER BY open_time,id LIMIT ?`, now, limit)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	opened := 0
	for _, id := range ids {
		ok, err := s.drawIssue(ctx, id, now)
		if err != nil {
			return opened, err
		}
		if ok {
			opened++
		}
	}
	return opened, nil
}

func (s *LotteryService) drawIssue(ctx context.Context, issueID, now int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var issue lotteryIssue
	err = tx.QueryRowContext(ctx, `
		SELECT id,game_id,issue_num,open_code,open_time,seal_time,next_open_time,status
		FROM cmf_lottery_issue WHERE id=? FOR UPDATE`, issueID).Scan(
		&issue.ID, &issue.GameID, &issue.IssueNum, &issue.OpenCode, &issue.OpenTime, &issue.SealTime, &issue.NextOpenTime, &issue.Status)
	if err != nil {
		return false, err
	}
	if issue.OpenCode != "" || issue.OpenTime > now || (issue.Status != lotteryIssueOpen && issue.Status != lotteryIssueSealed) {
		return false, nil
	}
	var config drawConfig
	var unique int
	err = tx.QueryRowContext(ctx, `
		SELECT game_id,template_code,draw_count,number_min,number_max,number_unique,number_pad,sum_big_threshold,status
		FROM cmf_lottery_draw_config WHERE game_id=? AND status=1 FOR UPDATE`, issue.GameID).Scan(
		&config.GameID, &config.Template, &config.Count, &config.Min, &config.Max, &unique, &config.Pad, &config.SumBig, &config.ConfigState)
	if err != nil {
		return false, err
	}
	config.Unique = unique == 1

	openCode, source := "", "crypto_random"
	var presetID int64
	err = tx.QueryRowContext(ctx, `SELECT id,open_code FROM cmf_lottery_preset_draw WHERE game_id=? AND issue_num=? AND status=1 ORDER BY id DESC LIMIT 1 FOR UPDATE`, issue.GameID, issue.IssueNum).Scan(&presetID, &openCode)
	if err == nil {
		source = "preset"
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if openCode == "" {
		openCode, err = generateOpenCode(config)
		if err != nil {
			return false, err
		}
	}
	if !validateDraw(openCode, config) {
		if presetID > 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE cmf_lottery_preset_draw SET status=3,update_time=? WHERE id=?", now, presetID)
		}
		return false, fmt.Errorf("invalid draw for game %d issue %s", issue.GameID, issue.IssueNum)
	}

	result, err := tx.ExecContext(ctx, `UPDATE cmf_lottery_issue SET open_code=?,status=2,sync_time=?,source_time='local',update_time=? WHERE id=? AND open_code='' AND status IN (0,1)`, openCode, now, now, issue.ID)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return false, nil
	}
	_, err = tx.ExecContext(ctx, `UPDATE cmf_lottery_game SET last_issue_num=?,last_open_code=?,last_open_time=?,update_time=? WHERE id=?`, issue.IssueNum, openCode, issue.OpenTime, now, issue.GameID)
	if err != nil {
		return false, err
	}
	if presetID > 0 {
		_, err = tx.ExecContext(ctx, "UPDATE cmf_lottery_preset_draw SET status=2,use_time=?,update_time=? WHERE id=?", now, now, presetID)
		if err != nil {
			return false, err
		}
	}
	auditEntropy := make([]byte, 32)
	if _, err := rand.Read(auditEntropy); err != nil {
		return false, err
	}
	hash := sha256.Sum256(append(auditEntropy, []byte(openCode)...))
	_, err = tx.ExecContext(ctx, `
		INSERT INTO cmf_lottery_draw_audit(issue_id,game_id,issue_num,draw_source,open_code,entropy_hash,engine_version,create_time)
		VALUES(?,?,?,?,?,?,'go-v1',?)`, issue.ID, issue.GameID, issue.IssueNum, source, openCode, hex.EncodeToString(hash[:]), now)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func validateDraw(openCode string, config drawConfig) bool {
	values := parseOpenCode(openCode)
	if len(values) != config.Count {
		return false
	}
	seen := map[int]bool{}
	for _, value := range values {
		if value < config.Min || value > config.Max || config.Unique && seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func (s *LotteryService) settleDueIssues(ctx context.Context, limit int) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cmf_lottery_issue WHERE status=2 AND open_code<>'' ORDER BY open_time,id LIMIT ?`, limit)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	settled := 0
	for _, id := range ids {
		ok, err := s.settleIssue(ctx, id)
		if err != nil {
			return settled, err
		}
		if ok {
			settled++
		}
	}
	return settled, nil
}

func (s *LotteryService) settleIssue(ctx context.Context, issueID int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var openCode string
	var status int
	if err := tx.QueryRowContext(ctx, "SELECT open_code,status FROM cmf_lottery_issue WHERE id=? FOR UPDATE", issueID).Scan(&openCode, &status); err != nil {
		return false, err
	}
	if status != lotteryIssueOpened || openCode == "" {
		return false, nil
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,uid,total_bet FROM cmf_lottery_bet_order WHERE issue_id=? AND status=0 ORDER BY id FOR UPDATE", issueID)
	if err != nil {
		return false, err
	}
	type orderRow struct{ id, uid, totalBet int64 }
	orders := make([]orderRow, 0)
	for rows.Next() {
		var order orderRow
		if err := rows.Scan(&order.id, &order.uid, &order.totalBet); err != nil {
			rows.Close()
			return false, err
		}
		orders = append(orders, order)
	}
	rows.Close()

	var ordersWin, ordersLose, payoutTotal int64
	now := time.Now().Unix()
	for _, order := range orders {
		itemRows, err := tx.QueryContext(ctx, `
			SELECT i.id,i.play_code,i.option_code,CAST(i.odds AS CHAR),i.bet_amount,p.result_rule
			FROM cmf_lottery_bet_item i JOIN cmf_lottery_play p ON p.id=i.play_id WHERE i.order_id=? FOR UPDATE`, order.id)
		if err != nil {
			return false, err
		}
		type settleItem struct {
			id, betAmount                    int64
			playCode, optionCode, odds, rule string
		}
		items := make([]settleItem, 0)
		for itemRows.Next() {
			var item settleItem
			if err := itemRows.Scan(&item.id, &item.playCode, &item.optionCode, &item.odds, &item.betAmount, &item.rule); err != nil {
				itemRows.Close()
				return false, err
			}
			items = append(items, item)
		}
		if err := itemRows.Err(); err != nil {
			itemRows.Close()
			return false, err
		}
		itemRows.Close()
		var orderPayout int64
		for _, item := range items {
			win := isLotteryWin(openCode, item.playCode, item.optionCode, item.rule)
			payout, winStatus := int64(0), lotteryOrderLose
			if win {
				scaled, err := parseOddsScaled(item.odds)
				if err != nil {
					return false, err
				}
				payout = item.betAmount * scaled / 10000
				winStatus = lotteryOrderWin
			}
			orderPayout += payout
			if _, err := tx.ExecContext(ctx, "UPDATE cmf_lottery_bet_item SET payout_amount=?,win_status=?,update_time=? WHERE id=?", payout, winStatus, now, item.id); err != nil {
				return false, err
			}
		}
		orderStatus := lotteryOrderLose
		if orderPayout > 0 {
			orderStatus = lotteryOrderWin
			ordersWin++
			payoutTotal += orderPayout
			if _, err := tx.ExecContext(ctx, "UPDATE cmf_user SET coin=coin+? WHERE id=?", orderPayout, order.uid); err != nil {
				return false, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO cmf_user_coinrecord(type,action,uid,touid,giftid,giftcount,totalcoin,showid,addtime) VALUES(1,25,?,?,?,1,?,0,?)`, order.uid, order.uid, order.id, orderPayout, now); err != nil {
				return false, err
			}
		} else {
			ordersLose++
		}
		if _, err := tx.ExecContext(ctx, `UPDATE cmf_lottery_bet_order SET total_payout=?,net_amount=?,status=?,settle_time=?,update_time=? WHERE id=? AND status=0`, orderPayout, orderPayout-order.totalBet, orderStatus, now, now, order.id); err != nil {
			return false, err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE cmf_lottery_issue SET status=3,settle_time=?,update_time=? WHERE id=? AND status=2", now, now, issueID); err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO cmf_lottery_settle_log(issue_id,settle_key,orders_total,orders_win,orders_lose,payout_total,success,message,create_time)
		VALUES(?,CONCAT('issue_',?),?,?,?,?,1,'ok',?)`, issueID, issueID, len(orders), ordersWin, ordersLose, payoutTotal, now)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func debugDraw(config drawConfig) string {
	return strings.Join([]string{strconv.Itoa(config.Count), strconv.Itoa(config.Min), strconv.Itoa(config.Max)}, ":")
}
