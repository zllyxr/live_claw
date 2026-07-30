package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

type sportsMarketUpdateRequest struct {
	MarketCode     string `json:"market_code"`
	Name           string `json:"name"`
	SettlementRule string `json:"settlement_rule"`
	Status         int    `json:"status"`
	SortOrder      int    `json:"sort_order"`
}

func (request *sportsMarketUpdateRequest) normalize() error {
	request.MarketCode = strings.ToLower(strings.TrimSpace(request.MarketCode))
	request.Name = strings.TrimSpace(request.Name)
	request.SettlementRule = strings.TrimSpace(request.SettlementRule)
	if !catalogKeyPattern.MatchString(request.MarketCode) ||
		request.Name == "" || len(request.Name) > 120 ||
		request.SettlementRule == "" || len(request.SettlementRule) > 80 ||
		request.Status < 0 || request.Status > 1 {
		return errors.New("体育盘口参数无效")
	}
	return nil
}

func (h *Handler) updateSportsMarket(w http.ResponseWriter, r *http.Request) {
	marketID, err := positivePathID(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "盘口编号无效")
		return
	}
	var request sportsMarketUpdateRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err = request.normalize(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, err.Error())
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新体育盘口失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var matchID int64
	if err = tx.QueryRowContext(r.Context(), `
		SELECT match_id FROM sports_markets WHERE id=?`,
		marketID,
	).Scan(&matchID); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "体育盘口不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新体育盘口失败")
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
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新体育盘口失败")
		return
	}
	if settleStatus != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "赛事已提交结算，盘口不可再修改")
		return
	}
	var before sportsMarketUpdateRequest
	if err = tx.QueryRowContext(r.Context(), `
		SELECT market_code,name,settlement_rule,status,sort_order
		FROM sports_markets WHERE id=? AND match_id=? FOR UPDATE`,
		marketID, matchID,
	).Scan(
		&before.MarketCode, &before.Name, &before.SettlementRule, &before.Status, &before.SortOrder,
	); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "体育盘口不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新体育盘口失败")
		return
	}
	if before.Status == 1 && request.Status == 0 {
		var unsettledBets int
		if err = tx.QueryRowContext(r.Context(), `
			SELECT COUNT(*)
			FROM sports_bet_items bet_item
			JOIN sports_bet_orders bet_order ON bet_order.id=bet_item.order_id
			WHERE bet_item.market_id=? AND bet_order.status=0`,
			marketID,
		).Scan(&unsettledBets); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "检查盘口投注失败")
			return
		}
		if unsettledBets > 0 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "盘口已有未结投注，不可停用")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE sports_markets
		SET market_code=?,name=?,settlement_rule=?,status=?,sort_order=?
		WHERE id=?`,
		request.MarketCode, request.Name, request.SettlementRule,
		request.Status, request.SortOrder, marketID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "盘口标识已存在或参数冲突")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "sports.market.update", "sports_market", marketID,
		map[string]any{
			"match_id": apiDecimalID(matchID), "market_code": before.MarketCode,
			"name":            before.Name,
			"settlement_rule": before.SettlementRule, "status": before.Status,
			"sort_order": before.SortOrder,
		},
		request,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录体育审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新体育盘口失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": apiDecimalID(marketID), "match_id": apiDecimalID(matchID),
		"updated": true, "status": request.Status,
	})
}
