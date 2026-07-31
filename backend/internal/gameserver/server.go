package gameserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/game"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

type Server struct {
	auth    *auth.Service
	game    *game.Service
	fishing *fishingHub
	logger  *slog.Logger
}

func New(authService *auth.Service, gameService *game.Service, logger *slog.Logger) *Server {
	return &Server{
		auth: authService, game: gameService,
		fishing: newFishingHub(gameService, logger), logger: logger,
	}
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v2/games", s.catalog)
	mux.HandleFunc("POST /api/v2/games/fishing/enter", s.enterFishing)
	mux.HandleFunc("POST /api/v2/games/fishing/{session_id}/leave", s.leaveFishing)
	mux.HandleFunc("GET /minigame/fish", s.fishingIndex)
	mux.Handle("GET /minigame/fish/", fishingAssetHandler)
	mux.HandleFunc("GET /ws/game/fishing", s.fishingSocket)
	mux.HandleFunc("GET /appapi/", s.compat)
	mux.HandleFunc("POST /appapi/", s.compat)
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	items, err := s.game.Catalog(r.Context(), category)
	if err != nil {
		s.logger.Error("game catalog", "error", err)
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "游戏列表暂不可用")
		return
	}
	venues := make([]game.FishingVenueItem, 0)
	if strings.TrimSpace(category) == "" || strings.TrimSpace(category) == "fishing" {
		venues, err = s.game.FishingVenues(r.Context())
		if err != nil {
			s.logger.Error("fishing venues", "error", err)
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "捕鱼场次暂不可用")
			return
		}
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"items": items, "fishing_venues": venues,
	})
}

func (s *Server) enterFishing(w http.ResponseWriter, r *http.Request) {
	authenticated, err := s.authenticate(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 700, "请先登录")
		return
	}
	var request struct {
		VenueCode string `json:"venue_code"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求格式错误")
		return
	}
	launch, err := s.game.EnterFishing(r.Context(), authenticated.ID, request.VenueCode)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, gameErrorCode(err), gameErrorMessage(err))
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"launch": launch, "launch_url": fishingLaunchURL(launch, authenticated.Nickname),
	})
}

func (s *Server) leaveFishing(w http.ResponseWriter, r *http.Request) {
	authenticated, err := s.authenticate(r)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 700, "请先登录")
		return
	}
	entry, err := s.game.LeaveFishing(r.Context(), authenticated.ID, r.PathValue("session_id"))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, gameErrorCode(err), gameErrorMessage(err))
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), entry)
}

func (s *Server) compat(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeCompat(w, 400, "请求参数错误", nil)
		return
	}
	switch strings.TrimSpace(r.FormValue("service")) {
	case "MiniGame.list":
		data, err := s.compatCatalog(r.Context(), r.FormValue("category"))
		if err != nil {
			s.logger.Error("compat game catalog", "error", err)
			writeCompat(w, 500, "游戏列表暂不可用", nil)
			return
		}
		writeCompat(w, 0, "", data)
	case "MiniGame.enter":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		authenticated, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token"))
		if err != nil {
			writeCompat(w, 700, "请先登录", nil)
			return
		}
		if code := strings.TrimSpace(r.FormValue("code")); code != "" && code != "deepsea_hunter" {
			writeCompat(w, 4002, "游戏不存在或已下架", nil)
			return
		}
		launch, err := s.game.EnterFishing(r.Context(), authenticated.ID, r.FormValue("room"))
		if err != nil {
			writeCompat(w, gameErrorCode(err), gameErrorMessage(err), nil)
			return
		}
		writeCompat(w, 0, "", compatFishingLaunch(launch, authenticated.Nickname))
	default:
		writeCompat(w, 404, "接口不存在", nil)
	}
}

func (s *Server) authenticate(r *http.Request) (auth.User, error) {
	userID, token := auth.Bearer(r)
	return s.auth.Authenticate(r.Context(), userID, token)
}

func (s *Server) compatCatalog(ctx context.Context, category string) (map[string]any, error) {
	items, err := s.game.Catalog(ctx, category)
	if err != nil {
		return nil, err
	}
	games := make([]map[string]any, 0, len(items))
	grouped := make(map[string][]map[string]any)
	for _, item := range items {
		players := strconv.Itoa(item.PlayersMin)
		if item.PlayersMax > item.PlayersMin {
			players = strconv.Itoa(item.PlayersMin) + "-" + strconv.Itoa(item.PlayersMax)
		}
		formatted := map[string]any{
			"id": strconv.FormatInt(item.ID, 10), "code": item.Code, "name": item.Name,
			"name_en": "", "category": item.Category, "cover": "", "entry_type": "local",
			"players_min": strconv.Itoa(item.PlayersMin), "players_max": strconv.Itoa(item.PlayersMax),
			"players_text": players + "人", "play_mode": "match", "need_login": "1",
			"use_wallet": boolString(item.UseWallet), "orientation": item.Orientation,
			"remark": "", "table_count": "300", "entry_mode": "match", "is_hot": "1", "is_new": "0",
		}
		games = append(games, formatted)
		grouped[item.Category] = append(grouped[item.Category], formatted)
	}
	categories := make([]map[string]any, 0, len(grouped))
	for _, key := range []string{"fishing", "lottery", "card", "casual"} {
		group := grouped[key]
		if len(group) == 0 {
			continue
		}
		name := map[string]string{"fishing": "捕鱼", "lottery": "彩票", "card": "棋牌", "casual": "休闲"}[key]
		categories = append(categories, map[string]any{
			"key": key, "name": name, "count": strconv.Itoa(len(group)), "games": group,
		})
	}
	venues := make([]game.FishingVenueItem, 0)
	if strings.TrimSpace(category) == "" || strings.TrimSpace(category) == "fishing" {
		venues, err = s.game.FishingVenues(ctx)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"total": strconv.Itoa(len(games)), "games": games,
		"categories": categories, "fishing_venues": venues,
	}, nil
}

func compatFishingLaunch(launch game.FishingLaunch, nickname string) map[string]any {
	return map[string]any{
		"code": launch.GameCode, "name": launch.GameName, "category": "fishing",
		"entry_type": "local", "players_min": "1", "players_max": "4", "players_text": "1-4人",
		"play_mode": "match", "need_login": "1", "use_wallet": "1", "orientation": "landscape",
		"table_count": "300", "table_no": strconv.Itoa(launch.Table), "seat_no": strconv.Itoa(launch.Seat),
		"entry_mode": "match", "launch_url": fishingLaunchURL(launch, nickname), "nickname": nickname,
		"session_id": launch.SessionID, "venue_code": launch.VenueCode,
		"venue_name": launch.VenueName, "multiplier": launch.Multiplier,
		"wallet_balance": launch.WalletBalance, "escrow_amount": 0,
	}
}

func fishingLaunchURL(launch game.FishingLaunch, nickname string) string {
	parameters := url.Values{}
	parameters.Set("session", launch.SessionID)
	parameters.Set("resume", launch.ResumeToken)
	parameters.Set("venue", launch.VenueCode)
	parameters.Set("table", strconv.Itoa(launch.Table))
	parameters.Set("seat", strconv.Itoa(launch.Seat))
	parameters.Set("name", strings.TrimSpace(nickname))
	return strings.TrimSpace(launch.EntryPath) + "?" + parameters.Encode()
}

func gameErrorCode(err error) int {
	switch {
	case errors.Is(err, wallet.ErrInsufficientFunds):
		return 4021
	case errors.Is(err, game.ErrVenueNotFound), errors.Is(err, game.ErrSessionNotFound):
		return 404
	default:
		return 400
	}
}

func gameErrorMessage(err error) string {
	switch {
	case errors.Is(err, wallet.ErrInsufficientFunds):
		return "余额不足，无法进入该场次"
	case errors.Is(err, game.ErrVenueNotFound):
		return "捕鱼场次不存在"
	case errors.Is(err, game.ErrSessionNotFound):
		return "游戏会话不存在"
	default:
		return "游戏操作失败"
	}
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func writeCompat(w http.ResponseWriter, code int, message string, data any) {
	info := make([]any, 0, 1)
	if data != nil {
		info = append(info, data)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"code": code, "msg": message, "info": info},
	})
}
