package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/live"
)

var douyinRoomIDPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{3,128}$`)

const defaultLiveCategory = "聊天"

type liveHostProfile struct {
	ID        int64
	Username  string
	Nickname  string
	AvatarURL string
	CoverURL  string
	IsVirtual bool
}

func (host liveHostProfile) title() string {
	return host.Nickname + "的直播间"
}

func (h *Handler) listLiveRooms(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		status, _ = strconv.Atoi(rawStatus)
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		status, status, keyword,
		like, like, like, like, like, like, like, like,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM live_rooms room
		LEFT JOIN users user ON user.id=room.host_user_id
		LEFT JOIN douyin_room_profiles profile ON profile.live_room_id=room.id
		WHERE (? < 0 OR room.status=?)
		  AND (?='' OR room.room_no LIKE ? OR room.title LIKE ?
		       OR room.provider_room_id LIKE ? OR profile.nickname LIKE ?
		       OR CAST(room.host_user_id AS CHAR) LIKE ? OR user.username LIKE ?
		       OR user.nickname LIKE ? OR profile.unique_id LIKE ?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取直播间失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT room.id,room.room_no,room.host_user_id,
		       COALESCE(NULLIF(user.nickname,''),user.username),
		       COALESCE(user.username,''),COALESCE(user.is_virtual,0),
		       room.title,room.category,
		       room.provider,room.provider_room_id,room.provider_page,room.status,room.sort_order,
		       room.last_seen_at,room.created_at,
		       COALESCE(profile.nickname,''),COALESCE(profile.unique_id,''),
		       COALESCE(profile.avatar_url,''),COALESCE(profile.cover_url,''),
		       COALESCE(profile.resolution,''),COALESCE(profile.stream_format,''),
		       COALESCE(profile.verify_status,0),COALESCE(profile.last_resolve_status,0),
		       COALESCE(profile.last_resolve_error,''),profile.last_resolved_at
		FROM live_rooms room
		LEFT JOIN users user ON user.id=room.host_user_id
		LEFT JOIN douyin_room_profiles profile ON profile.live_room_id=room.id
		WHERE (? < 0 OR room.status=?)
		  AND (?='' OR room.room_no LIKE ? OR room.title LIKE ?
		       OR room.provider_room_id LIKE ? OR profile.nickname LIKE ?
		       OR CAST(room.host_user_id AS CHAR) LIKE ? OR user.username LIKE ?
		       OR user.nickname LIKE ? OR profile.unique_id LIKE ?)
		ORDER BY room.sort_order DESC,room.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取直播间失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, hostUserID int64
		var roomNo, hostName, hostUsername, title, category, provider, providerRoomID, providerPage string
		var nickname, uniqueID, avatarURL, coverURL, resolution, streamFormat, resolveError string
		var hostVirtual, statusValue, sortOrder, verifyStatus, resolveStatus int
		var lastSeen, lastResolved sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &roomNo, &hostUserID, &hostName, &hostUsername, &hostVirtual, &title, &category,
			&provider, &providerRoomID, &providerPage, &statusValue, &sortOrder,
			&lastSeen, &createdAt, &nickname, &uniqueID, &avatarURL, &coverURL,
			&resolution, &streamFormat, &verifyStatus, &resolveStatus, &resolveError, &lastResolved,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取直播间失败")
			return
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(id, 10), "room_no": roomNo,
			"host_user_id": strconv.FormatInt(hostUserID, 10), "host_name": hostName,
			"host_username": hostUsername, "host_is_virtual": hostVirtual == 1,
			"title": title, "category": category, "provider": provider,
			"provider_room_id": providerRoomID, "provider_page": providerPage,
			"status": statusValue, "sort_order": sortOrder, "last_seen_at": nullTime(lastSeen),
			"created_at": createdAt.Unix(), "nickname": nickname, "unique_id": uniqueID,
			"avatar_url": avatarURL, "cover_url": coverURL, "resolution": resolution,
			"stream_format": streamFormat, "verify_status": verifyStatus,
			"last_resolve_status": resolveStatus, "last_resolve_error": resolveError,
			"last_resolved_at": nullTime(lastResolved),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取直播间失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total,
		"items":    items, "provider": "douyin",
	})
}

func (h *Handler) listLiveHosts(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + escapeLike(keyword) + "%"
	keywordID, _ := strconv.ParseInt(keyword, 10, 64)
	queryArguments := []any{keyword, like, like, keywordID, keywordID}

	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM users app_user
		WHERE app_user.status=1
		  AND (?='' OR app_user.username LIKE ? OR app_user.nickname LIKE ?
		       OR (? > 0 AND app_user.id=?))`,
		queryArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取主播失败")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT app_user.id,app_user.username,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username),app_user.is_virtual,
		       COALESCE(avatar.bucket,''),COALESCE(avatar.object_key,''),
		       COALESCE(background.bucket,''),COALESCE(background.object_key,'')
		FROM users app_user
		LEFT JOIN media_assets avatar
		  ON avatar.id=app_user.avatar_asset_id AND avatar.status=1
		LEFT JOIN media_assets background
		  ON background.id=app_user.background_asset_id AND background.status=1
		WHERE app_user.status=1
		  AND (?='' OR app_user.username LIKE ? OR app_user.nickname LIKE ?
		       OR (? > 0 AND app_user.id=?))
		ORDER BY app_user.is_virtual DESC,app_user.created_at DESC,app_user.id DESC
		LIMIT ? OFFSET ?`,
		append(queryArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取主播失败")
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var host liveHostProfile
		var virtual int
		var avatarBucket, avatarKey, backgroundBucket, backgroundKey string
		if err = rows.Scan(
			&host.ID, &host.Username, &host.Nickname, &virtual,
			&avatarBucket, &avatarKey, &backgroundBucket, &backgroundKey,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取主播失败")
			return
		}
		host.IsVirtual = virtual == 1
		host.AvatarURL = h.mediaAssetURL(avatarBucket, avatarKey)
		host.CoverURL = h.mediaAssetURL(backgroundBucket, backgroundKey)
		if host.CoverURL == "" {
			host.CoverURL = host.AvatarURL
		}
		items = append(items, map[string]any{
			"id": strconv.FormatInt(host.ID, 10), "username": host.Username,
			"nickname": host.Nickname, "avatar_url": host.AvatarURL,
			"cover_url": host.CoverURL, "title": host.title(), "is_virtual": host.IsVirtual,
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取主播失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page*pageSize) < total, "items": items,
	})
}

func (h *Handler) createLiveRoom(w http.ResponseWriter, r *http.Request) {
	var request struct {
		HostUserID     string `json:"host_user_id"`
		ProviderRoomID string `json:"provider_room_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	hostUserID, err := positiveDecimalID(request.HostUserID)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "主播用户编号无效")
		return
	}
	canonicalPage, providerRoomID, err := normalizeDouyinRoomID(request.ProviderRoomID)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "抖音房间 ID 无效")
		return
	}
	host, err := h.enabledLiveHost(r.Context(), hostUserID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "主播用户不存在或已停用")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取主播失败")
		return
	}
	source, err := h.probeDouyin(r.Context(), providerRoomID)
	if err != nil {
		h.writeLiveProbeError(w, r, err)
		return
	}
	roomNo, err := idgen.New()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建直播间失败")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建直播间失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	host, err = h.enabledLiveHostWith(r.Context(), tx, hostUserID, true)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "主播用户不存在或已停用")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取主播失败")
		return
	}
	hasOnlineRoom, err := liveHostHasOnlineRoom(r.Context(), tx, host.ID, 0)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "检查主播直播状态失败")
		return
	}
	if hasOnlineRoom {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该主播已有在线直播间")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO live_rooms
			(room_no,host_user_id,title,category,provider,provider_room_id,provider_page,
			 status,sort_order,last_seen_at)
		VALUES(?,?,?,?,'douyin',?,?,1,?,CURRENT_TIMESTAMP(3))`,
		roomNo, host.ID, host.title(), defaultLiveCategory,
		providerRoomID, canonicalPage, 0,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "抖音直播间已存在或参数冲突")
		return
	}
	liveRoomID, _ := result.LastInsertId()
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO douyin_room_profiles
			(live_room_id,nickname,unique_id,avatar_url,cover_url,resolution,stream_format,
			 verify_status,verified_by,verified_at,last_resolve_status,last_resolve_error,last_resolved_at)
		VALUES(?,?,?,?,?,?,?,1,?,CURRENT_TIMESTAMP(3),1,'',CURRENT_TIMESTAMP(3))`,
		liveRoomID, host.Nickname, host.Username, host.AvatarURL, host.CoverURL,
		source.Resolution, source.Format, adminID(r),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建直播间失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "live.room.create", "live_room", liveRoomID,
		nil, map[string]any{
			"provider": "douyin", "provider_room_id": providerRoomID,
			"host_user_id": host.ID, "title": host.title(), "probe": "passed",
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录直播审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建直播间失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": strconv.FormatInt(liveRoomID, 10), "room_no": roomNo,
		"host_user_id": strconv.FormatInt(host.ID, 10), "provider": "douyin",
		"provider_room_id": providerRoomID, "provider_page": canonicalPage,
		"title": host.title(), "nickname": host.Nickname,
		"avatar_url": host.AvatarURL, "cover_url": host.CoverURL,
		"resolution": source.Resolution, "stream_format": source.Format,
		"verify_status": 1, "status": 1,
	})
}

func (h *Handler) updateLiveRoom(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || roomID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "直播间编号无效")
		return
	}
	var request struct {
		HostUserID     string `json:"host_user_id"`
		ProviderRoomID string `json:"provider_room_id"`
		Status         int    `json:"status"`
		SortOrder      int    `json:"sort_order"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	requestedHostUserID, err := positiveDecimalID(request.HostUserID)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "主播用户编号无效")
		return
	}
	canonicalPage, providerRoomID, err := normalizeDouyinRoomID(request.ProviderRoomID)
	if err != nil || request.Status < 0 || request.Status > 2 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "直播间参数无效")
		return
	}
	var hostUserID int64
	var roomNo, provider, beforeProviderRoomID string
	var beforeStatus, beforeVerifyStatus int
	if err = h.db.QueryRowContext(r.Context(), `
		SELECT room.room_no,room.host_user_id,room.provider,room.provider_room_id,room.status,
		       COALESCE(profile.verify_status,0)
		FROM live_rooms room
		LEFT JOIN douyin_room_profiles profile ON profile.live_room_id=room.id
		WHERE room.id=?`,
		roomID,
	).Scan(
		&roomNo, &hostUserID, &provider, &beforeProviderRoomID, &beforeStatus, &beforeVerifyStatus,
	); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "直播间不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新直播间失败")
		return
	}
	if provider != "douyin" {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "非抖音直播间禁止启用")
		return
	}
	host, err := h.enabledLiveHost(r.Context(), requestedHostUserID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "主播用户不存在或已停用")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取主播失败")
		return
	}
	needsProbe := requestedHostUserID != hostUserID || providerRoomID != beforeProviderRoomID ||
		request.Status == 1 || beforeVerifyStatus != 1
	var source live.Source
	if needsProbe {
		source, err = h.probeDouyin(r.Context(), providerRoomID)
		if err != nil {
			h.writeLiveProbeError(w, r, err)
			return
		}
	}

	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新直播间失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	host, err = h.enabledLiveHostWith(r.Context(), tx, requestedHostUserID, true)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "主播用户不存在或已停用")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取主播失败")
		return
	}
	var lockedHostUserID int64
	var lockedRoomNo, lockedProvider, lockedProviderRoomID string
	var lockedStatus int
	if err = tx.QueryRowContext(r.Context(), `
		SELECT room_no,host_user_id,provider,provider_room_id,status
		FROM live_rooms WHERE id=? FOR UPDATE`,
		roomID,
	).Scan(
		&lockedRoomNo, &lockedHostUserID, &lockedProvider, &lockedProviderRoomID, &lockedStatus,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "直播间已被其他管理员修改，请刷新后重试")
		return
	}
	if lockedRoomNo != roomNo || lockedHostUserID != hostUserID || lockedProvider != provider ||
		lockedProviderRoomID != beforeProviderRoomID || lockedStatus != beforeStatus {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "直播间已被其他管理员修改，请刷新后重试")
		return
	}
	if request.Status == 1 {
		hasOnlineRoom, conflictErr := liveHostHasOnlineRoom(r.Context(), tx, host.ID, roomID)
		if conflictErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "检查主播直播状态失败")
			return
		}
		if hasOnlineRoom {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该主播已有其他在线直播间")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE live_rooms
		SET host_user_id=?,title=?,category=?,provider_room_id=?,provider_page=?,status=?,sort_order=?,
		    last_seen_at=IF(?=1,CURRENT_TIMESTAMP(3),last_seen_at)
		WHERE id=?`,
		host.ID, host.title(), defaultLiveCategory, providerRoomID, canonicalPage,
		request.Status, request.SortOrder, request.Status, roomID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "抖音直播间已存在或参数冲突")
		return
	}
	if err = syncLiveConversationOwner(
		r.Context(), tx, roomID, roomNo, host.ID, host.title(), request.Status,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "同步直播聊天主播失败")
		return
	}
	if needsProbe {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO douyin_room_profiles
				(live_room_id,nickname,unique_id,avatar_url,cover_url,resolution,stream_format,
				 verify_status,verified_by,verified_at,last_resolve_status,last_resolve_error,last_resolved_at)
			VALUES(?,?,?,?,?,?,?,1,?,CURRENT_TIMESTAMP(3),1,'',CURRENT_TIMESTAMP(3))
			ON DUPLICATE KEY UPDATE
				nickname=VALUES(nickname),unique_id=VALUES(unique_id),
				avatar_url=VALUES(avatar_url),cover_url=VALUES(cover_url),
				resolution=VALUES(resolution),stream_format=VALUES(stream_format),
				verify_status=1,verified_by=VALUES(verified_by),verified_at=CURRENT_TIMESTAMP(3),
				last_resolve_status=1,last_resolve_error='',last_resolved_at=CURRENT_TIMESTAMP(3)`,
			roomID, host.Nickname, host.Username, host.AvatarURL, host.CoverURL,
			source.Resolution, source.Format, adminID(r),
		)
	} else {
		_, err = tx.ExecContext(r.Context(), `
			INSERT INTO douyin_room_profiles
				(live_room_id,nickname,unique_id,avatar_url,cover_url)
			VALUES(?,?,?,?,?)
			ON DUPLICATE KEY UPDATE
				nickname=VALUES(nickname),unique_id=VALUES(unique_id),
				avatar_url=VALUES(avatar_url),cover_url=VALUES(cover_url)`,
			roomID, host.Nickname, host.Username, host.AvatarURL, host.CoverURL,
		)
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新直播间失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "live.room.update", "live_room", roomID,
		map[string]any{
			"status": beforeStatus, "provider": provider, "provider_room_id": beforeProviderRoomID,
			"host_user_id": hostUserID,
		},
		map[string]any{
			"status": request.Status, "provider": provider, "provider_room_id": providerRoomID,
			"host_user_id": host.ID, "title": host.title(), "probe": needsProbe,
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录直播审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新直播间失败")
		return
	}
	verifyStatus := beforeVerifyStatus
	if needsProbe {
		verifyStatus = 1
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"id": strconv.FormatInt(roomID, 10), "host_user_id": strconv.FormatInt(host.ID, 10),
		"provider_room_id": providerRoomID, "provider_page": canonicalPage,
		"title": host.title(), "nickname": host.Nickname,
		"avatar_url": host.AvatarURL, "cover_url": host.CoverURL,
		"status": request.Status, "verify_status": verifyStatus,
	})
}

type liveHostQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (h *Handler) enabledLiveHost(ctx context.Context, userID int64) (liveHostProfile, error) {
	return h.enabledLiveHostWith(ctx, h.db, userID, false)
}

func (h *Handler) enabledLiveHostWith(
	ctx context.Context,
	queryer liveHostQueryer,
	userID int64,
	lock bool,
) (liveHostProfile, error) {
	var host liveHostProfile
	var virtual int
	var avatarBucket, avatarKey, backgroundBucket, backgroundKey string
	query := `
		SELECT app_user.id,app_user.username,
		       COALESCE(NULLIF(app_user.nickname,''),app_user.username),app_user.is_virtual,
		       COALESCE(avatar.bucket,''),COALESCE(avatar.object_key,''),
		       COALESCE(background.bucket,''),COALESCE(background.object_key,'')
		FROM users app_user
		LEFT JOIN media_assets avatar
		  ON avatar.id=app_user.avatar_asset_id AND avatar.status=1
		LEFT JOIN media_assets background
		  ON background.id=app_user.background_asset_id AND background.status=1
		WHERE app_user.id=? AND app_user.status=1`
	if lock {
		query += " FOR UPDATE"
	}
	err := queryer.QueryRowContext(ctx, query,
		userID,
	).Scan(
		&host.ID, &host.Username, &host.Nickname, &virtual,
		&avatarBucket, &avatarKey, &backgroundBucket, &backgroundKey,
	)
	if err != nil {
		return liveHostProfile{}, err
	}
	host.IsVirtual = virtual == 1
	host.AvatarURL = h.mediaAssetURL(avatarBucket, avatarKey)
	host.CoverURL = h.mediaAssetURL(backgroundBucket, backgroundKey)
	if host.CoverURL == "" {
		host.CoverURL = host.AvatarURL
	}
	return host, nil
}

func liveHostHasOnlineRoom(
	ctx context.Context,
	tx *sql.Tx,
	hostUserID int64,
	excludeRoomID int64,
) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM live_rooms
			WHERE host_user_id=? AND status=1 AND (?=0 OR id<>?)
		)`,
		hostUserID, excludeRoomID, excludeRoomID,
	).Scan(&exists)
	return exists == 1, err
}

func syncLiveConversationOwner(
	ctx context.Context,
	tx *sql.Tx,
	liveRoomID int64,
	conversationID string,
	hostUserID int64,
	title string,
	roomStatus int,
) error {
	conversationStatus := 0
	if roomStatus == 1 {
		conversationStatus = 1
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE im_conversations
		SET created_by=?,title=?,status=?
		WHERE id=? AND conversation_type=3`,
		hostUserID, title, conversationStatus, conversationID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE im_conversation_members member
		LEFT JOIN live_room_managers manager
		  ON manager.live_room_id=? AND manager.user_id=member.user_id
		SET member.role=IF(manager.user_id IS NULL,10,60)
		WHERE member.conversation_id=? AND member.user_id<>? AND member.role=100`,
		liveRoomID, conversationID, hostUserID,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO im_conversation_members
			(conversation_id,user_id,role,member_status,mute_until,left_at)
		SELECT conversation.id,?,100,1,NULL,NULL
		FROM im_conversations conversation
		WHERE conversation.id=? AND conversation.conversation_type=3
		ON DUPLICATE KEY UPDATE
			role=100,member_status=1,mute_until=NULL,left_at=NULL`,
		hostUserID, conversationID,
	)
	return err
}

func (h *Handler) mediaAssetURL(bucket, objectKey string) string {
	bucket = strings.Trim(bucket, "/")
	objectKey = strings.Trim(objectKey, "/")
	if bucket == "" || objectKey == "" {
		return ""
	}
	segments := strings.Split(objectKey, "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	escapedBucket := url.PathEscape(bucket)
	base := h.mediaBaseURL
	if base == "" {
		return "/" + escapedBucket + "/" + strings.Join(segments, "/")
	}
	if strings.HasSuffix(base, "/"+escapedBucket) {
		return base + "/" + strings.Join(segments, "/")
	}
	return base + "/" + escapedBucket + "/" + strings.Join(segments, "/")
}

func (h *Handler) probeDouyin(ctx context.Context, providerRoomID string) (live.Source, error) {
	if h.liveProbe == nil {
		return live.Source{}, errors.New("live probe is not configured")
	}
	return h.liveProbe.ProbeDouyin(ctx, providerRoomID)
}

func (h *Handler) writeLiveProbeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, live.ErrSourceUnavailable) {
		httpx.Error(
			w, httpx.RequestID(r.Context()), http.StatusUnprocessableEntity, 422,
			"抖音房间当前没有可用直播流，未保存任何变更",
		)
		return
	}
	httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadGateway, 502, "抖音直播流探测失败，请稍后重试")
}

func positiveDecimalID(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != value {
		return 0, errors.New("invalid decimal id")
	}
	return id, nil
}

func normalizeDouyinRoomID(raw string) (string, string, error) {
	roomID := strings.TrimSpace(raw)
	if !douyinRoomIDPattern.MatchString(roomID) {
		return "", "", errors.New("invalid douyin room id")
	}
	return "https://live.douyin.com/" + url.PathEscape(roomID), roomID, nil
}

func normalizeDouyinPage(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" {
		return "", "", errors.New("invalid douyin URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "live.douyin.com" && host != "www.douyin.com" && host != "douyin.com" {
		return "", "", errors.New("invalid douyin host")
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	roomID := ""
	for index := len(segments) - 1; index >= 0; index-- {
		candidate, unescapeErr := url.PathUnescape(segments[index])
		if unescapeErr == nil && douyinRoomIDPattern.MatchString(candidate) && candidate != "live" {
			roomID = candidate
			break
		}
	}
	if roomID == "" {
		return "", "", errors.New("missing douyin room id")
	}
	return "https://live.douyin.com/" + url.PathEscape(roomID), roomID, nil
}

func adminID(r *http.Request) int64 {
	adminUser, _ := adminFromRequest(r)
	return adminUser.ID
}
