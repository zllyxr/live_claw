package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"image/jpeg"
	"log/slog"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/storage"
)

type profilePhotoChange struct {
	PreviousID    *decimalIDInput `json:"previous_id"`
	Data          []byte          `json:"data"`
	width, height int
}

func (p *profilePhotoChange) conflicts(current decimalIDInput) bool {
	return p != nil && *p.PreviousID != current
}

func (p *profilePhotoChange) validate(width, height int) error {
	if p == nil {
		return nil
	}
	if p.PreviousID == nil {
		return errors.New("缺少原图编号")
	}
	if len(p.Data) == 0 || len(p.Data) > 2<<20 {
		return errors.New("图片为空或过大")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(p.Data))
	if err != nil || cfg.Width != width || cfg.Height != height {
		return fmt.Errorf("须为 %d × %d 的 JPEG 裁剪图", width, height)
	}
	img, err := jpeg.Decode(bytes.NewReader(p.Data))
	if err != nil {
		return errors.New("文件无法解码")
	}
	// Store only the decoded image, without client metadata or trailing bytes.
	var b bytes.Buffer
	if err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 92}); err != nil {
		return errors.New("图片处理失败")
	}
	p.Data = b.Bytes()
	p.width, p.height = cfg.Width, cfg.Height
	return nil
}

func (h *Handler) storeProfilePhoto(ctx context.Context, tx *sql.Tx, userID int64, photo *profilePhotoChange) (int64, string, error) {
	data := photo.Data
	name, err := idgen.New()
	if err != nil {
		return 0, "", err
	}
	key := fmt.Sprintf("users/%d/profile/%s.jpg", userID, name)
	if err = h.storage.PutObject(ctx, storage.PublicBucket, key, bytes.NewReader(data), int64(len(data)), "image/jpeg"); err != nil {
		return 0, key, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO media_assets
			(owner_user_id,bucket,object_key,media_type,mime_type,size_bytes,width,height,sha256,status)
		VALUES(?,?,?,'image','image/jpeg',?,?,?,?,1)`, userID, storage.PublicBucket, key, len(data), photo.width, photo.height, fmt.Sprintf("%x", sha256.Sum256(data)))
	if err != nil {
		return 0, key, err
	}
	id, err := result.LastInsertId()
	return id, key, err
}

func (h *Handler) discardProfileUploads(keys []string) {
	if len(keys) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, key := range keys {
		if err := h.storage.RemoveObject(ctx, storage.PublicBucket, key); err != nil {
			slog.Error("remove unused profile image", "object_key", key, "error", err)
		}
	}
}
