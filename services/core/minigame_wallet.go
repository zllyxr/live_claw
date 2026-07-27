package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// MiniGameWalletService 是所有游戏资金变化的唯一入口。
// 游戏容器只能提交幂等订单，不能直接连接数据库或自行维护一份余额。
type MiniGameWalletService struct {
	db     *sql.DB
	secret string
	logger *slog.Logger
}

func NewMiniGameWalletService(db *sql.DB, secret string, logger *slog.Logger) *MiniGameWalletService {
	return &MiniGameWalletService{db: db, secret: secret, logger: logger}
}

func (s *MiniGameWalletService) EnsureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cmf_minigame_wallet_order (
		id bigint unsigned NOT NULL AUTO_INCREMENT,
		order_no varchar(80) NOT NULL,
		uid bigint unsigned NOT NULL,
		game_code varchar(60) NOT NULL,
		table_no smallint unsigned NOT NULL,
		round_no varchar(80) NOT NULL DEFAULT '',
		reason varchar(80) NOT NULL DEFAULT '',
		amount bigint NOT NULL,
		balance_before bigint NOT NULL,
		balance_after bigint NOT NULL,
		create_time int unsigned NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY uk_order_no (order_no),
		KEY idx_uid_time (uid,create_time),
		KEY idx_game_round (game_code,round_no)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='小游戏平台钱包幂等流水'`)
	return err
}

type walletRequest struct {
	OrderNo  string `json:"order_no"`
	UID      int64  `json:"uid"`
	GameCode string `json:"game_code"`
	TableNo  int    `json:"table_no"`
	RoundNo  string `json:"round_no"`
	Reason   string `json:"reason"`
	Amount   int64  `json:"amount"`
}

func (s *MiniGameWalletService) authorized(r *http.Request) bool {
	return s.secret != "" && r.Header.Get("X-Minigame-Secret") == s.secret
}

func (s *MiniGameWalletService) Balance(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeAPIError(w, http.StatusUnauthorized, appError(700, "游戏服务鉴权失败"))
		return
	}
	var request walletRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request) != nil || request.UID < 1 {
		writeAPIError(w, http.StatusBadRequest, appError(400, "请求参数错误"))
		return
	}
	var balance int64
	if err := s.db.QueryRowContext(r.Context(), "SELECT coin FROM cmf_user WHERE id=? AND user_status=1", request.UID).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, appError(404, "用户不存在或已停用"))
		} else {
			s.logger.Error("minigame wallet balance", "uid", request.UID, "error", err)
			writeAPIError(w, http.StatusInternalServerError, appError(500, "钱包查询失败"))
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "balance": balance})
}

func (s *MiniGameWalletService) Adjust(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeAPIError(w, http.StatusUnauthorized, appError(700, "游戏服务鉴权失败"))
		return
	}
	var request walletRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request) != nil {
		writeAPIError(w, http.StatusBadRequest, appError(400, "请求参数错误"))
		return
	}
	request.OrderNo = strings.TrimSpace(request.OrderNo)
	request.GameCode = strings.TrimSpace(request.GameCode)
	request.RoundNo = strings.TrimSpace(request.RoundNo)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.UID < 1 || request.OrderNo == "" || len(request.OrderNo) > 80 ||
		request.GameCode == "" || len(request.GameCode) > 60 || request.Amount == 0 ||
		request.TableNo < 1 || request.TableNo > 1000 {
		writeAPIError(w, http.StatusBadRequest, appError(400, "钱包订单参数错误"))
		return
	}

	tx, err := s.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, appError(500, "钱包暂不可用"))
		return
	}
	defer tx.Rollback()

	var existingUID, existingAmount, existingBalance int64
	err = tx.QueryRowContext(r.Context(),
		"SELECT uid,amount,balance_after FROM cmf_minigame_wallet_order WHERE order_no=?",
		request.OrderNo,
	).Scan(&existingUID, &existingAmount, &existingBalance)
	if err == nil {
		if existingUID != request.UID || existingAmount != request.Amount {
			writeAPIError(w, http.StatusConflict, appError(409, "幂等订单参数不一致"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "balance": existingBalance, "duplicate": true})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, http.StatusInternalServerError, appError(500, "钱包订单查询失败"))
		return
	}

	var before int64
	if err = tx.QueryRowContext(r.Context(),
		"SELECT coin FROM cmf_user WHERE id=? AND user_status=1 FOR UPDATE", request.UID,
	).Scan(&before); err != nil {
		writeAPIError(w, http.StatusNotFound, appError(404, "用户不存在或已停用"))
		return
	}
	after := before + request.Amount
	if after < 0 {
		writeAPIError(w, http.StatusConflict, appError(4003, "钱包余额不足"))
		return
	}
	if _, err = tx.ExecContext(r.Context(),
		"UPDATE cmf_user SET coin=?,consumption=consumption+? WHERE id=?",
		after, maxInt64(0, -request.Amount), request.UID,
	); err != nil {
		writeAPIError(w, http.StatusInternalServerError, appError(500, "钱包扣款失败"))
		return
	}
	now := time.Now().Unix()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO cmf_minigame_wallet_order
		(order_no,uid,game_code,table_no,round_no,reason,amount,balance_before,balance_after,create_time)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		request.OrderNo, request.UID, request.GameCode, request.TableNo, request.RoundNo,
		request.Reason, request.Amount, before, after, now,
	); err != nil {
		writeAPIError(w, http.StatusInternalServerError, appError(500, "钱包流水写入失败"))
		return
	}
	action := 40
	if request.Amount > 0 {
		action = 41
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO cmf_user_coinrecord
		(type,action,uid,touid,giftid,giftcount,totalcoin,showid,addtime)
		VALUES(?,?,?,?,?,1,?,0,?)`,
		boolInt(request.Amount > 0), action, request.UID, request.UID, 0, absInt64(request.Amount), now,
	); err != nil {
		writeAPIError(w, http.StatusInternalServerError, appError(500, "平台流水写入失败"))
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, http.StatusInternalServerError, appError(500, "钱包结算失败"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "balance": after, "duplicate": false})
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
