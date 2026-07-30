package admin

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

var catalogKeyPattern = regexp.MustCompile(`^[0-9a-z][0-9a-z_-]{1,59}$`)

func lotteryCategoryAllowed(key, name string) bool {
	value := strings.ToLower(strings.Join([]string{key, name}, " "))
	for _, blocked := range []string{
		"低频", "波场", "波厂", "区块链", "low frequency", "low_frequency",
		"blockchain", "tron", "trx",
	} {
		if strings.Contains(value, blocked) {
			return false
		}
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "lf", "low", "blockchain", "tron", "trx":
		return false
	default:
		return true
	}
}

func lotteryGameAllowed(code, name string) bool {
	value := strings.ToLower(strings.Join([]string{code, name}, " "))
	for _, blocked := range []string{"trx", "tron", "波场", "波厂"} {
		if strings.Contains(value, blocked) {
			return false
		}
	}
	return true
}

func (h *Handler) listLotteryCatalog(w http.ResponseWriter, r *http.Request) {
	categoryRows, err := h.db.QueryContext(r.Context(), `
		SELECT id,category_key,name,status,sort_order
		FROM lottery_categories
		ORDER BY sort_order DESC,id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票分类失败")
		return
	}
	defer categoryRows.Close()
	categories := make([]map[string]any, 0, 8)
	for categoryRows.Next() {
		var id int64
		var key, name string
		var status, sortOrder int
		if err = categoryRows.Scan(&id, &key, &name, &status, &sortOrder); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票分类失败")
			return
		}
		categories = append(categories, map[string]any{
			"id": apiDecimalID(id), "category_key": key, "name": name,
			"status": status, "sort_order": sortOrder,
		})
	}
	if err = categoryRows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票分类失败")
		return
	}

	gameRows, err := h.db.QueryContext(r.Context(), `
		SELECT game.id,game.category_id,category.name,game.game_code,game.name,
		       game.issue_interval_seconds,game.sale_close_seconds,game.min_bet,game.max_bet,
		       game.status,game.sort_order,game.config,
		       issue.id,issue.issue_no,issue.sale_close_at,issue.draw_at,issue.status
		FROM lottery_games game
		JOIN lottery_categories category ON category.id=game.category_id
		LEFT JOIN lottery_issues issue ON issue.id=(
			SELECT latest.id FROM lottery_issues latest
			WHERE latest.game_id=game.id
			ORDER BY latest.draw_at DESC,latest.id DESC LIMIT 1
			)
		ORDER BY game.sort_order DESC,game.id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票游戏失败")
		return
	}
	defer gameRows.Close()
	games := make([]map[string]any, 0, 16)
	for gameRows.Next() {
		var id, categoryID, minBet, maxBet int64
		var categoryName, gameCode, name string
		var interval, closeSeconds, status, sortOrder int
		var configJSON []byte
		var issueID sql.NullInt64
		var issueNo sql.NullString
		var saleClose, drawAt sql.NullTime
		var issueStatus sql.NullInt64
		if err = gameRows.Scan(
			&id, &categoryID, &categoryName, &gameCode, &name,
			&interval, &closeSeconds, &minBet, &maxBet, &status, &sortOrder, &configJSON,
			&issueID, &issueNo, &saleClose, &drawAt, &issueStatus,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票游戏失败")
			return
		}
		config := jsonOrNil(configJSON)
		var currentIssue any
		if issueID.Valid {
			currentIssue = map[string]any{
				"id": apiDecimalID(issueID.Int64), "issue_no": issueNo.String,
				"sale_close_at": nullTime(saleClose), "draw_at": nullTime(drawAt),
				"status": issueStatus.Int64,
			}
		}
		games = append(games, map[string]any{
			"id": apiDecimalID(id), "category_id": apiDecimalID(categoryID),
			"category_name": categoryName,
			"game_code":     gameCode, "name": name, "issue_interval_seconds": interval,
			"sale_close_seconds": closeSeconds, "min_bet": minBet, "max_bet": maxBet,
			"status": status, "sort_order": sortOrder, "config": config,
			"latest_issue": currentIssue,
		})
	}
	if err = gameRows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票游戏失败")
		return
	}

	playRows, err := h.db.QueryContext(r.Context(), `
		SELECT play.id,play.game_id,game.name,play.play_code,play.name,
		       play.settlement_rule,play.status,play.sort_order,
		       (SELECT COUNT(*) FROM lottery_options option_row WHERE option_row.play_id=play.id)
		FROM lottery_plays play
		JOIN lottery_games game ON game.id=play.game_id
		ORDER BY game.sort_order DESC,play.sort_order DESC,play.id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票玩法失败")
		return
	}
	defer playRows.Close()
	plays := make([]map[string]any, 0, 32)
	for playRows.Next() {
		var id, gameID, optionCount int64
		var gameName, playCode, name, settlementRule string
		var status, sortOrder int
		if err = playRows.Scan(
			&id, &gameID, &gameName, &playCode, &name, &settlementRule,
			&status, &sortOrder, &optionCount,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票玩法失败")
			return
		}
		plays = append(plays, map[string]any{
			"id": apiDecimalID(id), "game_id": apiDecimalID(gameID),
			"game_name": gameName, "play_code": playCode,
			"name": name, "settlement_rule": settlementRule, "status": status,
			"sort_order": sortOrder, "option_count": optionCount,
		})
	}
	if err = playRows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票玩法失败")
		return
	}

	optionRows, err := h.db.QueryContext(r.Context(), `
		SELECT option_row.id,option_row.play_id,play.name,game.name,
		       option_row.option_code,option_row.name,option_row.odds_scaled,
		       option_row.status,option_row.sort_order
		FROM lottery_options option_row
		JOIN lottery_plays play ON play.id=option_row.play_id
		JOIN lottery_games game ON game.id=play.game_id
		ORDER BY game.sort_order DESC,play.sort_order DESC,option_row.sort_order DESC,option_row.id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票选项失败")
		return
	}
	defer optionRows.Close()
	options := make([]map[string]any, 0, 64)
	for optionRows.Next() {
		var id, playID, oddsScaled int64
		var playName, gameName, optionCode, name string
		var status, sortOrder int
		if err = optionRows.Scan(
			&id, &playID, &playName, &gameName, &optionCode, &name,
			&oddsScaled, &status, &sortOrder,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票选项失败")
			return
		}
		options = append(options, map[string]any{
			"id": apiDecimalID(id), "play_id": apiDecimalID(playID),
			"play_name": playName, "game_name": gameName,
			"option_code": optionCode, "name": name, "odds_scaled": oddsScaled,
			"status": status, "sort_order": sortOrder,
		})
	}
	if err = optionRows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票选项失败")
		return
	}

	issueRows, err := h.db.QueryContext(r.Context(), `
		SELECT issue.id,issue.game_id,game.name,issue.issue_no,issue.sale_open_at,
		       issue.sale_close_at,issue.draw_at,issue.status,issue.result_source,
		       (SELECT COUNT(*) FROM lottery_bet_orders bet_order WHERE bet_order.issue_id=issue.id),
		       (SELECT COALESCE(SUM(bet_order.total_bet),0)
		        FROM lottery_bet_orders bet_order WHERE bet_order.issue_id=issue.id),
		       (SELECT COALESCE(SUM(bet_order.total_payout),0)
		        FROM lottery_bet_orders bet_order WHERE bet_order.issue_id=issue.id)
		FROM lottery_issues issue
		JOIN lottery_games game ON game.id=issue.game_id
		ORDER BY issue.draw_at DESC,issue.id DESC LIMIT 100`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
		return
	}
	defer issueRows.Close()
	issues := make([]map[string]any, 0, 100)
	for issueRows.Next() {
		var id, gameID, orderCount, totalBet, totalPayout int64
		var gameName, issueNo, resultSource string
		var saleOpen, saleClose, drawAt time.Time
		var status int
		if err = issueRows.Scan(
			&id, &gameID, &gameName, &issueNo, &saleOpen, &saleClose, &drawAt,
			&status, &resultSource, &orderCount, &totalBet, &totalPayout,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
			return
		}
		issues = append(issues, map[string]any{
			"id": apiDecimalID(id), "game_id": apiDecimalID(gameID),
			"game_name": gameName, "issue_no": issueNo,
			"sale_open_at": saleOpen.Unix(), "sale_close_at": saleClose.Unix(),
			"draw_at": drawAt.Unix(), "status": status, "result_source": resultSource,
			"order_count": orderCount, "total_bet": totalBet, "total_payout": totalPayout,
		})
	}
	if err = issueRows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"categories": categories, "games": games, "plays": plays,
		"options": options, "issues": issues,
	})
}

func (h *Handler) createLotteryCategory(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CategoryKey string `json:"category_key"`
		Name        string `json:"name"`
		Status      int    `json:"status"`
		SortOrder   int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.CategoryKey = strings.ToLower(strings.TrimSpace(request.CategoryKey))
	request.Name = strings.TrimSpace(request.Name)
	if !catalogKeyPattern.MatchString(request.CategoryKey) || request.Name == "" ||
		len(request.Name) > 100 || request.Status != 1 ||
		!lotteryCategoryAllowed(request.CategoryKey, request.Name) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票分类参数无效")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		INSERT INTO lottery_categories(category_key,name,status,sort_order) VALUES(?,?,?,?)`,
		request.CategoryKey, request.Name, request.Status, request.SortOrder,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "彩票分类标识已存在")
		return
	}
	id, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), h.db, r, "lottery.category.create", "lottery_category", id, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(id)})
}

func (h *Handler) updateLotteryCategory(w http.ResponseWriter, r *http.Request) {
	categoryID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票分类编号无效")
		return
	}
	var request struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	var categoryKey, beforeName string
	var beforeSort int
	if err = h.db.QueryRowContext(r.Context(), `
		SELECT category_key,name,sort_order
		FROM lottery_categories WHERE id=? AND status=1`,
		categoryID,
	).Scan(&categoryKey, &beforeName, &beforeSort); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "彩票分类不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票分类失败")
		return
	}
	if request.Name == "" || len(request.Name) > 100 ||
		!lotteryCategoryAllowed(categoryKey, request.Name) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票分类参数无效")
		return
	}
	if _, err = h.db.ExecContext(r.Context(), `
		UPDATE lottery_categories SET name=?,sort_order=? WHERE id=? AND status=1`,
		request.Name, request.SortOrder, categoryID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票分类失败")
		return
	}
	if err = auditAdmin(
		r.Context(), h.db, r, "lottery.category.update", "lottery_category", categoryID,
		map[string]any{"name": beforeName, "sort_order": beforeSort}, request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(categoryID), "updated": true,
	})
}

func (h *Handler) createLotteryGame(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CategoryID           decimalIDInput `json:"category_id"`
		GameCode             string         `json:"game_code"`
		Name                 string         `json:"name"`
		IssueIntervalSeconds int            `json:"issue_interval_seconds"`
		SaleCloseSeconds     int            `json:"sale_close_seconds"`
		MinBet               int64          `json:"min_bet"`
		MaxBet               int64          `json:"max_bet"`
		Status               int            `json:"status"`
		SortOrder            int            `json:"sort_order"`
		Config               map[string]any `json:"config"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.GameCode = strings.ToLower(strings.TrimSpace(request.GameCode))
	request.Name = strings.TrimSpace(request.Name)
	if request.CategoryID < 1 || !catalogKeyPattern.MatchString(request.GameCode) ||
		request.Name == "" || len(request.Name) > 120 ||
		request.IssueIntervalSeconds < 30 || request.IssueIntervalSeconds > 31*24*3600 ||
		request.SaleCloseSeconds < 0 || request.SaleCloseSeconds >= request.IssueIntervalSeconds ||
		request.MinBet < 1 || request.MaxBet < request.MinBet || request.Status != 1 ||
		!lotteryGameAllowed(request.GameCode, request.Name) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票游戏参数无效")
		return
	}
	configJSON, err := json.Marshal(request.Config)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票配置无效")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		INSERT INTO lottery_games
			(category_id,game_code,name,issue_interval_seconds,sale_close_seconds,
			 min_bet,max_bet,status,sort_order,config)
		SELECT ?,?,?,?,?,?,?,?,?,? FROM lottery_categories WHERE id=? AND status=1`,
		request.CategoryID.Int64(), request.GameCode, request.Name, request.IssueIntervalSeconds,
		request.SaleCloseSeconds, request.MinBet, request.MaxBet, request.Status,
		request.SortOrder, configJSON, request.CategoryID.Int64(),
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "彩票标识已存在")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票分类不存在")
		return
	}
	id, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), h.db, r, "lottery.game.create", "lottery_game", id, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(id)})
}

func (h *Handler) updateLotteryGame(w http.ResponseWriter, r *http.Request) {
	gameID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票编号无效")
		return
	}
	var request struct {
		CategoryID           decimalIDInput `json:"category_id"`
		GameCode             string         `json:"game_code"`
		Name                 string         `json:"name"`
		IssueIntervalSeconds int            `json:"issue_interval_seconds"`
		SaleCloseSeconds     int            `json:"sale_close_seconds"`
		MinBet               int64          `json:"min_bet"`
		MaxBet               int64          `json:"max_bet"`
		SortOrder            int            `json:"sort_order"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.GameCode = strings.ToLower(strings.TrimSpace(request.GameCode))
	request.Name = strings.TrimSpace(request.Name)
	if request.CategoryID < 1 || !catalogKeyPattern.MatchString(request.GameCode) ||
		request.Name == "" || len(request.Name) > 120 ||
		request.IssueIntervalSeconds < 30 || request.IssueIntervalSeconds > 31*24*3600 ||
		request.SaleCloseSeconds < 0 || request.SaleCloseSeconds >= request.IssueIntervalSeconds ||
		request.MinBet < 1 || request.MaxBet < request.MinBet ||
		!lotteryGameAllowed(request.GameCode, request.Name) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票游戏参数无效")
		return
	}
	var categoryExists int
	if err = h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM lottery_categories WHERE id=? AND status=1`,
		request.CategoryID.Int64(),
	).Scan(&categoryExists); err != nil || categoryExists != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票分类不存在")
		return
	}
	var beforeCategoryID int64
	var beforeCode, beforeName string
	if err = h.db.QueryRowContext(r.Context(), `
		SELECT category_id,game_code,name
		FROM lottery_games WHERE id=?`,
		gameID,
	).Scan(&beforeCategoryID, &beforeCode, &beforeName); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "彩票不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票失败")
		return
	}
	if _, err = h.db.ExecContext(r.Context(), `
		UPDATE lottery_games
		SET category_id=?,game_code=?,name=?,issue_interval_seconds=?,
		    sale_close_seconds=?,min_bet=?,max_bet=?,sort_order=?
		WHERE id=?`,
		request.CategoryID.Int64(), request.GameCode, request.Name, request.IssueIntervalSeconds,
		request.SaleCloseSeconds, request.MinBet, request.MaxBet, request.SortOrder, gameID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "彩票标识已存在")
		return
	}
	if err = auditAdmin(
		r.Context(), h.db, r, "lottery.game.update", "lottery_game", gameID,
		map[string]any{
			"category_id": apiDecimalID(beforeCategoryID),
			"game_code":   beforeCode, "name": beforeName,
		},
		request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(gameID), "updated": true,
	})
}

func (h *Handler) setLotteryGameStatus(w http.ResponseWriter, r *http.Request) {
	gameID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || gameID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票编号无效")
		return
	}
	var request struct {
		Status int `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Status < 0 || request.Status > 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票状态无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票状态失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var before int
	if err = tx.QueryRowContext(r.Context(), "SELECT status FROM lottery_games WHERE id=? FOR UPDATE", gameID).Scan(&before); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "彩票不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票状态失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), "UPDATE lottery_games SET status=? WHERE id=?", request.Status, gameID); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票状态失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "lottery.game.status", "lottery_game", gameID,
		map[string]int{"status": before}, request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票状态失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(gameID), "status": request.Status,
	})
}

func (h *Handler) createLotteryPlay(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GameID         decimalIDInput `json:"game_id"`
		PlayCode       string         `json:"play_code"`
		Name           string         `json:"name"`
		SettlementRule string         `json:"settlement_rule"`
		Status         int            `json:"status"`
		SortOrder      int            `json:"sort_order"`
		Config         map[string]any `json:"config"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.PlayCode = strings.ToLower(strings.TrimSpace(request.PlayCode))
	request.Name = strings.TrimSpace(request.Name)
	request.SettlementRule = strings.TrimSpace(request.SettlementRule)
	if request.GameID < 1 || !catalogKeyPattern.MatchString(request.PlayCode) ||
		request.Name == "" || len(request.Name) > 120 || request.SettlementRule == "" ||
		len(request.SettlementRule) > 80 || request.Status != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票玩法参数无效")
		return
	}
	configJSON, _ := json.Marshal(request.Config)
	result, err := h.db.ExecContext(r.Context(), `
		INSERT INTO lottery_plays(game_id,play_code,name,settlement_rule,status,sort_order,config)
		SELECT ?,?,?,?,?,?,? FROM lottery_games WHERE id=? AND status=1`,
		request.GameID.Int64(), request.PlayCode, request.Name, request.SettlementRule,
		request.Status, request.SortOrder, configJSON, request.GameID.Int64(),
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "彩票玩法已存在")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票游戏不存在")
		return
	}
	id, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), h.db, r, "lottery.play.create", "lottery_play", id, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(id)})
}

func (h *Handler) updateLotteryPlay(w http.ResponseWriter, r *http.Request) {
	playID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票玩法编号无效")
		return
	}
	var request struct {
		Name           string `json:"name"`
		SettlementRule string `json:"settlement_rule"`
		SortOrder      int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.SettlementRule = strings.TrimSpace(request.SettlementRule)
	if request.Name == "" || len(request.Name) > 120 ||
		request.SettlementRule == "" || len(request.SettlementRule) > 80 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票玩法参数无效")
		return
	}
	var beforeName, beforeRule string
	if err = h.db.QueryRowContext(r.Context(), `
		SELECT name,settlement_rule FROM lottery_plays WHERE id=? AND status=1`,
		playID,
	).Scan(&beforeName, &beforeRule); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "彩票玩法不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票玩法失败")
		return
	}
	if _, err = h.db.ExecContext(r.Context(), `
		UPDATE lottery_plays SET name=?,settlement_rule=?,sort_order=?
		WHERE id=? AND status=1`,
		request.Name, request.SettlementRule, request.SortOrder, playID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票玩法失败")
		return
	}
	if err = auditAdmin(
		r.Context(), h.db, r, "lottery.play.update", "lottery_play", playID,
		map[string]any{"name": beforeName, "settlement_rule": beforeRule}, request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(playID), "updated": true,
	})
}

func (h *Handler) createLotteryOption(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PlayID     decimalIDInput `json:"play_id"`
		OptionCode string         `json:"option_code"`
		Name       string         `json:"name"`
		OddsScaled int64          `json:"odds_scaled"`
		Status     int            `json:"status"`
		SortOrder  int            `json:"sort_order"`
		Config     map[string]any `json:"config"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.OptionCode = strings.ToLower(strings.TrimSpace(request.OptionCode))
	request.Name = strings.TrimSpace(request.Name)
	if request.PlayID < 1 || !catalogKeyPattern.MatchString(request.OptionCode) ||
		request.Name == "" || len(request.Name) > 120 || request.OddsScaled < 1 ||
		request.OddsScaled > 1_000_000_000_000 || request.Status != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票选项参数无效")
		return
	}
	configJSON, _ := json.Marshal(request.Config)
	result, err := h.db.ExecContext(r.Context(), `
		INSERT INTO lottery_options(play_id,option_code,name,odds_scaled,status,sort_order,config)
		SELECT ?,?,?,?,?,?,? FROM lottery_plays WHERE id=? AND status=1`,
		request.PlayID.Int64(), request.OptionCode, request.Name, request.OddsScaled,
		request.Status, request.SortOrder, configJSON, request.PlayID.Int64(),
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "彩票选项已存在")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票玩法不存在")
		return
	}
	id, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), h.db, r, "lottery.option.create", "lottery_option", id, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": apiDecimalID(id)})
}

func (h *Handler) updateLotteryOption(w http.ResponseWriter, r *http.Request) {
	optionID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票选项编号无效")
		return
	}
	var request struct {
		Name       string `json:"name"`
		OddsScaled int64  `json:"odds_scaled"`
		SortOrder  int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > 120 || request.OddsScaled < 1 ||
		request.OddsScaled > 1_000_000_000_000 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票选项参数无效")
		return
	}
	var beforeName string
	var beforeOdds int64
	if err = h.db.QueryRowContext(r.Context(), `
		SELECT name,odds_scaled FROM lottery_options WHERE id=? AND status=1`,
		optionID,
	).Scan(&beforeName, &beforeOdds); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "彩票选项不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票选项失败")
		return
	}
	if _, err = h.db.ExecContext(r.Context(), `
		UPDATE lottery_options SET name=?,odds_scaled=?,sort_order=?
		WHERE id=? AND status=1`,
		request.Name, request.OddsScaled, request.SortOrder, optionID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新彩票选项失败")
		return
	}
	if err = auditAdmin(
		r.Context(), h.db, r, "lottery.option.update", "lottery_option", optionID,
		map[string]any{"name": beforeName, "odds_scaled": beforeOdds}, request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(optionID), "updated": true,
	})
}

func (h *Handler) createLotteryIssue(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GameID      decimalIDInput `json:"game_id"`
		IssueNo     string         `json:"issue_no"`
		SaleOpenAt  int64          `json:"sale_open_at"`
		SaleCloseAt int64          `json:"sale_close_at"`
		DrawAt      int64          `json:"draw_at"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.IssueNo = strings.TrimSpace(request.IssueNo)
	if request.GameID < 1 || request.IssueNo == "" || len(request.IssueNo) > 80 ||
		request.SaleOpenAt < 1 || request.SaleCloseAt <= request.SaleOpenAt ||
		request.DrawAt < request.SaleCloseAt {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "期号参数无效")
		return
	}
	status := 0
	now := time.Now().Unix()
	if request.SaleOpenAt <= now && request.SaleCloseAt > now {
		status = 1
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建彩票期号失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO lottery_issues
			(game_id,issue_no,sale_open_at,sale_close_at,draw_at,status)
		SELECT ?,?,FROM_UNIXTIME(?),FROM_UNIXTIME(?),FROM_UNIXTIME(?),?
		FROM lottery_games WHERE id=? AND status=1`,
		request.GameID.Int64(), request.IssueNo, request.SaleOpenAt, request.SaleCloseAt,
		request.DrawAt, status, request.GameID.Int64(),
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "彩票期号已存在或时间冲突")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票游戏不存在或已停用")
		return
	}
	id, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), tx, r, "lottery.issue.create", "lottery_issue", id, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建彩票期号失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(id), "status": status,
	})
}

func (h *Handler) closeLotteryIssue(w http.ResponseWriter, r *http.Request) {
	issueID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || issueID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "期号编号无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "封盘失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(r.Context(), `
		UPDATE lottery_issues SET status=2,sale_close_at=LEAST(sale_close_at,CURRENT_TIMESTAMP(3))
		WHERE id=? AND status IN (0,1)`,
		issueID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "封盘失败")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "期号不存在或当前状态不能封盘")
		return
	}
	if err = auditAdmin(r.Context(), tx, r, "lottery.issue.close", "lottery_issue", issueID, nil, map[string]int{"status": 2}); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "封盘失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(issueID), "status": 2,
	})
}

func (h *Handler) drawLotteryIssue(w http.ResponseWriter, r *http.Request) {
	issueID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || issueID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "期号编号无效")
		return
	}
	var request struct {
		Result json.RawMessage `json:"result"`
		Source string          `json:"source"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Source = strings.TrimSpace(request.Source)
	if len(request.Result) < 2 || len(request.Result) > 64<<10 || !json.Valid(request.Result) ||
		request.Source == "" || len(request.Source) > 40 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "开奖结果参数无效")
		return
	}
	canonical := make([]byte, 0, len(request.Result))
	var resultValue any
	decoder := json.NewDecoder(strings.NewReader(string(request.Result)))
	decoder.UseNumber()
	if err = decoder.Decode(&resultValue); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "开奖结果参数无效")
		return
	}
	canonical, _ = json.Marshal(resultValue)
	digest := sha256.Sum256(canonical)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "开奖失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var status int
	var previous []byte
	if err = tx.QueryRowContext(r.Context(), `
		SELECT status,COALESCE(draw_result,JSON_OBJECT()) FROM lottery_issues WHERE id=? FOR UPDATE`,
		issueID,
	).Scan(&status, &previous); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "彩票期号不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "开奖失败")
		return
	}
	if status != 2 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "只有已封盘期号可以开奖")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE lottery_issues
		SET draw_result=?,result_source=?,status=3 WHERE id=?`,
		canonical, request.Source, issueID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "开奖失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO lottery_draw_audits
			(issue_id,action,source,before_result,after_result,payload_hash,actor_type,actor_id)
		VALUES(?,'draw',?,?,?, ?,1,?)`,
		issueID, request.Source, previous, canonical, hex.EncodeToString(digest[:]), adminID(r),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录开奖审计失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "lottery.issue.draw", "lottery_issue", issueID,
		map[string]any{"status": status}, map[string]any{"status": 3, "source": request.Source, "sha256": hex.EncodeToString(digest[:])},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录彩票审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "开奖失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(issueID), "status": 3,
		"result_sha256": hex.EncodeToString(digest[:]),
	})
}

func (h *Handler) listLotteryOrders(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		status, _ = strconv.Atoi(rawStatus)
	}
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		userID, userID,
		status, status,
		keyword, like, like, like,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM lottery_bet_orders order_row
		JOIN lottery_games game ON game.id=order_row.game_id
		JOIN lottery_issues issue ON issue.id=order_row.issue_id
		WHERE (?=0 OR order_row.user_id=?)
		  AND (? < 0 OR order_row.status=?)
		  AND (?='' OR order_row.order_no LIKE ? OR game.game_code LIKE ?
		       OR issue.issue_no LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票订单失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT order_row.id,order_row.order_no,order_row.user_id,game.game_code,
		       issue.issue_no,order_row.total_bet,order_row.total_payout,
		       order_row.status,order_row.settled_at,order_row.created_at
		FROM lottery_bet_orders order_row
		JOIN lottery_games game ON game.id=order_row.game_id
		JOIN lottery_issues issue ON issue.id=order_row.issue_id
		WHERE (?=0 OR order_row.user_id=?)
		  AND (? < 0 OR order_row.status=?)
		  AND (?='' OR order_row.order_no LIKE ? OR game.game_code LIKE ?
		       OR issue.issue_no LIKE ?)
		ORDER BY order_row.created_at DESC,order_row.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票订单失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, rowUserID, totalBet, totalPayout int64
		var orderNo, gameCode, issueNo string
		var status int
		var settledAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &rowUserID, &gameCode, &issueNo, &totalBet, &totalPayout,
			&status, &settledAt, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票订单失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(rowUserID), "game_code": gameCode,
			"issue_no": issueNo, "total_bet": totalBet, "total_payout": totalPayout,
			"status": status, "settled_at": nullTime(settledAt), "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票订单失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}
