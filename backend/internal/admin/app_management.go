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
		httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{"asset_id": apiDecimalID(existingID)})
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
		"asset_id": apiDecimalID(assetID), "size": info.Size, "sha256": info.SHA256,
	})
}

func (h *Handler) createAppRelease(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Platform       string         `json:"platform"`
		ReleaseType    string         `json:"release_type"`
		VersionName    string         `json:"version_name"`
		VersionCode    int64          `json:"version_code"`
		MinNativeCode  int64          `json:"min_native_code"`
		ForceUpdate    bool           `json:"force_update"`
		SilentUpdate   bool           `json:"silent_update"`
		RolloutPercent int            `json:"rollout_percent"`
		AssetID        decimalIDInput `json:"asset_id"`
		ReleaseNotes   string         `json:"release_notes"`
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
		request.RolloutPercent < 0 || request.RolloutPercent > 100 || request.AssetID.Int64() < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "版本参数不合法")
		return
	}
	if request.ReleaseType == "native" && request.SilentUpdate {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "原生整包版本不支持静默更新")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建版本失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	var packageSize int64
	var packageHash string
	var bucket string
	var objectKey string
	if err = tx.QueryRowContext(r.Context(), `
		SELECT size_bytes,sha256,bucket,object_key FROM media_assets
		WHERE id=? AND media_type='app_release' AND status=1
		FOR SHARE`,
		request.AssetID.Int64(),
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
	result, err := tx.ExecContext(r.Context(), `
		INSERT INTO app_releases
			(platform,release_type,version_name,version_code,min_native_code,force_update,silent_update,
			 rollout_percent,package_asset_id,package_size,package_sha256,release_notes,status,published_by)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,0,?)`,
		request.Platform, request.ReleaseType, request.VersionName, request.VersionCode,
		request.MinNativeCode, request.ForceUpdate, request.SilentUpdate, request.RolloutPercent,
		request.AssetID.Int64(), packageSize, packageHash, request.ReleaseNotes, adminUser.ID,
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
	if err = auditAdmin(
		r.Context(), tx, r, "app.release.create", "app_release", releaseID,
		nil, map[string]any{
			"platform":         request.Platform,
			"release_type":     request.ReleaseType,
			"version_name":     request.VersionName,
			"version_code":     request.VersionCode,
			"min_native_code":  request.MinNativeCode,
			"force_update":     request.ForceUpdate,
			"silent_update":    request.SilentUpdate,
			"rollout_percent":  request.RolloutPercent,
			"package_asset_id": apiDecimalID(request.AssetID.Int64()),
			"status":           "draft",
		},
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录版本审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "创建版本失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"release_id": apiDecimalID(releaseID), "status": "draft",
	})
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
	var status int
	var silentUpdate bool
	if err = tx.QueryRowContext(r.Context(), `
		SELECT platform,release_type,status,silent_update
		FROM app_releases WHERE id=? FOR UPDATE`,
		releaseID,
	).Scan(&platform, &releaseType, &status, &silentUpdate); errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "版本不存在")
		return
	} else if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "发布版本失败")
		return
	}
	if status != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "仅草稿版本可以发布")
		return
	}
	if releaseType != "wgt" && silentUpdate {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "原生整包版本不能启用静默热更新")
		return
	}
	if err = lockAppReleaseNamespace(r, tx, platform, releaseType); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "锁定版本发布通道失败")
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
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"release_id": apiDecimalID(releaseID), "status": "active",
	})
}

type updateAppReleaseRequest struct {
	VersionName    *string `json:"version_name"`
	VersionCode    *int64  `json:"version_code"`
	MinNativeCode  *int64  `json:"min_native_code"`
	ForceUpdate    *bool   `json:"force_update"`
	SilentUpdate   *bool   `json:"silent_update"`
	RolloutPercent *int    `json:"rollout_percent"`
	ReleaseNotes   *string `json:"release_notes"`
}

func (request *updateAppReleaseRequest) normalize() error {
	if request.VersionName == nil && request.VersionCode == nil &&
		request.MinNativeCode == nil && request.ForceUpdate == nil &&
		request.SilentUpdate == nil && request.RolloutPercent == nil &&
		request.ReleaseNotes == nil {
		return errors.New("empty app release update")
	}
	if request.VersionName != nil {
		value := strings.TrimSpace(*request.VersionName)
		request.VersionName = &value
		if value == "" || len(value) > 40 {
			return errors.New("invalid version name")
		}
	}
	if request.VersionCode != nil && *request.VersionCode < 1 {
		return errors.New("invalid version code")
	}
	if request.MinNativeCode != nil && *request.MinNativeCode < 0 {
		return errors.New("invalid minimum native code")
	}
	if request.RolloutPercent != nil &&
		(*request.RolloutPercent < 0 || *request.RolloutPercent > 100) {
		return errors.New("invalid rollout percent")
	}
	if request.ReleaseNotes != nil {
		value := strings.TrimSpace(*request.ReleaseNotes)
		request.ReleaseNotes = &value
		if len(value) > 2000 {
			return errors.New("release notes too long")
		}
	}
	return nil
}

type managedAppRelease struct {
	ID             int64
	Platform       string
	ReleaseType    string
	VersionName    string
	VersionCode    int64
	MinNativeCode  int64
	ForceUpdate    bool
	SilentUpdate   bool
	RolloutPercent int
	ReleaseNotes   string
	PackageAssetID int64
	PackageSize    int64
	PackageSHA256  string
	Status         int
}

func loadManagedAppRelease(
	r *http.Request,
	tx *sql.Tx,
	releaseID int64,
) (managedAppRelease, error) {
	var release managedAppRelease
	err := tx.QueryRowContext(r.Context(), `
		SELECT id,platform,release_type,version_name,version_code,min_native_code,
		       force_update,silent_update,rollout_percent,release_notes,
		       package_asset_id,package_size,package_sha256,status
		FROM app_releases WHERE id=? FOR UPDATE`,
		releaseID,
	).Scan(
		&release.ID, &release.Platform, &release.ReleaseType,
		&release.VersionName, &release.VersionCode, &release.MinNativeCode,
		&release.ForceUpdate, &release.SilentUpdate, &release.RolloutPercent,
		&release.ReleaseNotes, &release.PackageAssetID, &release.PackageSize,
		&release.PackageSHA256, &release.Status,
	)
	return release, err
}

func (release managedAppRelease) auditData() map[string]any {
	return map[string]any{
		"platform":         release.Platform,
		"release_type":     release.ReleaseType,
		"version_name":     release.VersionName,
		"version_code":     release.VersionCode,
		"min_native_code":  release.MinNativeCode,
		"force_update":     release.ForceUpdate,
		"silent_update":    release.SilentUpdate,
		"rollout_percent":  release.RolloutPercent,
		"release_notes":    release.ReleaseNotes,
		"package_asset_id": apiDecimalID(release.PackageAssetID),
		"package_size":     release.PackageSize,
		"package_sha256":   release.PackageSHA256,
		"status":           appReleaseStatusName(release.Status),
	}
}

func (h *Handler) updateAppRelease(w http.ResponseWriter, r *http.Request) {
	releaseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || releaseID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "版本编号无效")
		return
	}
	var request updateAppReleaseRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err = request.normalize(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "版本更新参数不合法")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新版本失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	release, err := loadManagedAppRelease(r, tx, releaseID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "版本不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新版本失败")
		return
	}
	if release.Status != 0 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "仅草稿版本可以编辑")
		return
	}
	if release.ReleaseType != "wgt" && request.SilentUpdate != nil && *request.SilentUpdate {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "原生整包版本不能启用静默热更新")
		return
	}
	before := release.auditData()
	if request.VersionName != nil {
		release.VersionName = *request.VersionName
	}
	if request.VersionCode != nil {
		release.VersionCode = *request.VersionCode
	}
	if request.MinNativeCode != nil {
		release.MinNativeCode = *request.MinNativeCode
	}
	if request.ForceUpdate != nil {
		release.ForceUpdate = *request.ForceUpdate
	}
	if request.SilentUpdate != nil {
		release.SilentUpdate = *request.SilentUpdate
	}
	if request.RolloutPercent != nil {
		release.RolloutPercent = *request.RolloutPercent
	}
	if request.ReleaseNotes != nil {
		release.ReleaseNotes = *request.ReleaseNotes
	}
	if release.ReleaseType != "wgt" && release.SilentUpdate {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "原生整包版本不能启用静默热更新")
		return
	}
	_, err = tx.ExecContext(r.Context(), `
		UPDATE app_releases
		SET version_name=?,version_code=?,min_native_code=?,force_update=?,
		    silent_update=?,rollout_percent=?,release_notes=?
		WHERE id=? AND status=0`,
		release.VersionName, release.VersionCode, release.MinNativeCode,
		release.ForceUpdate, release.SilentUpdate, release.RolloutPercent,
		release.ReleaseNotes, releaseID,
	)
	if err != nil {
		var mysqlErr *mysqlDriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "该平台版本号已存在")
			return
		}
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新版本失败")
		return
	}
	if err = auditAdmin(
		r.Context(), tx, r, "app.release.update", "app_release", releaseID,
		before, release.auditData(),
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录版本审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新版本失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"release_id": apiDecimalID(releaseID), "status": "draft", "updated": true,
	})
}

func (h *Handler) pauseAppRelease(w http.ResponseWriter, r *http.Request) {
	h.changeAppReleaseLifecycle(w, r, "pause")
}

func (h *Handler) resumeAppRelease(w http.ResponseWriter, r *http.Request) {
	h.changeAppReleaseLifecycle(w, r, "resume")
}

func (h *Handler) archiveAppRelease(w http.ResponseWriter, r *http.Request) {
	h.changeAppReleaseLifecycle(w, r, "archive")
}

func (h *Handler) changeAppReleaseLifecycle(w http.ResponseWriter, r *http.Request, action string) {
	releaseID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || releaseID < 1 {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "版本编号无效")
		return
	}
	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新版本状态失败")
		return
	}
	defer tx.Rollback() //nolint:errcheck
	release, err := loadManagedAppRelease(r, tx, releaseID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusNotFound, 404, "版本不存在")
		return
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新版本状态失败")
		return
	}
	previousStatus := release.Status
	nextStatus := -1
	switch action {
	case "pause":
		if previousStatus != 1 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "仅已发布版本可以暂停")
			return
		}
		nextStatus = 2
	case "resume":
		if previousStatus != 2 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "仅已暂停版本可以恢复")
			return
		}
		if release.ReleaseType != "wgt" && release.SilentUpdate {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "原生整包版本不能启用静默热更新")
			return
		}
		nextStatus = 1
	case "archive":
		if previousStatus < 0 || previousStatus > 2 {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusConflict, 409, "版本已归档")
			return
		}
		nextStatus = 3
	default:
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusBadRequest, 400, "版本状态操作无效")
		return
	}
	retiredActive := int64(0)
	if action == "resume" {
		if err = lockAppReleaseNamespace(r, tx, release.Platform, release.ReleaseType); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "锁定版本发布通道失败")
			return
		}
		result, updateErr := tx.ExecContext(r.Context(), `
			UPDATE app_releases SET status=3
			WHERE platform=? AND release_type=? AND status=1 AND id<>?`,
			release.Platform, release.ReleaseType, releaseID,
		)
		if updateErr != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "恢复版本失败")
			return
		}
		retiredActive, _ = result.RowsAffected()
	}
	adminUser, _ := adminFromRequest(r)
	if action == "resume" {
		_, err = tx.ExecContext(r.Context(), `
			UPDATE app_releases
			SET status=?,published_by=?,published_at=COALESCE(published_at,CURRENT_TIMESTAMP(3))
			WHERE id=?`,
			nextStatus, adminUser.ID, releaseID,
		)
	} else {
		_, err = tx.ExecContext(r.Context(), "UPDATE app_releases SET status=? WHERE id=?", nextStatus, releaseID)
	}
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新版本状态失败")
		return
	}
	before := release.auditData()
	release.Status = nextStatus
	after := release.auditData()
	if action == "resume" {
		after["retired_active_releases"] = retiredActive
	}
	if err = auditAdmin(
		r.Context(), tx, r, "app.release."+action, "app_release", releaseID,
		before, after,
	); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "记录版本审计失败")
		return
	}
	if err = tx.Commit(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "更新版本状态失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"release_id": apiDecimalID(releaseID), "status": appReleaseStatusName(nextStatus),
	})
}

func lockAppReleaseNamespace(
	r *http.Request,
	tx *sql.Tx,
	platform string,
	releaseType string,
) error {
	_, err := tx.ExecContext(r.Context(), `
		INSERT INTO app_release_lifecycle_locks(platform,release_type)
		VALUES(?,?)
		ON DUPLICATE KEY UPDATE platform=VALUES(platform)`,
		platform, releaseType,
	)
	return err
}

func appReleaseStatusName(status int) string {
	switch status {
	case 0:
		return "draft"
	case 1:
		return "active"
	case 2:
		return "paused"
	case 3:
		return "archived"
	default:
		return "unknown"
	}
}

func (h *Handler) listAppReleases(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	platform := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("platform")))
	releaseType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("release_type")))
	status := -1
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		status, _ = strconv.Atoi(rawStatus)
	}
	like := "%" + escapeLike(keyword) + "%"
	filterArguments := []any{
		keyword, like, like, like, like, like,
		platform, platform,
		releaseType, releaseType,
		status, status,
	}
	var total int64
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM app_releases app_release
		WHERE (?='' OR CAST(app_release.id AS CHAR) LIKE ?
		       OR app_release.version_name LIKE ?
		       OR CAST(app_release.version_code AS CHAR) LIKE ?
		       OR app_release.package_sha256 LIKE ?
		       OR app_release.release_notes LIKE ?)
		  AND (?='' OR app_release.platform=?)
		  AND (?='' OR app_release.release_type=?)
		  AND (? < 0 OR app_release.status=?)`,
		filterArguments...,
	).Scan(&total); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取版本失败")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT app_release.id,app_release.platform,app_release.release_type,app_release.version_name,app_release.version_code,
		       app_release.min_native_code,app_release.force_update,app_release.silent_update,app_release.rollout_percent,
		       app_release.package_asset_id,app_release.package_size,app_release.package_sha256,
		       app_release.release_notes,app_release.status,app_release.published_at,app_release.created_at
		FROM app_releases app_release
		WHERE (?='' OR CAST(app_release.id AS CHAR) LIKE ?
		       OR app_release.version_name LIKE ?
		       OR CAST(app_release.version_code AS CHAR) LIKE ?
		       OR app_release.package_sha256 LIKE ?
		       OR app_release.release_notes LIKE ?)
		  AND (?='' OR app_release.platform=?)
		  AND (?='' OR app_release.release_type=?)
		  AND (? < 0 OR app_release.status=?)
		ORDER BY app_release.created_at DESC,app_release.id DESC
		LIMIT ? OFFSET ?`,
		append(filterArguments, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取版本失败")
		return
	}
	defer rows.Close()
	items := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, versionCode, minNativeCode, packageAssetID, packageSize int64
		var platform, releaseType, versionName, packageHash, releaseNotes string
		var force, silent bool
		var rollout, status int
		var publishedAt sql.NullTime
		var createdAt time.Time
		if err = rows.Scan(
			&id, &platform, &releaseType, &versionName, &versionCode, &minNativeCode,
			&force, &silent, &rollout, &packageAssetID, &packageSize, &packageHash,
			&releaseNotes, &status, &publishedAt, &createdAt,
		); err != nil {
			httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取版本失败")
			return
		}
		items = append(items, map[string]any{
			"id": apiDecimalID(id), "platform": platform, "release_type": releaseType,
			"version_name": versionName, "version_code": versionCode, "min_native_code": minNativeCode,
			"force_update": force, "silent_update": silent, "rollout_percent": rollout,
			"package_asset_id": apiDecimalID(packageAssetID), "package_size": packageSize,
			"package_sha256": packageHash, "release_notes": releaseNotes, "status": status,
			"published_at": nullTime(publishedAt), "created_at": createdAt.Unix(),
		})
	}
	if err = rows.Err(); err != nil {
		httpx.Error(w, httpx.RequestID(r.Context()), http.StatusInternalServerError, 500, "读取版本失败")
		return
	}
	httpx.OK(w, httpx.RequestID(r.Context()), map[string]any{
		"page": page, "page_size": pageSize, "total": total,
		"has_more": int64(page)*int64(pageSize) < total, "items": items,
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
