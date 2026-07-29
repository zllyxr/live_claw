package lottery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	var lotteryError *Error
	if errors.As(err, &lotteryError) {
		return lotteryError.Code, lotteryError.Message
	}
	if errors.Is(err, wallet.ErrInsufficientFunds) {
		return 1012, "余额不足"
	}
	return 500, "彩票服务暂不可用"
}

type Service struct {
	db           *sql.DB
	wallet       *wallet.Service
	mediaBaseURL string
	now          func() time.Time
}

type BetRequest struct {
	UserID        int64
	GameID        int64
	GameCode      string
	IssueID       int64
	ClientTraceID string
	ItemsJSON     string
}

type betInput struct {
	OptionID json.Number `json:"option_id"`
	Amount   json.Number `json:"amount"`
}

type betOption struct {
	ID         int64
	PlayID     int64
	PlayCode   string
	OptionCode string
	OptionName string
	OddsScaled int64
	Amount     int64
}

func New(db *sql.DB, walletService *wallet.Service, mediaBaseURL string) *Service {
	return &Service{
		db: db, wallet: walletService,
		mediaBaseURL: strings.TrimRight(mediaBaseURL, "/"), now: time.Now,
	}
}

func (s *Service) Home(ctx context.Context, userID int64) (map[string]any, error) {
	categories, err := s.categories(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT game.id,game.category_id,game.game_code,game.name,
		       game.issue_interval_seconds,game.sale_close_seconds,
		       game.min_bet,game.max_bet,game.status,
		       issue.id,issue.issue_no,CAST(UNIX_TIMESTAMP(issue.sale_open_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(issue.sale_close_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(issue.draw_at) AS UNSIGNED),issue.status
		FROM lottery_games game
		JOIN lottery_issues issue ON issue.id=(
			SELECT current_issue.id FROM lottery_issues current_issue
			WHERE current_issue.game_id=game.id AND current_issue.status=1
			  AND current_issue.sale_close_at>CURRENT_TIMESTAMP(3)
			ORDER BY current_issue.sale_close_at,current_issue.id LIMIT 1
		)
		WHERE game.status=1
		ORDER BY game.sort_order DESC,game.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	games := make([]map[string]any, 0, 16)
	activeCategories := make(map[int64]struct{})
	for rows.Next() {
		var gameID, categoryID, minBet, maxBet, issueID int64
		var gameCode, name, issueNo string
		var interval, closeSeconds, gameStatus, issueStatus int
		var saleOpen, saleClose, drawAt int64
		if err = rows.Scan(
			&gameID, &categoryID, &gameCode, &name,
			&interval, &closeSeconds, &minBet, &maxBet, &gameStatus,
			&issueID, &issueNo, &saleOpen, &saleClose, &drawAt, &issueStatus,
		); err != nil {
			return nil, err
		}
		activeCategories[categoryID] = struct{}{}
		icon := staticLotteryIconURL(gameCode)
		games = append(games, map[string]any{
			"id": strconv.FormatInt(gameID, 10), "category_id": strconv.FormatInt(categoryID, 10),
			"game_code": gameCode, "game_name": name, "game_name_en": "",
			"icon": icon, "icon_url": icon,
			"interval_sec": strconv.Itoa(interval), "seal_advance_sec": strconv.Itoa(closeSeconds),
			"min_bet": strconv.FormatInt(minBet, 10), "max_bet": strconv.FormatInt(maxBet, 10),
			"status": strconv.Itoa(gameStatus),
			"current_issue": formatIssue(
				issueID, gameID, issueNo, saleOpen, saleClose, drawAt, issueStatus, nil, s.now(),
			),
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	filteredCategories := make([]map[string]any, 0, len(categories))
	for _, category := range categories {
		id, _ := strconv.ParseInt(fmt.Sprint(category["id"]), 10, 64)
		if _, active := activeCategories[id]; active {
			filteredCategories = append(filteredCategories, category)
		}
	}
	balance, err := s.wallet.Balance(ctx, userID)
	if userID < 1 {
		balance.Available = 0
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"categories": filteredCategories, "games": games,
		"coin": strconv.FormatInt(balance.Available, 10),
	}, nil
}

func (s *Service) Detail(
	ctx context.Context,
	gameID int64,
	gameCode string,
	userID int64,
) (map[string]any, error) {
	game, err := s.game(ctx, gameID, gameCode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1003, Message: "游戏不存在或维护中"}
	}
	if err != nil {
		return nil, err
	}
	issue, err := s.currentIssue(ctx, game.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1004, Message: "当前暂无可投注期号"}
	}
	if err != nil {
		return nil, err
	}
	plays, err := s.plays(ctx, game.ID)
	if err != nil {
		return nil, err
	}
	history, err := s.issueHistory(ctx, game.ID, 1, 30)
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
	return map[string]any{
		"game": game.format(), "current_issue": issue,
		"plays": plays, "history": history, "analysis": map[string]any{},
		"coin": strconv.FormatInt(balance.Available, 10),
	}, nil
}

func (s *Service) CurrentIssue(ctx context.Context, gameID int64, gameCode string) (map[string]any, error) {
	game, err := s.game(ctx, gameID, gameCode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1003, Message: "游戏不存在或维护中"}
	}
	if err != nil {
		return nil, err
	}
	issue, err := s.currentIssue(ctx, game.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1004, Message: "当前暂无可投注期号"}
	}
	return issue, err
}

func (s *Service) IssueHistory(
	ctx context.Context,
	gameID int64,
	gameCode string,
	page int,
) (map[string]any, error) {
	game, err := s.game(ctx, gameID, gameCode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1003, Message: "游戏不存在"}
	}
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	items, err := s.issueHistory(ctx, game.ID, page, 30)
	if err != nil {
		return nil, err
	}
	return map[string]any{"list": items, "page": strconv.Itoa(page)}, nil
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
	inputs, err := decodeBetInputs(request.ItemsJSON)
	if err != nil || len(inputs) < 1 || len(inputs) > 50 {
		return nil, &Error{Code: 1002, Message: "下注内容错误"}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	gameID := request.GameID
	if gameID < 1 && strings.TrimSpace(request.GameCode) != "" {
		if err = tx.QueryRowContext(ctx, `
			SELECT id FROM lottery_games WHERE game_code=? AND status=1`,
			strings.ToLower(strings.TrimSpace(request.GameCode)),
		).Scan(&gameID); err != nil {
			return nil, &Error{Code: 1003, Message: "游戏不存在或维护中", Cause: err}
		}
	}
	var gameCode string
	var minBet, maxBet int64
	if err = tx.QueryRowContext(ctx, `
		SELECT game_code,min_bet,max_bet FROM lottery_games
		WHERE id=? AND status=1 FOR UPDATE`,
		gameID,
	).Scan(&gameCode, &minBet, &maxBet); errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1003, Message: "游戏不存在或维护中"}
	} else if err != nil {
		return nil, err
	}
	var issueNo string
	var issueStatus int
	var saleClose time.Time
	if err = tx.QueryRowContext(ctx, `
		SELECT issue_no,status,sale_close_at FROM lottery_issues
		WHERE id=? AND game_id=? FOR UPDATE`,
		request.IssueID, gameID,
	).Scan(&issueNo, &issueStatus, &saleClose); errors.Is(err, sql.ErrNoRows) {
		return nil, &Error{Code: 1004, Message: "期号不存在"}
	} else if err != nil {
		return nil, err
	}
	if issueStatus != 1 || !saleClose.After(s.now()) {
		return nil, &Error{Code: 1005, Message: "本期已封盘"}
	}
	options := make([]betOption, 0, len(inputs))
	seen := make(map[int64]struct{}, len(inputs))
	var totalBet int64
	for _, input := range inputs {
		optionID, parseOptionErr := input.OptionID.Int64()
		amount, parseAmountErr := input.Amount.Int64()
		if parseOptionErr != nil || parseAmountErr != nil || optionID < 1 || amount < 1 {
			return nil, &Error{Code: 1006, Message: "下注项错误"}
		}
		if _, exists := seen[optionID]; exists {
			return nil, &Error{Code: 1006, Message: "下注项重复"}
		}
		seen[optionID] = struct{}{}
		if totalBet > maxBet-amount {
			return nil, &Error{Code: 1010, Message: "下注金额超过单次限制"}
		}
		var option betOption
		err = tx.QueryRowContext(ctx, `
			SELECT opt.id,play.id,play.play_code,opt.option_code,opt.name,opt.odds_scaled
			FROM lottery_options opt
			JOIN lottery_plays play ON play.id=opt.play_id AND play.status=1
			WHERE opt.id=? AND opt.status=1 AND play.game_id=?`,
			optionID, gameID,
		).Scan(
			&option.ID, &option.PlayID, &option.PlayCode, &option.OptionCode,
			&option.OptionName, &option.OddsScaled,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &Error{Code: 1007, Message: "玩法选项不存在"}
		}
		if err != nil {
			return nil, err
		}
		option.Amount = amount
		totalBet += amount
		options = append(options, option)
	}
	if totalBet < minBet {
		return nil, &Error{Code: 1009, Message: "下注金额低于最低限制"}
	}
	if totalBet > maxBet {
		return nil, &Error{Code: 1010, Message: "下注金额超过单次限制"}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	hold, err := s.wallet.PlaceHold(ctx, wallet.HoldRequest{
		UserID: request.UserID, Amount: totalBet,
		BusinessType: "lottery_bet", BusinessID: request.ClientTraceID,
		ExpiresAt: saleClose.Add(48 * time.Hour), Description: "彩票投注冻结",
		Metadata: map[string]any{
			"game_id": gameID, "issue_id": request.IssueID, "client_trace_id": request.ClientTraceID,
		},
		GameCode: gameCode, RoundNo: issueNo,
	})
	if err != nil {
		return nil, err
	}
	releaseFailedHold := func() {
		_, _ = s.wallet.ReleaseHold(ctx, hold.HoldNo, "彩票订单创建失败退回", map[string]any{
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
		INSERT INTO lottery_bet_orders
			(order_no,user_id,game_id,issue_id,hold_no,total_bet,status,client_trace_id)
		VALUES(?,?,?,?,?,?,0,?)`,
		orderNo, request.UserID, gameID, request.IssueID, hold.HoldNo, totalBet, request.ClientTraceID,
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
			INSERT INTO lottery_bet_items
				(order_id,play_id,option_id,bet_amount,odds_scaled,payout_amount,result)
			VALUES(?,?,?,?,?,0,0)`,
			orderID, option.PlayID, option.ID, option.Amount, option.OddsScaled,
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
	gameID int64,
	gameCode string,
	page int,
) (map[string]any, error) {
	if userID < 1 {
		return nil, &Error{Code: 700, Message: "登录已失效"}
	}
	if gameID < 1 && strings.TrimSpace(gameCode) != "" {
		_ = s.db.QueryRowContext(ctx, `
			SELECT id FROM lottery_games WHERE game_code=?`,
			strings.ToLower(strings.TrimSpace(gameCode)),
		).Scan(&gameID)
	}
	if page < 1 {
		page = 1
	}
	args := []any{userID}
	query := "SELECT id FROM lottery_bet_orders WHERE user_id=?"
	if gameID > 0 {
		query += " AND game_id=?"
		args = append(args, gameID)
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
	summaryArgs := []any{userID}
	summaryQuery := `
		SELECT COUNT(*),COALESCE(SUM(total_bet),0),COALESCE(SUM(total_payout),0),
		       COALESCE(SUM(CASE WHEN status IN (1,2) THEN total_payout-total_bet ELSE 0 END),0)
		FROM lottery_bet_orders WHERE user_id=?`
	if gameID > 0 {
		summaryQuery += " AND game_id=?"
		summaryArgs = append(summaryArgs, gameID)
	}
	var count, totalBet, totalPayout, profitLoss int64
	if err = s.db.QueryRowContext(ctx, summaryQuery, summaryArgs...).Scan(
		&count, &totalBet, &totalPayout, &profitLoss,
	); err != nil {
		return nil, err
	}
	balance, err := s.wallet.Balance(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"list": items, "items": items, "orders": items, "page": strconv.Itoa(page),
		"total_count":  strconv.FormatInt(count, 10),
		"total_bet":    strconv.FormatInt(totalBet, 10),
		"total_payout": strconv.FormatInt(totalPayout, 10),
		"profit_loss":  strconv.FormatInt(profitLoss, 10),
		"net_amount":   strconv.FormatInt(profitLoss, 10),
		"coin":         strconv.FormatInt(balance.Available, 10),
	}, nil
}

type gameRecord struct {
	ID           int64
	CategoryID   int64
	Code         string
	Name         string
	Interval     int
	CloseSeconds int
	MinBet       int64
	MaxBet       int64
	Status       int
	Config       []byte
	Icon         string
}

func (game gameRecord) format() map[string]any {
	var config any
	_ = json.Unmarshal(game.Config, &config)
	return map[string]any{
		"id":          strconv.FormatInt(game.ID, 10),
		"category_id": strconv.FormatInt(game.CategoryID, 10),
		"game_code":   game.Code, "game_name": game.Name, "game_name_en": "",
		"icon": game.Icon, "icon_url": game.Icon,
		"interval_sec":     strconv.Itoa(game.Interval),
		"seal_advance_sec": strconv.Itoa(game.CloseSeconds),
		"min_bet":          strconv.FormatInt(game.MinBet, 10),
		"max_bet":          strconv.FormatInt(game.MaxBet, 10),
		"status":           strconv.Itoa(game.Status), "config": config,
	}
}

func (s *Service) game(ctx context.Context, gameID int64, gameCode string) (gameRecord, error) {
	query := `
		SELECT game.id,game.category_id,game.game_code,game.name,
		       game.issue_interval_seconds,game.sale_close_seconds,game.min_bet,game.max_bet,
		       game.status,game.config
		FROM lottery_games game
		WHERE game.status=1 AND `
	var row *sql.Row
	if gameID > 0 {
		row = s.db.QueryRowContext(ctx, query+"game.id=?", gameID)
	} else {
		row = s.db.QueryRowContext(
			ctx, query+"game.game_code=?", strings.ToLower(strings.TrimSpace(gameCode)),
		)
	}
	var result gameRecord
	err := row.Scan(
		&result.ID, &result.CategoryID, &result.Code, &result.Name,
		&result.Interval, &result.CloseSeconds, &result.MinBet, &result.MaxBet,
		&result.Status, &result.Config,
	)
	result.Icon = staticLotteryIconURL(result.Code)
	return result, err
}

func (s *Service) categories(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT category.id,category.name,COALESCE(asset.bucket,''),COALESCE(asset.object_key,'')
		FROM lottery_categories category
		LEFT JOIN media_assets asset ON asset.id=category.icon_asset_id AND asset.status=1
		WHERE category.status=1 ORDER BY category.sort_order DESC,category.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 8)
	for rows.Next() {
		var id int64
		var name, bucket, objectKey string
		if err = rows.Scan(&id, &name, &bucket, &objectKey); err != nil {
			return nil, err
		}
		icon := s.assetURL(bucket, objectKey)
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "name": name, "name_en": "",
			"icon": icon, "icon_url": icon,
		})
	}
	return items, rows.Err()
}

func staticLotteryIconURL(gameCode string) string {
	code := strings.ToUpper(strings.TrimSpace(gameCode))
	if code == "" {
		return ""
	}
	return "/lottery-icons/" + url.PathEscape(code) + ".png"
}

func (s *Service) currentIssue(ctx context.Context, gameID int64) (map[string]any, error) {
	var id, saleOpen, saleClose, drawAt int64
	var issueNo string
	var status int
	err := s.db.QueryRowContext(ctx, `
		SELECT id,issue_no,CAST(UNIX_TIMESTAMP(sale_open_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(sale_close_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(draw_at) AS UNSIGNED),status
		FROM lottery_issues
		WHERE game_id=? AND status=1 AND sale_close_at>CURRENT_TIMESTAMP(3)
		ORDER BY sale_close_at,id LIMIT 1`,
		gameID,
	).Scan(&id, &issueNo, &saleOpen, &saleClose, &drawAt, &status)
	if err != nil {
		return nil, err
	}
	return formatIssue(id, gameID, issueNo, saleOpen, saleClose, drawAt, status, nil, s.now()), nil
}

func (s *Service) issueHistory(
	ctx context.Context,
	gameID int64,
	page int,
	limit int,
) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,issue_no,CAST(UNIX_TIMESTAMP(sale_open_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(sale_close_at) AS UNSIGNED),
		       CAST(UNIX_TIMESTAMP(draw_at) AS UNSIGNED),status,draw_result
		FROM lottery_issues
		WHERE game_id=? AND status IN (3,4,5)
		ORDER BY draw_at DESC,id DESC LIMIT ? OFFSET ?`,
		gameID, limit, (page-1)*limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, saleOpen, saleClose, drawAt int64
		var issueNo string
		var status int
		var result []byte
		if err = rows.Scan(&id, &issueNo, &saleOpen, &saleClose, &drawAt, &status, &result); err != nil {
			return nil, err
		}
		items = append(items, formatIssue(
			id, gameID, issueNo, saleOpen, saleClose, drawAt, status, result, s.now(),
		))
	}
	return items, rows.Err()
}

func (s *Service) plays(ctx context.Context, gameID int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT play.id,play.play_code,play.name,play.settlement_rule,
		       opt.id,opt.option_code,opt.name,opt.odds_scaled
		FROM lottery_plays play
		JOIN lottery_options opt ON opt.play_id=play.id AND opt.status=1
		WHERE play.game_id=? AND play.status=1
		ORDER BY play.sort_order DESC,play.id,opt.sort_order DESC,opt.id`,
		gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 8)
	indexes := make(map[int64]int)
	for rows.Next() {
		var playID, optionID, oddsScaled int64
		var playCode, playName, rule, optionCode, optionName string
		if err = rows.Scan(
			&playID, &playCode, &playName, &rule,
			&optionID, &optionCode, &optionName, &oddsScaled,
		); err != nil {
			return nil, err
		}
		index, exists := indexes[playID]
		if !exists {
			index = len(items)
			indexes[playID] = index
			items = append(items, map[string]any{
				"id": strconv.FormatInt(playID, 10), "play_code": playCode,
				"play_name": playName, "result_rule": rule,
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

func (s *Service) orderByTrace(ctx context.Context, userID int64, traceID string) (map[string]any, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM lottery_bet_orders WHERE user_id=? AND client_trace_id=?`,
		userID, traceID,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.orderByID(ctx, id)
}

func (s *Service) orderByID(ctx context.Context, orderID int64) (map[string]any, error) {
	var id, userID, gameID, issueID, totalBet, totalPayout int64
	var orderNo, traceID, gameName, gameCode, issueNo string
	var status int
	var settledAt sql.NullTime
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT order_row.id,order_row.order_no,order_row.client_trace_id,order_row.user_id,
		       order_row.game_id,game.name,game.game_code,order_row.issue_id,issue.issue_no,
		       order_row.total_bet,order_row.total_payout,order_row.status,
		       order_row.settled_at,order_row.created_at
		FROM lottery_bet_orders order_row
		JOIN lottery_games game ON game.id=order_row.game_id
		JOIN lottery_issues issue ON issue.id=order_row.issue_id
		WHERE order_row.id=?`,
		orderID,
	).Scan(
		&id, &orderNo, &traceID, &userID, &gameID, &gameName, &gameCode,
		&issueID, &issueNo, &totalBet, &totalPayout, &status, &settledAt, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT item.id,item.play_id,play.play_code,item.option_id,opt.option_code,
		       opt.name,item.odds_scaled,item.bet_amount,item.payout_amount,item.result
		FROM lottery_bet_items item
		JOIN lottery_plays play ON play.id=item.play_id
		JOIN lottery_options opt ON opt.id=item.option_id
		WHERE item.order_id=? ORDER BY item.id`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 8)
	for rows.Next() {
		var itemID, playID, optionID, oddsScaled, amount, payout int64
		var playCode, optionCode, optionName string
		var result int
		if err = rows.Scan(
			&itemID, &playID, &playCode, &optionID, &optionCode, &optionName,
			&oddsScaled, &amount, &payout, &result,
		); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(itemID, 10), "play_id": strconv.FormatInt(playID, 10),
			"play_code": playCode, "option_id": strconv.FormatInt(optionID, 10),
			"option_code": optionCode, "option_name": optionName,
			"odds": formatOdds(oddsScaled), "odds_scaled": oddsScaled,
			"bet_amount":    strconv.FormatInt(amount, 10),
			"payout_amount": strconv.FormatInt(payout, 10), "win_status": strconv.Itoa(result),
		})
	}
	netAmount := int64(0)
	if status == 1 || status == 2 {
		netAmount = totalPayout - totalBet
	}
	return map[string]any{
		"id": strconv.FormatInt(id, 10), "orderid": strconv.FormatInt(id, 10),
		"order_no": orderNo, "client_trace_id": traceID,
		"uid": strconv.FormatInt(userID, 10), "game_id": strconv.FormatInt(gameID, 10),
		"game_name": gameName, "game_code": gameCode,
		"issue_id": strconv.FormatInt(issueID, 10), "issue_num": issueNo,
		"total_bet":    strconv.FormatInt(totalBet, 10),
		"total_payout": strconv.FormatInt(totalPayout, 10),
		"net_amount":   strconv.FormatInt(netAmount, 10),
		"status":       strconv.Itoa(status), "status_text": orderStatusText(status),
		"bet_time":    strconv.FormatInt(createdAt.Unix(), 10),
		"settle_time": nullableUnix(settledAt), "items": items,
	}, rows.Err()
}

func decodeBetInputs(raw string) ([]betInput, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(strings.TrimSpace(raw)))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var inputs []betInput
	if err := decoder.Decode(&inputs); err != nil {
		return nil, err
	}
	return inputs, nil
}

func formatIssue(
	id int64,
	gameID int64,
	issueNo string,
	saleOpen int64,
	saleClose int64,
	drawAt int64,
	status int,
	result []byte,
	now time.Time,
) map[string]any {
	var decodedResult any
	if len(result) > 0 {
		_ = json.Unmarshal(result, &decodedResult)
	}
	openCode := ""
	if decodedResult != nil {
		if resultObject, ok := decodedResult.(map[string]any); ok {
			if legacyCode, present := resultObject["open_code"].(string); present {
				openCode = legacyCode
			}
		}
		if openCode == "" {
			encoded, _ := json.Marshal(decodedResult)
			openCode = string(encoded)
		}
	}
	sealCountdown := max64(saleClose-now.Unix(), 0)
	openCountdown := max64(drawAt-now.Unix(), 0)
	canBet := status == 1 && sealCountdown > 0
	return map[string]any{
		"id": strconv.FormatInt(id, 10), "game_id": strconv.FormatInt(gameID, 10),
		"issue_num": issueNo, "issue_no": issueNo, "open_code": openCode,
		"draw_result":  decodedResult,
		"sale_open_at": saleOpen, "sale_close_at": saleClose, "draw_at": drawAt,
		"open_time":      strconv.FormatInt(drawAt, 10),
		"seal_time":      strconv.FormatInt(saleClose, 10),
		"next_open_time": strconv.FormatInt(drawAt, 10),
		"status":         strconv.Itoa(status),
		"can_bet":        boolNumber(canBet),
		"open_time_text": time.Unix(drawAt, 0).Format("2006-01-02 15:04:05"),
		"seal_countdown": strconv.FormatInt(sealCountdown, 10),
		"bet_countdown":  strconv.FormatInt(sealCountdown, 10),
		"countdown":      strconv.FormatInt(sealCountdown, 10),
		"open_countdown": strconv.FormatInt(openCountdown, 10),
	}
}

func boolNumber(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func (s *Service) assetURL(bucket, objectKey string) string {
	bucket = strings.Trim(bucket, "/")
	objectKey = strings.Trim(objectKey, "/")
	if bucket == "" || objectKey == "" {
		return ""
	}
	segments := strings.Split(objectKey, "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	escapedBucket := url.PathEscape(bucket)
	if strings.HasSuffix(s.mediaBaseURL, "/"+escapedBucket) {
		return s.mediaBaseURL + "/" + strings.Join(segments, "/")
	}
	return s.mediaBaseURL + "/" + escapedBucket + "/" + strings.Join(segments, "/")
}

func formatOdds(scaled int64) string {
	whole := scaled / 1_000_000
	fraction := scaled % 1_000_000
	value := strconv.FormatInt(whole, 10) + "." + fmt.Sprintf("%06d", fraction)
	return strings.TrimRight(strings.TrimRight(value, "0"), ".")
}

func nullableUnix(value sql.NullTime) string {
	if value.Valid {
		return strconv.FormatInt(value.Time.Unix(), 10)
	}
	return "0"
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
		return "待开奖"
	}
}

func max64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
