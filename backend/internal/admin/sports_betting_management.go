package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

var sportsMatchStatuses = map[string]bool{
	"NS": true, "LIVE": true, "HT": true, "FT": true, "CANCELLED": true,
}

func (h *Handler) sportsSyncStatus(w http.ResponseWriter, r *http.Request) {
	var futureMatches, activeMarkets, activeOptions int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT
			(SELECT COUNT(*) FROM sports_matches
			 WHERE source='api-football' AND kickoff_at>=CURRENT_TIMESTAMP(3)-INTERVAL 6 HOUR
			   AND EXISTS (
				   SELECT 1
				   FROM sports_markets visible_market
				   JOIN sports_market_options visible_option
				     ON visible_option.market_id=visible_market.id
				    AND visible_option.status=1
				    AND visible_option.odds_scaled>1000000
				   WHERE visible_market.match_id=sports_matches.id
				     AND visible_market.status=1
			   )),
			(SELECT COUNT(*) FROM sports_markets market
			 JOIN sports_matches match_row ON match_row.id=market.match_id
			 WHERE match_row.source='api-football' AND market.status=1
			   AND match_row.kickoff_at>=CURRENT_TIMESTAMP(3)-INTERVAL 6 HOUR),
			(SELECT COUNT(*) FROM sports_market_options option_item
			 JOIN sports_markets market ON market.id=option_item.market_id
			 JOIN sports_matches match_row ON match_row.id=market.match_id
			 WHERE match_row.source='api-football' AND market.status=1
			   AND option_item.status=1
			   AND match_row.kickoff_at>=CURRENT_TIMESTAMP(3)-INTERVAL 6 HOUR)`,
	).Scan(&futureMatches, &activeMarkets, &activeOptions); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育同步状态失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id,sync_type,source,status,received_count,changed_count,
		       error_message,created_at
		FROM sports_sync_logs
		ORDER BY created_at DESC,id DESC LIMIT 20`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育同步状态失败")
		return
	}
	defer rows.Close()
	logs := make([]map[string]any, 0, 20)
	for rows.Next() {
		var id int64
		var syncType, source, errorMessage string
		var status, received, changed int
		var createdAt time.Time
		if err = rows.Scan(
			&id, &syncType, &source, &status, &received, &changed,
			&errorMessage, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育同步状态失败")
			return
		}
		logs = append(logs, map[string]any{
			"id": apiDecimalID(id), "sync_type": syncType, "source": source, "status": status,
			"received_count": received, "changed_count": changed,
			"error_message": errorMessage, "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育同步状态失败")
		return
	}
	state := "未配置或尚未同步"
	if len(logs) > 0 {
		state = "同步正常"
		if logs[0]["status"] != 1 {
			state = "同步异常"
		}
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"state": state, "future_matches": futureMatches,
		"active_markets": activeMarkets, "active_options": activeOptions,
		"logs": logs,
	})
}

func (h *Handler) listSportsMatches(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if status != "" && !sportsMatchStatuses[status] {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "赛事状态筛选无效")
		return
	}
	likeQuery := "%" + escapeLike(query) + "%"
	filterArguments := []any{
		status, status,
		query, likeQuery, likeQuery, likeQuery, likeQuery, likeQuery,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM sports_matches match_row
		WHERE (?='' OR match_row.match_status=?)
		  AND (?='' OR CAST(match_row.id AS CHAR) LIKE ?
		       OR match_row.competition LIKE ? OR match_row.home_name LIKE ?
		       OR match_row.away_name LIKE ? OR match_row.public_match_id LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育赛事失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT match_row.id,match_row.public_match_id,match_row.source,
		       match_row.source_match_id,match_row.competition,match_row.competition_type,
		       match_row.home_name,match_row.away_name,match_row.kickoff_at,
		       match_row.bet_close_at,match_row.home_score,match_row.away_score,
		       match_row.match_status,match_row.bet_status,match_row.settle_status,
		       match_row.min_bet,match_row.max_bet,
		       (SELECT COUNT(*) FROM sports_markets market WHERE market.match_id=match_row.id),
		       (SELECT COUNT(*) FROM sports_market_options market_option
		        JOIN sports_markets market ON market.id=market_option.market_id
		        WHERE market.match_id=match_row.id),
		       (SELECT COUNT(*) FROM sports_bet_orders bet_order WHERE bet_order.match_id=match_row.id),
		       (SELECT COALESCE(SUM(bet_order.total_bet),0)
		        FROM sports_bet_orders bet_order WHERE bet_order.match_id=match_row.id),
		       (SELECT COALESCE(SUM(bet_order.total_payout),0)
		        FROM sports_bet_orders bet_order WHERE bet_order.match_id=match_row.id)
		FROM sports_matches match_row
		WHERE (?='' OR match_row.match_status=?)
		  AND (?='' OR CAST(match_row.id AS CHAR) LIKE ?
		       OR match_row.competition LIKE ? OR match_row.home_name LIKE ?
		       OR match_row.away_name LIKE ? OR match_row.public_match_id LIKE ?)
		ORDER BY match_row.kickoff_at DESC,match_row.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育赛事失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, minBet, maxBet, marketCount, optionCount, orderCount, totalBet, totalPayout int64
		var publicID, source, sourceMatchID, competition, competitionType, homeName, awayName, matchStatus string
		var kickoffAt, betCloseAt time.Time
		var homeScore, awayScore, betStatus, settleStatus int
		if err = rows.Scan(
			&id, &publicID, &source, &sourceMatchID, &competition, &competitionType,
			&homeName, &awayName, &kickoffAt, &betCloseAt, &homeScore, &awayScore,
			&matchStatus, &betStatus, &settleStatus, &minBet, &maxBet,
			&marketCount, &optionCount, &orderCount, &totalBet, &totalPayout,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育赛事失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "public_match_id": publicID, "source": source,
			"source_match_id": sourceMatchID, "competition": competition,
			"competition_type": competitionType, "home_name": homeName, "away_name": awayName,
			"kickoff_at": kickoffAt.Unix(), "bet_close_at": betCloseAt.Unix(),
			"home_score": homeScore, "away_score": awayScore, "match_status": matchStatus,
			"bet_status": betStatus, "settle_status": settleStatus,
			"min_bet": minBet, "max_bet": maxBet, "market_count": marketCount,
			"option_count": optionCount, "order_count": orderCount,
			"total_bet": totalBet, "total_payout": totalPayout,
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育赛事失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
	})
}

func (h *Handler) createSportsMatch(w http.ResponseWriter, r *http.Request) {
	var request sportsMatchRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := validateSportsMatchRequest(&request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, err.Error())
		return
	}
	publicID, err := idgen.New()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成赛事编号失败")
		return
	}
	sourceMatchID := "admin:" + strings.ToLower(publicID)
	result, err := h.db.ExecContext(r.Context(), `
		INSERT INTO sports_matches
			(public_match_id,source,source_match_id,competition,competition_type,
			 home_name,away_name,kickoff_at,bet_close_at,home_score,away_score,
			 match_status,bet_status,settle_status,min_bet,max_bet,source_updated_at)
		VALUES(?,'manual_admin',?,?,?,?,?,FROM_UNIXTIME(?),FROM_UNIXTIME(?),?,?,?, ?,0,?,?,CURRENT_TIMESTAMP(3))`,
		publicID, sourceMatchID, request.Competition, request.CompetitionType,
		request.HomeName, request.AwayName, request.KickoffAt, request.BetCloseAt,
		request.HomeScore, request.AwayScore, request.MatchStatus, request.BetStatus,
		request.MinBet, request.MaxBet,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "创建赛事失败或赛事已存在")
		return
	}
	id, _ := result.LastInsertId()
	if err = auditAdmin(r.Context(), h.db, r, "sports.match.create", "sports_match", id, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录体育审计失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(id), "public_match_id": publicID,
	})
}

func (h *Handler) updateSportsMatch(w http.ResponseWriter, r *http.Request) {
	matchID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "赛事编号无效")
		return
	}
	var request sportsMatchRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err = validateSportsMatchRequest(&request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, err.Error())
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新赛事失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var beforeStatus string
	var beforeBetStatus, beforeHomeScore, beforeAwayScore, settleStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT match_status,bet_status,home_score,away_score,settle_status
		FROM sports_matches WHERE id=? FOR UPDATE`,
		matchID,
	).Scan(
		&beforeStatus, &beforeBetStatus, &beforeHomeScore, &beforeAwayScore, &settleStatus,
	); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "赛事不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新赛事失败")
		return
	}
	if settleStatus != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "赛事已提交结算，不可再修改")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE sports_matches
		SET competition=?,competition_type=?,home_name=?,away_name=?,
		    kickoff_at=FROM_UNIXTIME(?),bet_close_at=FROM_UNIXTIME(?),
		    home_score=?,away_score=?,match_status=?,bet_status=?,min_bet=?,max_bet=?,
		    source_updated_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		request.Competition, request.CompetitionType, request.HomeName, request.AwayName,
		request.KickoffAt, request.BetCloseAt, request.HomeScore, request.AwayScore,
		request.MatchStatus, request.BetStatus, request.MinBet, request.MaxBet, matchID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新赛事失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "sports.match.update", "sports_match", matchID,
		map[string]any{
			"match_status": beforeStatus, "bet_status": beforeBetStatus,
			"home_score": beforeHomeScore, "away_score": beforeAwayScore,
		},
		request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录体育审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新赛事失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(matchID), "updated": true,
	})
}

func (h *Handler) listSportsMarkets(w http.ResponseWriter, r *http.Request) {
	matchID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "赛事编号无效")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT market.id,market.market_code,market.name,market.settlement_rule,
		       market.status,market.sort_order,market_option.id,market_option.option_code,
		       market_option.name,market_option.odds_scaled,market_option.result,
		       market_option.status
		FROM sports_markets market
		LEFT JOIN sports_market_options market_option ON market_option.market_id=market.id
		WHERE market.match_id=?
		ORDER BY market.sort_order DESC,market.id,market_option.id`,
		matchID,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育盘口失败")
		return
	}
	defer rows.Close()
	markets := make([]map[string]any, 0, 8)
	byID := make(map[int64]map[string]any)
	for rows.Next() {
		var marketID int64
		var marketCode, marketName, settlementRule string
		var marketStatus, sortOrder int
		var optionID sql.NullInt64
		var optionCode, optionName sql.NullString
		var oddsScaled sql.NullInt64
		var optionResult, optionStatus sql.NullInt64
		if err = rows.Scan(
			&marketID, &marketCode, &marketName, &settlementRule, &marketStatus, &sortOrder,
			&optionID, &optionCode, &optionName, &oddsScaled, &optionResult, &optionStatus,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育盘口失败")
			return
		}
		market, found := byID[marketID]
		if !found {
			market = map[string]any{
				"id": apiDecimalID(marketID), "market_code": marketCode, "name": marketName,
				"settlement_rule": settlementRule, "status": marketStatus,
				"sort_order": sortOrder, "options": []map[string]any{},
			}
			byID[marketID] = market
			markets = append(markets, market)
		}
		if optionID.Valid {
			options := market["options"].([]map[string]any)
			market["options"] = append(options, map[string]any{
				"id": apiDecimalID(optionID.Int64), "option_code": optionCode.String,
				"name":        optionName.String,
				"odds_scaled": oddsScaled.Int64, "result": optionResult.Int64,
				"status": optionStatus.Int64,
			})
		}
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育盘口失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"match_id": apiDecimalID(matchID), "items": markets,
	})
}

func (h *Handler) createSportsMarket(w http.ResponseWriter, r *http.Request) {
	var request struct {
		MatchID        decimalIDInput `json:"match_id"`
		MarketCode     string         `json:"market_code"`
		Name           string         `json:"name"`
		SettlementRule string         `json:"settlement_rule"`
		Status         int            `json:"status"`
		SortOrder      int            `json:"sort_order"`
		Options        []struct {
			OptionCode string `json:"option_code"`
			Name       string `json:"name"`
			OddsScaled int64  `json:"odds_scaled"`
		} `json:"options"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.MarketCode = strings.ToLower(strings.TrimSpace(request.MarketCode))
	request.Name = strings.TrimSpace(request.Name)
	request.SettlementRule = strings.TrimSpace(request.SettlementRule)
	if request.MatchID < 1 || !catalogKeyPattern.MatchString(request.MarketCode) ||
		request.Name == "" || len(request.Name) > 120 ||
		request.SettlementRule == "" || len(request.SettlementRule) > 80 ||
		request.Status < 0 || request.Status > 1 || len(request.Options) < 2 || len(request.Options) > 20 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "体育盘口参数无效")
		return
	}
	for index := range request.Options {
		request.Options[index].OptionCode = strings.ToLower(strings.TrimSpace(request.Options[index].OptionCode))
		request.Options[index].Name = strings.TrimSpace(request.Options[index].Name)
		if !catalogKeyPattern.MatchString(request.Options[index].OptionCode) ||
			request.Options[index].Name == "" || len(request.Options[index].Name) > 120 ||
			request.Options[index].OddsScaled < 1 || request.Options[index].OddsScaled > 1_000_000_000_000 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "体育盘口选项无效")
			return
		}
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建体育盘口失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var settleStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT settle_status FROM sports_matches WHERE id=? FOR UPDATE`,
		request.MatchID.Int64(),
	).Scan(&settleStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "赛事不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建体育盘口失败")
		return
	}
	if settleStatus != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "赛事已提交结算，不可新增盘口")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO sports_markets(match_id,market_code,name,settlement_rule,status,sort_order)
		VALUES(?,?,?,?,?,?)`,
		request.MatchID.Int64(), request.MarketCode, request.Name, request.SettlementRule,
		request.Status, request.SortOrder,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "体育盘口已存在")
		return
	}
	marketID, _ := result.LastInsertId()
	for _, marketOption := range request.Options {
		if _, err = tx.ExecContext(r.Context(), `
			INSERT INTO sports_market_options
				(market_id,option_code,name,odds_scaled,result,status)
			VALUES(?,?,?,?,0,1)`,
			marketID, marketOption.OptionCode, marketOption.Name, marketOption.OddsScaled,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "盘口选项标识重复")
			return
		}
	}
	if err = auditAdmin(r.Context(), tx, r, "sports.market.create", "sports_market", marketID, nil, request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录体育审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建体育盘口失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(marketID), "match_id": apiDecimalID(request.MatchID.Int64()),
	})
}

func (h *Handler) updateSportsOption(w http.ResponseWriter, r *http.Request) {
	optionID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "盘口选项编号无效")
		return
	}
	var request struct {
		OddsScaled int64 `json:"odds_scaled"`
		Result     int   `json:"result"`
		Status     int   `json:"status"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.OddsScaled < 1 || request.OddsScaled > 1_000_000_000_000 ||
		request.Result < 0 || request.Result > 2 || request.Status < 0 || request.Status > 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "盘口选项参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新盘口选项失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var marketID, matchID int64
	if err = tx.QueryRowContext(r.Context(), `
		SELECT market_option.market_id,market.match_id
		FROM sports_market_options market_option
		JOIN sports_markets market ON market.id=market_option.market_id
		WHERE market_option.id=?`,
		optionID,
	).Scan(&marketID, &matchID); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "盘口选项不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新盘口选项失败")
		return
	}
	var settleStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT settle_status FROM sports_matches WHERE id=? FOR UPDATE`,
		matchID,
	).Scan(&settleStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "体育赛事不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新盘口选项失败")
		return
	}
	if settleStatus != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "赛事已提交结算，盘口选项不可再修改")
		return
	}
	var lockedMarketID int64
	if err = tx.QueryRowContext(r.Context(), `
		SELECT id FROM sports_markets WHERE id=? AND match_id=? FOR UPDATE`,
		marketID, matchID,
	).Scan(&lockedMarketID); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "体育盘口不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新盘口选项失败")
		return
	}
	var beforeOdds int64
	var beforeResult, beforeStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT odds_scaled,result,status
		FROM sports_market_options
		WHERE id=? AND market_id=? FOR UPDATE`,
		optionID, marketID,
	).Scan(&beforeOdds, &beforeResult, &beforeStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "盘口选项不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新盘口选项失败")
		return
	}
	if beforeStatus == 1 && request.Status == 0 {
		var unsettledBets int
		if err = tx.QueryRowContext(r.Context(), `
			SELECT COUNT(*)
			FROM sports_bet_items bet_item
			JOIN sports_bet_orders bet_order ON bet_order.id=bet_item.order_id
			WHERE bet_item.option_id=? AND bet_order.status=0`,
			optionID,
		).Scan(&unsettledBets); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "检查选项投注失败")
			return
		}
		if unsettledBets > 0 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "选项已有未结投注，不可停用")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE sports_market_options
		SET odds_scaled=?,result=?,status=?,source_updated_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		request.OddsScaled, request.Result, request.Status, optionID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新盘口选项失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "sports.option.update", "sports_option", optionID,
		map[string]any{"odds_scaled": beforeOdds, "result": beforeResult, "status": beforeStatus},
		request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录体育审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新盘口选项失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(optionID), "updated": true,
	})
}

func (h *Handler) markSportsSettlementReady(w http.ResponseWriter, r *http.Request) {
	matchID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "赛事编号无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "提交体育结算失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var matchStatus string
	var settleStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT match_status,settle_status FROM sports_matches WHERE id=? FOR UPDATE`,
		matchID,
	).Scan(&matchStatus, &settleStatus); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "赛事不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "提交体育结算失败")
		return
	}
	if settleStatus != 0 {
		message := "赛事已经提交结算"
		if settleStatus == 2 {
			message = "赛事已经结算"
		}
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, message)
		return
	}
	if matchStatus != "FT" && matchStatus != "CANCELLED" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "只有完赛或取消赛事可以提交结算")
		return
	}
	if matchStatus == "FT" {
		var pendingOptions int
		if err = tx.QueryRowContext(r.Context(), `
			SELECT COUNT(DISTINCT market_option.id)
			FROM sports_bet_orders bet_order
			JOIN sports_bet_items bet_item ON bet_item.order_id=bet_order.id
			JOIN sports_market_options market_option ON market_option.id=bet_item.option_id
			WHERE bet_order.match_id=? AND bet_order.status=0 AND market_option.result=0`,
			matchID,
		).Scan(&pendingOptions); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "检查盘口结果失败")
			return
		}
		if pendingOptions > 0 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "仍有盘口未录入输赢结果")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE sports_matches SET bet_status=0,settle_status=1 WHERE id=?`,
		matchID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "提交体育结算失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "sports.match.settle_ready", "sports_match", matchID,
		map[string]any{"settle_status": settleStatus},
		map[string]any{"settle_status": 1, "match_status": matchStatus},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录体育审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "提交体育结算失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(matchID), "settle_status": 1, "queued": true,
	})
}

func (h *Handler) bettingDashboard(w http.ResponseWriter, r *http.Request) {
	userID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("user_id")), 10, 64)
	_, pageSize := pageParams(r)
	summary := map[string]any{}
	var (
		summaryLotteryOrders, summaryLotteryBet, summaryLotteryPayout int64
		summarySportsOrders, summarySportsBet, summarySportsPayout    int64
		summaryGameOrders, summaryGameBet, summaryGamePayout          int64
	)
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT
			(SELECT COUNT(*) FROM lottery_bet_orders WHERE (?=0 OR user_id=?)),
			(SELECT COALESCE(SUM(total_bet),0) FROM lottery_bet_orders WHERE (?=0 OR user_id=?)),
			(SELECT COALESCE(SUM(total_payout),0) FROM lottery_bet_orders WHERE (?=0 OR user_id=?)),
			(SELECT COUNT(*) FROM sports_bet_orders WHERE (?=0 OR user_id=?)),
			(SELECT COALESCE(SUM(total_bet),0) FROM sports_bet_orders WHERE (?=0 OR user_id=?)),
			(SELECT COALESCE(SUM(total_payout),0) FROM sports_bet_orders WHERE (?=0 OR user_id=?)),
			(SELECT COUNT(*) FROM game_settlement_items WHERE (?=0 OR user_id=?)),
			(SELECT COALESCE(SUM(bet_amount),0) FROM game_settlement_items WHERE (?=0 OR user_id=?)),
			(SELECT COALESCE(SUM(payout_amount),0) FROM game_settlement_items WHERE (?=0 OR user_id=?))`,
		userID, userID, userID, userID, userID, userID,
		userID, userID, userID, userID, userID, userID,
		userID, userID, userID, userID, userID, userID,
	).Scan(
		&summaryLotteryOrders, &summaryLotteryBet, &summaryLotteryPayout,
		&summarySportsOrders, &summarySportsBet, &summarySportsPayout,
		&summaryGameOrders, &summaryGameBet, &summaryGamePayout,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取投注汇总失败")
		return
	}
	summary["lottery"] = betSummary(summaryLotteryOrders, summaryLotteryBet, summaryLotteryPayout)
	summary["sports"] = betSummary(summarySportsOrders, summarySportsBet, summarySportsPayout)
	summary["games"] = betSummary(summaryGameOrders, summaryGameBet, summaryGamePayout)

	lotteryOrders, err := h.bettingLotteryOrders(r, userID, pageSize)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取彩票投注失败")
		return
	}
	sportsOrders, err := h.bettingSportsOrders(r, userID, pageSize)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取体育投注失败")
		return
	}
	gameOrders, err := h.bettingGameOrders(r, userID, pageSize)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏投注失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"user_id": apiDecimalID(userID), "summary": summary,
		"lottery_orders": lotteryOrders, "sports_orders": sportsOrders, "game_orders": gameOrders,
	})
}

func betSummary(orders, bet, payout int64) map[string]int64 {
	return map[string]int64{
		"orders": orders, "total_bet": bet, "total_payout": payout, "net": payout - bet,
	}
}

func (h *Handler) bettingLotteryOrders(r *http.Request, userID int64, limit int) ([]map[string]any, error) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT bet_order.id,bet_order.order_no,bet_order.user_id,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username,''),
		       game.name,issue.issue_no,bet_order.total_bet,bet_order.total_payout,
		       bet_order.status,bet_order.created_at
		FROM lottery_bet_orders bet_order
		LEFT JOIN users app_user ON app_user.id=bet_order.user_id
		JOIN lottery_games game ON game.id=bet_order.game_id
		JOIN lottery_issues issue ON issue.id=bet_order.issue_id
		WHERE (?=0 OR bet_order.user_id=?)
		ORDER BY bet_order.created_at DESC,bet_order.id DESC LIMIT ?`,
		userID, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, rowUserID, totalBet, totalPayout int64
		var orderNo, nickname, gameName, issueNo string
		var status int
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &rowUserID, &nickname, &gameName, &issueNo,
			&totalBet, &totalPayout, &status, &createdAt,
		); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(rowUserID), "nickname": nickname,
			"event": gameName + " · " + issueNo, "total_bet": totalBet,
			"total_payout": totalPayout, "status": status, "created_at": createdAt.Unix(),
		})
	}
	return items, rows.Err()
}

func (h *Handler) bettingSportsOrders(r *http.Request, userID int64, limit int) ([]map[string]any, error) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT bet_order.id,bet_order.order_no,bet_order.user_id,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username,''),
		       match_row.competition,match_row.home_name,match_row.away_name,
		       bet_order.total_bet,bet_order.total_payout,bet_order.status,bet_order.created_at
		FROM sports_bet_orders bet_order
		LEFT JOIN users app_user ON app_user.id=bet_order.user_id
		JOIN sports_matches match_row ON match_row.id=bet_order.match_id
		WHERE (?=0 OR bet_order.user_id=?)
		ORDER BY bet_order.created_at DESC,bet_order.id DESC LIMIT ?`,
		userID, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, rowUserID, totalBet, totalPayout int64
		var orderNo, nickname, competition, homeName, awayName string
		var status int
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &rowUserID, &nickname, &competition, &homeName, &awayName,
			&totalBet, &totalPayout, &status, &createdAt,
		); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(rowUserID), "nickname": nickname,
			"event":     competition + " · " + homeName + " VS " + awayName,
			"total_bet": totalBet, "total_payout": totalPayout,
			"status": status, "created_at": createdAt.Unix(),
		})
	}
	return items, rows.Err()
}

func (h *Handler) bettingGameOrders(r *http.Request, userID int64, limit int) ([]map[string]any, error) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT settlement.id,settlement.settlement_no,item.user_id,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username,''),
		       game.name,venue.name,settlement.table_no,settlement.session_id,
		       item.bet_amount,item.payout_amount,settlement.status,settlement.created_at
		FROM game_settlement_items item
		JOIN game_settlements settlement ON settlement.id=item.settlement_id
		LEFT JOIN users app_user ON app_user.id=item.user_id
		JOIN games game ON game.id=settlement.game_id
		JOIN game_venues venue ON venue.id=settlement.venue_id
		WHERE (?=0 OR item.user_id=?)
		ORDER BY settlement.created_at DESC,settlement.id DESC LIMIT ?`,
		userID, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, limit)
	for rows.Next() {
		var id, rowUserID, totalBet, totalPayout int64
		var orderNo, nickname, gameName, venueName string
		var tableNo, status int
		var sessionID sql.NullString
		var createdAt time.Time
		if err = rows.Scan(
			&id, &orderNo, &rowUserID, &nickname, &gameName, &venueName,
			&tableNo, &sessionID, &totalBet, &totalPayout, &status, &createdAt,
		); err != nil {
			return nil, err
		}
		event := gameName + " · " + venueName + " · 桌 " + strconv.Itoa(tableNo)
		if sessionID.Valid {
			event += " · " + sessionID.String
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "order_no": orderNo,
			"user_id": apiDecimalID(rowUserID), "nickname": nickname,
			"event": event, "total_bet": totalBet, "total_payout": totalPayout,
			"status": status, "created_at": createdAt.Unix(),
		})
	}
	return items, rows.Err()
}

type sportsMatchRequest struct {
	Competition     string `json:"competition"`
	CompetitionType string `json:"competition_type"`
	HomeName        string `json:"home_name"`
	AwayName        string `json:"away_name"`
	KickoffAt       int64  `json:"kickoff_at"`
	BetCloseAt      int64  `json:"bet_close_at"`
	HomeScore       int    `json:"home_score"`
	AwayScore       int    `json:"away_score"`
	MatchStatus     string `json:"match_status"`
	BetStatus       int    `json:"bet_status"`
	MinBet          int64  `json:"min_bet"`
	MaxBet          int64  `json:"max_bet"`
}

func validateSportsMatchRequest(request *sportsMatchRequest) error {
	request.Competition = strings.TrimSpace(request.Competition)
	request.CompetitionType = strings.ToLower(strings.TrimSpace(request.CompetitionType))
	request.HomeName = strings.TrimSpace(request.HomeName)
	request.AwayName = strings.TrimSpace(request.AwayName)
	request.MatchStatus = strings.ToUpper(strings.TrimSpace(request.MatchStatus))
	if request.Competition == "" || len(request.Competition) > 190 ||
		request.CompetitionType == "" || len(request.CompetitionType) > 60 ||
		request.HomeName == "" || len(request.HomeName) > 190 ||
		request.AwayName == "" || len(request.AwayName) > 190 ||
		request.KickoffAt < 1 || request.BetCloseAt < 1 || request.BetCloseAt > request.KickoffAt ||
		request.HomeScore < 0 || request.HomeScore > 999 ||
		request.AwayScore < 0 || request.AwayScore > 999 ||
		!sportsMatchStatuses[request.MatchStatus] ||
		request.BetStatus < 0 || request.BetStatus > 1 ||
		request.MinBet < 1 || request.MaxBet < request.MinBet {
		return errors.New("体育赛事参数无效")
	}
	if request.MatchStatus == "FT" || request.MatchStatus == "CANCELLED" {
		request.BetStatus = 0
	}
	return nil
}

func positivePathID(r *http.Request) (int64, error) {
	value, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || value < 1 {
		return 0, errors.New("invalid path id")
	}
	return value, nil
}
