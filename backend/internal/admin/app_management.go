package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/storage"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (h *Handler) prepareAppUpload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(request.FileName)))
	if !allowedReleaseExtension(extension) || request.Size < 1 || request.Size > 2<<30 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "安装包参数不合法")
		return
	}
	if request.ContentType == "" {
		request.ContentType = "application/octet-stream"
	}
	objectID, err := idgen.New()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成上传地址失败")
		return
	}
	objectKey := fmt.Sprintf("app/%s/%s%s", time.Now().Format("2006/01"), objectID, extension)
	uploadURL, err := h.storage.PresignedPut(
		r.Context(), storage.ReleasesBucket, objectKey, request.ContentType, 20*time.Minute,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "生成上传地址失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"bucket": storage.ReleasesBucket, "object_key": objectKey,
		"upload_url": uploadURL, "expires_in": 1200,
		"headers": map[string]string{"Content-Type": request.ContentType},
	})
}

func (h *Handler) finalizeAppUpload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ObjectKey string `json:"object_key"`
		SHA256    string `json:"sha256"`
		Size      int64  `json:"size"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.ObjectKey = strings.TrimSpace(request.ObjectKey)
	request.SHA256 = strings.ToLower(strings.TrimSpace(request.SHA256))
	if !strings.HasPrefix(request.ObjectKey, "app/") ||
		(request.SHA256 != "" && !sha256Pattern.MatchString(request.SHA256)) || request.Size < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "安装包校验参数不合法")
		return
	}
	var existingID int64
	var existingSize int64
	var existingHash string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id,size_bytes,sha256 FROM media_assets
		WHERE bucket=? AND object_key=?`,
		storage.ReleasesBucket, request.ObjectKey,
	).Scan(&existingID, &existingSize, &existingHash)
	if err == nil {
		if existingSize != request.Size ||
			(request.SHA256 != "" && existingHash != request.SHA256) {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "对象已使用且校验信息不同")
			return
		}
		httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"asset_id": existingID})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取安装包失败")
		return
	}
	inspectContext, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	info, err := h.storage.InspectObject(inspectContext, storage.ReleasesBucket, request.ObjectKey)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "安装包不存在或不可读")
		return
	}
	if info.Size != request.Size ||
		(request.SHA256 != "" && info.SHA256 != request.SHA256) {
		_ = h.storage.RemoveObject(context.Background(), storage.ReleasesBucket, request.ObjectKey)
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "安装包大小或 SHA-256 校验失败")
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		INSERT INTO media_assets
			(owner_user_id,bucket,object_key,media_type,mime_type,size_bytes,sha256,status)
		VALUES(0,?,?, 'app_release',?,?,?,1)`,
		storage.ReleasesBucket, request.ObjectKey, info.ContentType, info.Size, info.SHA256,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "登记安装包失败")
		return
	}
	assetID, err := result.LastInsertId()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "登记安装包失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"asset_id": assetID, "size": info.Size, "sha256": info.SHA256,
	})
}

func (h *Handler) createAppRelease(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Platform       string `json:"platform"`
		ReleaseType    string `json:"release_type"`
		VersionName    string `json:"version_name"`
		VersionCode    int64  `json:"version_code"`
		MinNativeCode  int64  `json:"min_native_code"`
		ForceUpdate    bool   `json:"force_update"`
		SilentUpdate   bool   `json:"silent_update"`
		RolloutPercent int    `json:"rollout_percent"`
		AssetID        int64  `json:"asset_id"`
		ReleaseNotes   string `json:"release_notes"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Platform = strings.ToLower(strings.TrimSpace(request.Platform))
	request.ReleaseType = strings.ToLower(strings.TrimSpace(request.ReleaseType))
	request.VersionName = strings.TrimSpace(request.VersionName)
	if !validReleasePlatform(request.Platform) || !validReleaseType(request.ReleaseType) ||
		request.VersionName == "" || len(request.VersionName) > 40 ||
		request.VersionCode < 1 || request.MinNativeCode < 0 ||
		request.RolloutPercent < 0 || request.RolloutPercent > 100 || request.AssetID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "版本参数不合法")
		return
	}
	var packageSize int64
	var packageHash string
	var bucket string
	var objectKey string
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT size_bytes,sha256,bucket,object_key FROM media_assets
		WHERE id=? AND media_type='app_release' AND status=1`,
		request.AssetID,
	).Scan(&packageSize, &packageHash, &bucket, &objectKey); err != nil || bucket != storage.ReleasesBucket {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "安装包素材无效")
		return
	}
	extension := strings.ToLower(filepath.Ext(objectKey))
	if (request.ReleaseType == "wgt" && extension != ".wgt") ||
		(request.ReleaseType == "native" && extension == ".wgt") ||
		(request.ReleaseType == "native" && request.Platform == "app") ||
		(request.ReleaseType == "native" && request.Platform == "android" && extension != ".apk") ||
		(request.ReleaseType == "native" && request.Platform == "ios" && extension != ".ipa") {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "安装包类型与发布类型不匹配")
		return
	}
	adminUser, _ := adminFromRequest(r)
	result, err := h.db.ExecContext(r.Context(), `
		INSERT INTO app_releases
			(platform,release_type,version_name,version_code,min_native_code,force_update,silent_update,
			 rollout_percent,package_asset_id,package_size,package_sha256,release_notes,status,published_by)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,0,?)`,
		request.Platform, request.ReleaseType, request.VersionName, request.VersionCode,
		request.MinNativeCode, request.ForceUpdate, request.SilentUpdate, request.RolloutPercent,
		request.AssetID, packageSize, packageHash, request.ReleaseNotes, adminUser.ID,
	)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该平台版本号已存在")
			return
		}
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建版本失败")
		return
	}
	releaseID, err := result.LastInsertId()
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建版本失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"release_id": releaseID, "status": "draft"})
}

func (h *Handler) publishAppRelease(w http.ResponseWriter, r *http.Request) {
	releaseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || releaseID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "版本编号无效")
		return
	}
	adminUser, _ := adminFromRequest(r)
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发布版本失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var platform, releaseType string
	if err = tx.QueryRowContext(r.Context(), `
		SELECT platform,release_type FROM app_releases WHERE id=? FOR UPDATE`,
		releaseID,
	).Scan(&platform, &releaseType); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "版本不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发布版本失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE app_releases SET status=3
		WHERE platform=? AND release_type=? AND status=1 AND id<>?`,
		platform, releaseType, releaseID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发布版本失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		UPDATE app_releases
		SET status=1,published_by=?,published_at=CURRENT_TIMESTAMP(3)
		WHERE id=?`,
		adminUser.ID, releaseID,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发布版本失败")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `
		INSERT INTO audit_logs
			(request_id,actor_type,actor_id,action,resource_type,resource_id,ip,user_agent,after_data)
		VALUES(?,1,?,'app.release.publish','app_release',?,?,?,JSON_OBJECT('status','active'))`,
		httpx.RequestID(r.Context()), adminUser.ID, releaseID, clientIP(r), r.UserAgent(),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发布版本失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发布版本失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"release_id": releaseID, "status": "active"})
}

func (h *Handler) listAppReleases(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT app_release.id,app_release.platform,app_release.release_type,app_release.version_name,app_release.version_code,
		       app_release.min_native_code,app_release.force_update,app_release.silent_update,app_release.rollout_percent,
		       app_release.package_size,app_release.package_sha256,app_release.status,app_release.published_at,app_release.created_at
		FROM app_releases app_release
		ORDER BY app_release.created_at DESC,app_release.id DESC
		LIMIT ? OFFSET ?`,
		pageSize, (page-1)*pageSize,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取版本失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, versionCode, minNativeCode, packageSize int64
		var platform, releaseType, versionName, packageHash string
		var force, silent bool
		var rollout, status int
		var publishedAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &platform, &releaseType, &versionName, &versionCode, &minNativeCode,
			&force, &silent, &rollout, &packageSize, &packageHash, &status, &publishedAt, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取版本失败")
			return
		}
		items = append(items, map[string]any{
			"id": id, "platform": platform, "release_type": releaseType,
			"version_name": versionName, "version_code": versionCode, "min_native_code": minNativeCode,
			"force_update": force, "silent_update": silent, "rollout_percent": rollout,
			"package_size": packageSize, "package_sha256": packageHash, "status": status,
			"published_at": nullTime(publishedAt), "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取版本失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "items": items,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "请求格式错误")
		return false
	}
	return true
}

func pageParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func allowedReleaseExtension(extension string) bool {
	switch extension {
	case ".wgt", ".apk", ".aab", ".ipa":
		return true
	default:
		return false
	}
}

func validReleasePlatform(value string) bool {
	return value == "android" || value == "ios" || value == "app"
}

func validReleaseType(value string) bool {
	return value == "native" || value == "wgt"
}

func nullTime(value sql.NullTime) any {
	if value.Valid {
		return value.Time.Unix()
	}
	return nil
}
