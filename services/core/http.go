package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AppError struct {
	Code    int
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func appError(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

type APIServer struct {
	db         *sql.DB
	auth       *Authenticator
	lottery    *LotteryService
	sports     *SportsService
	openIM     *OpenIMClient
	hotUpdate  *HotUpdateService
	miniGame   *MiniGameService
	gameWallet *MiniGameWalletService
	logger     *slog.Logger
}

func NewAPIServer(db *sql.DB, auth *Authenticator, lottery *LotteryService, sports *SportsService, openIM *OpenIMClient, hotUpdate *HotUpdateService, miniGame *MiniGameService, gameWallet *MiniGameWalletService, logger *slog.Logger) *APIServer {
	return &APIServer{db: db, auth: auth, lottery: lottery, sports: sports, openIM: openIM, hotUpdate: hotUpdate, miniGame: miniGame, gameWallet: gameWallet, logger: logger}
}

func (s *APIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.health)
	mux.HandleFunc("GET /appapi/", s.compat)
	mux.HandleFunc("POST /appapi/", s.compat)
	mux.HandleFunc("POST /v1/im/session", s.imSession)
	mux.HandleFunc("POST /v1/im/prepare-users", s.imPrepareUsers)
	mux.HandleFunc("POST /v1/im/live-session", s.imLiveSession)
	mux.HandleFunc("POST /v1/im/live-message", s.imLiveMessage)
	mux.HandleFunc("POST /v1/minigame/wallet/balance", s.gameWallet.Balance)
	mux.HandleFunc("POST /v1/minigame/wallet/adjust", s.gameWallet.Adjust)
	mux.Handle("GET /v1/im/ws", newOpenIMWebSocketProxy(s.openIM.gatewayURL, s.logger))
	mux.HandleFunc("GET /appapi/hotupdate/download", s.hotUpdate.Download)
	return s.middleware(mux)
}

func (s *APIServer) imLiveSession(w http.ResponseWriter, r *http.Request) {
	var request struct {
		UID      int64  `json:"uid"`
		Token    string `json:"token"`
		LiveID   string `json:"liveID"`
		LiveName string `json:"liveName"`
		Stream   string `json:"stream"`
		Platform int    `json:"platformID"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, appError(400, "请求参数错误"))
		return
	}
	if err := s.auth.Verify(r.Context(), request.UID, request.Token); err != nil {
		writeAPIError(w, http.StatusUnauthorized, normalizeError(err))
		return
	}
	if request.Platform < 1 {
		request.Platform = 5
	}
	if strings.TrimSpace(request.Stream) == "" {
		request.Stream = strings.TrimSpace(request.LiveName)
	}
	liveUID, parseErr := strconv.ParseInt(strings.TrimSpace(request.LiveID), 10, 64)
	if parseErr != nil || liveUID < 1 {
		writeAPIError(w, http.StatusBadRequest, appError(400, "直播间参数错误"))
		return
	}
	liveName, err := s.requireLiveRoom(r.Context(), liveUID, request.Stream, false)
	if err != nil {
		writeAPIError(w, http.StatusConflict, normalizeError(err))
		return
	}
	var kicked int
	if err := s.db.QueryRowContext(r.Context(),
		"SELECT EXISTS(SELECT 1 FROM cmf_live_kick WHERE uid=? AND liveuid=?)",
		request.UID, liveUID,
	).Scan(&kicked); err != nil {
		writeAPIError(w, http.StatusInternalServerError, normalizeError(err))
		return
	}
	if kicked == 1 {
		writeAPIError(w, http.StatusForbidden, appError(1008, "您已被踢出房间"))
		return
	}
	user, err := loadIMUser(r.Context(), s.db, request.UID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, normalizeError(err))
		return
	}
	session, err := s.openIM.EnsureUserSession(r.Context(), user, request.Platform)
	if err != nil {
		s.logger.Error("openim live session", "uid", request.UID, "error", err)
		writeAPIError(w, http.StatusBadGateway, appError(502, "IM 服务暂不可用"))
		return
	}
	groupID, err := s.openIM.EnsureLiveGroup(r.Context(), request.LiveID, liveName, user)
	if err != nil {
		s.logger.Error("openim live group", "uid", request.UID, "live_id", request.LiveID, "error", err)
		writeAPIError(w, http.StatusBadGateway, appError(502, "直播群组暂不可用"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": map[string]any{"session": session, "groupID": groupID}})
}

func (s *APIServer) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		started := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path != "/v1/im/ws" && time.Since(started) > time.Second {
			s.logger.Warn("slow request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
		}
	})
}

func (s *APIServer) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "unhealthy", "database": "down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"database":              "up",
		"sports_collector":      s.sports.Status(),
		"lottery_engine":        s.lottery.Status(),
		"openim_api_configured": s.openIM.Configured(),
	})
}

func (s *APIServer) compat(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeCompatError(w, appError(400, "请求参数错误"))
		return
	}
	service := strings.TrimSpace(r.FormValue("service"))
	ctx := r.Context()
	uid := formInt64(r, "uid")

	var (
		data any
		err  error
	)

	switch service {
	case "LotteryGame.home":
		data, err = s.lottery.Home(ctx, uid)
	case "LotteryGame.detail":
		data, err = s.lottery.Detail(ctx, formInt64(r, "game_id"), r.FormValue("game_code"), uid)
	case "LotteryGame.currentIssue":
		data, err = s.lottery.CurrentIssue(ctx, formInt64(r, "game_id"), r.FormValue("game_code"))
	case "LotteryGame.issueHistory":
		data, err = s.lottery.IssueHistory(ctx, formInt64(r, "game_id"), r.FormValue("game_code"), formPage(r))
	case "LotteryGame.bet":
		if err = s.requireAuth(ctx, r); err == nil {
			data, err = s.lottery.PlaceBet(ctx, LotteryBetRequest{
				UID: uid, GameID: formInt64(r, "game_id"), IssueID: formInt64(r, "issue_id"),
				ClientTraceID: r.FormValue("client_trace_id"), ItemsJSON: r.FormValue("items"),
			})
		}
	case "LotteryGame.orderList":
		if err = s.requireAuth(ctx, r); err == nil {
			data, err = s.lottery.OrderList(ctx, uid, formInt64(r, "game_id"), r.FormValue("game_code"), formPage(r))
		}
	case "Sports.home":
		data, err = s.sports.Home(ctx, r.FormValue("tab"), r.FormValue("date"), r.FormValue("competition_type"))
	case "Sports.matchDetail":
		data, err = s.sports.MatchDetail(ctx, r.FormValue("match_id"))
	case "Sports.matchUpdates":
		data, err = s.sports.MatchUpdates(ctx, r.FormValue("match_id"), formInt64(r, "since"))
	case "SportsBet.matchMarkets":
		data, err = s.sports.MatchMarkets(ctx, r.FormValue("match_id"), uid)
	case "SportsBet.bet":
		if err = s.requireAuth(ctx, r); err == nil {
			data, err = s.sports.PlaceBet(ctx, SportsBetRequest{
				UID: uid, MatchID: r.FormValue("match_id"), ClientTraceID: r.FormValue("client_trace_id"), ItemsJSON: r.FormValue("items"),
			})
		}
	case "MiniGame.list":
		data, err = s.miniGame.List(ctx, r.FormValue("category"))
	case "MiniGame.enter":
		if err = s.requireAuth(ctx, r); err == nil {
			data, err = s.miniGame.Enter(ctx, r.FormValue("code"), uid)
		}
	case "App.checkUpdate":
		data, err = s.hotUpdate.Check(ctx, formInt64(r, "version_code"), formInt64(r, "app_code"))
	case "SportsBet.orderList", "SportsBet.recordList":
		if err = s.requireAuth(ctx, r); err == nil {
			data, err = s.sports.OrderList(ctx, uid, r.FormValue("match_id"), formPage(r))
		}
	default:
		err = appError(404, "接口不存在")
	}

	if err != nil {
		writeCompatError(w, normalizeError(err))
		return
	}
	writeCompat(w, data)
}

func (s *APIServer) imSession(w http.ResponseWriter, r *http.Request) {
	var request struct {
		UID        int64  `json:"uid"`
		Token      string `json:"token"`
		PlatformID int    `json:"platformID"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, appError(400, "请求参数错误"))
		return
	}
	if err := s.auth.Verify(r.Context(), request.UID, request.Token); err != nil {
		writeAPIError(w, http.StatusUnauthorized, normalizeError(err))
		return
	}
	if request.PlatformID < 1 {
		request.PlatformID = 5
	}
	user, err := loadIMUser(r.Context(), s.db, request.UID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, normalizeError(err))
		return
	}
	session, err := s.openIM.EnsureUserSession(r.Context(), user, request.PlatformID)
	if err != nil {
		s.logger.Error("openim session", "uid", request.UID, "error", err)
		writeAPIError(w, http.StatusBadGateway, appError(502, "IM 服务暂不可用"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": session})
}

func (s *APIServer) imPrepareUsers(w http.ResponseWriter, r *http.Request) {
	var request struct {
		UID     int64   `json:"uid"`
		Token   string  `json:"token"`
		UserIDs []int64 `json:"userIDs"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, appError(400, "用户参数错误"))
		return
	}
	if err := s.auth.Verify(r.Context(), request.UID, request.Token); err != nil {
		writeAPIError(w, http.StatusUnauthorized, normalizeError(err))
		return
	}
	if len(request.UserIDs) == 0 || len(request.UserIDs) > 100 {
		writeAPIError(w, http.StatusBadRequest, appError(400, "请选择1至100名用户"))
		return
	}
	seen := make(map[int64]struct{}, len(request.UserIDs))
	users := make([]IMUser, 0, len(request.UserIDs))
	for _, userID := range request.UserIDs {
		if userID < 1 {
			writeAPIError(w, http.StatusBadRequest, appError(400, "用户ID不正确"))
			return
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		user, err := loadIMUser(r.Context(), s.db, userID)
		if err != nil {
			writeAPIError(w, http.StatusNotFound, normalizeError(err))
			return
		}
		users = append(users, user)
	}
	if err := s.openIM.EnsureUsers(r.Context(), users); err != nil {
		s.logger.Error("openim prepare users", "uid", request.UID, "count", len(users), "error", err)
		writeAPIError(w, http.StatusBadGateway, appError(502, "聊天用户初始化失败"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"data": map[string]any{"prepared": len(users)},
	})
}

func (s *APIServer) requireAuth(ctx context.Context, r *http.Request) error {
	if err := s.auth.Verify(ctx, formInt64(r, "uid"), r.FormValue("token")); err != nil {
		return err
	}
	return nil
}

func normalizeError(err error) *AppError {
	var target *AppError
	if errors.As(err, &target) {
		return target
	}
	if errors.Is(err, ErrUnauthorized) {
		return appError(700, ErrUnauthorized.Error())
	}
	return &AppError{Code: 500, Message: "服务内部错误", Cause: err}
}

func formInt64(r *http.Request, key string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue(key)), 10, 64)
	return value
}

func formPage(r *http.Request) int {
	page, _ := strconv.Atoi(r.FormValue("p"))
	if page < 1 {
		return 1
	}
	return page
}

func writeCompat(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ret":  200,
		"data": map[string]any{"code": 0, "msg": "", "info": []any{data}},
	})
}

func writeCompatError(w http.ResponseWriter, err *AppError) {
	status := http.StatusOK
	writeJSON(w, status, map[string]any{
		"ret":  200,
		"data": map[string]any{"code": err.Code, "msg": err.Message, "info": []any{}},
	})
}

func writeAPIError(w http.ResponseWriter, status int, err *AppError) {
	writeJSON(w, status, map[string]any{"code": err.Code, "message": err.Message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Fprintln(w, `{"code":500,"message":"encode response failed"}`)
	}
}
