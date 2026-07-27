package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sportsOrderPending  = 0
	sportsOrderWin      = 1
	sportsOrderLose     = 2
	sportsOrderRefund   = 3
	sportsOrderCanceled = 4
)

type SportsService struct {
	db     *sql.DB
	cfg    Config
	logger *slog.Logger
	mu     sync.RWMutex
	status map[string]any
}

func NewSportsService(db *sql.DB, cfg Config, logger *slog.Logger) *SportsService {
	state := "starting"
	if cfg.SportsAPIKey == "" {
		state = "disabled_missing_api_key"
	}
	return &SportsService{db: db, cfg: cfg, logger: logger, status: map[string]any{"state": state}}
}

func (s *SportsService) Status() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.status))
	for key, value := range s.status {
		out[key] = value
	}
	return out
}

func (s *SportsService) setStatus(values map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range values {
		s.status[key] = value
	}
}

type sportsMatch struct {
	ID              int64
	Source          string
	SourceMatchID   string
	Competition     string
	CompetitionType string
	Country         string
	HomeName        string
	AwayName        string
	HomeLogo        string
	AwayLogo        string
	MatchDate       string
	KickoffTime     int64
	BetCloseTime    int64
	HomeScore       int
	AwayScore       int
	Status          string
	StatusText      string
	RawStatus       string
	BetStatus       int
	SettleStatus    int
	MinBet          int64
	MaxBet          int64
	MaxMatchBet     int64
	SyncTime        int64
}

func (s *SportsService) Home(ctx context.Context, tab, date, competitionType string) (map[string]any, error) {
	nowInstant := time.Now()
	nowUnix := nowInstant.Unix()
	now := nowInstant.In(sportsTimezone)
	tab = normalizeSportsTab(tab)
	selectedDate := strings.TrimSpace(date)
	if selectedDate == "" {
		offset := 0
		if tab == "yesterday" {
			offset = -1
		} else if tab == "tomorrow" {
			offset = 1
		}
		selectedDate = now.AddDate(0, 0, offset).Format("2006-01-02")
	}

	query := sportsMatchSelect + " WHERE 1=1"
	args := []any{}
	if tab == "live" {
		query += " AND status IN ('1H','HT','2H','ET','BT','P','LIVE','INT')"
	} else if tab == "fixtures" {
		query += " AND kickoff_time>?"
		args = append(args, nowUnix)
	} else {
		query += " AND match_date=?"
		args = append(args, selectedDate)
	}
	if strings.TrimSpace(competitionType) != "" {
		query += " AND competition_type=?"
		args = append(args, competitionType)
	}
	query += " ORDER BY kickoff_time,id LIMIT 500"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matches := make([]map[string]any, 0)
	leagues := map[string]int{}
	liveCount, scoredCount, totalGoals := 0, 0, 0
	for rows.Next() {
		match, err := scanSportsMatch(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, formatSportsMatchAt(match, nowUnix))
		league := match.CompetitionType
		if league == "" {
			league = match.Competition
		}
		if league != "" {
			leagues[league]++
		}
		if isLiveSportsStatus(match.Status) {
			liveCount++
		}
		if match.HomeScore >= 0 && match.AwayScore >= 0 {
			scoredCount++
			totalGoals += match.HomeScore + match.AwayScore
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	leagueList := make([]map[string]any, 0, len(leagues))
	for name, count := range leagues {
		leagueList = append(leagueList, map[string]any{"key": name, "name": name, "name_cn": name, "count": strconv.Itoa(count)})
	}
	sort.Slice(leagueList, func(i, j int) bool {
		left, _ := strconv.Atoi(leagueList[i]["count"].(string))
		right, _ := strconv.Atoi(leagueList[j]["count"].(string))
		return left > right
	})
	if len(leagueList) > 12 {
		leagueList = leagueList[:12]
	}
	average := "0.00"
	if scoredCount > 0 {
		average = fmt.Sprintf("%.2f", float64(totalGoals)/float64(scoredCount))
	}
	updatedAt := ""
	var latest int64
	_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sync_time),0) FROM cmf_sports_match").Scan(&latest)
	if latest > 0 {
		updatedAt = sportsTimestampText(latest, "2006-01-02 15:04:05")
	}
	return map[string]any{
		"source": "database", "source_name": "本地赛事数据库", "updated_at": updatedAt, "poll_interval": "15",
		"server_time": nowUnix, "timezone": sportsTimezoneName, "timezone_offset": sportsTimezoneOffset(nowUnix),
		"selected_tab": tab, "selected_date": selectedDate, "selected_competition_type": competitionType,
		"tabs": sportsTabs(), "top_leagues": leagueList, "competition_filters": leagueList,
		"matches": matches, "upcoming": matches, "competitions": leagueList,
		"matches_title": fmt.Sprintf("%s（%d场）", func() string {
			if tab == "live" {
				return "进行中"
			}
			if tab == "fixtures" {
				return "赛程列表"
			}
			return "即时赛况"
		}(), len(matches)),
		"quick_stats_title": "赛事数据", "quick_stats": []map[string]any{
			{"name": "比赛总数", "value": strconv.Itoa(len(matches)), "desc": "数据库当前列表"},
			{"name": "进行中", "value": strconv.Itoa(liveCount), "desc": "实时比分"},
			{"name": "进球均值", "value": average, "desc": "已出比分场次"},
		},
		"total_count": strconv.Itoa(len(matches)), "fetch_status": "database_only",
	}, nil
}

const sportsMatchSelect = `SELECT id,source,source_match_id,competition,competition_type,country,home_name,away_name,home_logo,away_logo,match_date,kickoff_time,bet_close_time,home_score,away_score,status,status_text,raw_status,bet_status,settle_status,min_bet,max_bet,max_match_bet,sync_time FROM cmf_sports_match`

func scanSportsMatch(row rowScanner) (sportsMatch, error) {
	var match sportsMatch
	err := row.Scan(&match.ID, &match.Source, &match.SourceMatchID, &match.Competition, &match.CompetitionType, &match.Country,
		&match.HomeName, &match.AwayName, &match.HomeLogo, &match.AwayLogo, &match.MatchDate, &match.KickoffTime,
		&match.BetCloseTime, &match.HomeScore, &match.AwayScore, &match.Status, &match.StatusText, &match.RawStatus,
		&match.BetStatus, &match.SettleStatus, &match.MinBet, &match.MaxBet, &match.MaxMatchBet, &match.SyncTime)
	return match, err
}

func (s *SportsService) findMatch(ctx context.Context, requestID string) (sportsMatch, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return sportsMatch{}, sql.ErrNoRows
	}
	return scanSportsMatch(s.db.QueryRowContext(ctx, sportsMatchSelect+" WHERE CAST(id AS CHAR)=? OR source_match_id=? ORDER BY source_match_id=? DESC LIMIT 1", requestID, requestID, requestID))
}

func formatSportsMatch(match sportsMatch) map[string]any {
	return formatSportsMatchAt(match, time.Now().Unix())
}

func formatSportsMatchAt(match sportsMatch, now int64) map[string]any {
	homeScore, awayScore := any("-"), any("-")
	if match.HomeScore >= 0 {
		homeScore = strconv.Itoa(match.HomeScore)
	}
	if match.AwayScore >= 0 {
		awayScore = strconv.Itoa(match.AwayScore)
	}
	competitionType := match.CompetitionType
	if competitionType == "" {
		competitionType = match.Competition
	}
	betStatus := effectiveSportsBetStatus(match, now)
	betStatusText := "已封盘"
	if betStatus == 0 {
		betStatusText = "盘口已停用"
	} else if betStatus == 1 {
		betStatusText = "盘口开放"
	}
	return map[string]any{
		"id": match.SourceMatchID, "match_id": match.SourceMatchID, "local_id": strconv.FormatInt(match.ID, 10),
		"source_match_id": match.SourceMatchID, "source": match.Source, "competition": match.Competition,
		"competition_type": competitionType, "country": match.Country, "home_name": match.HomeName, "away_name": match.AwayName,
		"home_logo": match.HomeLogo, "away_logo": match.AwayLogo, "match_date": match.MatchDate,
		"kickoff_ts": match.KickoffTime, "kickoff_time": match.KickoffTime,
		"kickoff_text": sportsTimestampText(match.KickoffTime, "01-02 15:04"), "match_time": sportsTimestampText(match.KickoffTime, "01-02 15:04"),
		"kickoff_date_text": sportsTimestampText(match.KickoffTime, "01月02日"), "kickoff_clock_text": sportsTimestampText(match.KickoffTime, "15:04"),
		"bet_close_ts": match.BetCloseTime, "bet_close_time": match.BetCloseTime,
		"home_score": homeScore, "away_score": awayScore, "status": match.Status, "status_text": match.StatusText,
		"raw_status": match.RawStatus, "bet_status": strconv.Itoa(betStatus), "bet_status_text": betStatusText, "settle_status": strconv.Itoa(match.SettleStatus),
		"trend": fmt.Sprintf("实时比分 %v : %v", homeScore, awayScore), "updated_at_ts": strconv.FormatInt(match.SyncTime, 10),
	}
}

func (s *SportsService) MatchDetail(ctx context.Context, requestID string) (map[string]any, error) {
	match, err := s.findMatch(ctx, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appError(3003, "比赛不存在或暂未同步")
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	result := map[string]any{
		"match": formatSportsMatchAt(match, now), "teams": []any{}, "events": []any{}, "stats": []any{}, "h2h": []any{},
		"lineups": map[string]any{}, "timeline": []any{}, "predictions": []any{}, "injuries": []any{}, "player_stats": []any{},
		"source":     map[string]any{"name": "本地赛事数据库", "live_update": map[bool]string{true: "1", false: "0"}[isLiveSportsStatus(match.Status)], "detail_status": "database_only"},
		"updated_at": sportsTimestampText(match.SyncTime, "2006-01-02 15:04:05"), "updated_at_ts": match.SyncTime, "poll_interval": "15",
		"server_time": now, "timezone": sportsTimezoneName, "timezone_offset": sportsTimezoneOffset(now),
	}
	var raw []byte
	if err := s.db.QueryRowContext(ctx, "SELECT payload FROM cmf_sports_snapshot WHERE match_id=?", match.ID).Scan(&raw); err == nil && len(raw) > 0 {
		var payload map[string]any
		if json.Unmarshal(raw, &payload) == nil {
			result["snapshot"] = payload
		}
	}
	return result, nil
}

func (s *SportsService) MatchUpdates(ctx context.Context, requestID string, since int64) (map[string]any, error) {
	now := time.Now().Unix()
	match, err := s.findMatch(ctx, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]any{"match_id": requestID, "changed": "0", "server_time": now, "timezone": sportsTimezoneName, "timezone_offset": sportsTimezoneOffset(now)}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"match_id": match.SourceMatchID, "server_time": now, "timezone": sportsTimezoneName, "timezone_offset": sportsTimezoneOffset(now),
		"changed": map[bool]string{true: "1", false: "0"}[match.SyncTime > since], "match": formatSportsMatchAt(match, now),
		"events": []any{}, "stats": []any{}, "timeline": []any{}, "poll_interval": "15",
	}, nil
}

func normalizeSportsTab(tab string) string {
	switch strings.ToLower(strings.TrimSpace(tab)) {
	case "live", "yesterday", "tomorrow", "fixtures":
		return strings.ToLower(strings.TrimSpace(tab))
	default:
		return "today"
	}
}

func sportsTabs() []map[string]any {
	return []map[string]any{
		{"key": "live", "name": "进行中"}, {"key": "yesterday", "name": "昨日"}, {"key": "today", "name": "今日"},
		{"key": "tomorrow", "name": "明日"}, {"key": "fixtures", "name": "赛程"},
	}
}

func isLiveSportsStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "1H", "HT", "2H", "ET", "BT", "P", "LIVE", "INT":
		return true
	default:
		return false
	}
}

func isFinishedSportsStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "FT", "AET", "PEN", "AWD", "WO", "FIN", "FINAL", "RES":
		return true
	default:
		return false
	}
}

func isCanceledSportsStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "CANC", "ABD", "SUSP", "PST":
		return true
	default:
		return false
	}
}
