package gameserver

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zllyxr/live_claw/backend/internal/game"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
)

//go:embed fishingassets
var fishingAssets embed.FS

var fishingAssetHandler = func() http.Handler {
	root, err := fs.Sub(fishingAssets, "fishingassets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/minigame/fish/", http.FileServer(http.FS(root)))
}()

type fishingHub struct {
	game           *game.Service
	logger         *slog.Logger
	lifecycleLocks [64]sync.Mutex
	mu             sync.Mutex
	rooms          map[string]*fishingRoom
	hits           chan fishingHit
}

type fishingRoom struct {
	id             string
	startedAt      time.Time
	lastAdvancedAt time.Time
	sequence       int64
	players        map[string]*fishingSocketPlayer
	fishEpoch      []int64
	bullets        map[string]*fishingBullet
}

type fishingSocketPlayer struct {
	profile  game.FishingPlayer
	conn     *websocket.Conn
	writeMu  sync.Mutex
	angle    float64
	bet      int64
	eventSeq int64
	fireRate fishingFireLimiter
}

type fishingSocketMessage struct {
	Event     string          `json:"event"`
	RequestID string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
}

type fishingSocketEnvelope struct {
	Event     string `json:"event"`
	RequestID string `json:"requestId,omitempty"`
	Data      any    `json:"data"`
}

type fishingFish struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	AssetKey   string  `json:"assetKey"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Angle      float64 `json:"angle"`
	Scale      float64 `json:"scale"`
	Radius     float64 `json:"radius"`
	Multiplier int     `json:"multiplier"`
	Reward     int     `json:"reward"`
	Tier       string  `json:"tier"`
}

type fishingBullet struct {
	ID         string         `json:"id"`
	CommandID  string         `json:"commandId"`
	OwnerID    string         `json:"ownerId"`
	UserID     int64          `json:"-"`
	PlayerName string         `json:"playerName"`
	X          float64        `json:"x"`
	Y          float64        `json:"y"`
	VX         float64        `json:"vx"`
	VY         float64        `json:"vy"`
	Angle      float64        `json:"angle"`
	Radius     float64        `json:"radius"`
	Power      int64          `json:"power"`
	Path       []fishingPoint `json:"path,omitempty"`
}

type fishingPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type fishingHit struct {
	roomID    string
	bullet    fishingBullet
	fish      fishingFish
	fishIndex int
}

var fishingSpecies = []struct {
	key        string
	multiplier int
	radius     float64
	scale      float64
	speed      float64
}{
	{"tuna", 2, 22, 1.15, 44},
	{"lionfish", 3, 26, 1.20, 37},
	{"puffer", 5, 28, 1.16, 34},
	{"grouper", 8, 31, 1.24, 30},
	{"turtle", 12, 38, 1.05, 25},
	{"manta", 20, 44, 1.12, 22},
	{"hammerhead", 30, 52, 0.78, 19},
	{"octopus", 40, 46, 0.50, 21},
	{"orca", 60, 58, 0.72, 17},
	{"anglerfish", 80, 50, 1.05, 18},
}

const (
	fishingFireTokensPerSecond = 9.0
	fishingFireBurst           = 2.0
	fishingBulletSpeed         = 820.0
	fishingArenaWidth          = 1280.0
	fishingArenaHeight         = 720.0
)

type fishingFireLimiter struct {
	tokens       float64
	lastRefilled time.Time
}

func (limiter *fishingFireLimiter) allow(now time.Time) bool {
	if limiter.lastRefilled.IsZero() {
		limiter.tokens = fishingFireBurst
		limiter.lastRefilled = now
	} else {
		elapsed := now.Sub(limiter.lastRefilled).Seconds()
		if elapsed > 0 {
			limiter.tokens = math.Min(
				fishingFireBurst,
				limiter.tokens+elapsed*fishingFireTokensPerSecond,
			)
			limiter.lastRefilled = now
		}
	}
	if limiter.tokens < 1 {
		return false
	}
	limiter.tokens--
	return true
}

func newFishingHub(gameService *game.Service, logger *slog.Logger) *fishingHub {
	hub := &fishingHub{
		game: gameService, logger: logger, rooms: make(map[string]*fishingRoom),
		hits: make(chan fishingHit, 1024),
	}
	go hub.run()
	go hub.resolveHits()
	return hub
}

func (h *fishingHub) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for now := range ticker.C {
		for _, hit := range h.advanceFishingBullets(now) {
			h.hits <- hit
		}
		h.broadcastSnapshots(now)
	}
}

func (h *fishingHub) resolveHits() {
	for hit := range h.hits {
		h.resolveFishingHit(hit)
	}
}

func (s *Server) fishingIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/minigame/fish/", http.StatusTemporaryRedirect)
}

func (s *Server) fishingSocket(w http.ResponseWriter, r *http.Request) {
	profile, err := s.game.AuthenticateFishingSession(
		r.Context(), r.URL.Query().Get("session"), r.URL.Query().Get("resume"),
	)
	if err != nil {
		http.Error(w, "捕鱼会话已失效，请返回大厅重新进入", http.StatusUnauthorized)
		return
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 16384,
		CheckOrigin:     fishingOriginAllowed,
	}
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.fishing.serve(connection, profile)
}

func fishingOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	originHost := parsed.Hostname()
	requestHost, _, splitErr := net.SplitHostPort(r.Host)
	if splitErr != nil {
		requestHost = r.Host
	}
	return isLoopbackHost(originHost) && isLoopbackHost(requestHost)
}

func isLoopbackHost(value string) bool {
	value = strings.Trim(strings.ToLower(strings.TrimSpace(value)), "[]")
	return value == "localhost" || value == "127.0.0.1" || value == "::1"
}

func (h *fishingHub) serve(connection *websocket.Conn, profile game.FishingPlayer) {
	player := &fishingSocketPlayer{
		profile: profile, conn: connection,
		angle: seatFacing(profile.Seat - 1), bet: profile.BetLevels[0],
		eventSeq: profile.EventSeq,
	}
	lifecycle := h.lifecycleLock(profile.SessionID)
	lifecycle.Lock()
	room := h.add(player)
	connectContext, cancelConnect := context.WithTimeout(context.Background(), 3*time.Second)
	connectErr := h.game.MarkFishingConnected(
		connectContext, profile.SessionID, profile.UserID,
	)
	cancelConnect()
	if connectErr != nil {
		h.remove(room.id, player)
	}
	lifecycle.Unlock()
	if connectErr != nil {
		if h.logger != nil {
			h.logger.Error(
				"fishing connect",
				"error", connectErr,
				"session_id", profile.SessionID,
				"user_id", profile.UserID,
			)
		}
		player.write(fishingSocketEnvelope{
			Event: "fatal",
			Data:  map[string]any{"message": "捕鱼会话已结束，请返回大厅重新进入"},
		})
		connection.Close() //nolint:errcheck
		return
	}
	defer func() {
		lifecycle.Lock()
		removed := h.remove(room.id, player)
		if removed {
			disconnectContext, cancelDisconnect := context.WithTimeout(context.Background(), 3*time.Second)
			h.game.MarkFishingDisconnected(
				disconnectContext, profile.SessionID, profile.UserID,
			)
			cancelDisconnect()
		}
		lifecycle.Unlock()
		connection.Close() //nolint:errcheck
	}()
	connection.SetReadLimit(16 << 10)
	h.broadcast(room.id, "room:notice", map[string]any{
		"type": "player-joined", "playerId": profile.SessionID,
		"name": profile.Name, "message": profile.Name + " 加入了海域",
	})
	for {
		var message fishingSocketMessage
		if err := connection.ReadJSON(&message); err != nil {
			return
		}
		switch message.Event {
		case "room:join":
			h.ack(player, message.RequestID, map[string]any{
				"ok": true, "roomId": room.id, "playerId": profile.SessionID,
				"seat": profile.Seat - 1, "resumeToken": "",
				"state": h.snapshot(room.id, time.Now()), "resumed": true,
			})
		case "player:aim":
			var payload struct {
				Angle float64 `json:"angle"`
			}
			if json.Unmarshal(message.Data, &payload) == nil && !math.IsNaN(payload.Angle) &&
				!math.IsInf(payload.Angle, 0) {
				h.mu.Lock()
				player.angle = payload.Angle
				h.mu.Unlock()
			}
		case "player:power", "player:bet":
			var payload struct {
				Power int64 `json:"power"`
				Bet   int64 `json:"bet"`
			}
			if json.Unmarshal(message.Data, &payload) == nil {
				next := payload.Power
				if next == 0 {
					next = payload.Bet
				}
				if containsBet(profile.BetLevels, next) {
					h.mu.Lock()
					player.bet = next
					h.mu.Unlock()
				}
			}
		case "player:fire":
			h.fire(player, message)
		case "session:leave":
			entry, leaveErr := h.game.LeaveFishing(
				contextWithoutCancel(), profile.UserID, profile.SessionID,
			)
			if leaveErr != nil {
				h.ack(player, message.RequestID, fishingFailure("LEAVE_FAILED", "捕鱼余额结算失败"))
				continue
			}
			h.ack(player, message.RequestID, map[string]any{
				"ok": true, "available": entry.Available, "frozen": entry.Frozen,
			})
		}
	}
}

func (h *fishingHub) lifecycleLock(sessionID string) *sync.Mutex {
	hash := uint32(2166136261)
	for index := 0; index < len(sessionID); index++ {
		hash ^= uint32(sessionID[index])
		hash *= 16777619
	}
	return &h.lifecycleLocks[hash%uint32(len(h.lifecycleLocks))]
}

func (h *fishingHub) add(player *fishingSocketPlayer) *fishingRoom {
	h.mu.Lock()
	defer h.mu.Unlock()
	roomID := player.profile.RoomID()
	room := h.rooms[roomID]
	if room == nil {
		room = &fishingRoom{
			id: roomID, startedAt: time.Now(), lastAdvancedAt: time.Now(),
			players:   make(map[string]*fishingSocketPlayer),
			fishEpoch: make([]int64, 18),
			bullets:   make(map[string]*fishingBullet),
		}
		h.rooms[roomID] = room
	}
	if previous := room.players[player.profile.SessionID]; previous != nil {
		previous.conn.Close() //nolint:errcheck
	}
	room.players[player.profile.SessionID] = player
	return room
}

func (h *fishingHub) remove(roomID string, player *fishingSocketPlayer) bool {
	h.mu.Lock()
	room := h.rooms[roomID]
	if room == nil || room.players[player.profile.SessionID] != player {
		h.mu.Unlock()
		return false
	}
	delete(room.players, player.profile.SessionID)
	noPlayers := len(room.players) == 0
	if noPlayers && len(room.bullets) == 0 {
		delete(h.rooms, roomID)
	}
	h.mu.Unlock()
	if player != nil && !noPlayers {
		h.broadcast(roomID, "room:notice", map[string]any{
			"type": "player-left", "playerId": player.profile.SessionID,
			"name": player.profile.Name, "message": player.profile.Name + " 离开了海域",
		})
	}
	return true
}

func (h *fishingHub) fire(player *fishingSocketPlayer, message fishingSocketMessage) {
	var payload struct {
		CommandID string  `json:"commandId"`
		Angle     float64 `json:"angle"`
	}
	if err := json.Unmarshal(message.Data, &payload); err != nil {
		h.ack(player, message.RequestID, fishingFailure("BAD_REQUEST", "开炮参数错误"))
		return
	}
	h.mu.Lock()
	if !player.fireRate.allow(time.Now()) {
		h.mu.Unlock()
		h.ack(player, message.RequestID, fishingFailure("RATE_LIMITED", "开炮速度过快"))
		return
	}
	if !math.IsNaN(payload.Angle) && !math.IsInf(payload.Angle, 0) {
		player.angle = payload.Angle
	}
	room := h.rooms[player.profile.RoomID()]
	if room == nil {
		h.mu.Unlock()
		h.ack(player, message.RequestID, fishingFailure("NOT_JOINED", "捕鱼房间不存在"))
		return
	}
	angle := player.angle
	bet := player.bet
	h.mu.Unlock()

	// Launches are charged immediately, but the capture chance is deliberately
	// deferred until the authoritative projectile physically touches a fish.
	result, err := h.game.FireFishing(
		contextWithoutCancel(), player.profile.SessionID, player.profile.UserID,
		payload.CommandID, bet, 0,
	)
	if err != nil {
		code, label := "FIRE_FAILED", "开炮失败"
		switch {
		case errors.Is(err, wallet.ErrInsufficientFunds):
			code, label = "INSUFFICIENT_FUNDS", "捕鱼托管余额不足"
		case errors.Is(err, game.ErrSessionNotFound):
			code, label = "SESSION_EXPIRED", "捕鱼会话已失效，请重新进入"
		case errors.Is(err, game.ErrInvalidFishingCannon):
			code, label = "INVALID_BET", "当前炮值不可用，请重新选择"
		default:
			if h.logger != nil {
				h.logger.Error(
					"fishing fire",
					"error", err,
					"session_id", player.profile.SessionID,
					"user_id", player.profile.UserID,
					"command_id", payload.CommandID,
				)
			}
		}
		h.ack(player, message.RequestID, fishingFailure(code, label))
		return
	}
	h.mu.Lock()
	if result.EventSeq >= player.eventSeq {
		player.eventSeq = result.EventSeq
		player.profile.EscrowBalance = result.Balance
	}
	activePlayer := room.players[player.profile.SessionID]
	if activePlayer != nil && activePlayer != player && result.EventSeq >= activePlayer.eventSeq {
		activePlayer.eventSeq = result.EventSeq
		activePlayer.profile.EscrowBalance = result.Balance
	}
	currentBalance := player.profile.EscrowBalance
	if !result.Replayed {
		originX, originY := seatOrigin(player.profile.Seat - 1)
		radius := fishingBulletRadius(result.Bet)
		originX += math.Cos(angle) * 72
		originY += math.Sin(angle) * 72
		originX = math.Max(radius, math.Min(fishingArenaWidth-radius, originX))
		originY = math.Max(radius, math.Min(fishingArenaHeight-radius, originY))
		room.bullets[result.CommandID] = &fishingBullet{
			ID: result.CommandID, CommandID: result.CommandID,
			OwnerID: player.profile.SessionID, UserID: player.profile.UserID,
			PlayerName: player.profile.Name,
			X:          originX, Y: originY,
			VX:    math.Cos(angle) * fishingBulletSpeed,
			VY:    math.Sin(angle) * fishingBulletSpeed,
			Angle: angle, Radius: radius, Power: result.Bet,
		}
	}
	h.mu.Unlock()
	h.ack(player, message.RequestID, map[string]any{
		"ok": true, "commandId": result.CommandID, "shotId": result.CommandID,
		"bet": result.Bet, "power": result.Bet, "score": currentBalance,
		"replayed": result.Replayed,
	})
}

func (h *fishingHub) advanceFishingBullets(now time.Time) []fishingHit {
	h.mu.Lock()
	defer h.mu.Unlock()
	hits := make([]fishingHit, 0)
	for _, room := range h.rooms {
		elapsed := now.Sub(room.lastAdvancedAt).Seconds()
		room.lastAdvancedAt = now
		if elapsed <= 0 {
			continue
		}
		// Avoid a process pause turning one physics update into an unbounded
		// collision loop. Projectiles remain alive; only excess elapsed wall
		// time is discarded.
		elapsed = math.Min(elapsed, 0.25)
		fishes := fishingFishes(room, now)
		for id, bullet := range room.bullets {
			fishIndex := advanceFishingBullet(bullet, fishes, elapsed)
			if fishIndex < 0 {
				continue
			}
			hits = append(hits, fishingHit{
				roomID: room.id, bullet: *bullet,
				fish: fishes[fishIndex], fishIndex: fishIndex,
			})
			delete(room.bullets, id)
		}
		if len(room.players) == 0 && len(room.bullets) == 0 {
			delete(h.rooms, room.id)
		}
	}
	return hits
}

func advanceFishingBullet(bullet *fishingBullet, fishes []fishingFish, elapsed float64) int {
	if bullet == nil || elapsed <= 0 {
		return -1
	}
	bullet.X = math.Max(bullet.Radius, math.Min(fishingArenaWidth-bullet.Radius, bullet.X))
	bullet.Y = math.Max(bullet.Radius, math.Min(fishingArenaHeight-bullet.Radius, bullet.Y))
	bullet.Path = append(bullet.Path[:0], fishingPoint{X: bullet.X, Y: bullet.Y})
	remaining := elapsed
	for segment := 0; segment < 12 && remaining > 0.000001; segment++ {
		timeX := fishingBoundaryTime(
			bullet.X, bullet.VX, bullet.Radius, fishingArenaWidth-bullet.Radius,
		)
		timeY := fishingBoundaryTime(
			bullet.Y, bullet.VY, bullet.Radius, fishingArenaHeight-bullet.Radius,
		)
		step := remaining
		if timeX < step {
			step = timeX
		}
		if timeY < step {
			step = timeY
		}
		if step < 0 {
			step = 0
		}
		hitTime, fishIndex := firstFishingBulletHit(bullet, fishes, step)
		if fishIndex >= 0 {
			bullet.X += bullet.VX * hitTime
			bullet.Y += bullet.VY * hitTime
			bullet.Path = appendFishingPathPoint(bullet.Path, bullet.X, bullet.Y)
			bullet.Angle = math.Atan2(bullet.VY, bullet.VX)
			return fishIndex
		}
		bullet.X += bullet.VX * step
		bullet.Y += bullet.VY * step
		bullet.Path = appendFishingPathPoint(bullet.Path, bullet.X, bullet.Y)
		remaining -= step

		bounced := false
		if timeX <= step+0.000001 {
			bullet.X = math.Max(bullet.Radius, math.Min(fishingArenaWidth-bullet.Radius, bullet.X))
			bullet.VX = -bullet.VX
			bounced = true
		}
		if timeY <= step+0.000001 {
			bullet.Y = math.Max(bullet.Radius, math.Min(fishingArenaHeight-bullet.Radius, bullet.Y))
			bullet.VY = -bullet.VY
			bounced = true
		}
		bullet.Angle = math.Atan2(bullet.VY, bullet.VX)
		if !bounced {
			break
		}
	}
	return -1
}

func appendFishingPathPoint(path []fishingPoint, x, y float64) []fishingPoint {
	if len(path) > 0 {
		last := path[len(path)-1]
		if math.Abs(last.X-x) < 0.000001 && math.Abs(last.Y-y) < 0.000001 {
			return path
		}
	}
	return append(path, fishingPoint{X: x, Y: y})
}

func fishingBoundaryTime(position, velocity, minimum, maximum float64) float64 {
	if velocity > 0.000001 {
		return math.Max(0, (maximum-position)/velocity)
	}
	if velocity < -0.000001 {
		return math.Max(0, (minimum-position)/velocity)
	}
	return math.Inf(1)
}

func firstFishingBulletHit(
	bullet *fishingBullet,
	fishes []fishingFish,
	duration float64,
) (float64, int) {
	if duration < 0 || bullet == nil {
		return 0, -1
	}
	speedSquared := bullet.VX*bullet.VX + bullet.VY*bullet.VY
	if speedSquared < 0.000001 {
		return 0, -1
	}
	firstTime := math.Inf(1)
	firstIndex := -1
	for index := range fishes {
		radius := bullet.Radius + fishingFishCollisionRadius(fishes[index])
		offsetX := bullet.X - fishes[index].X
		offsetY := bullet.Y - fishes[index].Y
		c := offsetX*offsetX + offsetY*offsetY - radius*radius
		hitTime := 0.0
		if c > 0 {
			b := 2 * (offsetX*bullet.VX + offsetY*bullet.VY)
			discriminant := b*b - 4*speedSquared*c
			if discriminant < 0 {
				continue
			}
			hitTime = (-b - math.Sqrt(discriminant)) / (2 * speedSquared)
			if hitTime < 0 {
				continue
			}
		}
		if hitTime <= duration+0.000001 && hitTime < firstTime {
			firstTime = hitTime
			firstIndex = index
		}
	}
	return firstTime, firstIndex
}

func fishingFishCollisionRadius(fish fishingFish) float64 {
	radius := fish.Radius * fish.Scale * 1.9
	if fish.Multiplier >= 30 {
		radius *= 1.1
	}
	return math.Max(30, radius)
}

func fishingBulletRadius(power int64) float64 {
	return 6 + math.Min(3, math.Sqrt(float64(power))*0.35)
}

func (h *fishingHub) resolveFishingHit(hit fishingHit) {
	multiplier := hit.fish.Multiplier
	h.mu.Lock()
	room := h.rooms[hit.roomID]
	if room != nil && (hit.fishIndex < 0 || hit.fishIndex >= len(room.fishEpoch) ||
		hit.fish.ID != fishingFishID(hit.fishIndex, room.fishEpoch[hit.fishIndex])) {
		// Another projectile already captured this exact fish before this hit
		// could settle. The physical hit and net still count, but it cannot pay
		// the same fish twice.
		multiplier = 0
	}
	h.mu.Unlock()

	var result game.FishingFireResult
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		result, err = h.game.ResolveFishingHit(
			contextWithoutCancel(), hit.bullet.OwnerID, hit.bullet.UserID,
			hit.bullet.CommandID, multiplier,
		)
		if err == nil || errors.Is(err, game.ErrSessionNotFound) {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
	}
	if err != nil && h.logger != nil {
		h.logger.Error(
			"fishing hit resolution",
			"error", err,
			"session_id", hit.bullet.OwnerID,
			"user_id", hit.bullet.UserID,
			"command_id", hit.bullet.CommandID,
			"fish_id", hit.fish.ID,
		)
	}

	score := int64(0)
	captured := false
	reward := int64(0)
	resolvedMultiplier := multiplier
	h.mu.Lock()
	room = h.rooms[hit.roomID]
	if err == nil {
		captured = result.Captured
		reward = result.Reward
		resolvedMultiplier = result.Multiplier
		score = result.Balance
		if room != nil {
			if player := room.players[hit.bullet.OwnerID]; player != nil &&
				result.EventSeq >= player.eventSeq {
				player.eventSeq = result.EventSeq
				player.profile.EscrowBalance = result.Balance
			}
			if captured && hit.fishIndex >= 0 && hit.fishIndex < len(room.fishEpoch) &&
				hit.fish.ID == fishingFishID(hit.fishIndex, room.fishEpoch[hit.fishIndex]) {
				room.fishEpoch[hit.fishIndex]++
			}
		}
	} else if room != nil {
		if player := room.players[hit.bullet.OwnerID]; player != nil {
			score = player.profile.EscrowBalance
		}
	}
	h.mu.Unlock()

	event := map[string]any{
		"roomId": hit.roomID, "playerId": hit.bullet.OwnerID,
		"playerName": hit.bullet.PlayerName,
		"commandId":  hit.bullet.CommandID, "shotId": hit.bullet.CommandID,
		"captured": captured, "multiplier": resolvedMultiplier,
		"bet": hit.bullet.Power, "power": hit.bullet.Power,
		"reward": reward, "payout": reward, "score": score,
		"x": hit.bullet.X, "y": hit.bullet.Y,
		"fishId": hit.fish.ID, "type": hit.fish.Type,
		"fishType": hit.fish.Type, "assetKey": hit.fish.AssetKey,
		"serverTime": time.Now().UnixMilli(),
	}
	h.broadcast(hit.roomID, "shot:resolved", event)
	if captured {
		h.broadcast(hit.roomID, "game:catch", event)
	} else {
		h.broadcast(hit.roomID, "game:catch-failed", event)
	}
}

func fishingFailure(code string, message string) map[string]any {
	return map[string]any{
		"ok":    false,
		"error": map[string]any{"code": code, "message": message},
	}
}

func (h *fishingHub) ack(player *fishingSocketPlayer, requestID string, data any) {
	if requestID == "" {
		return
	}
	player.write(fishingSocketEnvelope{Event: "ack", RequestID: requestID, Data: data})
}

func (h *fishingHub) broadcast(roomID string, event string, data any) {
	h.mu.Lock()
	room := h.rooms[roomID]
	players := make([]*fishingSocketPlayer, 0, 4)
	if room != nil {
		for _, player := range room.players {
			players = append(players, player)
		}
	}
	h.mu.Unlock()
	for _, player := range players {
		player.write(fishingSocketEnvelope{Event: event, Data: data})
	}
}

func (h *fishingHub) broadcastSnapshots(now time.Time) {
	type roomBroadcast struct {
		players  []*fishingSocketPlayer
		snapshot map[string]any
	}
	h.mu.Lock()
	broadcasts := make([]roomBroadcast, 0, len(h.rooms))
	for _, room := range h.rooms {
		room.sequence++
		players := make([]*fishingSocketPlayer, 0, len(room.players))
		for _, player := range room.players {
			players = append(players, player)
		}
		broadcasts = append(broadcasts, roomBroadcast{
			players: players, snapshot: fishingSnapshot(room, now),
		})
	}
	h.mu.Unlock()
	for _, broadcast := range broadcasts {
		envelope := fishingSocketEnvelope{Event: "game:snapshot", Data: broadcast.snapshot}
		for _, player := range broadcast.players {
			player.write(envelope)
		}
	}
}

func (h *fishingHub) snapshot(roomID string, now time.Time) map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[roomID]
	if room == nil {
		return nil
	}
	return fishingSnapshot(room, now)
}

func fishingSnapshot(room *fishingRoom, now time.Time) map[string]any {
	players := make([]map[string]any, 0, len(room.players))
	for _, player := range room.players {
		seat := player.profile.Seat - 1
		x, y := seatOrigin(seat)
		players = append(players, map[string]any{
			"id": player.profile.SessionID, "name": player.profile.Name,
			"seat": seat, "score": player.profile.EscrowBalance,
			"bet": player.bet, "power": player.bet, "angle": player.angle,
			"betLevels": player.profile.BetLevels,
			"color":     []string{"#55e7f0", "#f5c65f", "#ff7188", "#a88cff"}[seat%4],
			"x":         x, "y": y,
		})
	}
	bullets := make([]fishingBullet, 0, len(room.bullets))
	for _, bullet := range room.bullets {
		item := *bullet
		item.Path = append([]fishingPoint(nil), bullet.Path...)
		bullets = append(bullets, item)
	}
	return map[string]any{
		"seq": room.sequence, "serverTime": now.UnixMilli(), "roomId": room.id,
		"width": 1280, "height": 720, "seatLayout": "four-seat-top-bottom",
		"players": players, "fishes": fishingFishes(room, now), "bullets": bullets,
	}
}

func fishingFishes(room *fishingRoom, now time.Time) []fishingFish {
	elapsed := now.Sub(room.startedAt).Seconds()
	items := make([]fishingFish, 0, len(room.fishEpoch))
	for index, epoch := range room.fishEpoch {
		species := fishingSpecies[(index+int(epoch))%len(fishingSpecies)]
		direction := 1.0
		if index%2 == 1 {
			direction = -1
		}
		progress := math.Mod(elapsed*species.speed+float64(index*97)+float64(epoch*211), 1510) - 115
		x := progress
		angle := 0.0
		if direction < 0 {
			x = 1280 - progress
			angle = math.Pi
		}
		y := 105 + float64(index%6)*88 + math.Sin(elapsed*0.65+float64(index)*1.37)*32
		tier := "common"
		if species.multiplier >= 30 {
			tier = "boss"
		}
		items = append(items, fishingFish{
			ID: fishingFishID(index, epoch), Type: species.key, AssetKey: species.key,
			X: math.Round(x*100) / 100, Y: math.Round(y*100) / 100,
			Angle: angle, Scale: species.scale, Radius: species.radius,
			Multiplier: species.multiplier, Reward: species.multiplier, Tier: tier,
		})
	}
	return items
}

func fishingFishID(index int, epoch int64) string {
	return "fish-" + strconv.Itoa(index) + "-" + strconv.FormatInt(epoch, 10)
}

func nearestFishingTarget(seat int, angle float64, fishes []fishingFish) *fishingFish {
	originX, originY := seatOrigin(seat)
	bestIndex := -1
	bestDelta := math.MaxFloat64
	for index := range fishes {
		targetAngle := math.Atan2(fishes[index].Y-originY, fishes[index].X-originX)
		delta := math.Abs(math.Atan2(math.Sin(targetAngle-angle), math.Cos(targetAngle-angle)))
		if delta < bestDelta {
			bestDelta = delta
			bestIndex = index
		}
	}
	if bestIndex < 0 || bestDelta > 0.32 {
		return nil
	}
	return &fishes[bestIndex]
}

func fishingRayExitPoint(seat int, angle float64) (float64, float64) {
	originX, originY := seatOrigin(seat)
	dx, dy := math.Cos(angle), math.Sin(angle)
	distance := math.Inf(1)
	for _, candidate := range []float64{
		rayBoundaryDistance(originX, dx, 0),
		rayBoundaryDistance(originX, dx, 1280),
		rayBoundaryDistance(originY, dy, 0),
		rayBoundaryDistance(originY, dy, 720),
	} {
		if candidate > 0 && candidate < distance {
			distance = candidate
		}
	}
	if math.IsInf(distance, 1) {
		return originX, originY
	}
	return originX + dx*distance, originY + dy*distance
}

func rayBoundaryDistance(origin float64, direction float64, boundary float64) float64 {
	if math.Abs(direction) < 0.0001 {
		return math.Inf(1)
	}
	return (boundary - origin) / direction
}

func seatOrigin(seat int) (float64, float64) {
	switch seat {
	case 1:
		return 850, 690
	case 2:
		return 430, 30
	case 3:
		return 850, 30
	default:
		return 430, 690
	}
}

func seatFacing(seat int) float64 {
	if seat >= 2 {
		return math.Pi / 2
	}
	return -math.Pi / 2
}

func containsBet(levels []int64, value int64) bool {
	for _, level := range levels {
		if level == value {
			return true
		}
	}
	return false
}

func (p *fishingSocketPlayer) write(envelope fishingSocketEnvelope) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_ = p.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_ = p.conn.WriteJSON(envelope)
}

func contextWithoutCancel() context.Context {
	return context.Background()
}
