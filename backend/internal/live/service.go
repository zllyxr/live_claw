package live

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrRoomNotFound      = errors.New("live room not found")
	ErrSourceUnavailable = errors.New("douyin source unavailable")

	douyinRoomIDPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{3,128}$`)
	streamURLPattern    = regexp.MustCompile(`https?://[^"'<>\\\s]+?\.(?:m3u8|flv)(?:\?[^"'<>\\\s]*)?`)
	heightPattern       = regexp.MustCompile(`(?i)(?:^|[_/\-])(\d{3,4})p?(?:[_./\-]|$)`)
)

type Service struct {
	db     *sql.DB
	redis  *redis.Client
	client *http.Client
	now    func() time.Time
}

type Room struct {
	ID             int64
	RoomNo         string
	HostUserID     int64
	Title          string
	ProviderRoomID string
	ProviderPage   string
	Status         int
	Nickname       string
	AvatarURL      string
	CoverURL       string
}

type Source struct {
	URL          string `json:"url"`
	Format       string `json:"format"`
	Height       int    `json:"height"`
	Resolution   string `json:"resolution"`
	Provider     string `json:"provider"`
	RoomID       string `json:"room_id"`
	RoomPage     string `json:"room_page"`
	CacheSeconds int    `json:"cache_seconds"`
	Delivery     string `json:"delivery"`
}

func New(db *sql.DB, redisClient *redis.Client) *Service {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 20
	transport.MaxIdleConnsPerHost = 4
	transport.ResponseHeaderTimeout = 12 * time.Second
	return &Service{
		db: db, redis: redisClient, now: time.Now,
		client: &http.Client{
			Transport: transport, Timeout: 25 * time.Second,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if len(via) >= 4 || !isDouyinHost(request.URL.Hostname()) {
					return errors.New("douyin redirect rejected")
				}
				return nil
			},
		},
	}
}

func (s *Service) Hot(ctx context.Context, page int, pageSize int) ([]map[string]any, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 6
	}
	if pageSize > 30 {
		pageSize = 30
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT room.id,room.room_no,room.host_user_id,room.title,
		       room.provider_room_id,room.provider_page,room.status,
		       COALESCE(NULLIF(profile.nickname,''),NULLIF(user.nickname,''),user.username),
		       COALESCE(NULLIF(profile.avatar_url,''),''),
		       COALESCE(NULLIF(profile.cover_url,''),'')
		FROM live_rooms room
		JOIN users user ON user.id=room.host_user_id AND user.status=1
		LEFT JOIN douyin_room_profiles profile ON profile.live_room_id=room.id
		WHERE room.status=1 AND room.provider='douyin' AND profile.verify_status=1
		ORDER BY room.sort_order DESC,room.last_seen_at DESC,room.id DESC
		LIMIT ? OFFSET ?`,
		pageSize, (page-1)*pageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		room, scanErr := scanRoom(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, formatRoom(room))
	}
	return items, rows.Err()
}

func (s *Service) Room(ctx context.Context, liveUserID int64, stream string) (Room, error) {
	stream = strings.TrimSpace(stream)
	var row *sql.Row
	if stream != "" {
		row = s.db.QueryRowContext(ctx, `
			SELECT room.id,room.room_no,room.host_user_id,room.title,
			       room.provider_room_id,room.provider_page,room.status,
			       COALESCE(NULLIF(profile.nickname,''),NULLIF(user.nickname,''),user.username),
			       COALESCE(NULLIF(profile.avatar_url,''),''),
			       COALESCE(NULLIF(profile.cover_url,''),'')
			FROM live_rooms room
			JOIN users user ON user.id=room.host_user_id AND user.status=1
			LEFT JOIN douyin_room_profiles profile ON profile.live_room_id=room.id
			WHERE room.room_no=? AND room.host_user_id=? AND room.status=1
			  AND room.provider='douyin' AND profile.verify_status=1`,
			stream, liveUserID,
		)
	} else {
		row = s.db.QueryRowContext(ctx, `
			SELECT room.id,room.room_no,room.host_user_id,room.title,
			       room.provider_room_id,room.provider_page,room.status,
			       COALESCE(NULLIF(profile.nickname,''),NULLIF(user.nickname,''),user.username),
			       COALESCE(NULLIF(profile.avatar_url,''),''),
			       COALESCE(NULLIF(profile.cover_url,''),'')
			FROM live_rooms room
			JOIN users user ON user.id=room.host_user_id AND user.status=1
			LEFT JOIN douyin_room_profiles profile ON profile.live_room_id=room.id
			WHERE room.host_user_id=? AND room.status=1
			  AND room.provider='douyin' AND profile.verify_status=1
			ORDER BY room.sort_order DESC,room.id DESC LIMIT 1`,
			liveUserID,
		)
	}
	room, err := scanRoom(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, ErrRoomNotFound
	}
	return room, err
}

func (s *Service) Enter(
	ctx context.Context,
	userID int64,
	liveUserID int64,
	stream string,
) (map[string]any, error) {
	room, err := s.Room(ctx, liveUserID, stream)
	if err != nil {
		return nil, err
	}
	conversationID, err := s.joinConversation(ctx, room, userID)
	if err != nil {
		return nil, err
	}
	viewer := map[string]any{}
	if userID > 0 {
		var nickname, avatarURL string
		err = s.db.QueryRowContext(ctx, `
			SELECT COALESCE(NULLIF(user.nickname,''),user.username),
			       COALESCE(CONCAT(asset.bucket,'/',asset.object_key),'')
			FROM users user
			LEFT JOIN media_assets asset ON asset.id=user.avatar_asset_id AND asset.status=1
			WHERE user.id=? AND user.status=1`,
			userID,
		).Scan(&nickname, &avatarURL)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			viewer = map[string]any{
				"id": strconv.FormatInt(userID, 10), "uid": strconv.FormatInt(userID, 10),
				"user_nicename": nickname, "user_nickname": nickname,
				"avatar": avatarURL, "avatar_thumb": avatarURL,
			}
		}
	}
	host := formatRoomUser(room)
	users := []map[string]any{host}
	if userID > 0 && userID != liveUserID && len(viewer) > 0 {
		users = append(users, viewer)
	}
	return map[string]any{
		"liveuid": strconv.FormatInt(room.HostUserID, 10), "stream": room.RoomNo,
		"title": room.Title, "source_page": room.ProviderPage, "pull": room.ProviderPage,
		"userlist": users, "nums": strconv.Itoa(len(users)), "votestotal": "0",
		"live_user": host, "vip": map[string]string{"type": "1"},
		"im": map[string]any{
			"transport": "native", "websocket": "/ws/im",
			"conversation_id": conversationID,
		},
	}, nil
}

func (s *Service) Join(
	ctx context.Context,
	userID int64,
	liveUserID int64,
	stream string,
) (map[string]any, error) {
	room, err := s.Room(ctx, liveUserID, stream)
	if err != nil {
		return nil, err
	}
	conversationID, err := s.joinConversation(ctx, room, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"conversation_id": conversationID, "websocket": "/ws/im",
		"transport": "native", "room_no": room.RoomNo,
	}, nil
}

func (s *Service) Leave(ctx context.Context, userID int64, liveUserID int64, stream string) error {
	room, err := s.Room(ctx, liveUserID, stream)
	if err != nil {
		return err
	}
	if userID < 1 || userID == room.HostUserID {
		return nil
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE im_conversation_members
		SET member_status=2,left_at=CURRENT_TIMESTAMP(3)
		WHERE conversation_id=? AND user_id=? AND member_status=1`,
		room.RoomNo, userID,
	)
	return err
}

func (s *Service) joinConversation(ctx context.Context, room Room, userID int64) (string, error) {
	if userID < 1 {
		return "", errors.New("invalid live viewer")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_conversations
			(id,conversation_type,direct_key,title,status,created_by)
		VALUES(?,3,NULL,?,1,?)
		ON DUPLICATE KEY UPDATE
			title=VALUES(title),status=1,created_by=VALUES(created_by)`,
		room.RoomNo, room.Title, room.HostUserID,
	); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE im_conversation_members member
		LEFT JOIN live_room_managers manager
		  ON manager.live_room_id=? AND manager.user_id=member.user_id
		SET member.role=IF(manager.user_id IS NULL,10,60)
		WHERE member.conversation_id=? AND member.user_id<>? AND member.role=100`,
		room.ID, room.RoomNo, room.HostUserID,
	); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO im_conversation_members
			(conversation_id,user_id,role,member_status,mute_until,left_at)
		VALUES(?,?,100,1,NULL,NULL)
		ON DUPLICATE KEY UPDATE
			role=100,member_status=1,mute_until=NULL,left_at=NULL`,
		room.RoomNo, room.HostUserID,
	); err != nil {
		return "", err
	}
	role := 10
	if userID == room.HostUserID {
		role = 100
	} else {
		var manager int
		if err = tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM live_room_managers
				WHERE live_room_id=? AND user_id=?
			)`,
			room.ID, userID,
		).Scan(&manager); err != nil {
			return "", err
		}
		if manager == 1 {
			role = 60
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO im_conversation_members
			(conversation_id,user_id,role,member_status,left_at)
		SELECT ?,?,?,1,NULL FROM users WHERE id=? AND status=1
		ON DUPLICATE KEY UPDATE
			role=VALUES(role),member_status=1,left_at=NULL`,
		room.RoomNo, userID, role, userID,
	)
	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return room.RoomNo, nil
}

func (s *Service) Users(ctx context.Context, liveUserID int64, stream string) (map[string]any, error) {
	room, err := s.Room(ctx, liveUserID, stream)
	if err != nil {
		return nil, err
	}
	users := []map[string]any{formatRoomUser(room)}
	return map[string]any{
		"userlist": users, "nums": strconv.Itoa(len(users)), "votestotal": "0",
	}, nil
}

func (s *Service) Resolve(
	ctx context.Context,
	liveUserID int64,
	stream string,
	refresh bool,
) (Source, error) {
	room, err := s.Room(ctx, liveUserID, stream)
	if err != nil {
		return Source{}, err
	}
	cacheKey := sourceCacheKey(room)
	if !refresh && s.redis != nil {
		cached, cacheErr := s.redis.Get(ctx, cacheKey).Bytes()
		if cacheErr == nil {
			var source Source
			if json.Unmarshal(cached, &source) == nil && source.URL != "" {
				return source, nil
			}
		}
	}
	source, err := s.resolveDouyin(ctx, room)
	if err != nil {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE douyin_room_profiles
			SET last_resolve_status=2,last_resolve_error=?,last_resolved_at=CURRENT_TIMESTAMP(3)
			WHERE live_room_id=?`,
			truncate(err.Error(), 500), room.ID,
		)
		return Source{}, err
	}
	payload, _ := json.Marshal(source)
	if s.redis != nil {
		_ = s.redis.Set(ctx, cacheKey, payload, 30*time.Second).Err()
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE live_rooms SET last_seen_at=CURRENT_TIMESTAMP(3) WHERE id=?`,
		room.ID,
	)
	_, _ = s.db.ExecContext(ctx, `
		UPDATE douyin_room_profiles
		SET resolution=?,stream_format=?,last_resolve_status=1,last_resolve_error='',
		    last_resolved_at=CURRENT_TIMESTAMP(3)
		WHERE live_room_id=?`,
		source.Resolution, source.Format, room.ID,
	)
	return source, nil
}

// ProbeDouyin validates a provider room before the administrative workflow
// persists it or marks it online. It deliberately bypasses the room/cache
// lookup so an edited provider room ID can be checked before the database is
// changed.
func (s *Service) ProbeDouyin(ctx context.Context, providerRoomID string) (Source, error) {
	providerRoomID = strings.TrimSpace(providerRoomID)
	if !douyinRoomIDPattern.MatchString(providerRoomID) {
		return Source{}, ErrSourceUnavailable
	}
	return s.resolveDouyin(ctx, Room{
		ProviderRoomID: providerRoomID,
		ProviderPage:   "https://live.douyin.com/" + url.PathEscape(providerRoomID),
	})
}

func sourceCacheKey(room Room) string {
	return "live:v2:douyin:source:" + strconv.FormatInt(room.ID, 10) + ":" + room.ProviderRoomID
}

func (s *Service) resolveDouyin(ctx context.Context, room Room) (Source, error) {
	page, err := url.Parse(strings.TrimSpace(room.ProviderPage))
	if err != nil || page.Scheme != "https" || !isDouyinHost(page.Hostname()) {
		return Source{}, ErrSourceUnavailable
	}
	page.Path = "/" + room.ProviderRoomID
	page.RawQuery = ""
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, page.String(), nil)
	if err != nil {
		return Source{}, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 Chrome/124.0 Mobile Safari/537.36")
	request.Header.Set("Referer", "https://live.douyin.com/")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	response, err := s.client.Do(request)
	if err != nil {
		return Source{}, fmt.Errorf("%w: %v", ErrSourceUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Source{}, fmt.Errorf("%w: HTTP %d", ErrSourceUnavailable, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 12<<20))
	if err != nil {
		return Source{}, fmt.Errorf("%w: %v", ErrSourceUnavailable, err)
	}
	candidates := extractStreamCandidates(string(body))
	if len(candidates) == 0 {
		return Source{}, ErrSourceUnavailable
	}
	selected := selectStream(candidates, 720, "hls")
	if selected.URL == "" {
		return Source{}, ErrSourceUnavailable
	}
	if err = s.validateHLS(ctx, selected.URL); err != nil {
		return Source{}, fmt.Errorf("%w: stream manifest unavailable", ErrSourceUnavailable)
	}
	return Source{
		URL: selected.URL, Format: selected.Format, Height: selected.Height,
		Resolution: resolution(selected.Height), Provider: "douyin",
		RoomID: room.ProviderRoomID, RoomPage: page.String(),
		CacheSeconds: 30, Delivery: "direct",
	}, nil
}

func (s *Service) validateHLS(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || !isDouyinMediaHost(parsed.Hostname()) {
		return ErrSourceUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 Chrome/124.0 Mobile Safari/537.36")
	request.Header.Set("Referer", "https://live.douyin.com/")
	request.Header.Set("Accept", "application/vnd.apple.mpegurl,application/x-mpegURL,*/*")
	response, err := s.client.Do(request)
	if err != nil {
		return ErrSourceUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ErrSourceUnavailable
	}
	manifest, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(manifest) > 1<<20 || !isHLSManifest(manifest) {
		return ErrSourceUnavailable
	}
	return nil
}

func isHLSManifest(value []byte) bool {
	return strings.HasPrefix(strings.TrimSpace(string(value)), "#EXTM3U")
}

type streamCandidate struct {
	URL    string
	Format string
	Height int
	Index  int
}

func extractStreamCandidates(body string) []streamCandidate {
	decoded := decodePage(body)
	matches := streamURLPattern.FindAllString(decoded, -1)
	seen := make(map[string]struct{}, len(matches))
	result := make([]streamCandidate, 0, len(matches))
	for index, match := range matches {
		cleaned := strings.TrimRight(match, "),;]}")
		parsed, err := url.Parse(cleaned)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" {
			continue
		}
		// The Douyin page still publishes some HLS candidates with an http
		// scheme even though the same CDN endpoint supports TLS. Returning
		// those URLs to the HTTPS frontend makes browsers block the stream as
		// mixed content, so always hand the player the secure variant.
		if parsed.Scheme == "http" {
			parsed.Scheme = "https"
			cleaned = parsed.String()
		}
		path := strings.ToLower(parsed.Path)
		format := ""
		switch {
		case strings.HasSuffix(path, ".m3u8"):
			format = "hls"
		case strings.HasSuffix(path, ".flv"):
			format = "flv"
		default:
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, streamCandidate{
			URL: cleaned, Format: format, Height: inferHeight(cleaned), Index: index,
		})
	}
	return result
}

func selectStream(candidates []streamCandidate, maxHeight int, preferredFormat string) streamCandidate {
	if len(candidates) == 0 {
		return streamCandidate{}
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		leftScore := streamScore(candidates[left], maxHeight, preferredFormat)
		rightScore := streamScore(candidates[right], maxHeight, preferredFormat)
		for index := range leftScore {
			if leftScore[index] != rightScore[index] {
				return leftScore[index] > rightScore[index]
			}
		}
		return candidates[left].Index < candidates[right].Index
	})
	return candidates[0]
}

func streamScore(candidate streamCandidate, maxHeight int, preferredFormat string) [4]int {
	withinLimit := candidate.Height == 0 || maxHeight <= 0 || candidate.Height <= maxHeight
	formatScore := 0
	if candidate.Format == preferredFormat {
		formatScore = 1
	}
	limitScore := 0
	if withinLimit {
		limitScore = 1
	}
	heightScore := candidate.Height
	if !withinLimit {
		heightScore = -candidate.Height
	}
	exactScore := 0
	if candidate.Height == maxHeight {
		exactScore = 1
	}
	return [4]int{formatScore, limitScore, exactScore, heightScore}
}

func decodePage(value string) string {
	result := value
	replacer := strings.NewReplacer(
		`\/`, `/`, `\u002F`, `/`, `\u002f`, `/`,
		`\u0026`, `&`, `\u003D`, `=`, `\u003d`, `=`,
		`\u003F`, `?`, `\u003f`, `?`, `\"`, `"`,
	)
	for iteration := 0; iteration < 4; iteration++ {
		previous := result
		result = html.UnescapeString(result)
		result = replacer.Replace(result)
		if decoded, err := url.PathUnescape(result); err == nil {
			result = decoded
		}
		if result == previous {
			break
		}
	}
	return result
}

func inferHeight(raw string) int {
	path := strings.ToLower(raw)
	switch {
	case strings.Contains(path, "_or4"), strings.Contains(path, "/origin"), strings.Contains(path, "_uhd"):
		return 1080
	case strings.Contains(path, "_hd"), strings.Contains(path, "720p"):
		return 720
	case strings.Contains(path, "_ld"), strings.Contains(path, "540p"):
		return 540
	case strings.Contains(path, "_sd"), strings.Contains(path, "360p"):
		return 360
	}
	match := heightPattern.FindStringSubmatch(path)
	if len(match) == 2 {
		height, _ := strconv.Atoi(match[1])
		return height
	}
	return 0
}

func resolution(height int) string {
	if height < 1 {
		return ""
	}
	return "1280x" + strconv.Itoa(height)
}

func isDouyinHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "live.douyin.com" || host == "webcast.amemv.com"
}

func isDouyinMediaHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "douyincdn.com" || strings.HasSuffix(host, ".douyincdn.com")
}

func formatRoom(room Room) map[string]any {
	return map[string]any{
		"id":      strconv.FormatInt(room.ID, 10),
		"uid":     strconv.FormatInt(room.HostUserID, 10),
		"liveuid": strconv.FormatInt(room.HostUserID, 10),
		"stream":  room.RoomNo, "pull": room.ProviderPage, "flvpull": "",
		"user_nicename": room.Nickname, "user_nickname": room.Nickname,
		"title": room.Title, "avatar": room.AvatarURL, "avatar_thumb": room.AvatarURL,
		"thumb": room.CoverURL, "city": "", "nums": "1", "hotvotes": "0",
		"type": "0", "type_val": "", "provider": "douyin",
		"provider_room_id": room.ProviderRoomID, "source_page": room.ProviderPage,
	}
}

func formatRoomUser(room Room) map[string]any {
	return map[string]any{
		"id":            strconv.FormatInt(room.HostUserID, 10),
		"uid":           strconv.FormatInt(room.HostUserID, 10),
		"user_nicename": room.Nickname, "user_nickname": room.Nickname,
		"avatar": room.AvatarURL, "avatar_thumb": room.AvatarURL,
	}
}

func scanRoom(row interface{ Scan(...any) error }) (Room, error) {
	var room Room
	err := row.Scan(
		&room.ID, &room.RoomNo, &room.HostUserID, &room.Title,
		&room.ProviderRoomID, &room.ProviderPage, &room.Status,
		&room.Nickname, &room.AvatarURL, &room.CoverURL,
	)
	return room, err
}

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
