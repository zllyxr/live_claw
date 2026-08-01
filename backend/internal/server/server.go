package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/apprelease"
	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/bankpayment"
	"github.com/zllyxr/live_claw/backend/internal/home"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/im"
	"github.com/zllyxr/live_claw/backend/internal/invite"
	"github.com/zllyxr/live_claw/backend/internal/live"
	"github.com/zllyxr/live_claw/backend/internal/lottery"
	"github.com/zllyxr/live_claw/backend/internal/payment"
	"github.com/zllyxr/live_claw/backend/internal/sports"
	"github.com/zllyxr/live_claw/backend/internal/storage"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

type Server struct {
	db           *sql.DB
	redis        *redis.Client
	auth         *auth.Service
	home         *home.Service
	app          *apprelease.Service
	lottery      *lottery.Service
	sports       *sports.Service
	live         *live.Service
	invite       *invite.Service
	storage      *storage.Service
	wallet       *wallet.Service
	payments     *payment.Service
	im           *im.Service
	logger       *slog.Logger
	mediaBaseURL string
	publicURL    string
	environment  string
	dataKey      string
	bankCipher   *bankpayment.Cipher
}

func New(
	db *sql.DB,
	redisClient *redis.Client,
	authService *auth.Service,
	homeService *home.Service,
	appService *apprelease.Service,
	lotteryService *lottery.Service,
	sportsService *sports.Service,
	liveService *live.Service,
	inviteService *invite.Service,
	storageService *storage.Service,
	walletService *wallet.Service,
	paymentService *payment.Service,
	imService *im.Service,
	mediaBaseURL string,
	publicURL string,
	environment string,
	dataKey string,
	logger *slog.Logger,
) *Server {
	bankCipher, _ := bankpayment.NewCipher(dataKey)
	return &Server{
		db: db, redis: redisClient, auth: authService, home: homeService,
		app: appService, lottery: lotteryService, sports: sportsService,
		live: liveService, invite: inviteService,
		storage:      storageService,
		wallet:       walletService,
		payments:     paymentService,
		im:           imService,
		mediaBaseURL: strings.TrimRight(mediaBaseURL, "/"),
		publicURL:    strings.TrimRight(publicURL, "/"),
		environment:  strings.ToLower(strings.TrimSpace(environment)),
		dataKey:      dataKey,
		bankCipher:   bankCipher,
		logger:       logger,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.health)
	mux.HandleFunc("GET /api/v2/home", s.homeV2)
	mux.HandleFunc("GET /api/v2/app/update", s.appUpdateV2)
	mux.HandleFunc("POST /api/v2/payments/bepusdt/notify", s.bepusdtNotify)
	mux.HandleFunc("GET /api/v2/", s.notFound)
	mux.HandleFunc("POST /api/v2/", s.notFound)
	mux.HandleFunc("GET /appapi/", s.compat)
	mux.HandleFunc("POST /appapi/", s.compat)

	var handler http.Handler = mux
	handler = s.accessLog(handler)
	handler = httpx.Recover(s.logger, handler)
	handler = httpx.RequestContext(handler)
	return handler
}

func (s *Server) homeV2(w http.ResponseWriter, r *http.Request) {
	var user *home.UserSummary
	userID, token := auth.Bearer(r)
	if userID > 0 || token != "" {
		authenticated, err := s.auth.Authenticate(r.Context(), userID, token)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 700, "登录已失效")
			return
		}
		user = &home.UserSummary{ID: authenticated.ID, Nickname: authenticated.Nickname}
	}
	httpx.OK(w, httpx.RequestID(r.Context()), s.home.Dashboard(r.Context(), user))
}

func (s *Server) compat(w http.ResponseWriter, r *http.Request) {
	queryService := strings.TrimSpace(r.URL.Query().Get("service"))
	if queryService == "Upload.uploadFile" {
		s.compatUpload(w, r)
		return
	}
	if queryService == "Charge.submitBankPaymentProof" {
		s.compatSubmitBankPaymentProof(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeCompat(w, 400, "请求参数错误", nil)
		return
	}
	service := strings.TrimSpace(r.FormValue("service"))
	if s.compatExtended(w, r, service) {
		return
	}
	switch service {
	case "Home.getConfig":
		writeCompat(w, 0, "", map[string]any{
			"site_name": "星域", "currency_name": "星币", "live_provider": "douyin",
			"invite_code_format": "xxx-xxxx", "backend_version": "v2",
		})
	case "Home.getLogin":
		writeCompat(w, 0, "", map[string]any{"login_img": ""})
	case "Upload.getCosInfo":
		writeCompat(w, 0, "", map[string]any{
			"cloudtype": "minio",
			"storageInfo": map[string]any{
				"upload_url": s.publicURL + "/appapi/?service=Upload.uploadFile",
				"field":      "file",
			},
			"localInfo": map[string]any{
				"upload_url": s.publicURL + "/appapi/?service=Upload.uploadFile",
			},
		})
	case "Home.getHot", "Home.getFollow":
		page, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("p")))
		items, err := s.live.Hot(r.Context(), page, 12)
		if err != nil {
			s.logger.Error("live room list", "error", err)
			writeCompat(w, 500, "直播列表暂不可用", nil)
			return
		}
		writeCompat(w, 0, "", map[string]any{
			"list": items, "slide": []any{}, "title": "热门直播", "des": "抖音直播",
		})
	case "Login.getCountrys":
		writeCompat(w, 0, "", compatCountries(r.FormValue("field")))
	case "Login.userLogin":
		session, err := s.auth.Login(
			r.Context(), r.FormValue("country_code"), r.FormValue("user_login"), r.FormValue("user_pass"),
			r.FormValue("device_id"), r.FormValue("source"), requestIP(r), r.UserAgent(),
		)
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeCompat(w, 1001, "账号或密码错误", nil)
			return
		}
		if err != nil {
			s.logger.Error("user login", "request_id", httpx.RequestID(r.Context()), "error", err)
			writeCompat(w, 500, "登录暂不可用", nil)
			return
		}
		profile, err := s.compatUserProfile(r.Context(), session.User.ID, session.Token)
		if err != nil {
			_ = s.auth.Revoke(r.Context(), session.User.ID, session.Token)
			writeCompat(w, 500, "读取用户资料失败", nil)
			return
		}
		writeCompat(w, 0, "", profile)
	case "User.ifToken":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		writeCompat(w, 0, "", map[string]string{"isvalid": "1"})
	case "User.getBaseInfo":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		profile, err := s.compatUserProfile(r.Context(), userID, "")
		if err != nil {
			writeCompat(w, 500, "读取用户资料失败", nil)
			return
		}
		writeCompat(w, 0, "", profile)
	case "User.getBalance":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		balance, err := s.compatWalletBalance(r.Context(), userID)
		if err != nil {
			writeCompat(w, 500, "读取余额失败", nil)
			return
		}
		writeCompat(w, 0, "", balance)
	case "Agent.getCode":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		code, err := s.invite.EnsureUserCode(r.Context(), userID)
		if err != nil {
			writeCompat(w, 500, "生成邀请码失败", nil)
			return
		}
		href := s.publicURL + "/h5/#/pages/auth/register?code=" + url.QueryEscape(code.FullCode)
		writeCompat(w, 0, "", map[string]any{
			"code": code.FullCode, "href": href, "url": href, "link": href, "qr": "", "qrcode": "",
		})
	case "Agent.checkAgent":
		writeCompat(w, 0, "", map[string]string{
			"agent_switch": "1", "agent_must": "0", "has_agent": "1", "openinstall_switch": "0",
		})
	case "User.setDistribut", "Invite.bind":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		rawCode := r.FormValue("code")
		if rawCode == "" {
			rawCode = r.FormValue("ref")
		}
		source := "manual"
		if strings.TrimSpace(r.FormValue("service")) == "Invite.bind" {
			source = "attribution"
		}
		bound, err := s.invite.Bind(r.Context(), userID, rawCode, source)
		if err != nil {
			writeCompat(w, 400, "邀请码无效或已绑定", nil)
			return
		}
		writeCompat(w, 0, "", map[string]any{
			"matched": "1", "already_bound": boolString(bound.AlreadyBound),
			"bound": boolString(!bound.AlreadyBound), "code": bound.InviteCode,
			"inviter_uid":  strconv.FormatInt(bound.InviterUserID, 10),
			"match_method": source, "confidence": "100", "msg": "绑定成功",
		})
	case "Home.dashboard":
		var user *home.UserSummary
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		token := strings.TrimSpace(r.FormValue("token"))
		if userID > 0 {
			authenticated, err := s.auth.Authenticate(r.Context(), userID, token)
			if err != nil {
				writeCompat(w, 700, "登录已失效", nil)
				return
			}
			user = &home.UserSummary{ID: authenticated.ID, Nickname: authenticated.Nickname}
		}
		writeCompat(w, 0, "", s.home.Dashboard(r.Context(), user))
	case "App.checkUpdate":
		currentCode, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("version_code")), 10, 64)
		appCode, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("app_code")), 10, 64)
		data, err := s.app.Check(
			r.Context(), currentCode, appCode,
			r.FormValue("platform"), r.FormValue("device_id"),
		)
		if err != nil {
			writeCompat(w, 500, "检查更新失败", nil)
			return
		}
		writeCompat(w, 0, "", data)
	case "LotteryGame.home":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if userID > 0 {
			if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
				writeCompat(w, 700, "登录已失效", nil)
				return
			}
		}
		data, err := s.lottery.Home(r.Context(), userID)
		s.writeLotteryCompat(w, err, data)
	case "LotteryGame.detail":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if userID > 0 {
			if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
				writeCompat(w, 700, "登录已失效", nil)
				return
			}
		}
		gameID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("game_id")), 10, 64)
		data, err := s.lottery.Detail(
			r.Context(), gameID, r.FormValue("game_code"), userID,
		)
		s.writeLotteryCompat(w, err, data)
	case "LotteryGame.currentIssue":
		gameID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("game_id")), 10, 64)
		data, err := s.lottery.CurrentIssue(r.Context(), gameID, r.FormValue("game_code"))
		s.writeLotteryCompat(w, err, data)
	case "LotteryGame.issueHistory":
		gameID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("game_id")), 10, 64)
		page, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("p")))
		data, err := s.lottery.IssueHistory(
			r.Context(), gameID, r.FormValue("game_code"), page,
		)
		s.writeLotteryCompat(w, err, data)
	case "LotteryGame.bet":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		gameID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("game_id")), 10, 64)
		issueID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("issue_id")), 10, 64)
		data, err := s.lottery.PlaceBet(r.Context(), lottery.BetRequest{
			UserID: userID, GameID: gameID, GameCode: r.FormValue("game_code"),
			IssueID: issueID, ClientTraceID: r.FormValue("client_trace_id"),
			ItemsJSON: r.FormValue("items"),
		})
		s.writeLotteryCompat(w, err, data)
	case "LotteryGame.orderList":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		gameID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("game_id")), 10, 64)
		page, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("p")))
		data, err := s.lottery.Orders(
			r.Context(), userID, gameID, r.FormValue("game_code"), page,
		)
		s.writeLotteryCompat(w, err, data)
	case "Sports.home":
		data, err := s.sports.Home(
			r.Context(), r.FormValue("tab"), r.FormValue("date"), r.FormValue("competition_type"),
		)
		s.writeSportsCompat(w, err, data)
	case "Sports.matchDetail":
		data, err := s.sports.MatchDetail(r.Context(), r.FormValue("match_id"))
		s.writeSportsCompat(w, err, data)
	case "SportsBet.matchMarkets":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if userID > 0 {
			if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
				writeCompat(w, 700, "登录已失效", nil)
				return
			}
		}
		data, err := s.sports.MatchMarkets(r.Context(), r.FormValue("match_id"), userID)
		s.writeSportsCompat(w, err, data)
	case "SportsBet.bet":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		data, err := s.sports.PlaceBet(r.Context(), sports.BetRequest{
			UserID: userID, MatchID: r.FormValue("match_id"),
			ClientTraceID: r.FormValue("client_trace_id"), ItemsJSON: r.FormValue("items"),
		})
		s.writeSportsCompat(w, err, data)
	case "SportsBet.orderList", "SportsBet.recordList":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		page, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("p")))
		data, err := s.sports.Orders(r.Context(), userID, r.FormValue("match_id"), page)
		s.writeSportsCompat(w, err, data)
	case "Live.resolveSource":
		liveUserID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("liveuid")), 10, 64)
		refresh := r.FormValue("refresh") == "1"
		source, err := s.live.Resolve(
			r.Context(), liveUserID, r.FormValue("stream"), refresh,
		)
		s.writeLiveCompat(w, err, source)
	case "Live.enterRoom":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		liveUserID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("liveuid")), 10, 64)
		data, err := s.live.Enter(r.Context(), userID, liveUserID, r.FormValue("stream"))
		s.writeLiveCompat(w, err, data)
	case "Live.getUserLists":
		liveUserID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("liveuid")), 10, 64)
		data, err := s.live.Users(r.Context(), liveUserID, r.FormValue("stream"))
		s.writeLiveCompat(w, err, data)
	case "IM.joinLive":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
			writeCompat(w, 700, "登录已失效", nil)
			return
		}
		liveUserID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("liveuid")), 10, 64)
		data, err := s.live.Join(r.Context(), userID, liveUserID, r.FormValue("stream"))
		s.writeLiveCompat(w, err, data)
	case "Live.signOutWatchLive":
		userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
		if userID > 0 {
			if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
				writeCompat(w, 700, "登录已失效", nil)
				return
			}
			liveUserID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("liveuid")), 10, 64)
			if err := s.live.Leave(r.Context(), userID, liveUserID, r.FormValue("stream")); err != nil &&
				!errors.Is(err, live.ErrRoomNotFound) {
				s.logger.Error("leave live room", "error", err)
			}
		}
		writeCompat(w, 0, "", map[string]string{"left": "1"})
	default:
		writeCompat(w, 404, "接口不存在", nil)
	}
}

func (s *Server) writeLiveCompat(w http.ResponseWriter, err error, data any) {
	if err == nil {
		writeCompat(w, 0, "", data)
		return
	}
	if errors.Is(err, live.ErrRoomNotFound) {
		writeCompat(w, 1001, "直播间不存在或已下线", nil)
		return
	}
	if errors.Is(err, live.ErrSourceUnavailable) {
		writeCompat(w, 1002, "抖音直播流暂不可用，请稍后重试", nil)
		return
	}
	s.logger.Error("live request", "error", err)
	writeCompat(w, 500, "直播服务暂不可用", nil)
}

func (s *Server) writeSportsCompat(w http.ResponseWriter, err error, data any) {
	if err == nil {
		writeCompat(w, 0, "", data)
		return
	}
	code, message := sports.PublicError(err)
	if code == 500 {
		s.logger.Error("sports request", "error", err)
	}
	writeCompat(w, code, message, nil)
}

func (s *Server) writeLotteryCompat(w http.ResponseWriter, err error, data any) {
	if err == nil {
		writeCompat(w, 0, "", data)
		return
	}
	code, message := lottery.PublicError(err)
	if code == 500 {
		s.logger.Error("lottery request", "error", err)
	}
	writeCompat(w, code, message, nil)
}

func (s *Server) compatUserProfile(ctx context.Context, userID int64, token string) (map[string]any, error) {
	var id int64
	var nickname, signature, objectKey string
	var gender int
	var coin, follows, fans int64
	err := s.db.QueryRowContext(ctx, `
		SELECT app_user.id,COALESCE(NULLIF(app_user.nickname,''),app_user.username),app_user.gender,app_user.signature,
		       COALESCE(asset.object_key,''),
		       COALESCE(wallet.available,0),
		       (SELECT COUNT(*) FROM user_follows WHERE target_user_id=app_user.id),
		       (SELECT COUNT(*) FROM user_follows WHERE user_id=app_user.id)
		FROM users app_user
		LEFT JOIN media_assets asset ON asset.id=app_user.avatar_asset_id AND asset.status=1
		LEFT JOIN wallet_accounts wallet ON wallet.user_id=app_user.id AND wallet.currency='COIN'
		WHERE app_user.id=?`,
		userID,
	).Scan(&id, &nickname, &gender, &signature, &objectKey, &coin, &fans, &follows)
	if err != nil {
		return nil, err
	}
	avatar := ""
	if objectKey != "" {
		avatar = s.mediaBaseURL + "/" + strings.TrimLeft(objectKey, "/")
	}
	return map[string]any{
		"id": strconv.FormatInt(id, 10), "uid": strconv.FormatInt(id, 10), "token": token,
		"user_nickname": nickname, "user_nicename": nickname, "userNiceName": nickname,
		"avatar": avatar, "avatar_thumb": avatar, "sex": strconv.Itoa(gender),
		"signature": signature, "coin": strconv.FormatInt(coin, 10), "votes": "0",
		"follows": strconv.FormatInt(follows, 10), "fans": strconv.FormatInt(fans, 10),
		"level": "1", "liang": map[string]string{"name": ""}, "liang_name": "",
	}, nil
}

func (s *Server) compatWalletBalance(ctx context.Context, userID int64) (map[string]any, error) {
	var coin int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE((SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'),0)`,
		userID,
	).Scan(&coin)
	if err != nil {
		return nil, err
	}
	channelRows, err := s.db.QueryContext(ctx, `
		SELECT channel_key,name FROM payment_channels WHERE status=1 ORDER BY sort_order DESC,id`)
	if err != nil {
		return nil, err
	}
	defer channelRows.Close()
	payList := make([]map[string]any, 0, 4)
	for channelRows.Next() {
		var key, name string
		if err = channelRows.Scan(&key, &name); err != nil {
			return nil, err
		}
		payList = append(payList, map[string]any{"id": key, "name": name, "thumb": "", "href": ""})
	}
	if err = channelRows.Err(); err != nil {
		return nil, err
	}
	productRows, err := s.db.QueryContext(ctx, `
		SELECT product.id,product.coin_amount,product.amount_minor,
		       product.bonus_coin,product.currency_scale
		FROM recharge_products product
		WHERE product.status=1
		  AND EXISTS (
		      SELECT 1
		      FROM payment_channels channel
		      WHERE channel.status=1
		        AND channel.currency=product.fiat_currency
		        AND channel.currency_scale=product.currency_scale
		        AND (channel.min_amount_minor=0
		             OR product.amount_minor>=channel.min_amount_minor)
		        AND (channel.max_amount_minor=0
		             OR product.amount_minor<=channel.max_amount_minor)
		  )
		ORDER BY product.sort_order DESC,product.id`)
	if err != nil {
		return nil, err
	}
	defer productRows.Close()
	rules := make([]map[string]any, 0, 8)
	for productRows.Next() {
		var id, coinAmount, amountMinor, bonus int64
		var scale int
		if err = productRows.Scan(&id, &coinAmount, &amountMinor, &bonus, &scale); err != nil {
			return nil, err
		}
		rules = append(rules, map[string]any{
			"id": id, "coin": coinAmount, "money_minor": amountMinor,
			"money":          formatMinorAmount(amountMinor, scale),
			"currency_scale": scale, "give": bonus,
		})
	}
	if err = productRows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{
		"coin": strconv.FormatInt(coin, 10), "score": "0", "votes": "0",
		"paylist": payList, "rules": rules,
	}, nil
}

func compatCountries(field string) []map[string]any {
	items := []map[string]any{
		{"name": "中国", "name_en": "China", "tel": "86", "index": "Z"},
		{"name": "中国香港", "name_en": "Hong Kong", "tel": "852", "index": "Z"},
		{"name": "中国澳门", "name_en": "Macao", "tel": "853", "index": "Z"},
		{"name": "中国台湾", "name_en": "Taiwan", "tel": "886", "index": "Z"},
		{"name": "新加坡", "name_en": "Singapore", "tel": "65", "index": "X"},
		{"name": "马来西亚", "name_en": "Malaysia", "tel": "60", "index": "M"},
		{"name": "美国", "name_en": "United States", "tel": "1", "index": "M"},
	}
	field = strings.ToLower(strings.TrimSpace(field))
	if field == "" {
		return items
	}
	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		search := strings.ToLower(fmt.Sprint(item["name"]) + " " + fmt.Sprint(item["name_en"]) + " " + fmt.Sprint(item["tel"]))
		if strings.Contains(search, field) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func requestIP(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) appUpdateV2(w http.ResponseWriter, r *http.Request) {
	currentCode, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("version_code")), 10, 64)
	appCode, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("app_code")), 10, 64)
	data, err := s.app.Check(
		r.Context(), currentCode, appCode,
		r.URL.Query().Get("platform"), r.URL.Query().Get("device_id"),
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "检查更新失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), data)
}

func (s *Server) authenticateRequest(r *http.Request) (auth.User, error) {
	userID, token := auth.Bearer(r)
	return s.auth.Authenticate(r.Context(), userID, token)
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status := http.StatusOK
	databaseStatus := "up"
	redisStatus := "up"
	if err := s.db.PingContext(ctx); err != nil {
		status = http.StatusServiceUnavailable
		databaseStatus = "down"
	}
	if s.redis == nil || s.redis.Ping(ctx).Err() != nil {
		status = http.StatusServiceUnavailable
		redisStatus = "down"
	}
	httpx.JSON(w, status, map[string]any{
		"status":   map[bool]string{true: "ok", false: "unhealthy"}[status == http.StatusOK],
		"database": databaseStatus,
		"redis":    redisStatus,
	})
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "接口不存在")
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("http request",
			"request_id", httpx.RequestID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
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
