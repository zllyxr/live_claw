package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

func (h *Handler) listGames(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT game.id,game.game_code,game.name,game.category,game.entry_path,
		       game.min_players,game.max_players,game.orientation,game.wallet_enabled,
		       game.status,game.sort_order,game.config,
		       COALESCE(active_sessions.count_value,0)
		FROM games game
		LEFT JOIN (
			SELECT game_id,COUNT(*) count_value FROM game_sessions
			WHERE status IN (1,2) GROUP BY game_id
		) active_sessions ON active_sessions.game_id=game.id
		ORDER BY game.sort_order DESC,game.id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, 16)
	for rows.Next() {
		var id, activeSessions int64
		var gameCode, name, category, entryPath, orientation string
		var minPlayers, maxPlayers, walletEnabled, status, sortOrder int
		var configJSON []byte
		if err = rows.Scan(
			&id, &gameCode, &name, &category, &entryPath,
			&minPlayers, &maxPlayers, &orientation, &walletEnabled,
			&status, &sortOrder, &configJSON, &activeSessions,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏失败")
			return
		}
		var config any
		_ = json.Unmarshal(configJSON, &config)
		items = append(items, map[string]any{
			"id": id, "game_code": gameCode, "name": name, "category": category,
			"entry_path": entryPath, "min_players": minPlayers, "max_players": maxPlayers,
			"orientation": orientation, "wallet_enabled": walletEnabled == 1,
			"status": status, "sort_order": sortOrder, "config": config,
			"active_sessions": activeSessions,
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏失败")
		return
	}

	venueRows, err := h.db.QueryContext(r.Context(), `
		SELECT venue.id,venue.game_id,venue.venue_code,venue.name,venue.multiplier,
		       venue.table_count,venue.seats_per_table,venue.min_balance,venue.escrow_amount,
		       venue.bet_levels,venue.target_rtp_ppm,venue.status,venue.sort_order,
		       COALESCE(active_sessions.count_value,0)
		FROM game_venues venue
		LEFT JOIN (
			SELECT venue_id,COUNT(*) count_value FROM game_sessions
			WHERE status IN (1,2) GROUP BY venue_id
		) active_sessions ON active_sessions.venue_id=venue.id
		ORDER BY venue.game_id,venue.sort_order DESC,venue.id`)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏场次失败")
		return
	}
	defer venueRows.Close()
	venues := make([]map[string]any, 0, 16)
	for venueRows.Next() {
		var id, gameID, minBalance, escrowAmount, activeSessions int64
		var venueCode, name string
		var multiplier, tableCount, seatsPerTable, targetRTP, status, sortOrder int
		var betLevelsJSON []byte
		if err = venueRows.Scan(
			&id, &gameID, &venueCode, &name, &multiplier, &tableCount, &seatsPerTable,
			&minBalance, &escrowAmount, &betLevelsJSON, &targetRTP, &status, &sortOrder,
			&activeSessions,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏场次失败")
			return
		}
		var betLevels any
		_ = json.Unmarshal(betLevelsJSON, &betLevels)
		venues = append(venues, map[string]any{
			"id": id, "game_id": gameID, "venue_code": venueCode, "name": name,
			"multiplier": multiplier, "table_count": tableCount, "seats_per_table": seatsPerTable,
			"min_balance": minBalance, "escrow_amount": escrowAmount, "bet_levels": betLevels,
			"target_rtp_ppm": targetRTP, "status": status, "sort_order": sortOrder,
			"active_sessions": activeSessions,
		})
	}
	if err = venueRows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏场次失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"items": items, "venues": venues})
}

func (h *Handler) updateGame(w http.ResponseWriter, r *http.Request) {
	gameID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || gameID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "游戏编号无效")
		return
	}
	var request struct {
		Name      string `json:"name"`
		Status    int    `json:"status"`
		SortOrder int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > 100 || request.Status < 0 || request.Status > 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "游戏参数无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新游戏失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var beforeName string
	var beforeStatus, beforeSort int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT name,status,sort_order FROM games WHERE id=? FOR UPDATE`,
		gameID,
	).Scan(&beforeName, &beforeStatus, &beforeSort); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "游戏不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新游戏失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE games SET name=?,status=?,sort_order=? WHERE id=?`,
		request.Name, request.Status, request.SortOrder, gameID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新游戏失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "game.update", "game", gameID,
		map[string]any{"name": beforeName, "status": beforeStatus, "sort_order": beforeSort},
		request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录游戏审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新游戏失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"id": gameID, "updated": true})
}

func (h *Handler) updateGameVenue(w http.ResponseWriter, r *http.Request) {
	venueID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || venueID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "场次编号无效")
		return
	}
	var request struct {
		Name         string  `json:"name"`
		MinBalance   int64   `json:"min_balance"`
		EscrowAmount int64   `json:"escrow_amount"`
		BetLevels    []int64 `json:"bet_levels"`
		TargetRTPPPM int     `json:"target_rtp_ppm"`
		Status       int     `json:"status"`
		SortOrder    int     `json:"sort_order"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > 80 || request.MinBalance < 0 ||
		request.EscrowAmount < 1 || request.TargetRTPPPM < 100000 ||
		request.TargetRTPPPM > 1000000 || request.Status < 0 || request.Status > 1 ||
		len(request.BetLevels) < 1 || len(request.BetLevels) > 20 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "场次参数无效")
		return
	}
	for _, level := range request.BetLevels {
		if level < 1 || level > 1000000 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "下注档位无效")
			return
		}
	}
	betLevelsJSON, _ := json.Marshal(request.BetLevels)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新游戏场次失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var venueCode string
	var multiplier, tableCount, seatsPerTable int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT venue_code,multiplier,table_count,seats_per_table
		FROM game_venues WHERE id=? FOR UPDATE`,
		venueID,
	).Scan(&venueCode, &multiplier, &tableCount, &seatsPerTable); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "游戏场次不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新游戏场次失败")
		return
	}
	expectedMultiplier := map[string]int{"novice": 1, "expert": 5, "master": 10}[venueCode]
	if expectedMultiplier == 0 || multiplier != expectedMultiplier || tableCount != 300 || seatsPerTable != 4 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "捕鱼场次结构不符合 300 桌、4 座和固定倍率约束")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE game_venues
		SET name=?,min_balance=?,escrow_amount=?,bet_levels=?,target_rtp_ppm=?,status=?,sort_order=?
		WHERE id=?`,
		request.Name, request.MinBalance, request.EscrowAmount, betLevelsJSON,
		request.TargetRTPPPM, request.Status, request.SortOrder, venueID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新游戏场次失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "game.venue.update", "game_venue", venueID,
		map[string]any{"venue_code": venueCode, "multiplier": multiplier}, request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录游戏审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新游戏场次失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": venueID, "venue_code": venueCode, "multiplier": multiplier,
		"table_count": 300, "seats_per_table": 4,
	})
}

func (h *Handler) listGameSettlements(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT settlement.id,settlement.settlement_no,settlement.session_id,
		       game.game_code,venue.venue_code,venue.multiplier,settlement.table_no,
		       item.user_id,item.bet_amount,item.payout_amount,item.fee_amount,item.net_amount,
		       settlement.status,settlement.created_at
		FROM game_settlements settlement
		JOIN games game ON game.id=settlement.game_id
		JOIN game_venues venue ON venue.id=settlement.venue_id
		JOIN game_settlement_items item ON item.settlement_id=settlement.id
		WHERE (?=0 OR item.user_id=?)
		ORDER BY settlement.created_at DESC,settlement.id DESC
		LIMIT ? OFFSET ?`,
		userID, userID, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏输赢记录失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, rowUserID, bet, payout, fee, net int64
		var settlementNo, gameCode, venueCode string
		var sessionID sql.NullString
		var multiplier, tableNo, status int
		var createdAt time.Time
		if err = rows.Scan(
			&id, &settlementNo, &sessionID, &gameCode, &venueCode, &multiplier, &tableNo,
			&rowUserID, &bet, &payout, &fee, &net, &status, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏输赢记录失败")
			return
		}
		items = append(items, map[string]any{
			"id": id, "settlement_no": settlementNo, "session_id": sessionID.String,
			"game_code": gameCode, "venue_code": venueCode, "multiplier": multiplier,
			"table_no": tableNo, "user_id": rowUserID, "bet_amount": bet,
			"payout_amount": payout, "fee_amount": fee, "net_amount": net,
			"status": status, "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取游戏输赢记录失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "items": items,
	})
}
