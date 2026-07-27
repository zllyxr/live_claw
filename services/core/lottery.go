package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	lotteryIssueOpen     = 0
	lotteryIssueSealed   = 1
	lotteryIssueOpened   = 2
	lotteryIssueSettled  = 3
	lotteryIssueCanceled = 4

	lotteryOrderPending = 0
	lotteryOrderWin     = 1
	lotteryOrderLose    = 2
	lotteryOrderRefund  = 3
)

type LotteryService struct {
	db     *sql.DB
	logger *slog.Logger
	mu     sync.RWMutex
	status map[string]any
}

func NewLotteryService(db *sql.DB, logger *slog.Logger) *LotteryService {
	return &LotteryService{db: db, logger: logger, status: map[string]any{"state": "starting"}}
}

func (s *LotteryService) Status() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.status))
	for key, value := range s.status {
		out[key] = value
	}
	return out
}

func (s *LotteryService) setStatus(values map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range values {
		s.status[key] = value
	}
}

type lotteryGame struct {
	ID             int64
	CategoryID     int64
	Code           string
	Name           string
	NameEN         string
	Icon           string
	Description    string
	RuleDesc       string
	IntervalSec    int64
	SealAdvanceSec int64
	MinBet         int64
	MaxBet         int64
	MaxIssueBet    int64
	Status         int
}

type lotteryIssue struct {
	ID           int64
	GameID       int64
	IssueNum     string
	OpenCode     string
	OpenTime     int64
	SealTime     int64
	NextOpenTime int64
	Status       int
}

func (s *LotteryService) Home(ctx context.Context, uid int64) (map[string]any, error) {
	categories, err := s.loadCategories(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,category_id,game_code,game_name,game_name_en,icon,COALESCE(description,''),COALESCE(rule_desc,''),
		       interval_sec,seal_advance_sec,min_bet,max_bet,max_issue_bet,status
		FROM cmf_lottery_game WHERE status IN (1,2) ORDER BY category_id,sort DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	games := make([]map[string]any, 0)
	activeCategories := map[string]bool{}
	for rows.Next() {
		game, err := scanLotteryGame(rows)
		if err != nil {
			return nil, err
		}
		issue, err := s.currentIssueByGame(ctx, game.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if issue == nil {
			continue
		}
		item := formatLotteryGame(game)
		item["current_issue"] = formatLotteryIssue(*issue)
		games = append(games, item)
		activeCategories[strconv.FormatInt(game.CategoryID, 10)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	filteredCategories := make([]map[string]any, 0, len(categories))
	for _, category := range categories {
		if activeCategories[fmt.Sprint(category["id"])] {
			filteredCategories = append(filteredCategories, category)
		}
	}
	coin, err := s.userCoin(ctx, uid)
	if err != nil {
		return nil, err
	}
	return map[string]any{"categories": filteredCategories, "games": games, "coin": strconv.FormatInt(coin, 10)}, nil
}

func (s *LotteryService) loadCategories(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,COALESCE(name_en,''),COALESCE(icon,'') FROM cmf_lottery_category WHERE status=1 ORDER BY sort DESC,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var name, nameEN, icon string
		if err := rows.Scan(&id, &name, &nameEN, &icon); err != nil {
			return nil, err
		}
		icon = usableLotteryAsset(icon)
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "name": name, "name_en": nameEN, "icon": icon, "icon_url": icon,
		})
	}
	return items, rows.Err()
}

func usableLotteryAsset(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(value, "//") || strings.HasPrefix(value, "/") ||
		strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "blob:") ||
		strings.HasPrefix(lower, "local_") || strings.HasPrefix(lower, "minio_") ||
		strings.Contains(value, ".") {
		return value
	}
	return ""
}

func (s *LotteryService) Detail(ctx context.Context, gameID int64, gameCode string, uid int64) (map[string]any, error) {
	game, err := s.getGame(ctx, gameID, gameCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appError(1003, "游戏不存在或维护中")
		}
		return nil, err
	}
	issue, err := s.currentIssueByGame(ctx, game.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appError(1004, "当前暂无可投注期号")
		}
		return nil, err
	}
	plays, err := s.playList(ctx, game.ID)
	if err != nil {
		return nil, err
	}
	history, err := s.issueHistoryByGame(ctx, game.ID, 1, 30)
	if err != nil {
		return nil, err
	}
	coin, err := s.userCoin(ctx, uid)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"game": formatLotteryGame(game), "current_issue": formatLotteryIssue(*issue),
		"plays": plays, "history": history, "analysis": map[string]any{}, "coin": strconv.FormatInt(coin, 10),
	}, nil
}

func (s *LotteryService) CurrentIssue(ctx context.Context, gameID int64, gameCode string) (map[string]any, error) {
	game, err := s.getGame(ctx, gameID, gameCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appError(1003, "游戏不存在或维护中")
		}
		return nil, err
	}
	issue, err := s.currentIssueByGame(ctx, game.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appError(1004, "当前暂无可投注期号")
	}
	if err != nil {
		return nil, err
	}
	return formatLotteryIssue(*issue), nil
}

func (s *LotteryService) IssueHistory(ctx context.Context, gameID int64, gameCode string, page int) (map[string]any, error) {
	game, err := s.getGame(ctx, gameID, gameCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appError(1003, "游戏不存在")
		}
		return nil, err
	}
	items, err := s.issueHistoryByGame(ctx, game.ID, page, 30)
	if err != nil {
		return nil, err
	}
	return map[string]any{"list": items, "page": strconv.Itoa(page)}, nil
}

func (s *LotteryService) issueHistoryByGame(ctx context.Context, gameID int64, page, limit int) ([]map[string]any, error) {
	offset := (max(page, 1) - 1) * limit
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,game_id,issue_num,open_code,open_time,seal_time,next_open_time,status
		FROM cmf_lottery_issue WHERE game_id=? AND open_code<>'' AND status IN (2,3)
		ORDER BY open_time DESC,id DESC LIMIT ? OFFSET ?`, gameID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		issue, err := scanLotteryIssue(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, formatLotteryIssue(issue))
	}
	return items, rows.Err()
}

func (s *LotteryService) playList(ctx context.Context, gameID int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id,p.play_code,p.play_name,p.result_rule,o.id,o.option_code,o.option_name,CAST(o.odds AS CHAR)
		FROM cmf_lottery_play p JOIN cmf_lottery_option o ON o.play_id=p.id AND o.status=1
		WHERE p.game_id=? AND p.status=1 ORDER BY p.sort DESC,p.id,o.sort DESC,o.id`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	indexes := map[int64]int{}
	for rows.Next() {
		var playID, optionID int64
		var playCode, playName, resultRule, optionCode, optionName, odds string
		if err := rows.Scan(&playID, &playCode, &playName, &resultRule, &optionID, &optionCode, &optionName, &odds); err != nil {
			return nil, err
		}
		index, ok := indexes[playID]
		if !ok {
			index = len(items)
			indexes[playID] = index
			items = append(items, map[string]any{
				"id": strconv.FormatInt(playID, 10), "play_code": playCode, "play_name": playName,
				"result_rule": resultRule, "options": []map[string]any{},
			})
		}
		options := items[index]["options"].([]map[string]any)
		items[index]["options"] = append(options, map[string]any{
			"id": strconv.FormatInt(optionID, 10), "option_code": optionCode, "option_name": optionName, "odds": odds,
		})
	}
	return items, rows.Err()
}

func (s *LotteryService) getGame(ctx context.Context, gameID int64, gameCode string) (lotteryGame, error) {
	query := `SELECT id,category_id,game_code,game_name,game_name_en,icon,COALESCE(description,''),COALESCE(rule_desc,''),interval_sec,seal_advance_sec,min_bet,max_bet,max_issue_bet,status FROM cmf_lottery_game WHERE status IN (1,2) AND `
	var row *sql.Row
	if gameID > 0 {
		row = s.db.QueryRowContext(ctx, query+"id=? LIMIT 1", gameID)
	} else {
		row = s.db.QueryRowContext(ctx, query+"game_code=? LIMIT 1", strings.ToUpper(strings.TrimSpace(gameCode)))
	}
	return scanLotteryGame(row)
}

func (s *LotteryService) currentIssueByGame(ctx context.Context, gameID int64) (*lotteryIssue, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id,game_id,issue_num,open_code,open_time,seal_time,next_open_time,status
		FROM cmf_lottery_issue WHERE game_id=? AND status=0 AND seal_time>?
		ORDER BY open_time,id LIMIT 1`, gameID, time.Now().Unix())
	issue, err := scanLotteryIssue(row)
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

type rowScanner interface{ Scan(...any) error }

func scanLotteryGame(row rowScanner) (lotteryGame, error) {
	var game lotteryGame
	err := row.Scan(&game.ID, &game.CategoryID, &game.Code, &game.Name, &game.NameEN, &game.Icon, &game.Description, &game.RuleDesc,
		&game.IntervalSec, &game.SealAdvanceSec, &game.MinBet, &game.MaxBet, &game.MaxIssueBet, &game.Status)
	return game, err
}

func scanLotteryIssue(row rowScanner) (lotteryIssue, error) {
	var issue lotteryIssue
	err := row.Scan(&issue.ID, &issue.GameID, &issue.IssueNum, &issue.OpenCode, &issue.OpenTime, &issue.SealTime, &issue.NextOpenTime, &issue.Status)
	return issue, err
}

func formatLotteryGame(game lotteryGame) map[string]any {
	return map[string]any{
		"id": strconv.FormatInt(game.ID, 10), "category_id": strconv.FormatInt(game.CategoryID, 10),
		"game_code": game.Code, "game_name": game.Name, "game_name_en": game.NameEN,
		"icon": game.Icon, "icon_url": game.Icon, "description": game.Description, "rule_desc": game.RuleDesc,
		"interval_sec": strconv.FormatInt(game.IntervalSec, 10), "seal_advance_sec": strconv.FormatInt(game.SealAdvanceSec, 10),
		"min_bet": strconv.FormatInt(game.MinBet, 10), "max_bet": strconv.FormatInt(game.MaxBet, 10),
		"max_issue_bet": strconv.FormatInt(game.MaxIssueBet, 10), "status": strconv.Itoa(game.Status),
	}
}

func formatLotteryIssue(issue lotteryIssue) map[string]any {
	now := time.Now().Unix()
	return map[string]any{
		"id": strconv.FormatInt(issue.ID, 10), "game_id": strconv.FormatInt(issue.GameID, 10), "issue_num": issue.IssueNum,
		"open_code": issue.OpenCode, "open_time": strconv.FormatInt(issue.OpenTime, 10), "seal_time": strconv.FormatInt(issue.SealTime, 10),
		"next_open_time": strconv.FormatInt(issue.NextOpenTime, 10), "status": strconv.Itoa(issue.Status),
		"open_time_text": time.Unix(issue.OpenTime, 0).Format("2006-01-02 15:04:05"),
		"seal_countdown": strconv.FormatInt(max(issue.SealTime-now, 0), 10), "open_countdown": strconv.FormatInt(max(issue.OpenTime-now, 0), 10),
	}
}

func (s *LotteryService) userCoin(ctx context.Context, uid int64) (int64, error) {
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
