package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

type SportsSyncConfig struct {
	BaseURL         string
	APIKey          string
	LiveInterval    time.Duration
	CatalogInterval time.Duration
	OddsInterval    time.Duration
}

type sportsSyncState struct {
	config      SportsSyncConfig
	nextLive    time.Time
	nextCatalog time.Time
	nextOdds    time.Time
	client      *http.Client
}

func (r *Runner) ConfigureSportsSync(config SportsSyncConfig) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.APIKey = strings.TrimSpace(config.APIKey)
	if config.LiveInterval < time.Minute {
		config.LiveInterval = 5 * time.Minute
	}
	if config.CatalogInterval < time.Minute {
		config.CatalogInterval = 6 * time.Hour
	}
	if config.OddsInterval < time.Minute {
		config.OddsInterval = 3 * time.Hour
	}
	r.sports = sportsSyncState{
		config: config,
		client: &http.Client{Timeout: 12 * time.Second},
	}
}

func (r *Runner) runSportsSync(ctx context.Context) error {
	config := r.sports.config
	if config.APIKey == "" || config.BaseURL == "" {
		return nil
	}
	now := r.now()
	failures := make([]error, 0, 3)
	if r.sports.nextLive.IsZero() || !now.Before(r.sports.nextLive) {
		count, err := r.syncSportsFixtures(ctx, url.Values{
			"live": {"all"}, "timezone": {"Asia/Shanghai"},
		}, false, nil)
		r.recordSportsSync(ctx, "api-football", "live", count, err)
		if err != nil {
			failures = append(failures, fmt.Errorf("live fixtures: %w", err))
			r.sports.nextLive = now.Add(15 * time.Minute)
		} else {
			r.sports.nextLive = now.Add(config.LiveInterval)
		}
	}
	if r.sports.nextCatalog.IsZero() || !now.Before(r.sports.nextCatalog) {
		catalogTotal := 0
		oddsTotal := 0
		var catalogErr error
		yesterday := now.In(time.FixedZone("CST", 8*60*60)).
			AddDate(0, 0, -1).Format("2006-01-02")
		count, err := r.syncSportsFixtures(ctx, url.Values{
			"date": {yesterday}, "timezone": {"Asia/Shanghai"},
		}, false, nil)
		catalogTotal += count
		if err != nil {
			catalogErr = fmt.Errorf("%s: %w", yesterday, err)
		}
		for _, offset := range []int{0, 1} {
			if catalogErr != nil {
				break
			}
			date := now.In(time.FixedZone("CST", 8*60*60)).
				AddDate(0, 0, offset).Format("2006-01-02")
			fixtureCount, optionCount, err := r.syncSportsCatalogWithOdds(ctx, date)
			catalogTotal += fixtureCount
			oddsTotal += optionCount
			if err != nil {
				catalogErr = fmt.Errorf("%s: %w", date, err)
			}
		}
		r.recordSportsSync(ctx, "api-football", "catalog", catalogTotal, catalogErr)
		r.recordSportsSync(ctx, "api-football", "odds", oddsTotal, catalogErr)
		if catalogErr != nil {
			failures = append(failures, fmt.Errorf("fixture catalog with odds: %w", catalogErr))
			r.sports.nextCatalog = now.Add(15 * time.Minute)
			r.sports.nextOdds = now.Add(15 * time.Minute)
		} else {
			pruned, pruneErr := r.pruneSportsMatchesWithoutOdds(ctx)
			r.recordSportsSync(ctx, "api-football", "odds_prune", pruned, pruneErr)
			if pruneErr != nil {
				failures = append(failures, fmt.Errorf("prune matches without odds: %w", pruneErr))
			}
			r.sports.nextCatalog = now.Add(config.CatalogInterval)
			r.sports.nextOdds = now.Add(config.OddsInterval)
		}
	} else if r.sports.nextOdds.IsZero() || !now.Before(r.sports.nextOdds) {
		total := 0
		var oddsErr error
		for _, offset := range []int{0, 1} {
			date := now.In(time.FixedZone("CST", 8*60*60)).
				AddDate(0, 0, offset).Format("2006-01-02")
			count, err := r.syncSportsOdds(ctx, date)
			total += count
			if err != nil {
				oddsErr = fmt.Errorf("%s: %w", date, err)
				break
			}
		}
		r.recordSportsSync(ctx, "api-football", "odds", total, oddsErr)
		if oddsErr != nil {
			failures = append(failures, fmt.Errorf("odds: %w", oddsErr))
			r.sports.nextOdds = now.Add(30 * time.Minute)
		} else {
			pruned, pruneErr := r.pruneSportsMatchesWithoutOdds(ctx)
			r.recordSportsSync(ctx, "api-football", "odds_prune", pruned, pruneErr)
			if pruneErr != nil {
				failures = append(failures, fmt.Errorf("prune matches without odds: %w", pruneErr))
			}
			r.sports.nextOdds = now.Add(config.OddsInterval)
		}
	}
	return errors.Join(failures...)
}

type apiFootballFixtureResponse struct {
	Errors   any                  `json:"errors"`
	Response []apiFootballFixture `json:"response"`
}

type apiFootballFixture struct {
	Fixture struct {
		ID        int64 `json:"id"`
		Timestamp int64 `json:"timestamp"`
		Status    struct {
			Long  string `json:"long"`
			Short string `json:"short"`
		} `json:"status"`
	} `json:"fixture"`
	League struct {
		Name    string `json:"name"`
		Country string `json:"country"`
		Round   string `json:"round"`
	} `json:"league"`
	Teams struct {
		Home struct {
			Name string `json:"name"`
			Logo string `json:"logo"`
		} `json:"home"`
		Away struct {
			Name string `json:"name"`
			Logo string `json:"logo"`
		} `json:"away"`
	} `json:"teams"`
	Goals struct {
		Home *int `json:"home"`
		Away *int `json:"away"`
	} `json:"goals"`
}

func (r *Runner) syncSportsFixtures(
	ctx context.Context,
	query url.Values,
	allowInsert bool,
	allowedIDs map[int64]struct{},
) (int, error) {
	var payload apiFootballFixtureResponse
	if err := r.fetchSports(ctx, "/fixtures", query, &payload); err != nil {
		return 0, err
	}
	if apiSportsHasErrors(payload.Errors) {
		return 0, fmt.Errorf("upstream errors: %v", payload.Errors)
	}
	stored := 0
	for _, fixture := range payload.Response {
		if fixture.Fixture.ID < 1 || fixture.Fixture.Timestamp < 1 {
			continue
		}
		if allowInsert && allowedIDs != nil {
			if _, allowed := allowedIDs[fixture.Fixture.ID]; !allowed {
				continue
			}
		}
		wasStored, err := r.storeSportsFixture(ctx, fixture, allowInsert)
		if err != nil {
			return stored, err
		}
		if wasStored {
			stored++
		}
	}
	return stored, nil
}

func (r *Runner) storeSportsFixture(
	ctx context.Context,
	fixture apiFootballFixture,
	allowInsert bool,
) (bool, error) {
	status := normalizedSportsStatus(fixture.Fixture.Status.Short)
	var exists int
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM sports_matches
			WHERE source='api-football' AND source_match_id=?
		)`,
		fixture.Fixture.ID,
	).Scan(&exists); err != nil {
		return false, err
	}
	if exists == 0 && (!allowInsert || sportsMatchIsTerminal(status)) {
		return false, nil
	}
	publicID, err := idgen.New()
	if err != nil {
		return false, err
	}
	kickoffAt := time.Unix(fixture.Fixture.Timestamp, 0)
	betCloseAt := kickoffAt.Add(-5 * time.Minute)
	betStatus := 0
	if status == "NS" && betCloseAt.After(r.now()) {
		betStatus = 1
	}
	homeScore, awayScore := 0, 0
	if fixture.Goals.Home != nil {
		homeScore = *fixture.Goals.Home
	}
	if fixture.Goals.Away != nil {
		awayScore = *fixture.Goals.Away
	}
	rawPayload, err := json.Marshal(fixture)
	if err != nil {
		return false, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sports_matches
			(public_match_id,source,source_match_id,competition,competition_type,
			 home_name,away_name,home_logo_url,away_logo_url,kickoff_at,bet_close_at,
			 home_score,away_score,match_status,bet_status,settle_status,min_bet,max_bet,
			 raw_payload,source_updated_at)
		VALUES(?,'api-football',?,?,?,?,?,?,?,?,?,?,?,?,?,0,10,500000,?,CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			competition=VALUES(competition),competition_type=VALUES(competition_type),
			home_name=VALUES(home_name),away_name=VALUES(away_name),
			home_logo_url=VALUES(home_logo_url),away_logo_url=VALUES(away_logo_url),
			kickoff_at=VALUES(kickoff_at),bet_close_at=VALUES(bet_close_at),
			home_score=IF(settle_status<>0,home_score,VALUES(home_score)),
			away_score=IF(settle_status<>0,away_score,VALUES(away_score)),
			match_status=IF(settle_status<>0,match_status,VALUES(match_status)),
			bet_status=IF(settle_status<>0,bet_status,VALUES(bet_status)),
			raw_payload=VALUES(raw_payload),source_updated_at=VALUES(source_updated_at)`,
		publicID, strconv.FormatInt(fixture.Fixture.ID, 10),
		boundedSportsText(fixture.League.Name, 190),
		boundedSportsText(fixture.League.Country, 60),
		boundedSportsText(fixture.Teams.Home.Name, 190),
		boundedSportsText(fixture.Teams.Away.Name, 190),
		boundedSportsText(fixture.Teams.Home.Logo, 1000),
		boundedSportsText(fixture.Teams.Away.Logo, 1000),
		kickoffAt, betCloseAt, homeScore, awayScore, status, betStatus, rawPayload,
	)
	if err != nil {
		return false, err
	}
	var matchID int64
	var settleStatus int
	if err = tx.QueryRowContext(ctx, `
		SELECT id,settle_status FROM sports_matches
		WHERE source='api-football' AND source_match_id=? FOR UPDATE`,
		fixture.Fixture.ID,
	).Scan(&matchID, &settleStatus); err != nil {
		return false, err
	}
	if settleStatus == 0 && (status == "FT" || status == "CANCELLED") {
		if status == "FT" {
			if err = finalizeSportsMarketResults(ctx, tx, matchID, homeScore, awayScore); err != nil {
				return false, err
			}
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE sports_matches SET bet_status=0,settle_status=1 WHERE id=?`,
			matchID,
		); err != nil {
			return false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func finalizeSportsMarketResults(
	ctx context.Context,
	tx *sql.Tx,
	matchID int64,
	homeScore int,
	awayScore int,
) error {
	matchWinner := "DRAW"
	if homeScore > awayScore {
		matchWinner = "HOME_WIN"
	} else if awayScore > homeScore {
		matchWinner = "AWAY_WIN"
	}
	if err := updateSportsMarketWinners(
		ctx, tx, matchID, "MATCH_RESULT", []string{matchWinner},
	); err != nil {
		return err
	}
	doubleChance := []string{"HOME_OR_DRAW", "DRAW_OR_AWAY"}
	if homeScore > awayScore {
		doubleChance = []string{"HOME_OR_DRAW", "HOME_OR_AWAY"}
	} else if awayScore > homeScore {
		doubleChance = []string{"HOME_OR_AWAY", "DRAW_OR_AWAY"}
	}
	if err := updateSportsMarketWinners(
		ctx, tx, matchID, "DOUBLE_CHANCE", doubleChance,
	); err != nil {
		return err
	}
	bothTeamsScore := "NO"
	if homeScore > 0 && awayScore > 0 {
		bothTeamsScore = "YES"
	}
	if err := updateSportsMarketWinners(
		ctx, tx, matchID, "BOTH_TEAMS_SCORE", []string{bothTeamsScore},
	); err != nil {
		return err
	}
	oddEven := "EVEN"
	if (homeScore+awayScore)%2 == 1 {
		oddEven = "ODD"
	}
	if err := updateSportsMarketWinners(
		ctx, tx, matchID, "ODD_EVEN", []string{oddEven},
	); err != nil {
		return err
	}
	totalCodes, err := sportsMarketOptionCodes(ctx, tx, matchID, "TOTAL_GOALS")
	if err != nil {
		return err
	}
	totalWinners := make([]string, 0, len(totalCodes)/2)
	for _, code := range totalCodes {
		if totalGoalsOptionWins(code, homeScore+awayScore) {
			totalWinners = append(totalWinners, code)
		}
	}
	if err = updateSportsMarketWinners(
		ctx, tx, matchID, "TOTAL_GOALS", totalWinners,
	); err != nil {
		return err
	}
	exactCode := fmt.Sprintf("CS_%d_%d", homeScore, awayScore)
	var hasExact int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sports_market_options option_item
		JOIN sports_markets market ON market.id=option_item.market_id
		WHERE market.match_id=? AND market.market_code='CORRECT_SCORE'
		  AND option_item.option_code=?`,
		matchID, exactCode,
	).Scan(&hasExact); err != nil {
		return err
	}
	winnerCode := exactCode
	if hasExact == 0 {
		winnerCode = "OTHER"
	}
	return updateSportsMarketWinners(
		ctx, tx, matchID, "CORRECT_SCORE", []string{winnerCode},
	)
}

func sportsMarketOptionCodes(
	ctx context.Context,
	tx *sql.Tx,
	matchID int64,
	marketCode string,
) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT option_item.option_code
		FROM sports_market_options option_item
		JOIN sports_markets market ON market.id=option_item.market_id
		WHERE market.match_id=? AND market.market_code=? AND option_item.status=1`,
		matchID, marketCode,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0, 16)
	for rows.Next() {
		var code string
		if err = rows.Scan(&code); err != nil {
			return nil, err
		}
		items = append(items, code)
	}
	return items, rows.Err()
}

func updateSportsMarketWinners(
	ctx context.Context,
	tx *sql.Tx,
	matchID int64,
	marketCode string,
	winners []string,
) error {
	if len(winners) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(winners)), ",")
	args := make([]any, 0, len(winners)+2)
	for _, winner := range winners {
		args = append(args, winner)
	}
	args = append(args, matchID, marketCode)
	_, err := tx.ExecContext(ctx, `
		UPDATE sports_market_options option_item
		JOIN sports_markets market ON market.id=option_item.market_id
		SET option_item.result=IF(option_item.option_code IN (`+placeholders+`),1,2),
		    option_item.source_updated_at=CURRENT_TIMESTAMP(3)
		WHERE market.match_id=? AND market.market_code=?`,
		args...,
	)
	return err
}

type apiSportsScalar string

func (value *apiSportsScalar) UnmarshalJSON(raw []byte) error {
	var text string
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		*value = apiSportsScalar(text)
		return nil
	}
	text = strings.TrimSpace(string(raw))
	if text == "null" {
		*value = ""
		return nil
	}
	if _, ok := new(big.Rat).SetString(text); !ok {
		return fmt.Errorf("invalid scalar %q", text)
	}
	*value = apiSportsScalar(text)
	return nil
}

type apiFootballOddsResponse struct {
	Errors any `json:"errors"`
	Paging struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"paging"`
	Response []apiFootballOdds `json:"response"`
}

type apiFootballOdds struct {
	Fixture struct {
		ID int64 `json:"id"`
	} `json:"fixture"`
	Bookmakers []struct {
		Bets []struct {
			ID     int64                 `json:"id"`
			Name   string                `json:"name"`
			Values []apiFootballOddValue `json:"values"`
		} `json:"bets"`
	} `json:"bookmakers"`
}

type apiFootballOddValue struct {
	Value apiSportsScalar `json:"value"`
	Odd   apiSportsScalar `json:"odd"`
}

type synchronizedSportsMarket struct {
	Code    string
	Name    string
	Rule    string
	Options []synchronizedSportsOption
}

type synchronizedSportsOption struct {
	Code       string
	Name       string
	OddsScaled int64
}

func (r *Runner) syncSportsOdds(ctx context.Context, date string) (int, error) {
	items, err := r.fetchSportsOdds(ctx, date)
	if err != nil {
		return 0, err
	}
	total, err := r.storeSportsOddsItems(ctx, items)
	if err != nil {
		return total, err
	}
	if err = r.reconcileSportsOddsSnapshot(
		ctx, date, qualifiedSportsFixtureIDs(items),
	); err != nil {
		return total, err
	}
	return total, nil
}

func (r *Runner) syncSportsCatalogWithOdds(
	ctx context.Context,
	date string,
) (int, int, error) {
	items, err := r.fetchSportsOdds(ctx, date)
	if err != nil {
		return 0, 0, err
	}
	allowedIDs := qualifiedSportsFixtureIDs(items)
	fixtureCount, err := r.syncSportsFixtures(ctx, url.Values{
		"date": {date}, "timezone": {"Asia/Shanghai"},
	}, true, allowedIDs)
	if err != nil {
		return fixtureCount, 0, err
	}
	optionCount, err := r.storeSportsOddsItems(ctx, items)
	if err == nil {
		err = r.reconcileSportsOddsSnapshot(ctx, date, allowedIDs)
	}
	return fixtureCount, optionCount, err
}

func qualifiedSportsFixtureIDs(items []apiFootballOdds) map[int64]struct{} {
	ids := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.Fixture.ID > 0 && len(normalizedSportsMarkets(item)) > 0 {
			ids[item.Fixture.ID] = struct{}{}
		}
	}
	return ids
}

func (r *Runner) fetchSportsOdds(ctx context.Context, date string) ([]apiFootballOdds, error) {
	items := make([]apiFootballOdds, 0, 256)
	for page := 1; page <= 100; page++ {
		var payload apiFootballOddsResponse
		if err := r.fetchSports(ctx, "/odds", url.Values{
			"date": {date}, "timezone": {"Asia/Shanghai"}, "page": {strconv.Itoa(page)},
		}, &payload); err != nil {
			return nil, err
		}
		if apiSportsHasErrors(payload.Errors) {
			return nil, fmt.Errorf("upstream errors: %v", payload.Errors)
		}
		items = append(items, payload.Response...)
		if payload.Paging.Total == 0 || page >= payload.Paging.Total {
			return items, nil
		}
	}
	return nil, errors.New("sports odds pagination exceeded safety limit")
}

func (r *Runner) storeSportsOddsItems(
	ctx context.Context,
	items []apiFootballOdds,
) (int, error) {
	total := 0
	for _, item := range items {
		count, err := r.storeSportsOdds(ctx, item)
		total += count
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (r *Runner) reconcileSportsOddsSnapshot(
	ctx context.Context,
	date string,
	qualifiedIDs map[int64]struct{},
) error {
	location := time.FixedZone("CST", 8*60*60)
	from, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		return fmt.Errorf("invalid sports snapshot date %q: %w", date, err)
	}
	to := from.Add(24 * time.Hour)
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, statement := range []string{
		`CREATE TEMPORARY TABLE current_sports_odds_fixture_ids (
			source_match_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
			PRIMARY KEY (source_match_id)
		) ENGINE=MEMORY`,
		`CREATE TEMPORARY TABLE stale_sports_matches_without_odds (
			id bigint unsigned NOT NULL,
			has_orders tinyint unsigned NOT NULL,
			PRIMARY KEY (id)
		) ENGINE=MEMORY`,
	} {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	defer func() {
		_, _ = tx.ExecContext(context.Background(), `
			DROP TEMPORARY TABLE IF EXISTS stale_sports_matches_without_odds`)
		_, _ = tx.ExecContext(context.Background(), `
			DROP TEMPORARY TABLE IF EXISTS current_sports_odds_fixture_ids`)
	}()
	insertCurrent, err := tx.PrepareContext(ctx, `
		INSERT IGNORE INTO current_sports_odds_fixture_ids(source_match_id) VALUES(?)`)
	if err != nil {
		return err
	}
	for fixtureID := range qualifiedIDs {
		if _, err = insertCurrent.ExecContext(ctx, strconv.FormatInt(fixtureID, 10)); err != nil {
			insertCurrent.Close() //nolint:errcheck
			return err
		}
	}
	if err = insertCurrent.Close(); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO stale_sports_matches_without_odds(id,has_orders)
		SELECT match_row.id,
		       EXISTS(
			       SELECT 1 FROM sports_bet_orders bet_order
			       WHERE bet_order.match_id=match_row.id
		       )
		FROM sports_matches match_row
		LEFT JOIN current_sports_odds_fixture_ids current_fixture
		  ON current_fixture.source_match_id=match_row.source_match_id
		WHERE match_row.source='api-football'
		  AND match_row.kickoff_at>=?
		  AND match_row.kickoff_at<?
		  AND current_fixture.source_match_id IS NULL`,
		from, to,
	); err != nil {
		return err
	}
	for _, statement := range []string{
		`UPDATE sports_matches match_row
		 JOIN stale_sports_matches_without_odds stale ON stale.id=match_row.id
		 SET match_row.bet_status=0`,
		`UPDATE sports_markets market
		 JOIN stale_sports_matches_without_odds stale ON stale.id=market.match_id
		 SET market.status=0`,
		`UPDATE sports_market_options option_item
		 JOIN sports_markets market ON market.id=option_item.market_id
		 JOIN stale_sports_matches_without_odds stale ON stale.id=market.match_id
		 SET option_item.status=0`,
		`DELETE settlement
		 FROM sports_settlement_runs settlement
		 JOIN stale_sports_matches_without_odds stale ON stale.id=settlement.match_id
		 WHERE stale.has_orders=0`,
		`DELETE score_event
		 FROM sports_score_events score_event
		 JOIN stale_sports_matches_without_odds stale ON stale.id=score_event.match_id
		 WHERE stale.has_orders=0`,
		`DELETE option_item
		 FROM sports_market_options option_item
		 JOIN sports_markets market ON market.id=option_item.market_id
		 JOIN stale_sports_matches_without_odds stale ON stale.id=market.match_id
		 WHERE stale.has_orders=0`,
		`DELETE market
		 FROM sports_markets market
		 JOIN stale_sports_matches_without_odds stale ON stale.id=market.match_id
		 WHERE stale.has_orders=0`,
		`DELETE match_row
		 FROM sports_matches match_row
		 JOIN stale_sports_matches_without_odds stale ON stale.id=match_row.id
		 WHERE stale.has_orders=0`,
	} {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	for _, statement := range []string{
		`DROP TEMPORARY TABLE stale_sports_matches_without_odds`,
		`DROP TEMPORARY TABLE current_sports_odds_fixture_ids`,
	} {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Runner) storeSportsOdds(ctx context.Context, item apiFootballOdds) (int, error) {
	if item.Fixture.ID < 1 {
		return 0, nil
	}
	var matchID int64
	var matchStatus string
	var settleStatus int
	err := r.db.QueryRowContext(ctx, `
		SELECT id,match_status,settle_status FROM sports_matches
		WHERE source='api-football' AND source_match_id=?`,
		item.Fixture.ID,
	).Scan(&matchID, &matchStatus, &settleStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	// Pre-match odds are mutable, but a late upstream odds response must never
	// reopen a finished/cancelled event or erase option settlement results.
	if settleStatus != 0 || sportsMatchIsTerminal(matchStatus) {
		return 0, nil
	}
	markets := normalizedSportsMarkets(item)
	if len(markets) == 0 {
		return 0, nil
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	if err = tx.QueryRowContext(ctx, `
		SELECT match_status,settle_status
		FROM sports_matches WHERE id=? FOR UPDATE`,
		matchID,
	).Scan(&matchStatus, &settleStatus); errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	if settleStatus != 0 || sportsMatchIsTerminal(matchStatus) {
		return 0, nil
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE sports_markets SET status=0 WHERE match_id=?`,
		matchID,
	); err != nil {
		return 0, err
	}
	stored := 0
	for marketIndex, market := range markets {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO sports_markets
				(match_id,market_code,name,settlement_rule,status,sort_order)
			VALUES(?,?,?,?,1,?)
			ON DUPLICATE KEY UPDATE
				name=VALUES(name),settlement_rule=VALUES(settlement_rule),
				status=1,sort_order=VALUES(sort_order)`,
			matchID, market.Code, market.Name, market.Rule, len(markets)-marketIndex,
		)
		if err != nil {
			return stored, err
		}
		var marketID int64
		if err = tx.QueryRowContext(ctx, `
			SELECT id FROM sports_markets WHERE match_id=? AND market_code=?`,
			matchID, market.Code,
		).Scan(&marketID); err != nil {
			return stored, err
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE sports_market_options SET status=0 WHERE market_id=?`,
			marketID,
		); err != nil {
			return stored, err
		}
		for _, option := range market.Options {
			if _, err = tx.ExecContext(ctx, `
				INSERT INTO sports_market_options
					(market_id,option_code,name,odds_scaled,result,status,source_updated_at)
				VALUES(?,?,?,?,0,1,CURRENT_TIMESTAMP(3))
				ON DUPLICATE KEY UPDATE
					name=VALUES(name),odds_scaled=VALUES(odds_scaled),
					result=0,status=1,source_updated_at=VALUES(source_updated_at)`,
				marketID, option.Code, option.Name, option.OddsScaled,
			); err != nil {
				return stored, err
			}
			stored++
		}
	}
	if err = tx.Commit(); err != nil {
		return stored, err
	}
	return stored, nil
}

func (r *Runner) pruneSportsMatchesWithoutOdds(ctx context.Context) (int, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(ctx, `
		CREATE TEMPORARY TABLE prune_sports_matches_without_odds (
			id bigint unsigned NOT NULL,
			PRIMARY KEY (id)
		) ENGINE=MEMORY`); err != nil {
		return 0, err
	}
	defer tx.ExecContext(context.Background(), `
		DROP TEMPORARY TABLE IF EXISTS prune_sports_matches_without_odds`) //nolint:errcheck
	result, err := tx.ExecContext(ctx, `
		INSERT INTO prune_sports_matches_without_odds(id)
		SELECT match_row.id
		FROM sports_matches match_row
		WHERE match_row.source='api-football'
		  AND NOT EXISTS (
			  SELECT 1
			  FROM sports_markets market
			  JOIN sports_market_options option_item
			    ON option_item.market_id=market.id
			   AND option_item.odds_scaled>1000000
			  WHERE market.match_id=match_row.id
		  )
		  AND NOT EXISTS (
			  SELECT 1 FROM sports_bet_orders bet_order
			  WHERE bet_order.match_id=match_row.id
		  )`)
	if err != nil {
		return 0, err
	}
	pruned, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	for _, statement := range []string{
		`DELETE settlement
		 FROM sports_settlement_runs settlement
		 JOIN prune_sports_matches_without_odds target ON target.id=settlement.match_id`,
		`DELETE score_event
		 FROM sports_score_events score_event
		 JOIN prune_sports_matches_without_odds target ON target.id=score_event.match_id`,
		`DELETE option_item
		 FROM sports_market_options option_item
		 JOIN sports_markets market ON market.id=option_item.market_id
		 JOIN prune_sports_matches_without_odds target ON target.id=market.match_id`,
		`DELETE market
		 FROM sports_markets market
		 JOIN prune_sports_matches_without_odds target ON target.id=market.match_id`,
		`DELETE match_row
		 FROM sports_matches match_row
		 JOIN prune_sports_matches_without_odds target ON target.id=match_row.id`,
	} {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return 0, err
		}
	}
	if _, err = tx.ExecContext(ctx, `
		DROP TEMPORARY TABLE prune_sports_matches_without_odds`); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return int(pruned), nil
}

func sportsMatchIsTerminal(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FT", "AET", "PEN", "CANC", "CANCELLED", "PST", "ABD", "AWD", "WO":
		return true
	default:
		return false
	}
}

func normalizedSportsMarkets(item apiFootballOdds) []synchronizedSportsMarket {
	for _, bookmaker := range item.Bookmakers {
		marketsByCode := make(map[string]synchronizedSportsMarket, 6)
		for _, bet := range bookmaker.Bets {
			switch {
			case bet.ID == 1 || strings.EqualFold(bet.Name, "Match Winner") ||
				strings.EqualFold(bet.Name, "Fulltime Result"):
				options := normalizedSportsOptions(bet.Values, normalizedMatchWinner)
				if len(options) == 3 {
					marketsByCode["MATCH_RESULT"] = synchronizedSportsMarket{
						Code: "MATCH_RESULT", Name: "胜平负",
						Rule: "full_time_result", Options: options,
					}
				}
			case bet.ID == 5 || strings.EqualFold(bet.Name, "Goals Over/Under"):
				options := normalizedSportsOptions(bet.Values, normalizedTotalGoals)
				if len(options) >= 2 {
					marketsByCode["TOTAL_GOALS"] = synchronizedSportsMarket{
						Code: "TOTAL_GOALS", Name: "全场大小球",
						Rule: "full_time_total_goals", Options: options,
					}
				}
			case bet.ID == 8 || strings.EqualFold(bet.Name, "Both Teams Score"):
				options := normalizedSportsOptions(bet.Values, normalizedBothTeamsScore)
				if len(options) == 2 {
					marketsByCode["BOTH_TEAMS_SCORE"] = synchronizedSportsMarket{
						Code: "BOTH_TEAMS_SCORE", Name: "双方进球",
						Rule: "full_time_both_teams_score", Options: options,
					}
				}
			case bet.ID == 12 || strings.EqualFold(bet.Name, "Double Chance"):
				options := normalizedSportsOptions(bet.Values, normalizedDoubleChance)
				if len(options) == 3 {
					marketsByCode["DOUBLE_CHANCE"] = synchronizedSportsMarket{
						Code: "DOUBLE_CHANCE", Name: "双重机会",
						Rule: "full_time_double_chance", Options: options,
					}
				}
			case bet.ID == 21 || strings.EqualFold(bet.Name, "Odd/Even"):
				options := normalizedSportsOptions(bet.Values, normalizedOddEven)
				if len(options) == 2 {
					marketsByCode["ODD_EVEN"] = synchronizedSportsMarket{
						Code: "ODD_EVEN", Name: "总进球单双",
						Rule: "full_time_total_odd_even", Options: options,
					}
				}
			case bet.ID == 10 || strings.EqualFold(bet.Name, "Exact Score") ||
				strings.EqualFold(bet.Name, "Correct Score"):
				options := normalizedSportsOptions(bet.Values, normalizedCorrectScore)
				if len(options) > 0 {
					marketsByCode["CORRECT_SCORE"] = synchronizedSportsMarket{
						Code: "CORRECT_SCORE", Name: "全场比分",
						Rule: "full_time_score", Options: options,
					}
				}
			}
		}
		markets := make([]synchronizedSportsMarket, 0, len(marketsByCode))
		for _, code := range []string{
			"MATCH_RESULT", "DOUBLE_CHANCE", "TOTAL_GOALS",
			"BOTH_TEAMS_SCORE", "ODD_EVEN", "CORRECT_SCORE",
		} {
			if market, ok := marketsByCode[code]; ok {
				markets = append(markets, market)
			}
		}
		if len(markets) > 0 {
			return markets
		}
	}
	return nil
}

func normalizedSportsOptions(
	values []apiFootballOddValue,
	normalizer func(string) (string, string),
) []synchronizedSportsOption {
	items := make([]synchronizedSportsOption, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		code, name := normalizer(string(raw.Value))
		odds, ok := scaledSportsOdds(string(raw.Odd))
		if code == "" || !ok {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		items = append(items, synchronizedSportsOption{
			Code: code, Name: name, OddsScaled: odds,
		})
	}
	return items
}

func normalizedMatchWinner(value string) (string, string) {
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

func normalizedTotalGoals(value string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) != 2 {
		return "", ""
	}
	side := strings.ToLower(parts[0])
	line := strings.TrimSpace(parts[1])
	if side != "over" && side != "under" || !strings.HasSuffix(line, ".5") {
		return "", ""
	}
	if _, err := strconv.ParseFloat(line, 64); err != nil {
		return "", ""
	}
	prefix, name := "OVER_", "大于 "
	if side == "under" {
		prefix, name = "UNDER_", "小于 "
	}
	return prefix + strings.ReplaceAll(line, ".", "_"), name + line
}

func normalizedBothTeamsScore(value string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes":
		return "YES", "是"
	case "no":
		return "NO", "否"
	default:
		return "", ""
	}
}

func normalizedDoubleChance(value string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "home/draw", "1/x":
		return "HOME_OR_DRAW", "主胜或平"
	case "home/away", "1/2":
		return "HOME_OR_AWAY", "主胜或客胜"
	case "draw/away", "x/2":
		return "DRAW_OR_AWAY", "平或客胜"
	default:
		return "", ""
	}
}

func normalizedOddEven(value string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "odd":
		return "ODD", "单"
	case "even":
		return "EVEN", "双"
	default:
		return "", ""
	}
}

func totalGoalsOptionWins(code string, total int) bool {
	parts := strings.SplitN(code, "_", 2)
	if len(parts) != 2 || (parts[0] != "OVER" && parts[0] != "UNDER") {
		return false
	}
	line, err := strconv.ParseFloat(strings.ReplaceAll(parts[1], "_", "."), 64)
	if err != nil {
		return false
	}
	if parts[0] == "OVER" {
		return float64(total) > line
	}
	return float64(total) < line
}

func normalizedCorrectScore(value string) (string, string) {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.Contains(lower, "other") || strings.Contains(lower, "any") {
		return "OTHER", "其他比分"
	}
	parts := strings.FieldsFunc(value, func(item rune) bool {
		return item == ':' || item == '-'
	})
	if len(parts) != 2 {
		return "", ""
	}
	home, homeErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	away, awayErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if homeErr != nil || awayErr != nil || home < 0 || home > 20 || away < 0 || away > 20 {
		return "", ""
	}
	return fmt.Sprintf("CS_%d_%d", home, away), fmt.Sprintf("%d:%d", home, away)
}

func scaledSportsOdds(value string) (int64, bool) {
	rational, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rational.Cmp(big.NewRat(1, 1)) <= 0 ||
		rational.Cmp(big.NewRat(1000, 1)) > 0 {
		return 0, false
	}
	scaled := new(big.Rat).Mul(rational, big.NewRat(1_000_000, 1))
	result := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	if !result.IsInt64() {
		return 0, false
	}
	return result.Int64(), true
}

func normalizedSportsStatus(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "FT", "AET", "PEN":
		return "FT"
	case "CANC", "PST", "ABD", "AWD", "WO":
		return "CANCELLED"
	case "HT":
		return "HT"
	case "1H", "2H", "ET", "BT", "P", "LIVE", "SUSP", "INT":
		return "LIVE"
	default:
		return "NS"
	}
}

func (r *Runner) fetchSports(
	ctx context.Context,
	path string,
	query url.Values,
	target any,
) error {
	endpoint, err := url.Parse(r.sports.config.BaseURL + path)
	if err != nil {
		return err
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	request.Header.Set("x-apisports-key", r.sports.config.APIKey)
	response, err := r.sports.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream returned HTTP %d", response.StatusCode)
	}
	if err = json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode upstream response: %w", err)
	}
	return nil
}

func (r *Runner) recordSportsSync(
	ctx context.Context,
	source string,
	syncType string,
	count int,
	failure error,
) {
	status := 1
	message := ""
	if failure != nil {
		status = 2
		message = boundedSportsText(failure.Error(), 1000)
	}
	_, _ = r.db.ExecContext(ctx, `
		INSERT INTO sports_sync_logs
			(sync_type,source,status,received_count,changed_count,error_message)
		VALUES(?,?,?,?,?,?)`,
		syncType, boundedSportsText(source, 40), status, count, count, message,
	)
}

func apiSportsHasErrors(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func boundedSportsText(value string, maximum int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maximum {
		return string(runes)
	}
	return string(runes[:maximum])
}
