package admin

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/bankpayment"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/live"
	"github.com/zllyxr/live_claw/backend/internal/paymentconfig"
	"github.com/zllyxr/live_claw/backend/internal/remoteassist"
	"github.com/zllyxr/live_claw/backend/internal/storage"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

const (
	sessionCookie = "claw_admin_session"
	csrfCookie    = "claw_admin_csrf"
)

//go:embed web
var webFiles embed.FS

type Handler struct {
	db                *sql.DB
	auth              *adminauth.Service
	loginTemplate     *template.Template
	appTemplate       *template.Template
	static            http.Handler
	storage           *storage.Service
	wallet            *wallet.Service
	liveProbe         liveSourceProber
	paymentCipher     *paymentconfig.Cipher
	bankPaymentCipher *bankpayment.Cipher
	paymentHTTPClient *http.Client
	mediaBaseURL      string
	publicURL         string
	environment       string
	secureCookies     bool
	remote            *remoteassist.Service
}

type liveSourceProber interface {
	ProbeDouyin(context.Context, string) (live.Source, error)
}

type contextAdminKey struct{}

func New(
	db *sql.DB,
	authService *adminauth.Service,
	storageService *storage.Service,
	walletService *wallet.Service,
	liveProbe liveSourceProber,
	mediaBaseURL string,
	publicURL string,
	environment string,
	dataEncryptionKey string,
	remoteService *remoteassist.Service,
) (*Handler, error) {
	loginBody, err := webFiles.ReadFile("web/login.html")
	if err != nil {
		return nil, err
	}
	appBody, err := webFiles.ReadFile("web/app.html")
	if err != nil {
		return nil, err
	}
	staticFS, err := fs.Sub(webFiles, "web/static")
	if err != nil {
		return nil, err
	}
	paymentCipher, err := paymentconfig.NewCipher(dataEncryptionKey)
	if err != nil {
		return nil, err
	}
	bankPaymentCipher, err := bankpayment.NewCipher(dataEncryptionKey)
	if err != nil {
		return nil, err
	}
	return &Handler{
		db:                db,
		auth:              authService,
		loginTemplate:     template.Must(template.New("login").Parse(string(loginBody))),
		appTemplate:       template.Must(template.New("app").Parse(string(appBody))),
		static:            http.StripPrefix("/admin/static/", http.FileServer(http.FS(staticFS))),
		storage:           storageService,
		wallet:            walletService,
		liveProbe:         liveProbe,
		paymentCipher:     paymentCipher,
		bankPaymentCipher: bankPaymentCipher,
		paymentHTTPClient: newPaymentHTTPClient(),
		mediaBaseURL:      strings.TrimRight(strings.TrimSpace(mediaBaseURL), "/"),
		publicURL:         strings.TrimRight(strings.TrimSpace(publicURL), "/"),
		environment:       strings.ToLower(strings.TrimSpace(environment)),
		secureCookies:     environment != "local" && environment != "development",
		remote:            remoteService,
	}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin", h.root)
	mux.HandleFunc("GET /admin/", h.root)
	mux.HandleFunc("GET /admin/login", h.loginPage)
	mux.HandleFunc("GET /admin/app", h.requirePage(h.appPage))
	mux.Handle("GET /admin/static/", h.static)
	mux.HandleFunc("POST /admin/api/login", h.login)
	mux.HandleFunc("POST /admin/api/logout", h.requireAPI("", true, h.logout))
	mux.HandleFunc("POST /admin/api/csrf", h.requireAPI("", false, h.refreshCSRF))
	mux.HandleFunc("GET /admin/api/me", h.requireAPI("", false, h.me))
	mux.HandleFunc("GET /admin/api/dashboard", h.requireAPI("dashboard.read", false, h.dashboard))
	mux.HandleFunc("GET /admin/api/app/releases", h.requireAPI("app.read", false, h.listAppReleases))
	mux.HandleFunc("POST /admin/api/app/uploads/prepare", h.requireAPI("app.write", true, h.prepareAppUpload))
	mux.HandleFunc("POST /admin/api/app/uploads/finalize", h.requireAPI("app.write", true, h.finalizeAppUpload))
	mux.HandleFunc("POST /admin/api/app/releases", h.requireAPI("app.write", true, h.createAppRelease))
	mux.HandleFunc("PATCH /admin/api/app/releases/{id}", h.requireAPI("app.write", true, h.updateAppRelease))
	mux.HandleFunc("POST /admin/api/app/releases/{id}/publish", h.requireAPI("app.write", true, h.publishAppRelease))
	mux.HandleFunc("POST /admin/api/app/releases/{id}/pause", h.requireAPI("app.write", true, h.pauseAppRelease))
	mux.HandleFunc("POST /admin/api/app/releases/{id}/resume", h.requireAPI("app.write", true, h.resumeAppRelease))
	mux.HandleFunc("POST /admin/api/app/releases/{id}/archive", h.requireAPI("app.write", true, h.archiveAppRelease))
	mux.HandleFunc("GET /admin/api/users", h.requireAPI("users.read", false, h.listUsers))
	mux.HandleFunc("POST /admin/api/users/{id}", h.requireAPI("users.write", true, h.updateUserProfile))
	mux.HandleFunc("POST /admin/api/users/{id}/password", h.requireAPI("users.write", true, h.resetUserPassword))
	mux.HandleFunc("POST /admin/api/users/{id}/status", h.requireAPI("users.write", true, h.setUserStatus))
	mux.HandleFunc("POST /admin/api/users/{id}/team", h.requireAPI("users.write", true, h.assignUserTeam))
	mux.HandleFunc("POST /admin/api/users/{id}/wallet-adjustments", h.requireAPI("wallet.adjust", true, h.adjustUserWallet))
	mux.HandleFunc("GET /admin/api/teams", h.requireAPI("users.read", false, h.listTeams))
	mux.HandleFunc("POST /admin/api/teams", h.requireAPI("users.write", true, h.createTeam))
	mux.HandleFunc("POST /admin/api/teams/{id}", h.requireAPI("users.write", true, h.updateTeam))
	mux.HandleFunc("GET /admin/api/wallet/ledger", h.requireAPI("wallet.read", false, h.listWalletLedger))
	mux.HandleFunc("GET /admin/api/wallet/recharges", h.requireAPI("wallet.read", false, h.listRechargeOrders))
	mux.HandleFunc("GET /admin/api/wallet/withdrawals", h.requireAPI("wallet.read", false, h.listWithdrawOrders))
	mux.HandleFunc("GET /admin/api/wallet/adjustments", h.requireAPI("wallet.read", false, h.listWalletAdjustments))
	mux.HandleFunc("POST /admin/api/wallet/adjustments", h.requireAPI("wallet.review", true, h.createWalletAdjustment))
	mux.HandleFunc("POST /admin/api/wallet/adjustments/{id}/approve", h.requireAPI("wallet.review", true, h.approveWalletAdjustment))
	mux.HandleFunc("POST /admin/api/wallet/adjustments/{id}/reject", h.requireAPI("wallet.review", true, h.rejectWalletAdjustment))
	mux.HandleFunc("POST /admin/api/wallet/recharges/{id}/mark-paid", h.requireAPI("wallet.review", true, h.markRechargePaid))
	mux.HandleFunc("POST /admin/api/wallet/withdrawals/{id}/review", h.requireAPI("wallet.review", true, h.reviewWithdrawal))
	mux.HandleFunc("GET /admin/api/payments/channels", h.requireAPI("payments.read", false, h.listPaymentChannels))
	mux.HandleFunc("POST /admin/api/payments/channels/{id}", h.requireAPI("payments.write", true, h.updatePaymentChannel))
	mux.HandleFunc("POST /admin/api/payments/channels/{id}/status", h.requireAPI("payments.write", true, h.setPaymentChannelStatus))
	mux.HandleFunc("POST /admin/api/payments/channels/{id}/check", h.requireAPI("payments.write", true, h.checkPaymentChannel))
	mux.HandleFunc("GET /admin/api/payments/bank-accounts", h.requireAPI("payments.read", false, h.listBankAccounts))
	mux.HandleFunc("POST /admin/api/payments/bank-accounts", h.requireAPI("payments.write", true, h.createBankAccount))
	mux.HandleFunc("POST /admin/api/payments/bank-accounts/{id}", h.requireAPI("payments.write", true, h.updateBankAccount))
	mux.HandleFunc("POST /admin/api/payments/bank-accounts/{id}/status", h.requireAPI("payments.write", true, h.setBankAccountStatus))
	mux.HandleFunc("GET /admin/api/payments/products", h.requireAPI("payments.read", false, h.listRechargeProducts))
	mux.HandleFunc("POST /admin/api/payments/products", h.requireAPI("payments.write", true, h.createRechargeProduct))
	mux.HandleFunc("POST /admin/api/payments/products/{id}", h.requireAPI("payments.write", true, h.updateRechargeProduct))
	mux.HandleFunc("POST /admin/api/payments/products/{id}/status", h.requireAPI("payments.write", true, h.setRechargeProductStatus))
	mux.HandleFunc("GET /admin/api/payments/recharges", h.requireAPI("payments.read", false, h.listDetailedRechargeOrders))
	mux.HandleFunc("POST /admin/api/payments/recharges/{id}/mark-paid", h.requireAPI("wallet.review", true, h.markRechargePaid))
	mux.HandleFunc("POST /admin/api/payments/recharges/{id}/assign-bank", h.requireAPI("wallet.review", true, h.assignRechargeBankAccount))
	mux.HandleFunc("POST /admin/api/payments/recharges/{id}/close-bank", h.requireAPI("wallet.review", true, h.closeBankRecharge))
	mux.HandleFunc("GET /admin/api/payments/recharges/{id}/bank-proof", h.requireAPI("wallet.review", false, h.bankRechargeProof))
	mux.HandleFunc("POST /admin/api/payments/recharges/{id}/bank-proof/approve", h.requireAPI("wallet.review", true, h.approveBankRechargeProof))
	mux.HandleFunc("POST /admin/api/payments/recharges/{id}/bank-proof/reject", h.requireAPI("wallet.review", true, h.rejectBankRechargeProof))
	mux.HandleFunc("GET /admin/api/games", h.requireAPI("games.read", false, h.listGames))
	mux.HandleFunc("POST /admin/api/games/{id}", h.requireAPI("games.write", true, h.updateGame))
	mux.HandleFunc("POST /admin/api/games/venues/{id}", h.requireAPI("games.write", true, h.updateGameVenue))
	mux.HandleFunc("GET /admin/api/games/settlements", h.requireAPI("games.read", false, h.listGameSettlements))
	mux.HandleFunc("GET /admin/api/live/rooms", h.requireAPI("live.read", false, h.listLiveRooms))
	mux.HandleFunc("GET /admin/api/live/hosts", h.requireAPI("live.write", false, h.listLiveHosts))
	mux.HandleFunc("POST /admin/api/live/rooms", h.requireAPI("live.write", true, h.createLiveRoom))
	mux.HandleFunc("POST /admin/api/live/rooms/{id}", h.requireAPI("live.write", true, h.updateLiveRoom))
	mux.HandleFunc("GET /admin/api/lottery/catalog", h.requireAPI("lottery.read", false, h.listLotteryCatalog))
	mux.HandleFunc("POST /admin/api/lottery/categories", h.requireAPI("lottery.write", true, h.createLotteryCategory))
	mux.HandleFunc("POST /admin/api/lottery/categories/{id}", h.requireAPI("lottery.write", true, h.updateLotteryCategory))
	mux.HandleFunc("POST /admin/api/lottery/games", h.requireAPI("lottery.write", true, h.createLotteryGame))
	mux.HandleFunc("POST /admin/api/lottery/games/{id}", h.requireAPI("lottery.write", true, h.updateLotteryGame))
	mux.HandleFunc("POST /admin/api/lottery/games/{id}/status", h.requireAPI("lottery.write", true, h.setLotteryGameStatus))
	mux.HandleFunc("POST /admin/api/lottery/plays", h.requireAPI("lottery.write", true, h.createLotteryPlay))
	mux.HandleFunc("POST /admin/api/lottery/plays/{id}", h.requireAPI("lottery.write", true, h.updateLotteryPlay))
	mux.HandleFunc("POST /admin/api/lottery/options", h.requireAPI("lottery.write", true, h.createLotteryOption))
	mux.HandleFunc("POST /admin/api/lottery/options/{id}", h.requireAPI("lottery.write", true, h.updateLotteryOption))
	mux.HandleFunc("POST /admin/api/lottery/issues", h.requireAPI("lottery.write", true, h.createLotteryIssue))
	mux.HandleFunc("GET /admin/api/lottery/issues", h.requireAPI("lottery.read", false, h.listLotteryIssues))
	mux.HandleFunc("POST /admin/api/lottery/issues/{id}/close", h.requireAPI("lottery.write", true, h.closeLotteryIssue))
	mux.HandleFunc("POST /admin/api/lottery/issues/{id}/draw", h.requireAPI("lottery.write", true, h.drawLotteryIssue))
	mux.HandleFunc("GET /admin/api/lottery/orders", h.requireAPI("lottery.read", false, h.listLotteryOrders))
	mux.HandleFunc("GET /admin/api/sports/matches", h.requireAPI("sports.read", false, h.listSportsMatches))
	mux.HandleFunc("GET /admin/api/sports/sync", h.requireAPI("sports.read", false, h.sportsSyncStatus))
	mux.HandleFunc("POST /admin/api/sports/matches", h.requireAPI("sports.write", true, h.createSportsMatch))
	mux.HandleFunc("POST /admin/api/sports/matches/{id}", h.requireAPI("sports.write", true, h.updateSportsMatch))
	mux.HandleFunc("POST /admin/api/sports/matches/{id}/settle", h.requireAPI("sports.write", true, h.markSportsSettlementReady))
	mux.HandleFunc("GET /admin/api/sports/matches/{id}/markets", h.requireAPI("sports.read", false, h.listSportsMarkets))
	mux.HandleFunc("POST /admin/api/sports/markets", h.requireAPI("sports.write", true, h.createSportsMarket))
	mux.HandleFunc("POST /admin/api/sports/markets/{id}", h.requireAPI("sports.write", true, h.updateSportsMarket))
	mux.HandleFunc("POST /admin/api/sports/options/{id}", h.requireAPI("sports.write", true, h.updateSportsOption))
	mux.HandleFunc("GET /admin/api/bets/dashboard", h.requireAPI("bets.read", false, h.bettingDashboard))
	mux.HandleFunc("GET /admin/api/bets/lottery", h.requireAPI("bets.read", false, h.listBetLotteryOrders))
	mux.HandleFunc("GET /admin/api/bets/sports", h.requireAPI("bets.read", false, h.listBetSportsOrders))
	mux.HandleFunc("GET /admin/api/bets/game", h.requireAPI("bets.read", false, h.listBetGameOrders))
	mux.HandleFunc("GET /admin/api/im/conversations", h.requireAPI("im.read", false, h.listIMConversations))
	mux.HandleFunc("GET /admin/api/im/conversations/{id}/members", h.requireAPI("im.read", false, h.listIMMembers))
	mux.HandleFunc("GET /admin/api/im/conversations/{id}/messages", h.requireAPI("im.read", false, h.listIMMessages))
	mux.HandleFunc("POST /admin/api/im/conversations/{id}", h.requireAPI("im.moderate", true, h.updateIMGroup))
	mux.HandleFunc("POST /admin/api/im/conversations/{id}/members/{user_id}", h.requireAPI("im.moderate", true, h.moderateIMMember))
	mux.HandleFunc("POST /admin/api/im/messages/{message_id}/delete", h.requireAPI("im.moderate", true, h.deleteIMMessage))
	mux.HandleFunc("GET /admin/api/system/settings", h.requireAPI("system.read", false, h.listSystemSettings))
	mux.HandleFunc("POST /admin/api/system/settings/{key}", h.requireAPI("system.write", true, h.updateSystemSetting))
	mux.HandleFunc("GET /admin/api/system/audit", h.requireAPI("system.read", false, h.listAuditLogs))
	mux.HandleFunc("GET /admin/api/rbac", h.requireAPI("rbac.read", false, h.listRBAC))
	mux.HandleFunc("POST /admin/api/rbac/roles", h.requireAPI("rbac.write", true, h.createRole))
	mux.HandleFunc("POST /admin/api/rbac/roles/{id}", h.requireAPI("rbac.write", true, h.updateRolePermissions))
	mux.HandleFunc("POST /admin/api/rbac/admins", h.requireAPI("rbac.write", true, h.createAdministrator))
	mux.HandleFunc("POST /admin/api/rbac/admins/{id}", h.requireAPI("rbac.write", true, h.updateAdministrator))
	mux.HandleFunc("POST /admin/api/rbac/admins/{id}/password", h.requireAPI("rbac.write", true, h.resetAdministratorPassword))
	mux.HandleFunc("GET /admin/api/remote/devices", h.requireAPI("remote.read", false, h.listRemoteDevices))
	mux.HandleFunc("POST /admin/api/remote/devices/{id}/credential-requests", h.requireAPI("remote.control", true, h.createRemoteCredential))
	mux.HandleFunc("GET /admin/api/remote/credential-requests/{id}", h.requireAPI("remote.control", false, h.remoteCredentialStatus))
	mux.HandleFunc("POST /admin/api/remote/credential-requests/{id}/reveal", h.requireAPI("remote.control", true, h.revealRemoteCredential))
	mux.HandleFunc("GET /admin/api/remote/devices/{id}/frame", h.requireAPI("remote.control", false, h.remoteFrame))
	mux.HandleFunc("POST /admin/api/remote/devices/{id}/control", h.requireAPI("remote.control", true, h.remoteControl))
	mux.HandleFunc("POST /admin/api/remote/devices/{id}/sessions/end", h.requireAPI("remote.control", true, h.endRemoteControl))
	mux.HandleFunc("POST /admin/api/remote/devices/{id}/revoke", h.requireAPI("remote.revoke", true, h.revokeRemoteDevice))
}

func (h *Handler) root(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentAdmin(r); err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/app", http.StatusSeeOther)
}

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentAdmin(r); err == nil {
		http.Redirect(w, r, "/admin/app", http.StatusSeeOther)
		return
	}
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.loginTemplate.Execute(w, nil)
}

func (h *Handler) appPage(w http.ResponseWriter, r *http.Request) {
	adminUser, _ := adminFromRequest(r)
	h.securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.appTemplate.Execute(w, adminUser)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求格式错误")
		return
	}
	session, err := h.auth.Login(r.Context(), request.Username, request.Password, clientIP(r), r.UserAgent())
	if errors.Is(err, adminauth.ErrInvalidCredentials) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "账号或密码错误")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "登录暂不可用")
		return
	}
	var supportOnly int
	err = h.db.QueryRowContext(r.Context(), `
		SELECT support_only FROM support_agents
		WHERE admin_user_id=?`,
		session.Admin.ID,
	).Scan(&supportOnly)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		_ = h.auth.Logout(r.Context(), session.Token)
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "登录暂不可用")
		return
	}
	if supportOnly == 1 {
		_ = h.auth.Logout(r.Context(), session.Token)
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "客服账号请从独立座席端登录")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: session.Token, Path: "/admin",
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: int(time.Until(session.ExpiresAt).Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: session.CSRFToken, Path: "/admin",
		HttpOnly: false, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		Expires: session.ExpiresAt, MaxAge: int(time.Until(session.ExpiresAt).Seconds()),
	})
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"admin": adminIdentityForAPI(session.Admin), "csrf_token": session.CSRFToken,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r)
	if err := h.auth.Logout(r.Context(), token); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "退出失败")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/admin", HttpOnly: true,
		Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Expires: time.Unix(1, 0),
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: "", Path: "/admin", HttpOnly: false,
		Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Expires: time.Unix(1, 0),
	})
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]bool{"logged_out": true})
}

func (h *Handler) refreshCSRF(w http.ResponseWriter, r *http.Request) {
	csrf, err := h.auth.RefreshCSRF(r.Context(), sessionToken(r))
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "请重新登录")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: csrf, Path: "/admin", HttpOnly: false,
		Secure: h.secureCookies, SameSite: http.SameSiteStrictMode,
		MaxAge: int((12 * time.Hour).Seconds()),
	})
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]string{"csrf_token": csrf})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	adminUser, _ := adminFromRequest(r)
	httpx.OK(w, httpx.RequestID(r.Context()), adminIdentityForAPI(adminUser))
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	var result struct {
		Users         int64 `json:"users"`
		ActiveUsers   int64 `json:"active_users"`
		WalletCoin    int64 `json:"wallet_coin"`
		FrozenCoin    int64 `json:"frozen_coin"`
		PendingTopups int64 `json:"pending_topups"`
		PendingPayout int64 `json:"pending_withdrawals"`
		LiveRooms     int64 `json:"live_rooms"`
		IMGroups      int64 `json:"im_groups"`
		TodayGames    int64 `json:"today_game_settlements"`
		SportsMatches int64 `json:"sports_matches"`
		LotteryBets   int64 `json:"pending_lottery_bets"`
		SportsBets    int64 `json:"pending_sports_bets"`
	}
	err := h.db.QueryRowContext(r.Context(), `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM users WHERE status=1),
			(SELECT COALESCE(SUM(available),0) FROM wallet_accounts WHERE currency='COIN'),
			(SELECT COALESCE(SUM(frozen),0) FROM wallet_accounts WHERE currency='COIN'),
			(SELECT COUNT(*) FROM recharge_orders WHERE status IN (0,1)),
			(SELECT COUNT(*) FROM withdraw_orders WHERE status IN (0,1,2)),
			(SELECT COUNT(*) FROM live_rooms WHERE status=1),
			(SELECT COUNT(*) FROM im_groups WHERE dissolved_at IS NULL),
			(SELECT COUNT(*) FROM game_settlements
			 WHERE created_at>=CURRENT_DATE AND created_at<CURRENT_DATE+INTERVAL 1 DAY),
			(SELECT COUNT(*) FROM sports_matches
			 WHERE match_status IN ('NS','LIVE','HT')),
			(SELECT COUNT(*) FROM lottery_bet_orders WHERE status=0),
			(SELECT COUNT(*) FROM sports_bet_orders WHERE status=0)`,
	).Scan(
		&result.Users, &result.ActiveUsers, &result.WalletCoin, &result.FrozenCoin,
		&result.PendingTopups, &result.PendingPayout, &result.LiveRooms,
		&result.IMGroups, &result.TodayGames, &result.SportsMatches,
		&result.LotteryBets, &result.SportsBets,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "统计暂不可用")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), result)
}

func (h *Handler) requirePage(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminUser, err := h.currentAdmin(r)
		if err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r.WithContext(withAdmin(r, adminUser)))
	}
}

func (h *Handler) requireAPI(permission string, requireCSRF bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminUser, err := h.currentAdmin(r)
		if err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusUnauthorized, 401, "请重新登录")
			return
		}
		if permission != "" && !hasPermission(adminUser, permission) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "无权执行此操作")
			return
		}
		if requireCSRF && !h.auth.VerifyCSRF(r.Context(), sessionToken(r), r.Header.Get("X-CSRF-Token")) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusForbidden, 403, "请求校验失败")
			return
		}
		next(w, r.WithContext(withAdmin(r, adminUser)))
	}
}

func (h *Handler) currentAdmin(r *http.Request) (adminauth.Admin, error) {
	adminUser, err := h.auth.Authenticate(r.Context(), sessionToken(r))
	if err != nil {
		return adminauth.Admin{}, err
	}
	var supportOnly int
	err = h.db.QueryRowContext(r.Context(), `
		SELECT support_only FROM support_agents WHERE admin_user_id=?`,
		adminUser.ID,
	).Scan(&supportOnly)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return adminauth.Admin{}, err
	}
	if supportOnly == 1 {
		return adminauth.Admin{}, adminauth.ErrInvalidCredentials
	}
	return adminUser, nil
}

func (h *Handler) securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func hasPermission(adminUser adminauth.Admin, permission string) bool {
	for _, granted := range adminUser.Permissions {
		if granted == permission {
			return true
		}
	}
	return false
}

func withAdmin(r *http.Request, adminUser adminauth.Admin) context.Context {
	return context.WithValue(r.Context(), contextAdminKey{}, adminUser)
}

func adminFromRequest(r *http.Request) (adminauth.Admin, bool) {
	adminUser, ok := r.Context().Value(contextAdminKey{}).(adminauth.Admin)
	return adminUser, ok
}

func clientIP(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
