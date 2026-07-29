package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/storage"
)

const maximumUserUpload = 50 << 20

var uploadMIMETypes = map[string]string{
	"image/jpeg":       ".jpg",
	"image/png":        ".png",
	"image/webp":       ".webp",
	"image/gif":        ".gif",
	"video/mp4":        ".mp4",
	"video/quicktime":  ".mov",
	"audio/mpeg":       ".mp3",
	"audio/mp4":        ".m4a",
	"audio/wav":        ".wav",
	"audio/x-wav":      ".wav",
	"application/pdf":  ".pdf",
	"application/zip":  ".zip",
	"text/plain":       ".txt",
	"application/json": ".json",
}

func (s *Server) compatUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.storage == nil {
		writeCompat(w, 405, "上传方式不受支持", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maximumUserUpload+(2<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeCompat(w, 400, "上传文件无效或超过 50MB", nil)
		return
	}
	userID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("uid")), 10, 64)
	if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
		writeCompat(w, 700, "登录已失效", nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeCompat(w, 400, "请选择上传文件", nil)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximumUserUpload+1))
	if err != nil || len(data) == 0 || len(data) > maximumUserUpload {
		writeCompat(w, 400, "上传文件无效或超过 50MB", nil)
		return
	}
	detected := http.DetectContentType(data[:min(len(data), 512)])
	declared := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	contentType, extension := acceptedUploadType(detected, declared, header.Filename)
	if contentType == "" {
		writeCompat(w, 400, "不支持该文件类型", nil)
		return
	}
	objectID, err := idgen.New()
	if err != nil {
		writeCompat(w, 500, "生成上传编号失败", nil)
		return
	}
	now := time.Now()
	objectKey := "users/" + strconv.FormatInt(userID, 10) + "/" +
		now.Format("2006/01") + "/" + strings.ToLower(objectID) + extension
	if err = s.storage.PutObject(
		r.Context(), storage.PublicBucket, objectKey, bytes.NewReader(data),
		int64(len(data)), contentType,
	); err != nil {
		s.logger.Error("user upload to minio", "user_id", userID, "error", err)
		writeCompat(w, 500, "文件存储失败", nil)
		return
	}
	digest := sha256.Sum256(data)
	result, err := s.db.ExecContext(r.Context(), `
		INSERT INTO media_assets
			(owner_user_id,bucket,object_key,media_type,mime_type,size_bytes,sha256,status)
		VALUES(?,?,?,?,?,?,?,1)`,
		userID, storage.PublicBucket, objectKey, mediaType(contentType), contentType,
		len(data), hex.EncodeToString(digest[:]),
	)
	if err != nil {
		_ = s.storage.RemoveObject(r.Context(), storage.PublicBucket, objectKey)
		s.logger.Error("record user upload", "user_id", userID, "error", err)
		writeCompat(w, 500, "记录上传文件失败", nil)
		return
	}
	assetID, _ := result.LastInsertId()
	publicURL := strings.TrimRight(s.mediaBaseURL, "/") + "/" + objectKey
	writeCompat(w, 0, "", map[string]any{
		"id": assetID, "asset_id": assetID, "file": publicURL, "file_name": publicURL,
		"filepath": publicURL, "url": publicURL, "mime_type": contentType, "size": len(data),
	})
}

func acceptedUploadType(detected, declared, originalName string) (string, string) {
	detected = strings.ToLower(strings.TrimSpace(strings.Split(detected, ";")[0]))
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	if extension, ok := uploadMIMETypes[detected]; ok {
		return detected, extension
	}
	if detected != "application/octet-stream" {
		return "", ""
	}
	if extension, ok := uploadMIMETypes[declared]; ok {
		return declared, extension
	}
	extension := strings.ToLower(filepath.Ext(originalName))
	for mimeType, allowedExtension := range uploadMIMETypes {
		if extension == allowedExtension {
			return mimeType, extension
		}
	}
	return "", ""
}

func mediaType(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	default:
		return "file"
	}
}
