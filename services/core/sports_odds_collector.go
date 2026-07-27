package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type apiFootballOddsResponse struct {
	Errors  any `json:"errors"`
	Results int `json:"results"`
	Paging  struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"paging"`
	Response []apiFootballOdds `json:"response"`
}

type apiFootballOdds struct {
	Fixture struct {
		ID int64 `json:"id"`
	} `json:"fixture"`
	Bookmakers []apiFootballBookmaker `json:"bookmakers"`
}

type apiFootballBookmaker struct {
	ID   int64            `json:"id"`
	Name string           `json:"name"`
	Bets []apiFootballBet `json:"bets"`
}

type apiFootballBet struct {
	ID     int64                 `json:"id"`
	Name   string                `json:"name"`
	Values []apiFootballOddValue `json:"values"`
}

type apiFootballOddValue struct {
	Value apiString `json:"value"`
	Odd   apiString `json:"odd"`
}

type apiString string

func (value *apiString) UnmarshalJSON(raw []byte) error {
	var text string
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		*value = apiString(text)
		return nil
	}
	text = strings.TrimSpace(string(raw))
	if text == "null" {
		*value = ""
		return nil
	}
	if _, err := strconv.ParseFloat(text, 64); err != nil {
		return fmt.Errorf("invalid scalar string %q", text)
	}
	*value = apiString(text)
	return nil
}

type collectedSportsMarket struct {
	Code    string
	Name    string
	Rule    string
	Options []collectedSportsOption
}

type collectedSportsOption struct {
	Code string
	Name string
	Odds string
	Sort int
}

func (s *SportsService) collectUpcomingOdds(ctx context.Context, collectedAt time.Time) (int, error) {
	total := 0
	for _, offset := range []int{0, 1} {
		date := collectedAt.In(sportsTimezone).AddDate(0, 0, offset).Format("2006-01-02")
		items, err := s.fetchOdds(ctx, date)
		if err != nil {
			return total, fmt.Errorf("%s: %w", date, err)
		}
		for _, item := range items {
			stored, err := s.storeOdds(ctx, item)
			if err != nil {
				return total, err
			}
			total += stored
		}
	}
	return total, nil
}

func (s *SportsService) fetchOdds(ctx context.Context, date string) ([]apiFootballOdds, error) {
	all := make([]apiFootballOdds, 0)
	for page := 1; ; page++ {
		target, err := url.Parse(s.cfg.SportsAPIBaseURL + "/odds")
		if err != nil {
			return nil, err
		}
		query := target.Query()
		query.Set("date", date)
		query.Set("timezone", sportsTimezoneName)
		query.Set("page", strconv.Itoa(page))
		target.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-apisports-key", s.cfg.SportsAPIKey)
		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
		}
		var payload apiFootballOddsResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		if payload.Results == 0 && hasAPIErrors(payload.Errors) {
			return nil, fmt.Errorf("upstream errors: %v", payload.Errors)
		}
		all = append(all, payload.Response...)
		if payload.Paging.Total <= page || payload.Paging.Total == 0 {
			break
		}
		if page >= 100 {
			return nil, fmt.Errorf("odds paging exceeded safety limit")
		}
	}
	return all, nil
}

func (s *SportsService) storeOdds(ctx context.Context, item apiFootballOdds) (int, error) {
	if item.Fixture.ID < 1 {
		return 0, nil
	}
	var matchID int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM cmf_sports_match WHERE source='api-football' AND source_match_id=? LIMIT 1", item.Fixture.ID).Scan(&matchID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	markets := normalizeCollectedMarkets(item)
	if len(markets) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now, stored := time.Now().Unix(), 0
	for marketSort, market := range markets {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO cmf_sports_market(match_id,market_code,market_name,market_rule,sort,status,create_time,update_time)
			VALUES(?,?,?,?,?,1,?,?) ON DUPLICATE KEY UPDATE market_name=VALUES(market_name),market_rule=VALUES(market_rule),sort=VALUES(sort),status=1,update_time=VALUES(update_time)`,
			matchID, market.Code, market.Name, market.Rule, len(markets)-marketSort, now, now)
		if err != nil {
			return 0, err
		}
		var marketID int64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM cmf_sports_market WHERE match_id=? AND market_code=?", matchID, market.Code).Scan(&marketID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE cmf_sports_option SET status=0,update_time=? WHERE market_id=?", now, marketID); err != nil {
			return 0, err
		}
		for _, option := range market.Options {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO cmf_sports_option(market_id,option_code,option_name,odds,sort,status,create_time,update_time)
				VALUES(?,?,?,?,?,1,?,?) ON DUPLICATE KEY UPDATE option_name=VALUES(option_name),odds=VALUES(odds),sort=VALUES(sort),status=1,update_time=VALUES(update_time)`,
				marketID, option.Code, option.Name, option.Odds, option.Sort, now, now)
			if err != nil {
				return 0, err
			}
			stored++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return stored, nil
}

func normalizeCollectedMarkets(item apiFootballOdds) []collectedSportsMarket {
	for _, bookmaker := range item.Bookmakers {
		markets := make([]collectedSportsMarket, 0, 2)
		for _, bet := range bookmaker.Bets {
			switch {
			case bet.ID == 1 || strings.EqualFold(bet.Name, "Match Winner") || strings.EqualFold(bet.Name, "Fulltime Result"):
				options := make([]collectedSportsOption, 0, 3)
				for _, value := range bet.Values {
					code, name := normalizeMatchWinner(string(value.Value))
					if code != "" && validCollectedOdds(string(value.Odd)) {
						options = append(options, collectedSportsOption{Code: code, Name: name, Odds: string(value.Odd), Sort: 10 - len(options)})
					}
				}
				if len(options) == 3 {
					markets = append(markets, collectedSportsMarket{Code: "MATCH_RESULT", Name: "胜平负", Rule: "full_time_result", Options: options})
				}
			case bet.ID == 10 || strings.EqualFold(bet.Name, "Exact Score") || strings.EqualFold(bet.Name, "Correct Score"):
				options := make([]collectedSportsOption, 0, len(bet.Values))
				for _, value := range bet.Values {
					parts := strings.FieldsFunc(string(value.Value), func(r rune) bool { return r == ':' || r == '-' })
					if len(parts) != 2 || !validCollectedOdds(string(value.Odd)) {
						continue
					}
					home, errHome := strconv.Atoi(strings.TrimSpace(parts[0]))
					away, errAway := strconv.Atoi(strings.TrimSpace(parts[1]))
					if errHome != nil || errAway != nil || home < 0 || home > 7 || away < 0 || away > 7 {
						continue
					}
					options = append(options, collectedSportsOption{Code: fmt.Sprintf("CS_%d_%d", home, away), Name: fmt.Sprintf("%d:%d", home, away), Odds: string(value.Odd), Sort: 100 - len(options)})
				}
				if len(options) > 0 {
					markets = append(markets, collectedSportsMarket{Code: "CORRECT_SCORE", Name: "全场比分", Rule: "full_time_score", Options: options})
				}
			}
		}
		if len(markets) > 0 {
			return markets
		}
	}
	return nil
}

func normalizeMatchWinner(value string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "home", "1":
		return "HOME_WIN", "主胜"
	case "draw", "x":
		return "DRAW", "平"
	case "away", "2":
		return "AWAY_WIN", "客胜"
	default:
		return "", ""
	}
}

func validCollectedOdds(value string) bool {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && parsed > 1 && parsed <= 1000
}
