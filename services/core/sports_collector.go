package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type apiFootballResponse struct {
	Errors   any                  `json:"errors"`
	Results  int                  `json:"results"`
	Response []apiFootballFixture `json:"response"`
}

type apiFootballFixture struct {
	Fixture struct {
		ID        int64  `json:"id"`
		Date      string `json:"date"`
		Timestamp int64  `json:"timestamp"`
		Status    struct {
			Long    string `json:"long"`
			Short   string `json:"short"`
			Elapsed *int   `json:"elapsed"`
		} `json:"status"`
	} `json:"fixture"`
	League struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Country string `json:"country"`
		Round   string `json:"round"`
		Logo    string `json:"logo"`
		Flag    string `json:"flag"`
	} `json:"league"`
	Teams struct {
		Home struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Logo string `json:"logo"`
		} `json:"home"`
		Away struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Logo string `json:"logo"`
		} `json:"away"`
	} `json:"teams"`
	Goals struct {
		Home *int `json:"home"`
		Away *int `json:"away"`
	} `json:"goals"`
	Score any `json:"score"`
}

func (s *SportsService) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	nextCatalog, nextOdds := time.Time{}, time.Time{}
	nextUpstreamAttempt := time.Time{}
	quotaMessage := ""
	if s.cfg.SportsAPIKey == "" {
		s.logger.Warn("sports collector disabled: SPORTS_API_KEY is empty; database settlement remains active")
	}
	for {
		started := time.Now()
		stored, settled := 0, 0
		var cycleErrors []error
		upstreamAttempted := s.cfg.SportsAPIKey != "" && !started.Before(nextUpstreamAttempt)
		if upstreamAttempted {
			live, err := s.collectLiveFixtures(ctx)
			stored += live
			if err != nil {
				cycleErrors = append(cycleErrors, fmt.Errorf("live fixtures: %w", err))
			}
			if !started.Before(nextCatalog) {
				catalog, err := s.collectFixtureCatalog(ctx, started)
				stored += catalog
				if err != nil {
					cycleErrors = append(cycleErrors, fmt.Errorf("fixture catalog: %w", err))
				} else {
					nextCatalog = started.Add(s.cfg.SportsCatalogInterval)
				}
			}
			if !started.Before(nextOdds) {
				odds, err := s.collectUpcomingOdds(ctx, started)
				if err != nil {
					cycleErrors = append(cycleErrors, fmt.Errorf("odds: %w", err))
				} else {
					nextOdds = started.Add(s.cfg.SportsOddsInterval)
					s.logCollection(ctx, "odds", true, fmt.Sprintf("stored=%d", odds))
				}
			}
		}
		if err := s.closeDueMatches(ctx); err != nil {
			cycleErrors = append(cycleErrors, err)
		}
		if count, err := s.settleDueMatches(ctx, 200); err != nil {
			cycleErrors = append(cycleErrors, err)
		} else {
			settled = count
		}
		cycleErr := errors.Join(cycleErrors...)
		if cycleErr != nil && isSportsQuotaError(cycleErr) {
			nextUpstreamAttempt = started.Add(time.Hour)
			quotaMessage = cycleErr.Error()
		} else if upstreamAttempted && cycleErr == nil {
			nextUpstreamAttempt = time.Time{}
			quotaMessage = ""
		}
		state := "running"
		if s.cfg.SportsAPIKey == "" {
			state = "disabled_missing_api_key"
		} else if !nextUpstreamAttempt.IsZero() && started.Before(nextUpstreamAttempt) {
			state = "rate_limited"
		} else if cycleErr != nil {
			state = "degraded"
		}
		lastError := ""
		if cycleErr != nil {
			lastError = cycleErr.Error()
			s.logger.Error("sports collection cycle", "error", cycleErr)
			s.logCollection(ctx, "cycle", false, lastError)
		} else if state == "rate_limited" {
			lastError = quotaMessage
		} else if upstreamAttempted {
			s.logCollection(ctx, "fixtures", true, fmt.Sprintf("stored=%d", stored))
		}
		retryAt := ""
		if !nextUpstreamAttempt.IsZero() {
			retryAt = nextUpstreamAttempt.Format(time.RFC3339)
		}
		s.setStatus(map[string]any{
			"state": state, "last_error": lastError, "last_run": started.Format(time.RFC3339),
			"last_duration_ms": time.Since(started).Milliseconds(), "matches": stored, "settled": settled,
			"next_upstream_attempt": retryAt,
		})
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func isSportsQuotaError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "request limit for the day") ||
		strings.Contains(message, "daily request limit")
}

func (s *SportsService) collectFixtureCatalog(ctx context.Context, collectedAt time.Time) (int, error) {
	now := collectedAt.In(sportsTimezone)
	total := 0
	for _, offset := range []int{-1, 0, 1} {
		date := now.AddDate(0, 0, offset).Format("2006-01-02")
		fixtures, err := s.fetchFixtures(ctx, url.Values{"date": {date}, "timezone": {sportsTimezoneName}})
		if err != nil {
			return total, fmt.Errorf("fixtures %s: %w", date, err)
		}
		for _, fixture := range fixtures {
			if err := s.storeFixture(ctx, fixture); err != nil {
				return total, err
			}
			total++
		}
	}
	return total, nil
}

func (s *SportsService) collectLiveFixtures(ctx context.Context) (int, error) {
	fixtures, err := s.fetchFixtures(ctx, url.Values{"live": {"all"}, "timezone": {sportsTimezoneName}})
	if err != nil {
		return 0, err
	}
	for _, fixture := range fixtures {
		if err := s.storeFixture(ctx, fixture); err != nil {
			return 0, err
		}
	}
	return len(fixtures), nil
}

func (s *SportsService) fetchFixtures(ctx context.Context, query url.Values) ([]apiFootballFixture, error) {
	target, err := url.Parse(s.cfg.SportsAPIBaseURL + "/fixtures")
	if err != nil {
		return nil, err
	}
	query = cloneValues(query)
	target.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apisports-key", s.cfg.SportsAPIKey)
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	var payload apiFootballResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Results == 0 && hasAPIErrors(payload.Errors) {
		return nil, fmt.Errorf("upstream errors: %v", payload.Errors)
	}
	return payload.Response, nil
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func hasAPIErrors(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	case string:
		return typed != ""
	default:
		return true
	}
}

func (s *SportsService) storeFixture(ctx context.Context, fixture apiFootballFixture) error {
	kickoffTime := normalizeSportsTimestamp(fixture.Fixture.Timestamp)
	if fixture.Fixture.ID < 1 || kickoffTime < 1 {
		return nil
	}
	now := time.Now().Unix()
	homeScore, awayScore := -1, -1
	if fixture.Goals.Home != nil {
		homeScore = *fixture.Goals.Home
	}
	if fixture.Goals.Away != nil {
		awayScore = *fixture.Goals.Away
	}
	status := fixture.Fixture.Status.Short
	betStatus := 1
	betCloseTime := kickoffTime - 300
	if betCloseTime <= now || !sportsStatusAllowsBet(status) {
		betStatus = 2
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO cmf_sports_match
		(source,source_match_id,competition,competition_type,country,home_name,away_name,home_logo,away_logo,match_date,
		 kickoff_time,bet_close_time,seal_advance_sec,home_score,away_score,status,status_text,raw_status,bet_status,settle_status,
		 min_bet,max_bet,max_match_bet,sync_time,create_time,update_time)
		VALUES('api-football',?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,10,500000,1000000,?,?,?)
		ON DUPLICATE KEY UPDATE
		 competition=VALUES(competition),competition_type=VALUES(competition_type),country=VALUES(country),
		 home_name=VALUES(home_name),away_name=VALUES(away_name),home_logo=VALUES(home_logo),away_logo=VALUES(away_logo),
		 match_date=VALUES(match_date),kickoff_time=VALUES(kickoff_time),bet_close_time=VALUES(bet_close_time),
		 home_score=IF(settle_status=1,home_score,VALUES(home_score)),away_score=IF(settle_status=1,away_score,VALUES(away_score)),
		 status=IF(settle_status=1,status,VALUES(status)),status_text=IF(settle_status=1,status_text,VALUES(status_text)),
		 raw_status=IF(settle_status=1,raw_status,VALUES(raw_status)),bet_status=IF(settle_status=0,VALUES(bet_status),bet_status),
		 sync_time=VALUES(sync_time),update_time=VALUES(update_time)`,
		strconv.FormatInt(fixture.Fixture.ID, 10), fixture.League.Name, fixture.League.Name, fixture.League.Country,
		fixture.Teams.Home.Name, fixture.Teams.Away.Name, fixture.Teams.Home.Logo, fixture.Teams.Away.Logo,
		sportsTimestampText(kickoffTime, "2006-01-02"), kickoffTime, betCloseTime, 300,
		homeScore, awayScore, status, fixture.Fixture.Status.Long, fixture.League.Round, betStatus, now, now, now,
	)
	if err != nil {
		return err
	}
	matchID, _ := result.LastInsertId()
	if matchID == 0 {
		if err := s.db.QueryRowContext(ctx, "SELECT id FROM cmf_sports_match WHERE source='api-football' AND source_match_id=?", fixture.Fixture.ID).Scan(&matchID); err != nil {
			return err
		}
	}
	payload, err := json.Marshal(fixture)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO cmf_sports_snapshot(match_id,source_match_id,payload,collected_at)
		VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE source_match_id=VALUES(source_match_id),payload=VALUES(payload),collected_at=VALUES(collected_at)`,
		matchID, fixture.Fixture.ID, payload, now)
	return err
}

func (s *SportsService) closeDueMatches(ctx context.Context) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE cmf_sports_match
		SET bet_status=CASE
				WHEN settle_status=0 AND status IN ('NS','TBD') AND bet_close_time>? THEN 1
				ELSE 2
			END,
			update_time=?
		WHERE bet_status<>0
		  AND bet_status<>CASE
				WHEN settle_status=0 AND status IN ('NS','TBD') AND bet_close_time>? THEN 1
				ELSE 2
			END`, now, now, now)
	return err
}

func (s *SportsService) logCollection(ctx context.Context, apiName string, success bool, message string) {
	value := 0
	if success {
		value = 1
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO cmf_sports_sync_log(source,api_name,success,message,raw_response,create_time) VALUES('api-football',?,?,?,'',?)`, apiName, value, truncateString(message, 255), time.Now().Unix())
}

func truncateString(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
