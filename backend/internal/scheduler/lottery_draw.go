package scheduler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const lotteryIssueBuffer = 12

type lotteryDrawConfig struct {
	Mode         string `json:"draw_mode"`
	Count        int    `json:"draw_count"`
	Minimum      int    `json:"number_min"`
	Maximum      int    `json:"number_max"`
	Pad          int    `json:"number_pad"`
	Unique       int    `json:"number_unique"`
	TemplateCode string `json:"template_code"`
}

type lotteryGameConfig struct {
	Draw lotteryDrawConfig `json:"draw"`
}

type lotteryGameSchedule struct {
	ID           int64
	Code         string
	Interval     int
	CloseSeconds int
	Config       lotteryDrawConfig
}

func (r *Runner) ensureLotteryIssues(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,game_code,issue_interval_seconds,sale_close_seconds,config
		FROM lottery_games
		WHERE status=1 AND issue_interval_seconds>=60
		ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	games := make([]lotteryGameSchedule, 0, 96)
	for rows.Next() {
		var item lotteryGameSchedule
		var rawConfig []byte
		if err = rows.Scan(
			&item.ID, &item.Code, &item.Interval, &item.CloseSeconds, &rawConfig,
		); err != nil {
			return err
		}
		var config lotteryGameConfig
		if len(rawConfig) > 0 {
			if decodeErr := json.Unmarshal(rawConfig, &config); decodeErr != nil {
				r.logger.Warn(
					"skip lottery game with invalid draw config",
					"game_id", item.ID, "error", decodeErr,
				)
				continue
			}
		}
		item.Config = config.Draw
		if item.Config.Mode != "local_auto" {
			continue
		}
		if validateDrawConfig(item.Config) != nil {
			r.logger.Warn(
				"skip lottery game with unsafe draw config",
				"game_id", item.ID, "game_code", item.Code,
			)
			continue
		}
		games = append(games, item)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	now := r.now()
	for _, game := range games {
		if err = r.ensureGameIssues(ctx, game, now); err != nil {
			return fmt.Errorf("ensure issues for %s: %w", game.Code, err)
		}
	}
	return nil
}

func (r *Runner) ensureGameIssues(
	ctx context.Context,
	game lotteryGameSchedule,
	now time.Time,
) error {
	firstDraw := nextDrawAt(now, game.Interval)
	if !firstDraw.Add(-time.Duration(game.CloseSeconds) * time.Second).After(now) {
		firstDraw = firstDraw.Add(time.Duration(game.Interval) * time.Second)
	}
	for index := 0; index < lotteryIssueBuffer; index++ {
		drawAt := firstDraw.Add(time.Duration(index*game.Interval) * time.Second)
		saleOpenAt := drawAt.Add(-time.Duration(game.Interval) * time.Second)
		saleCloseAt := drawAt.Add(-time.Duration(game.CloseSeconds) * time.Second)
		issueNo := drawAt.In(now.Location()).Format("20060102150405")
		if _, err := r.db.ExecContext(ctx, `
			INSERT IGNORE INTO lottery_issues
				(game_id,issue_no,sale_open_at,sale_close_at,draw_at,status)
			VALUES(?,?,?,?,?,1)`,
			game.ID, issueNo, saleOpenAt, saleCloseAt, drawAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func nextDrawAt(now time.Time, intervalSeconds int) time.Time {
	if intervalSeconds >= 86_400 && intervalSeconds%86_400 == 0 {
		startOfDay := time.Date(
			now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location(),
		)
		days := intervalSeconds / 86_400
		return startOfDay.AddDate(0, 0, days)
	}
	nextUnix := ((now.Unix() / int64(intervalSeconds)) + 1) * int64(intervalSeconds)
	return time.Unix(nextUnix, 0).In(now.Location())
}

func (r *Runner) drawLotteryIssues(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT issue.id
		FROM lottery_issues issue
		JOIN lottery_games game ON game.id=issue.game_id AND game.status=1
		WHERE issue.status=2
		  AND issue.draw_at<=CURRENT_TIMESTAMP(3)
		  AND JSON_UNQUOTE(JSON_EXTRACT(game.config,'$.draw.draw_mode'))='local_auto'
		ORDER BY issue.draw_at,issue.id
		LIMIT 250`)
	if err != nil {
		return err
	}
	issueIDs := make([]int64, 0, 250)
	for rows.Next() {
		var issueID int64
		if err = rows.Scan(&issueID); err != nil {
			rows.Close()
			return err
		}
		issueIDs = append(issueIDs, issueID)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, issueID := range issueIDs {
		if err = r.drawLotteryIssue(ctx, issueID); err != nil {
			r.logger.Error("draw lottery issue", "issue_id", issueID, "error", err)
		}
	}
	return nil
}

func (r *Runner) drawLotteryIssue(ctx context.Context, issueID int64) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var gameID int64
	var status int
	var rawConfig []byte
	if err = tx.QueryRowContext(ctx, `
		SELECT issue.game_id,issue.status,game.config
		FROM lottery_issues issue
		JOIN lottery_games game ON game.id=issue.game_id
		WHERE issue.id=? FOR UPDATE`,
		issueID,
	).Scan(&gameID, &status, &rawConfig); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if status != 2 {
		return nil
	}
	var config lotteryGameConfig
	if err = json.Unmarshal(rawConfig, &config); err != nil {
		return fmt.Errorf("decode draw config: %w", err)
	}
	if config.Draw.Mode != "local_auto" {
		return errors.New("lottery issue is not configured for local auto draw")
	}
	if err = validateDrawConfig(config.Draw); err != nil {
		return err
	}
	allowedByPosition, err := lotteryPositionCandidates(ctx, tx, gameID, config.Draw)
	if err != nil {
		return err
	}
	numbers, err := secureDrawNumbersWithCandidates(config.Draw, allowedByPosition)
	if err != nil {
		return err
	}
	winnerIDs, err := deriveLotteryWinnerOptionIDs(ctx, tx, gameID, config.Draw, numbers)
	if err != nil {
		return err
	}
	result := map[string]any{
		"numbers":           numbers,
		"open_code":         formatOpenCode(numbers, config.Draw.Pad),
		"winner_option_ids": winnerIDs,
		"draw_mode":         config.Draw.Mode,
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	payloadHash := sha256.Sum256(resultJSON)
	update, err := tx.ExecContext(ctx, `
		UPDATE lottery_issues
		SET status=3,draw_result=?,result_source='local_auto'
		WHERE id=? AND status=2`,
		resultJSON, issueID,
	)
	if err != nil {
		return err
	}
	affected, err := update.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO lottery_draw_audits
			(issue_id,action,source,after_result,payload_hash,actor_type,actor_id)
		VALUES(?,'auto_draw','local_auto',?,?,3,0)`,
		issueID, resultJSON, hex.EncodeToString(payloadHash[:]),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func validateDrawConfig(config lotteryDrawConfig) error {
	if config.Mode != "local_auto" {
		return errors.New("unsupported draw mode")
	}
	if config.Count < 1 || config.Count > 100 {
		return errors.New("draw count is outside the safe range")
	}
	if config.Minimum < 0 || config.Maximum < config.Minimum ||
		config.Maximum-config.Minimum > 10_000 {
		return errors.New("draw number range is invalid")
	}
	if config.Unique == 1 && config.Count > config.Maximum-config.Minimum+1 {
		return errors.New("unique draw count exceeds number range")
	}
	if config.Pad < 0 || config.Pad > 8 {
		return errors.New("draw number padding is invalid")
	}
	return nil
}

func secureDrawNumbers(config lotteryDrawConfig) ([]int, error) {
	return secureDrawNumbersWithCandidates(config, nil)
}

func secureDrawNumbersWithCandidates(
	config lotteryDrawConfig,
	allowedByPosition map[int][]int,
) ([]int, error) {
	numbers := make([]int, 0, config.Count)
	used := make(map[int]struct{}, config.Count)
	for position := 1; position <= config.Count; position++ {
		candidates := allowedByPosition[position]
		if len(candidates) == 0 {
			candidates = make([]int, 0, config.Maximum-config.Minimum+1)
			for value := config.Minimum; value <= config.Maximum; value++ {
				candidates = append(candidates, value)
			}
		}
		if config.Unique == 1 {
			available := candidates[:0]
			for _, value := range candidates {
				if _, exists := used[value]; !exists {
					available = append(available, value)
				}
			}
			candidates = available
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("no available draw number for position %d", position)
		}
		raw, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
		if err != nil {
			return nil, err
		}
		value := candidates[int(raw.Int64())]
		if config.Unique == 1 {
			used[value] = struct{}{}
		}
		numbers = append(numbers, value)
	}
	return numbers, nil
}

func lotteryPositionCandidates(
	ctx context.Context,
	tx *sql.Tx,
	gameID int64,
	config lotteryDrawConfig,
) (map[int][]int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT play.settlement_rule,option_item.option_code
		FROM lottery_plays play
		JOIN lottery_options option_item
		  ON option_item.play_id=play.id AND option_item.status=1
		WHERE play.game_id=? AND play.status=1
		ORDER BY play.id,option_item.id`,
		gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sets := make(map[int]map[int]struct{})
	for rows.Next() {
		var rule, rawCode string
		if err = rows.Scan(&rule, &rawCode); err != nil {
			return nil, err
		}
		position, parseErr := strconv.Atoi(strings.TrimPrefix(rule, "position_"))
		if parseErr != nil || position < 1 || position > config.Count {
			continue
		}
		value, parseErr := strconv.Atoi(strings.TrimSpace(rawCode))
		if parseErr != nil || value < config.Minimum || value > config.Maximum {
			continue
		}
		if sets[position] == nil {
			sets[position] = make(map[int]struct{})
		}
		sets[position][value] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result := make(map[int][]int, len(sets))
	for position, values := range sets {
		candidates := make([]int, 0, len(values))
		for value := config.Minimum; value <= config.Maximum; value++ {
			if _, exists := values[value]; exists {
				candidates = append(candidates, value)
			}
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf(
				"position %d has no option inside configured draw range", position,
			)
		}
		result[position] = candidates
	}
	return result, nil
}

func deriveLotteryWinnerOptionIDs(
	ctx context.Context,
	tx *sql.Tx,
	gameID int64,
	config lotteryDrawConfig,
	numbers []int,
) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT play.id,play.settlement_rule,option_item.id,option_item.option_code
		FROM lottery_plays play
		JOIN lottery_options option_item
		  ON option_item.play_id=play.id AND option_item.status=1
		WHERE play.game_id=? AND play.status=1
		ORDER BY play.id,option_item.id`,
		gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type option struct {
		ID   int64
		Code string
	}
	type play struct {
		Rule    string
		Options []option
	}
	plays := make([]play, 0, 8)
	var currentPlayID int64
	for rows.Next() {
		var playID, optionID int64
		var rule, code string
		if err = rows.Scan(&playID, &rule, &optionID, &code); err != nil {
			return nil, err
		}
		if len(plays) == 0 || playID != currentPlayID {
			plays = append(plays, play{Rule: rule, Options: make([]option, 0, 16)})
			currentPlayID = playID
		}
		plays[len(plays)-1].Options = append(
			plays[len(plays)-1].Options,
			option{ID: optionID, Code: strings.TrimSpace(code)},
		)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	winners := make([]int64, 0, len(plays))
	for _, item := range plays {
		winnerCode, ruleErr := lotteryWinnerCode(item.Rule, config, numbers)
		if ruleErr != nil {
			return nil, ruleErr
		}
		matched := false
		for _, candidate := range item.Options {
			if optionCodesEqual(candidate.Code, winnerCode) {
				winners = append(winners, candidate.ID)
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf(
				"no option matches settlement rule %q result %q",
				item.Rule, winnerCode,
			)
		}
	}
	return winners, nil
}

func lotteryWinnerCode(
	rule string,
	config lotteryDrawConfig,
	numbers []int,
) (string, error) {
	if len(numbers) == 0 {
		return "", errors.New("draw numbers are empty")
	}
	rule = strings.ToLower(strings.TrimSpace(rule))
	switch {
	case strings.HasPrefix(rule, "position_"):
		position, err := strconv.Atoi(strings.TrimPrefix(rule, "position_"))
		if err != nil || position < 1 || position > len(numbers) {
			return "", fmt.Errorf("invalid position settlement rule %q", rule)
		}
		return strconv.Itoa(numbers[position-1]), nil
	case rule == "exact_sum":
		return strconv.Itoa(lotteryAggregateSum(config, numbers)), nil
	case rule == "sum_odd_even":
		if lotteryAggregateSum(config, numbers)%2 == 0 {
			return "EVEN", nil
		}
		return "ODD", nil
	case strings.HasPrefix(rule, "sum_size:"):
		parts := strings.Split(rule, ":")
		if len(parts) != 3 {
			return "", fmt.Errorf("invalid size settlement rule %q", rule)
		}
		minimum, minErr := strconv.Atoi(parts[1])
		maximum, maxErr := strconv.Atoi(parts[2])
		if minErr != nil || maxErr != nil || maximum < minimum {
			return "", fmt.Errorf("invalid size settlement rule %q", rule)
		}
		count := len(numbers)
		if isFullRangeUniqueDraw(config) && count > 2 {
			count = 2
		}
		twiceSum := lotteryAggregateSum(config, numbers) * 2
		midpointTwice := count * (minimum + maximum)
		if twiceSum > midpointTwice {
			return "BIG", nil
		}
		return "SMALL", nil
	default:
		return "", fmt.Errorf("unsupported lottery settlement rule %q", rule)
	}
}

func lotteryAggregateSum(config lotteryDrawConfig, numbers []int) int {
	limit := len(numbers)
	if isFullRangeUniqueDraw(config) && limit > 2 {
		limit = 2
	}
	total := 0
	for _, value := range numbers[:limit] {
		total += value
	}
	return total
}

func isFullRangeUniqueDraw(config lotteryDrawConfig) bool {
	return config.Unique == 1 &&
		config.Count == config.Maximum-config.Minimum+1
}

func optionCodesEqual(actual string, expected string) bool {
	if strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(expected)) {
		return true
	}
	actualNumber, actualErr := strconv.Atoi(strings.TrimSpace(actual))
	expectedNumber, expectedErr := strconv.Atoi(strings.TrimSpace(expected))
	return actualErr == nil && expectedErr == nil && actualNumber == expectedNumber
}

func formatOpenCode(numbers []int, padding int) string {
	parts := make([]string, len(numbers))
	for index, value := range numbers {
		if padding > 0 {
			parts[index] = fmt.Sprintf("%0*d", padding, value)
		} else {
			parts[index] = strconv.Itoa(value)
		}
	}
	return strings.Join(parts, ",")
}
