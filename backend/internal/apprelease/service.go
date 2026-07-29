package apprelease

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/storage"
)

type Service struct {
	db      *sql.DB
	storage *storage.Service
	now     func() time.Time
}

type release struct {
	VersionName   string
	VersionCode   int64
	MinNativeCode int64
	Force         bool
	Silent        bool
	Rollout       int
	PackageSize   int64
	SHA256        string
	Notes         string
	Bucket        string
	ObjectKey     string
}

func New(db *sql.DB, storageService *storage.Service) *Service {
	return &Service{db: db, storage: storageService, now: time.Now}
}

func (s *Service) Check(
	ctx context.Context,
	currentWGTCode, nativeAppCode int64,
	platform, deviceKey string,
) (map[string]any, error) {
	if nativeAppCode <= 0 {
		nativeAppCode = currentWGTCode
	}
	platform = normalizePlatform(platform)
	result := map[string]any{
		"has_update":   "0",
		"current_code": strconv.FormatInt(currentWGTCode, 10),
		"server_time":  strconv.FormatInt(s.now().Unix(), 10),
	}

	wgt, found, err := s.latest(ctx, platform, "wgt", currentWGTCode)
	if err != nil {
		return nil, err
	}
	if found && rolloutAllows(wgt.Rollout, deviceKey, wgt.VersionCode) {
		if wgt.MinNativeCode > 0 && nativeAppCode < wgt.MinNativeCode {
			result["native_upgrade_required"] = "1"
			result["min_app_code"] = strconv.FormatInt(wgt.MinNativeCode, 10)
		} else {
			downloadURL, signErr := s.storage.PresignedGet(
				ctx, wgt.Bucket, wgt.ObjectKey, 30*time.Minute,
			)
			if signErr != nil {
				return nil, signErr
			}
			result["has_update"] = "1"
			result["version_name"] = wgt.VersionName
			result["version_code"] = strconv.FormatInt(wgt.VersionCode, 10)
			result["size"] = strconv.FormatInt(wgt.PackageSize, 10)
			result["sha256"] = wgt.SHA256
			result["note"] = wgt.Notes
			result["force"] = boolString(wgt.Force)
			result["silent"] = boolString(wgt.Silent)
			result["wgt_url"] = downloadURL
		}
	}

	native, nativeFound, err := s.latest(ctx, platform, "native", nativeAppCode)
	if err != nil {
		return nil, err
	}
	if nativeFound && rolloutAllows(native.Rollout, deviceKey, native.VersionCode) {
		downloadURL, signErr := s.storage.PresignedGet(
			ctx, native.Bucket, native.ObjectKey, 2*time.Hour,
		)
		if signErr != nil {
			return nil, signErr
		}
		result["native_update"] = map[string]any{
			"version_name": native.VersionName,
			"version_code": strconv.FormatInt(native.VersionCode, 10),
			"size":         strconv.FormatInt(native.PackageSize, 10),
			"sha256":       native.SHA256,
			"note":         native.Notes,
			"force":        boolString(native.Force),
			"download_url": downloadURL,
		}
		if native.Force || result["native_upgrade_required"] == "1" {
			result["native_upgrade_required"] = "1"
		}
	}
	return result, nil
}

func (s *Service) latest(ctx context.Context, platform, releaseType string, currentCode int64) (release, bool, error) {
	var result release
	err := s.db.QueryRowContext(ctx, `
		SELECT app_release.version_name,app_release.version_code,app_release.min_native_code,
		       app_release.force_update,app_release.silent_update,app_release.rollout_percent,
		       app_release.package_size,app_release.package_sha256,app_release.release_notes,
		       asset.bucket,asset.object_key
		FROM app_releases app_release
		JOIN media_assets asset ON asset.id=app_release.package_asset_id AND asset.status=1
		WHERE app_release.platform IN (?, 'app') AND app_release.release_type=?
		  AND app_release.status=1 AND app_release.version_code>?
		ORDER BY app_release.version_code DESC
		LIMIT 1`,
		platform, releaseType, currentCode,
	).Scan(
		&result.VersionName, &result.VersionCode, &result.MinNativeCode,
		&result.Force, &result.Silent, &result.Rollout, &result.PackageSize,
		&result.SHA256, &result.Notes, &result.Bucket, &result.ObjectKey,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return release{}, false, nil
	}
	return result, err == nil, err
}

func normalizePlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ios":
		return "ios"
	default:
		return "android"
	}
}

func rolloutAllows(percent int, deviceKey string, versionCode int64) bool {
	if percent >= 100 {
		return true
	}
	if percent <= 0 || strings.TrimSpace(deviceKey) == "" {
		return false
	}
	sum := sha256.Sum256([]byte(deviceKey + ":" + strconv.FormatInt(versionCode, 10)))
	return int(binary.BigEndian.Uint32(sum[:4])%100) < percent
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
