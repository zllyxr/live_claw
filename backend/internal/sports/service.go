package sports

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

type Error struct {
	Code    int
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func PublicError(err error) (int, string) {
	var sportsError *Error
	if errors.As(err, &sportsError) {
		return sportsError.Code, sportsError.Message
	}
	if errors.Is(err, wallet.ErrInsufficientFunds) {
		return 1012, "余额不足"
	}
	return 500, "体育服务暂不可用"
}

type Service struct {
	db     *sql.DB
	wallet *wallet.Service
	now    func() time.Time
}

type BetRequest struct {
	UserID        int64
	MatchID       string
	ClientTraceID string
	ItemsJSON     string
}

type betInput struct {
	OptionID json.Number `json:"option_id"`
	Amount   json.Number `json:"amount"`
}

type betOption struct {
	ID         int64
	MarketID   int64
	MarketCode string
	Name       string
	OddsScaled int64
	Amount     int64
}

func New(db *sql.DB, walletService *wallet.Service) *Service {
	return &Service{db: db, wallet: walletService, now: time.Now}
}

func (s *Service) Home(
	ctx context.Context,
	tab string,
	date string,
	competitionType string,
) (map[string]any, error) {
	tab = strings.ToLower(strings.TrimSpace(tab))
	if tab == "" {
		tab = "today"
	}
	selectedDate := strings.TrimSpace(date)
	if selectedDate == "" {
		selectedDate = s.now().Format("2006-01-02")
	}
	from, to := dayRange(selectedDate, s.now())
	statusFilter := ""
	switch tab {
	case "live":
		statusFilter = "live"
	case "tomorrow":
		from = from.Add(24 * time.Hour)
		to = to.Add(24 * time.Hour)
	case "upcoming":
		from = s.now()
		to = from.Add(7 * 24 * time.Hour)
	case "today":
	default:
		tab = "today"
	}
	query := `
		SELECT id,public_match_id,competition,competition_type,home_name,away_name,
		       home_logo_url,away_logo_url,kickoff_at,bet_close_at,
		       home_score,away_score,match_status,bet_status
		FROM sports_matches
		WHERE match_status NOT IN ('FT','CANCELLED')
		  AND EXISTS (
			  SELECT 1
			  FROM sports_markets visible_market
			  JOIN sports_market_options visible_option
			    ON visible_option.market_id=visible_market.id
			   AND visible_option.status=1
			   AND visible_option.odds_scaled>1000000
			  WHERE visible_market.match_id=sports_matches.id
			    AND visible_market.status=1
		  )`
	args := make([]any, 0, 6)
	if statusFilter == "live" {
		query += " AND match_status IN ('1H','HT','2H','LIVE','OT')"
	} else {
		query += " AND kickoff_at>=? AND kickoff_at<?"
		args = append(args, from, to)
	}
	competitionType = strings.TrimSpace(competitionType)
	if competitionType != "" {
		query += " AND competition_type=?"
		args = append(args, competitionType)
	}
	query += `
		ORDER BY (match_status NOT IN ('1H','HT','2H','LIVE','OT')) ASC,
		         kickoff_at,id LIMIT 100`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matches := make([]map[string]any, 0, 32)
	upcoming := make([]map[string]any, 0, 32)
	matchIDs := make([]int64, 0, 32)
	formattedByID := make(map[int64]map[string]any)
	competitionSet := make(map[string]struct{})
	for rows.Next() {
		match, scanErr := scanMatch(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		formatted := formatMatch(match, s.now())
		matches = append(matches, formatted)
		matchIDs = append(matchIDs, match.ID)
		formattedByID[match.ID] = formatted
		if !isLiveStatus(match.Status) {
			upcoming = append(upcoming, formatted)
		}
		if match.CompetitionType != "" {
			competitionSet[match.CompetitionType] = struct{}{}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	marketPreviews, marketCounts, err := s.marketPreviews(ctx, matchIDs)
	if err != nil {
		return nil, err
	}
	for matchID, formatted := range formattedByID {
		preview := marketPreviews[matchID]
		if preview == nil {
			preview = []map[string]any{}
		}
		formatted["markets"] = preview
		formatted["market_count"] = marketCounts[matchID]
	}
	competitions := make([]map[string]any, 0, len(competitionSet))
	for key := range competitionSet {
		competitions = append(competitions, map[string]any{"key": key, "name": key})
	}
	now := s.now()
	return map[string]any{
		"selected_tab": tab, "selected_date": selectedDate,
		"selected_competition_type": competitionType,
		"updated_at":                now.Format("2006-01-02 15:04:05"),
		"server_time":               strconv.FormatInt(now.Unix(), 10),
		"timezone":                  "Asia/Shanghai", "timezone_offset": "28800",
		"tabs": []map[string]string{
			{"key": "today", "name": "今日"},
			{"key": "live", "name": "滚球"},
			{"key": "tomorrow", "name": "明日"},
			{"key": "upcoming", "name": "近期"},
		},
		"matches": matches, "upcoming": upcoming, "competitions": competitions,
		"top_leagues": []any{}, "quick_stats": []any{}, "analysis": []any{},
		"matches_title": "体育赛事", "quick_stats_title": "赛事数据",
	}, nil
}

func (s *Service) MatchDetail(ctx context.Context, rawMatchID string) (map[string]any, error) {
	match, err := s.match(ctx, rawMatchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1003, Message: "赛事不存在"}
	}
	if err != nil {
		return nil, err
	}
	markets, err := s.markets(ctx, match.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"match": formatMatch(match, s.now()), "markets": markets,
		"server_time": strconv.FormatInt(s.now().Unix(), 10),
		"timezone":    "Asia/Shanghai", "timezone_offset": "28800",
	}, nil
}

func (s *Service) MatchMarkets(
	ctx context.Context,
	rawMatchID string,
	userID int64,
) (map[string]any, error) {
	match, err := s.match(ctx, rawMatchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1003, Message: "赛事不存在"}
	}
	if err != nil {
		return nil, err
	}
	markets, err := s.markets(ctx, match.ID)
	if err != nil {
		return nil, err
	}
	balance, err := s.wallet.Balance(ctx, userID)
	if userID < 1 {
		balance.Available = 0
		err = nil
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	betOpen := match.BetStatus == 1 && match.BetCloseAt.After(now) && !isFinishedStatus(match.Status)
	betStatusText := "盘口关闭"
	if betOpen {
		betStatusText = "可投注"
	}
	return map[string]any{
		"match": formatMatch(match, now), "markets": markets,
		"bet_enabled": boolNumber(betOpen), "bet_open": boolNumber(betOpen),
		"bet_status_text": betStatusText,
		"close_countdown": strconv.FormatInt(max64(match.BetCloseAt.Unix()-now.Unix(), 0), 10),
		"coin":            strconv.FormatInt(balance.Available, 10),
		"server_time":     strconv.FormatInt(now.Unix(), 10),
		"timezone":        "Asia/Shanghai", "timezone_offset": "28800",
	}, nil
}

func (s *Service) PlaceBet(ctx context.Context, request BetRequest) (map[string]any, error) {
	request.ClientTraceID = strings.TrimSpace(request.ClientTraceID)
	if request.UserID < 1 || request.ClientTraceID == "" || len(request.ClientTraceID) > 100 {
		return nil, &Error{Code: 1001, Message: "客户端订单号无效"}
	}
	if existing, err := s.orderByTrace(ctx, request.UserID, request.ClientTraceID); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	inputs, err := decodeInputs(request.ItemsJSON)
	if err != nil || len(inputs) < 1 || len(inputs) > 20 {
		return nil, &Error{Code: 1002, Message: "下注内容错误"}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	match, err := matchForUpdate(ctx, tx, request.MatchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1003, Message: "赛事不存在或已下架"}
	}
	if err != nil {
		return nil, err
	}
	if match.BetStatus != 1 || !match.BetCloseAt.After(s.now()) || isFinishedStatus(match.Status) {
		return nil, &Error{Code: 1005, Message: "本场已封盘"}
	}
	options := make([]betOption, 0, len(inputs))
	seen := make(map[int64]struct{}, len(inputs))
	var totalBet int64
	for _, input := range inputs {
		optionID, optionErr := input.OptionID.Int64()
		amount, amountErr := input.Amount.Int64()
		if optionErr != nil || amountErr != nil || optionID < 1 || amount < 1 {
			return nil, &Error{Code: 1006, Message: "下注项错误"}
		}
		if _, exists := seen[optionID]; exists {
			return nil, &Error{Code: 1006, Message: "下注项重复"}
		}
		seen[optionID] = struct{}{}
		if totalBet > match.MaxBet-amount {
			return nil, &Error{Code: 1010, Message: "下注金额超过单次限制"}
		}
		var option betOption
		err = tx.QueryRowContext(ctx, `
			SELECT opt.id,market.id,market.market_code,opt.name,opt.odds_scaled
			FROM sports_market_options opt
			JOIN sports_markets market ON market.id=opt.market_id AND market.status=1
			WHERE opt.id=? AND opt.status=1 AND market.match_id=?`,
			optionID, match.ID,
		).Scan(&option.ID, &option.MarketID, &option.MarketCode, &option.Name, &option.OddsScaled)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &Error{Code: 1007, Message: "盘口选项不存在"}
		}
		if err != nil {
			return nil, err
		}
		option.Amount = amount
		totalBet += amount
		options = append(options, option)
	}
	if totalBet < match.MinBet {
		return nil, &Error{Code: 1009, Message: "下注金额低于最低限制"}
	}
	if totalBet > match.MaxBet {
		return nil, &Error{Code: 1010, Message: "下注金额超过单次限制"}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	hold, err := s.wallet.PlaceHold(ctx, wallet.HoldRequest{
		UserID: request.UserID, Amount: totalBet,
		BusinessType: "sports_bet", BusinessID: request.ClientTraceID,
		ExpiresAt: match.KickoffAt.Add(7 * 24 * time.Hour), Description: "体育投注冻结",
		Metadata: map[string]any{
			"match_id": match.ID, "public_match_id": match.PublicID,
			"client_trace_id": request.ClientTraceID,
		},
		GameCode: "sports", RoundNo: match.PublicID,
	})
	if err != nil {
		return nil, err
	}
	releaseFailedHold := func() {
		_, _ = s.wallet.ReleaseHold(ctx, hold.HoldNo, "体育投注订单创建失败退回", map[string]any{
			"client_trace_id": request.ClientTraceID,
		})
	}
	orderNo, err := idgen.New()
	if err != nil {
		releaseFailedHold()
		return nil, err
	}
	insertTx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		releaseFailedHold()
		return nil, err
	}
	defer insertTx.Rollback() //nolint:errcheck
	result, err := insertTx.ExecContext(ctx, `
		INSERT INTO sports_bet_orders
			(order_no,user_id,match_id,hold_no,total_bet,status,client_trace_id)
		VALUES(?,?,?,?,?,0,?)`,
		orderNo, request.UserID, match.ID, hold.HoldNo, totalBet, request.ClientTraceID,
	)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			_ = insertTx.Rollback()
			return s.orderByTrace(ctx, request.UserID, request.ClientTraceID)
		}
		releaseFailedHold()
		return nil, err
	}
	orderID, _ := result.LastInsertId()
	for _, option := range options {
		if _, err = insertTx.ExecContext(ctx, `
			INSERT INTO sports_bet_items
				(order_id,market_id,option_id,bet_amount,odds_scaled,payout_amount,result)
			VALUES(?,?,?,?,?,0,0)`,
			orderID, option.MarketID, option.ID, option.Amount, option.OddsScaled,
		); err != nil {
			_ = insertTx.Rollback()
			releaseFailedHold()
			return nil, err
		}
	}
	if err = insertTx.Commit(); err != nil {
		if existing, lookupErr := s.orderByTrace(ctx, request.UserID, request.ClientTraceID); lookupErr == nil {
			return existing, nil
		}
		releaseFailedHold()
		return nil, err
	}
	return s.orderByID(ctx, orderID)
}

func (s *Service) Orders(
	ctx context.Context,
	userID int64,
	rawMatchID string,
	page int,
) (map[string]any, error) {
	if userID < 1 {
		return nil, &Error{Code: 700, Message: "登录已失效"}
	}
	var matchID int64
	if strings.TrimSpace(rawMatchID) != "" {
		match, err := s.match(ctx, rawMatchID)
		if err == nil {
			matchID = match.ID
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	if page < 1 {
		page = 1
	}
	query := "SELECT id FROM sports_bet_orders WHERE user_id=?"
	args := []any{userID}
	if matchID > 0 {
		query += " AND match_id=?"
		args = append(args, matchID)
	}
	query += " ORDER BY id DESC LIMIT 20 OFFSET ?"
	args = append(args, (page-1)*20)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, 20)
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		item, itemErr := s.orderByID(ctx, id)
		if itemErr != nil {
			return nil, itemErr
		}
		items = append(items, item)
	}
	summaryQuery := `
		SELECT COUNT(*),COALESCE(SUM(total_bet),0),COALESCE(SUM(total_payout),0),
		       COALESCE(SUM(CASE WHEN status IN (1,2) THEN total_payout-total_bet ELSE 0 END),0)
		FROM sports_bet_orders WHERE user_id=?`
	summaryArgs := []any{userID}
	if matchID > 0 {
		summaryQuery += " AND match_id=?"
		summaryArgs = append(summaryArgs, matchID)
	}
	var count, totalBet, totalPayout, profitLoss int64
	if err = s.db.QueryRowContext(ctx, summaryQuery, summaryArgs...).Scan(
		&count, &totalBet, &totalPayout, &profitLoss,
	); err != nil {
		return nil, err
	}
	return map[string]any{
		"list": items, "items": items, "orders": items, "page": strconv.Itoa(page),
		"total_count":  strconv.FormatInt(count, 10),
		"total_bet":    strconv.FormatInt(totalBet, 10),
		"total_payout": strconv.FormatInt(totalPayout, 10),
		"profit_loss":  strconv.FormatInt(profitLoss, 10),
		"net_amount":   strconv.FormatInt(profitLoss, 10),
	}, nil
}

type matchRecord struct {
	ID              int64
	PublicID        string
	Competition     string
	CompetitionType string
	HomeName        string
	AwayName        string
	HomeLogo        string
	AwayLogo        string
	KickoffAt       time.Time
	BetCloseAt      time.Time
	HomeScore       int
	AwayScore       int
	Status          string
	BetStatus       int
	MinBet          int64
	MaxBet          int64
}

type scanner interface {
	Scan(...any) error
}

func scanMatch(row scanner) (matchRecord, error) {
	var result matchRecord
	err := row.Scan(
		&result.ID, &result.PublicID, &result.Competition, &result.CompetitionType,
		&result.HomeName, &result.AwayName, &result.HomeLogo, &result.AwayLogo,
		&result.KickoffAt, &result.BetCloseAt, &result.HomeScore, &result.AwayScore,
		&result.Status, &result.BetStatus,
	)
	return result, err
}

func (s *Service) match(ctx context.Context, rawMatchID string) (matchRecord, error) {
	rawMatchID = strings.TrimSpace(rawMatchID)
	query := `
		SELECT id,public_match_id,competition,competition_type,home_name,away_name,
		       home_logo_url,away_logo_url,kickoff_at,bet_close_at,
		       home_score,away_score,match_status,bet_status
		FROM sports_matches WHERE EXISTS (
			SELECT 1
			FROM sports_markets visible_market
			JOIN sports_market_options visible_option
			  ON visible_option.market_id=visible_market.id
			 AND visible_option.status=1
			 AND visible_option.odds_scaled>1000000
			WHERE visible_market.match_id=sports_matches.id
			  AND visible_market.status=1
		) AND `
	if numericID, err := strconv.ParseInt(rawMatchID, 10, 64); err == nil && numericID > 0 {
		return scanMatch(s.db.QueryRowContext(ctx, query+"id=?", numericID))
	}
	return scanMatch(s.db.QueryRowContext(ctx, query+"public_match_id=?", rawMatchID))
}

func matchForUpdate(ctx context.Context, tx *sql.Tx, rawMatchID string) (matchRecord, error) {
	rawMatchID = strings.TrimSpace(rawMatchID)
	query := `
		SELECT id,public_match_id,competition,competition_type,home_name,away_name,
		       home_logo_url,away_logo_url,kickoff_at,bet_close_at,
		       home_score,away_score,match_status,bet_status,min_bet,max_bet
		FROM sports_matches WHERE `
	var row *sql.Row
	if numericID, err := strconv.ParseInt(rawMatchID, 10, 64); err == nil && numericID > 0 {
		row = tx.QueryRowContext(ctx, query+"id=? FOR UPDATE", numericID)
	} else {
		row = tx.QueryRowContext(ctx, query+"public_match_id=? FOR UPDATE", rawMatchID)
	}
	var result matchRecord
	err := row.Scan(
		&result.ID, &result.PublicID, &result.Competition, &result.CompetitionType,
		&result.HomeName, &result.AwayName, &result.HomeLogo, &result.AwayLogo,
		&result.KickoffAt, &result.BetCloseAt, &result.HomeScore, &result.AwayScore,
		&result.Status, &result.BetStatus, &result.MinBet, &result.MaxBet,
	)
	return result, err
}

func (s *Service) markets(ctx context.Context, matchID int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT market.id,market.market_code,market.name,market.settlement_rule,
		       opt.id,opt.option_code,opt.name,opt.odds_scaled
		FROM sports_markets market
		JOIN sports_market_options opt ON opt.market_id=market.id AND opt.status=1
		WHERE market.match_id=? AND market.status=1
		ORDER BY market.sort_order DESC,market.id,opt.id`,
		matchID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 8)
	indexes := make(map[int64]int)
	for rows.Next() {
		var marketID, optionID, oddsScaled int64
		var marketCode, marketName, rule, optionCode, optionName string
		if err = rows.Scan(
			&marketID, &marketCode, &marketName, &rule,
			&optionID, &optionCode, &optionName, &oddsScaled,
		); err != nil {
			return nil, err
		}
		index, exists := indexes[marketID]
		if !exists {
			index = len(items)
			indexes[marketID] = index
			items = append(items, map[string]any{
				"id": strconv.FormatInt(marketID, 10), "market_code": marketCode,
				"market_name": marketName, "market_rule": rule,
				"options": []map[string]any{},
			})
		}
		options := items[index]["options"].([]map[string]any)
		items[index]["options"] = append(options, map[string]any{
			"id": strconv.FormatInt(optionID, 10), "option_code": optionCode,
			"option_name": optionName, "odds": formatOdds(oddsScaled),
			"odds_scaled": oddsScaled,
		})
	}
	return items, rows.Err()
}

func (s *Service) marketPreviews(
	ctx context.Context,
	matchIDs []int64,
) (map[int64][]map[string]any, map[int64]int, error) {
	previews := make(map[int64][]map[string]any, len(matchIDs))
	counts := make(map[int64]int, len(matchIDs))
	if len(matchIDs) == 0 {
		return previews, counts, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(matchIDs)), ",")
	args := make([]any, 0, len(matchIDs))
	for _, matchID := range matchIDs {
		args = append(args, matchID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT market.match_id,market.id,market.market_code,market.name,market.settlement_rule,
		       option_item.id,option_item.option_code,option_item.name,option_item.odds_scaled,
		       totals.market_count
		FROM sports_markets market
		JOIN sports_market_options option_item
		  ON option_item.market_id=market.id AND option_item.status=1
		JOIN (
			SELECT match_id,COUNT(*) AS market_count
			FROM sports_markets
			WHERE status=1 AND match_id IN (`+placeholders+`)
			GROUP BY match_id
		) totals ON totals.match_id=market.match_id
		WHERE market.status=1 AND market.match_id IN (`+placeholders+`)
		ORDER BY market.match_id,(market.market_code='MATCH_RESULT') DESC,
		         market.sort_order DESC,market.id,option_item.id`,
		append(args, args...)...,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	selectedMarket := make(map[int64]int64, len(matchIDs))
	for rows.Next() {
		var matchID, marketID, optionID, oddsScaled int64
		var marketCount int
		var marketCode, marketName, rule, optionCode, optionName string
		if err = rows.Scan(
			&matchID, &marketID, &marketCode, &marketName, &rule,
			&optionID, &optionCode, &optionName, &oddsScaled, &marketCount,
		); err != nil {
			return nil, nil, err
		}
		counts[matchID] = marketCount
		if existing, ok := selectedMarket[matchID]; ok && existing != marketID {
			continue
		}
		if _, ok := selectedMarket[matchID]; !ok {
			selectedMarket[matchID] = marketID
			previews[matchID] = []map[string]any{{
				"id": strconv.FormatInt(marketID, 10), "market_code": marketCode,
				"market_name": marketName, "market_rule": rule,
				"options": []map[string]any{},
			}}
		}
		market := previews[matchID][0]
		options := market["options"].([]map[string]any)
		if len(options) >= 3 {
			continue
		}
		market["options"] = append(options, map[string]any{
			"id": strconv.FormatInt(optionID, 10), "option_code": optionCode,
			"option_name": optionName, "odds": formatOdds(oddsScaled),
			"odds_scaled": oddsScaled,
		})
	}
	return previews, counts, rows.Err()
}

func (s *Service) orderByTrace(ctx context.Context, userID int64, traceID string) (map[string]any, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM sports_bet_orders WHERE user_id=? AND client_trace_id=?`,
		userID, traceID,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.orderByID(ctx, id)
}

func (s *Service) orderByID(ctx context.Context, orderID int64) (map[string]any, error) {
	var id, userID, matchID, totalBet, totalPayout int64
	var orderNo, traceID, publicMatchID, homeName, awayName string
	var status int
	var settledAt sql.NullTime
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT order_row.id,order_row.order_no,order_row.client_trace_id,order_row.user_id,
		       order_row.match_id,match_row.public_match_id,match_row.home_name,match_row.away_name,
		       order_row.total_bet,order_row.total_payout,order_row.status,
		       order_row.settled_at,order_row.created_at
		FROM sports_bet_orders order_row
		JOIN sports_matches match_row ON match_row.id=order_row.match_id
		WHERE order_row.id=?`,
		orderID,
	).Scan(
		&id, &orderNo, &traceID, &userID, &matchID, &publicMatchID, &homeName, &awayName,
		&totalBet, &totalPayout, &status, &settledAt, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT item.id,item.market_id,market.market_code,item.option_id,opt.name,
		       item.odds_scaled,item.bet_amount,item.payout_amount,item.result
		FROM sports_bet_items item
		JOIN sports_markets market ON market.id=item.market_id
		JOIN sports_market_options opt ON opt.id=item.option_id
		WHERE item.order_id=? ORDER BY item.id`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 4)
	firstBetName := ""
	firstOdds := ""
	for rows.Next() {
		var itemID, marketID, optionID, oddsScaled, amount, payout int64
		var marketCode, optionName string
		var result int
		if err = rows.Scan(
			&itemID, &marketID, &marketCode, &optionID, &optionName,
			&oddsScaled, &amount, &payout, &result,
		); err != nil {
			return nil, err
		}
		if firstBetName == "" {
			firstBetName = optionName
			firstOdds = formatOdds(oddsScaled)
		}
		items = append(items, map[string]any{
			"id":        strconv.FormatInt(itemID, 10),
			"market_id": strconv.FormatInt(marketID, 10), "market_code": marketCode,
			"option_id": strconv.FormatInt(optionID, 10), "option_name": optionName,
			"odds": formatOdds(oddsScaled), "bet_amount": strconv.FormatInt(amount, 10),
			"payout_amount": strconv.FormatInt(payout, 10), "result": strconv.Itoa(result),
		})
	}
	netAmount := int64(0)
	if status == 1 || status == 2 {
		netAmount = totalPayout - totalBet
	}
	return map[string]any{
		"id": strconv.FormatInt(id, 10), "orderid": strconv.FormatInt(id, 10),
		"order_no": orderNo, "client_trace_id": traceID, "uid": strconv.FormatInt(userID, 10),
		"match_id": strconv.FormatInt(matchID, 10), "public_match_id": publicMatchID,
		"match_name": homeName + " VS " + awayName,
		"home_name":  homeName, "away_name": awayName,
		"bet_name": firstBetName, "odds": firstOdds,
		"money": strconv.FormatInt(totalBet, 10), "bet_money": strconv.FormatInt(totalBet, 10),
		"win_money":    strconv.FormatInt(totalPayout, 10),
		"total_bet":    strconv.FormatInt(totalBet, 10),
		"total_payout": strconv.FormatInt(totalPayout, 10),
		"net_amount":   strconv.FormatInt(netAmount, 10),
		"status":       strconv.Itoa(status), "status_text": orderStatusText(status),
		"addtime":     strconv.FormatInt(createdAt.Unix(), 10),
		"settle_time": nullableUnix(settledAt), "items": items,
	}, rows.Err()
}

func formatMatch(match matchRecord, now time.Time) map[string]any {
	live := isLiveStatus(match.Status)
	started := live || strings.EqualFold(match.Status, "FT")
	homeScore := ""
	awayScore := ""
	scoreText := "VS"
	if started {
		homeScore = strconv.Itoa(match.HomeScore)
		awayScore = strconv.Itoa(match.AwayScore)
		scoreText = homeScore + " : " + awayScore
	}
	return map[string]any{
		"id":       strconv.FormatInt(match.ID, 10),
		"match_id": match.PublicID, "public_match_id": match.PublicID,
		"competition": match.Competition, "competition_type": match.CompetitionType,
		"league_name": match.Competition,
		"home_name":   match.HomeName, "away_name": match.AwayName,
		"home_team": match.HomeName, "away_team": match.AwayName,
		"home_logo": match.HomeLogo, "away_logo": match.AwayLogo,
		"home_score": homeScore, "away_score": awayScore, "score_text": scoreText,
		"match_status": match.Status, "status_text": statusText(match.Status),
		"bet_status":         strconv.Itoa(match.BetStatus),
		"bet_status_text":    betStatusText(match.BetStatus, match.BetCloseAt, now),
		"kickoff_ts":         strconv.FormatInt(match.KickoffAt.Unix(), 10),
		"kickoff_time":       strconv.FormatInt(match.KickoffAt.Unix(), 10),
		"bet_close_ts":       strconv.FormatInt(match.BetCloseAt.Unix(), 10),
		"bet_close_time":     strconv.FormatInt(match.BetCloseAt.Unix(), 10),
		"kickoff_text":       match.KickoffAt.Format("01-02 15:04"),
		"kickoff_date_text":  match.KickoffAt.Format("01-02"),
		"kickoff_clock_text": match.KickoffAt.Format("15:04"),
		"kickoff_time_text":  match.KickoffAt.Format("15:04"),
		"match_time":         match.KickoffAt.Format("2006-01-02 15:04:05"),
		"is_live":            boolNumber(live),
		"has_started":        boolNumber(started),
	}
}

func decodeInputs(raw string) ([]betInput, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(strings.TrimSpace(raw)))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var inputs []betInput
	if err := decoder.Decode(&inputs); err != nil {
		return nil, err
	}
	return inputs, nil
}

func dayRange(value string, now time.Time) (time.Time, time.Time) {
	location := now.Location()
	parsed, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil {
		parsed = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	}
	return parsed, parsed.Add(24 * time.Hour)
}

func isLiveStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "1H", "HT", "2H", "LIVE", "OT":
		return true
	default:
		return false
	}
}

func isFinishedStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "FT", "CANCELLED", "POSTPONED":
		return true
	default:
		return false
	}
}

func statusText(status string) string {
	switch strings.ToUpper(status) {
	case "1H":
		return "上半场"
	case "HT":
		return "中场"
	case "2H":
		return "下半场"
	case "LIVE":
		return "进行中"
	case "FT":
		return "已结束"
	case "CANCELLED":
		return "已取消"
	default:
		return "未开始"
	}
}

func betStatusText(status int, closeAt, now time.Time) string {
	if status == 1 && closeAt.After(now) {
		return "可投注"
	}
	return "已封盘"
}

func formatOdds(scaled int64) string {
	whole := scaled / 1_000_000
	fraction := scaled % 1_000_000
	value := strconv.FormatInt(whole, 10) + "." + fmt.Sprintf("%06d", fraction)
	return strings.TrimRight(strings.TrimRight(value, "0"), ".")
}

func orderStatusText(status int) string {
	switch status {
	case 1:
		return "已中奖"
	case 2:
		return "未中奖"
	case 3:
		return "已退款"
	case 4:
		return "已取消"
	default:
		return "待结算"
	}
}

func nullableUnix(value sql.NullTime) string {
	if value.Valid {
		return strconv.FormatInt(value.Time.Unix(), 10)
	}
	return "0"
}

func boolNumber(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func max64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
