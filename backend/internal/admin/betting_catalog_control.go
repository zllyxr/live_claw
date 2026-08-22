package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

func parseControlIntFilter(r *http.Request, key string, minimum, maximum int64) (int64, bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < minimum || value > maximum {
		return 0, false, errors.New("invalid integer filter")
	}
	return value, true, nil
}

func lotteryIssueWhereClause(
	gameID int64,
	hasGameID bool,
	status int64,
	hasStatus bool,
	keyword string,
) (string, []any) {
	conditions := make([]string, 0, 3)
	arguments := make([]any, 0, 6)
	if hasGameID {
		conditions = append(conditions, "issue.game_id=?")
		arguments = append(arguments, gameID)
	}
	if hasStatus {
		conditions = append(conditions, "issue.status=?")
		arguments = append(arguments, status)
	}
	if keyword != "" {
		like := "%" + escapeLike(keyword) + "%"
		conditions = append(conditions, `(issue.issue_no LIKE ? OR game.game_code LIKE ?
			OR game.name LIKE ? OR issue.result_source LIKE ?)`)
		arguments = append(arguments, like, like, like, like)
	}
	if len(conditions) == 0 {
		return "", arguments
	}
	return " WHERE " + strings.Join(conditions, " AND "), arguments
}

type lotteryIssueListRow struct {
	id           int64
	gameID       int64
	gameCode     string
	gameName     string
	issueNo      string
	saleOpenAt   time.Time
	saleCloseAt  time.Time
	drawAt       time.Time
	drawResult   []byte
	resultSource string
	status       int
	createdAt    time.Time
	updatedAt    time.Time
}

type lotteryIssueOrderStats struct {
	orderCount  int64
	totalBet    int64
	totalPayout int64
}

func (h *Handler) listLotteryIssues(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	gameID, hasGameID, err := parseControlIntFilter(r, "game_id", 1, int64(^uint64(0)>>1))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "彩票游戏筛选无效")
		return
	}
	status, hasStatus, err := parseControlIntFilter(r, "status", 0, 5)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "期号状态筛选无效")
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	whereClause, filterArguments := lotteryIssueWhereClause(
		gameID, hasGameID, status, hasStatus, keyword,
	)
	countJoin := ""
	if keyword != "" {
		countJoin = " JOIN lottery_games game ON game.id=issue.game_id"
	}
	var total int64
	if err = h.db.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM lottery_issues issue"+countJoin+whereClause,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
		return
	}
	listQuery := `
		SELECT issue.id,issue.game_id,game.game_code,game.name,issue.issue_no,
		       issue.sale_open_at,issue.sale_close_at,issue.draw_at,issue.draw_result,
		       issue.result_source,issue.status,issue.created_at,issue.updated_at
		FROM lottery_issues issue
		JOIN lottery_games game ON game.id=issue.game_id` + whereClause + `
		ORDER BY issue.draw_at DESC,issue.id DESC
		LIMIT ? OFFSET ?`
	listArguments := append(append([]any{}, filterArguments...), pageSize, (page-1)*pageSize)
	rows, err := h.db.QueryContext(r.Context(),
		listQuery,
		listArguments...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
		return
	}
	issueRows := make([]lotteryIssueListRow, 0, pageSize)
	for rows.Next() {
		var row lotteryIssueListRow
		if err = rows.Scan(
			&row.id, &row.gameID, &row.gameCode, &row.gameName, &row.issueNo,
			&row.saleOpenAt, &row.saleCloseAt, &row.drawAt, &row.drawResult,
			&row.resultSource, &row.status, &row.createdAt, &row.updatedAt,
		); err != nil {
			_ = rows.Close()
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
			return
		}
		issueRows = append(issueRows, row)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
		return
	}
	if err = rows.Close(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
		return
	}

	statsByIssueID := make(map[int64]lotteryIssueOrderStats, len(issueRows))
	if len(issueRows) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(issueRows)), ",")
		statsArguments := make([]any, 0, len(issueRows))
		for _, row := range issueRows {
			statsArguments = append(statsArguments, row.id)
		}
		statsRows, statsErr := h.db.QueryContext(r.Context(), `
			SELECT issue_id,COUNT(*),COALESCE(SUM(total_bet),0),COALESCE(SUM(total_payout),0)
			FROM lottery_bet_orders
			WHERE issue_id IN (`+placeholders+`)
			GROUP BY issue_id`,
			statsArguments...,
		)
		if statsErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
			return
		}
		for statsRows.Next() {
			var issueID int64
			var stats lotteryIssueOrderStats
			if statsErr = statsRows.Scan(
				&issueID, &stats.orderCount, &stats.totalBet, &stats.totalPayout,
			); statsErr != nil {
				_ = statsRows.Close()
				httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
				return
			}
			statsByIssueID[issueID] = stats
		}
		if statsErr = statsRows.Err(); statsErr != nil {
			_ = statsRows.Close()
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
			return
		}
		if statsErr = statsRows.Close(); statsErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票期号失败")
			return
		}
	}

	items := make([]map[string]any, 0, len(issueRows))
	for _, row := range issueRows {
		stats := statsByIssueID[row.id]
		items = append(items, map[string]any{
			"id": apiDecimalID(row.id), "game_id": apiDecimalID(row.gameID),
			"game_code": row.gameCode, "game_name": row.gameName,
			"issue_no": row.issueNo, "sale_open_at": row.saleOpenAt.Unix(),
			"sale_close_at": row.saleCloseAt.Unix(), "draw_at": row.drawAt.Unix(),
			"draw_result": jsonOrNil(row.drawResult), "result_source": row.resultSource, "status": row.status,
			"order_count": stats.orderCount, "total_bet": stats.totalBet, "total_payout": stats.totalPayout,
			"created_at": row.createdAt.Unix(), "updated_at": row.updatedAt.Unix(),
		})
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) listBetLotteryOrders(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	userID, hasUserID, status, hasStatus, keyword, ok := betOrderFilters(w, r, 4)
	if !ok {
		return
	}
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		hasUserID, userID, hasStatus, status, keyword,
		like, like, like, like, like, like, like,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM lottery_bet_orders bet_order
		LEFT JOIN users app_user ON app_user.id=bet_order.user_id
		JOIN lottery_games game ON game.id=bet_order.game_id
		JOIN lottery_issues issue ON issue.id=bet_order.issue_id
		WHERE (?=FALSE OR bet_order.user_id=?)
		  AND (?=FALSE OR bet_order.status=?)
		  AND (?='' OR bet_order.order_no LIKE ? OR game.game_code LIKE ?
		       OR game.name LIKE ? OR issue.issue_no LIKE ? OR app_user.nickname LIKE ?
		       OR app_user.username LIKE ? OR CAST(bet_order.user_id AS CHAR) LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票投注失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT bet_order.id,bet_order.order_no,bet_order.user_id,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username,''),
		       game.game_code,game.name,issue.issue_no,bet_order.total_bet,
		       bet_order.total_payout,bet_order.status,bet_order.settled_at,
		       bet_order.created_at
		FROM lottery_bet_orders bet_order
		LEFT JOIN users app_user ON app_user.id=bet_order.user_id
		JOIN lottery_games game ON game.id=bet_order.game_id
		JOIN lottery_issues issue ON issue.id=bet_order.issue_id
		WHERE (?=FALSE OR bet_order.user_id=?)
		  AND (?=FALSE OR bet_order.status=?)
		  AND (?='' OR bet_order.order_no LIKE ? OR game.game_code LIKE ?
		       OR game.name LIKE ? OR issue.issue_no LIKE ? OR app_user.nickname LIKE ?
		       OR app_user.username LIKE ? OR CAST(bet_order.user_id AS CHAR) LIKE ?)
		ORDER BY bet_order.created_at DESC,bet_order.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票投注失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, rowUserID, totalBet, totalPayout int64
		var orderNo, nickname, gameCode, gameName, issueNo string
		var rowStatus int
		var settledAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &rowUserID, &nickname, &gameCode, &gameName, &issueNo,
			&totalBet, &totalPayout, &rowStatus, &settledAt, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票投注失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(rowUserID), "nickname": nickname,
			"event": gameName + " · " + issueNo, "game_code": gameCode, "issue_no": issueNo,
			"total_bet": totalBet, "total_payout": totalPayout, "status": rowStatus,
			"settled_at": nullTime(settledAt), "created_at": createdAt.Unix(),
		})
	}
	writeBetOrderList(w, r, page, pageSize, total, items, rows.Err(), "读取彩票投注失败")
}

func (h *Handler) listBetSportsOrders(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	userID, hasUserID, status, hasStatus, keyword, ok := betOrderFilters(w, r, 4)
	if !ok {
		return
	}
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		hasUserID, userID, hasStatus, status, keyword,
		like, like, like, like, like, like, like, like,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM sports_bet_orders bet_order
		LEFT JOIN users app_user ON app_user.id=bet_order.user_id
		JOIN sports_matches match_row ON match_row.id=bet_order.match_id
		WHERE (?=FALSE OR bet_order.user_id=?)
		  AND (?=FALSE OR bet_order.status=?)
		  AND (?='' OR bet_order.order_no LIKE ? OR match_row.public_match_id LIKE ?
		       OR match_row.competition LIKE ? OR match_row.home_name LIKE ?
		       OR match_row.away_name LIKE ? OR app_user.nickname LIKE ?
		       OR app_user.username LIKE ? OR CAST(bet_order.user_id AS CHAR) LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育投注失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT bet_order.id,bet_order.order_no,bet_order.user_id,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username,''),
		       match_row.public_match_id,match_row.competition,match_row.home_name,
		       match_row.away_name,bet_order.total_bet,bet_order.total_payout,
		       bet_order.status,bet_order.settled_at,bet_order.created_at
		FROM sports_bet_orders bet_order
		LEFT JOIN users app_user ON app_user.id=bet_order.user_id
		JOIN sports_matches match_row ON match_row.id=bet_order.match_id
		WHERE (?=FALSE OR bet_order.user_id=?)
		  AND (?=FALSE OR bet_order.status=?)
		  AND (?='' OR bet_order.order_no LIKE ? OR match_row.public_match_id LIKE ?
		       OR match_row.competition LIKE ? OR match_row.home_name LIKE ?
		       OR match_row.away_name LIKE ? OR app_user.nickname LIKE ?
		       OR app_user.username LIKE ? OR CAST(bet_order.user_id AS CHAR) LIKE ?)
		ORDER BY bet_order.created_at DESC,bet_order.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育投注失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, rowUserID, totalBet, totalPayout int64
		var orderNo, nickname, publicMatchID, competition, homeName, awayName string
		var rowStatus int
		var settledAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &rowUserID, &nickname, &publicMatchID, &competition,
			&homeName, &awayName, &totalBet, &totalPayout, &rowStatus,
			&settledAt, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育投注失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(rowUserID), "nickname": nickname,
			"event":           competition + " · " + homeName + " VS " + awayName,
			"public_match_id": publicMatchID, "total_bet": totalBet,
			"total_payout": totalPayout, "status": rowStatus,
			"settled_at": nullTime(settledAt), "created_at": createdAt.Unix(),
		})
	}
	writeBetOrderList(w, r, page, pageSize, total, items, rows.Err(), "读取体育投注失败")
}

func (h *Handler) listBetGameOrders(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	userID, hasUserID, status, hasStatus, keyword, ok := betOrderFilters(w, r, 3)
	if !ok {
		return
	}
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		hasUserID, userID, hasStatus, status, keyword,
		like, like, like, like, like, like, like, like, like,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM game_settlement_items item
		JOIN game_settlements settlement ON settlement.id=item.settlement_id
		LEFT JOIN users app_user ON app_user.id=item.user_id
		JOIN games game ON game.id=settlement.game_id
		JOIN game_venues venue ON venue.id=settlement.venue_id
		WHERE (?=FALSE OR item.user_id=?)
		  AND (?=FALSE OR settlement.status=?)
		  AND (?='' OR settlement.settlement_no LIKE ? OR settlement.session_id LIKE ?
		       OR game.game_code LIKE ? OR game.name LIKE ? OR venue.venue_code LIKE ?
		       OR venue.name LIKE ? OR app_user.nickname LIKE ? OR app_user.username LIKE ?
		       OR CAST(item.user_id AS CHAR) LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏投注失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT settlement.id,settlement.settlement_no,item.user_id,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username,''),
		       game.game_code,game.name,venue.venue_code,venue.name,venue.multiplier,
		       settlement.table_no,settlement.session_id,item.bet_amount,item.payout_amount,
		       item.fee_amount,item.net_amount,settlement.status,settlement.applied_at,
		       settlement.created_at
		FROM game_settlement_items item
		JOIN game_settlements settlement ON settlement.id=item.settlement_id
		LEFT JOIN users app_user ON app_user.id=item.user_id
		JOIN games game ON game.id=settlement.game_id
		JOIN game_venues venue ON venue.id=settlement.venue_id
		WHERE (?=FALSE OR item.user_id=?)
		  AND (?=FALSE OR settlement.status=?)
		  AND (?='' OR settlement.settlement_no LIKE ? OR settlement.session_id LIKE ?
		       OR game.game_code LIKE ? OR game.name LIKE ? OR venue.venue_code LIKE ?
		       OR venue.name LIKE ? OR app_user.nickname LIKE ? OR app_user.username LIKE ?
		       OR CAST(item.user_id AS CHAR) LIKE ?)
		ORDER BY settlement.created_at DESC,settlement.id DESC,item.user_id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏投注失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, rowUserID, totalBet, totalPayout, feeAmount, netAmount int64
		var orderNo, nickname, gameCode, gameName, venueCode, venueName string
		var multiplier, tableNo, rowStatus int
		var sessionID sql.NullString
		var appliedAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &rowUserID, &nickname, &gameCode, &gameName,
			&venueCode, &venueName, &multiplier, &tableNo, &sessionID,
			&totalBet, &totalPayout, &feeAmount, &netAmount, &rowStatus,
			&appliedAt, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏投注失败")
			return
		}
		event := gameName + " · " + venueName + " · 桌 " + strconv.Itoa(tableNo)
		if sessionID.Valid {
			event += " · " + sessionID.String
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(rowUserID), "nickname": nickname,
			"event": event, "game_code": gameCode, "venue_code": venueCode,
			"multiplier": multiplier, "table_no": tableNo, "session_id": sessionID.String,
			"total_bet": totalBet, "total_payout": totalPayout, "fee_amount": feeAmount,
			"net_amount": netAmount, "status": rowStatus,
			"settled_at": nullTime(appliedAt), "created_at": createdAt.Unix(),
		})
	}
	writeBetOrderList(w, r, page, pageSize, total, items, rows.Err(), "读取游戏投注失败")
}

func betOrderFilters(
	w http.ResponseWriter,
	r *http.Request,
	maximumStatus int64,
) (int64, bool, int64, bool, string, bool) {
	userID, hasUserID, err := parseControlIntFilter(r, "user_id", 1, int64(^uint64(0)>>1))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "用户筛选无效")
		return 0, false, 0, false, "", false
	}
	status, hasStatus, err := parseControlIntFilter(r, "status", 0, maximumStatus)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "投注状态筛选无效")
		return 0, false, 0, false, "", false
	}
	return userID, hasUserID, status, hasStatus, strings.TrimSpace(r.URL.Query().Get("q")), true
}

func writeBetOrderList(
	w http.ResponseWriter,
	r *http.Request,
	page int,
	pageSize int,
	total int64,
	items []map[string]any,
	rowsErr error,
	errorMessage string,
) {
	if rowsErr != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, errorMessage)
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}
