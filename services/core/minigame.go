package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MiniGameService 提供小游戏目录与进入鉴权。
//
// 设计目标：新增游戏只需往 cmf_minigame 插一行，客户端无需发版。
// 客户端读 MiniGame.list 渲染入口，点击时调 MiniGame.enter 换取带签名的启动地址。
type MiniGameService struct {
	db      *sql.DB
	secret  string
	matcher *tableMatcher
	logger  *slog.Logger
}

func NewMiniGameService(db *sql.DB, secret string, tableCount int, logger *slog.Logger) *MiniGameService {
	return &MiniGameService{
		db: db, secret: secret, matcher: newTableMatcher(tableCount), logger: logger,
	}
}

const (
	// 真人在短窗口内优先聚合到同桌；窗口结束后为下一位玩家分配新桌，
	// 避免机器人已经补齐并开局后，新玩家仍拿到同一张满桌。
	matchHumanWindow = 1200 * time.Millisecond
	matchTicketTTL   = 30 * time.Minute
)

type tableAssignment struct {
	Table     int
	ExpiresAt time.Time
}

// tableMatcher 将连续进入同一游戏的用户装入同一张桌，坐满后轮转到下一桌。
// 桌号始终落在 1..tableCount；当前产品约束为每款游戏 1000 张逻辑牌桌。
type tableMatcher struct {
	mu         sync.Mutex
	tableCount int
	next       map[string]int
	waiting    map[string]tableAssignment
	users      map[string]map[int64]tableAssignment
}

func newTableMatcher(tableCount int) *tableMatcher {
	if tableCount < 1 {
		tableCount = 1000
	}
	return &tableMatcher{
		tableCount: tableCount,
		next:       make(map[string]int),
		waiting:    make(map[string]tableAssignment),
		users:      make(map[string]map[int64]tableAssignment),
	}
}

func (m *tableMatcher) assign(code string, uid int64, seats int, now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seats < 1 {
		seats = 1
	}
	if cached := m.users[code][uid]; cached.Table > 0 && cached.ExpiresAt.After(now) {
		return cached.Table
	}
	current := m.waiting[code]
	if current.Table < 1 || !current.ExpiresAt.After(now) {
		if current.Table > 0 {
			delete(m.next, code+":"+strconv.Itoa(current.Table))
		}
		next := m.next[code] + 1
		if next > m.tableCount {
			next = 1
		}
		m.next[code] = next
		current = tableAssignment{Table: next, ExpiresAt: now.Add(matchHumanWindow)}
	}
	occupancyKey := code + ":" + strconv.Itoa(current.Table)
	occupied := m.next[occupancyKey] + 1
	m.next[occupancyKey] = occupied
	if m.users[code] == nil {
		m.users[code] = make(map[int64]tableAssignment)
	}
	m.users[code][uid] = tableAssignment{
		Table:     current.Table,
		ExpiresAt: now.Add(matchTicketTTL),
	}
	if occupied >= seats {
		delete(m.waiting, code)
		delete(m.next, occupancyKey)
	} else {
		m.waiting[code] = current
	}
	return current.Table
}

var miniGameCategories = []struct {
	Key  string
	Name string
}{
	{"arcade", "电子游戏"},
	{"casual", "休闲游戏"},
	{"battle", "对战游戏"},
}

type miniGame struct {
	ID          int64
	Code        string
	Name        string
	NameEn      string
	Category    string
	Cover       string
	EntryType   string
	EntryURL    string
	PlayersMin  int
	PlayersMax  int
	PlayMode    string
	NeedLogin   int
	UseWallet   int
	Orientation string
	Remark      string
	IsHot       int
	IsNew       int
}

func (s *MiniGameService) scan(rows *sql.Rows) ([]miniGame, error) {
	items := make([]miniGame, 0)
	for rows.Next() {
		var g miniGame
		if err := rows.Scan(&g.ID, &g.Code, &g.Name, &g.NameEn, &g.Category, &g.Cover,
			&g.EntryType, &g.EntryURL, &g.PlayersMin, &g.PlayersMax, &g.PlayMode,
			&g.NeedLogin, &g.UseWallet, &g.Orientation, &g.Remark, &g.IsHot, &g.IsNew); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	return items, rows.Err()
}

func formatMiniGame(g miniGame) map[string]any {
	players := strconv.Itoa(g.PlayersMin)
	if g.PlayersMax > g.PlayersMin {
		players = fmt.Sprintf("%d-%d", g.PlayersMin, g.PlayersMax)
	}
	return map[string]any{
		"id": strconv.FormatInt(g.ID, 10), "code": g.Code, "name": g.Name, "name_en": g.NameEn,
		"category": g.Category, "cover": g.Cover, "entry_type": g.EntryType,
		"players_min": strconv.Itoa(g.PlayersMin), "players_max": strconv.Itoa(g.PlayersMax),
		"players_text": players + "人", "play_mode": g.PlayMode,
		"need_login": strconv.Itoa(g.NeedLogin), "use_wallet": strconv.Itoa(g.UseWallet),
		"orientation": g.Orientation, "remark": g.Remark,
		"table_count": "1000", "entry_mode": "match",
		"is_hot": strconv.Itoa(g.IsHot), "is_new": strconv.Itoa(g.IsNew),
	}
}

// List 返回已上架小游戏，按分类分组。category 为空返回全部。
func (s *MiniGameService) List(ctx context.Context, category string) (map[string]any, error) {
	query := `SELECT id,code,name,name_en,category,cover,entry_type,entry_url,
	                 players_min,players_max,play_mode,need_login,use_wallet,orientation,remark,is_hot,is_new
	          FROM cmf_minigame WHERE status=1`
	args := []any{}
	if c := strings.TrimSpace(category); c != "" {
		query += " AND category=?"
		args = append(args, c)
	}
	query += " ORDER BY sort DESC, id ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	games, err := s.scan(rows)
	if err != nil {
		return nil, err
	}

	byCategory := map[string][]map[string]any{}
	all := make([]map[string]any, 0, len(games))
	for _, g := range games {
		item := formatMiniGame(g)
		all = append(all, item)
		byCategory[g.Category] = append(byCategory[g.Category], item)
	}

	// 仅返回有游戏的分类，避免客户端渲染空分组
	categories := make([]map[string]any, 0, len(miniGameCategories))
	for _, c := range miniGameCategories {
		list := byCategory[c.Key]
		if len(list) == 0 {
			continue
		}
		categories = append(categories, map[string]any{
			"key": c.Key, "name": c.Name,
			"count": strconv.Itoa(len(list)), "games": list,
		})
	}

	return map[string]any{
		"total": strconv.Itoa(len(all)), "games": all, "categories": categories,
	}, nil
}

// Enter 校验并下发启动地址。
//
// 对需要登录的游戏，会在 URL 上附带 uid、昵称与一个短期签名，
// 供游戏侧（若实现了校验）确认身份；未实现校验的游戏忽略这些参数即可。
func (s *MiniGameService) Enter(ctx context.Context, code string, uid int64) (map[string]any, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, appError(4001, "缺少游戏标识")
	}

	var g miniGame
	err := s.db.QueryRowContext(ctx, `SELECT id,code,name,name_en,category,cover,entry_type,entry_url,
	        players_min,players_max,play_mode,need_login,use_wallet,orientation,remark,is_hot,is_new
	    FROM cmf_minigame WHERE code=? AND status=1`, code).Scan(
		&g.ID, &g.Code, &g.Name, &g.NameEn, &g.Category, &g.Cover, &g.EntryType, &g.EntryURL,
		&g.PlayersMin, &g.PlayersMax, &g.PlayMode, &g.NeedLogin, &g.UseWallet,
		&g.Orientation, &g.Remark, &g.IsHot, &g.IsNew)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appError(4002, "游戏不存在或已下架")
	}
	if err != nil {
		return nil, err
	}
	if uid < 1 {
		return nil, appError(700, "请先登录")
	}

	nickname := ""
	if uid > 0 {
		var name sql.NullString
		if err := s.db.QueryRowContext(ctx,
			"SELECT COALESCE(NULLIF(user_nickname,''),user_login) FROM cmf_user WHERE id=?", uid).Scan(&name); err == nil {
			nickname = name.String
		}
	}

	launch := g.EntryURL
	table := s.matcher.assign(g.Code, uid, g.PlayersMax, time.Now())
	if g.NeedLogin == 1 {
		issued := time.Now().Unix()
		params := []string{
			"uid=" + strconv.FormatInt(uid, 10),
			"ts=" + strconv.FormatInt(issued, 10),
			"match=1",
			"table=" + strconv.Itoa(table),
		}
		if nickname != "" {
			params = append(params, "name="+urlQueryEscape(nickname))
		}
		params = append(params, "sig="+s.sign(g.Code, uid, issued))
		sep := "?"
		if strings.Contains(launch, "?") {
			sep = "&"
		}
		launch += sep + strings.Join(params, "&")
	}

	result := formatMiniGame(g)
	result["launch_url"] = launch
	result["nickname"] = nickname
	result["table_no"] = strconv.Itoa(table)
	result["table_count"] = strconv.Itoa(s.matcher.tableCount)
	result["entry_mode"] = "match"
	return result, nil
}

// sign 生成短期启动签名，游戏侧可用同一 secret 校验（30 分钟内有效由调用方判断 ts）。
func (s *MiniGameService) sign(code string, uid, issued int64) string {
	mac := hmac.New(sha256.New, []byte(s.secret))
	fmt.Fprintf(mac, "%s|%d|%d", code, uid, issued)
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

func urlQueryEscape(value string) string {
	var b strings.Builder
	for _, byteVal := range []byte(value) {
		switch {
		case byteVal >= 'a' && byteVal <= 'z', byteVal >= 'A' && byteVal <= 'Z',
			byteVal >= '0' && byteVal <= '9', byteVal == '-', byteVal == '_', byteVal == '.', byteVal == '~':
			b.WriteByte(byteVal)
		default:
			fmt.Fprintf(&b, "%%%02X", byteVal)
		}
	}
	return b.String()
}
