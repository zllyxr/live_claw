package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/config"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/live"
	"github.com/zllyxr/live_claw/backend/internal/storage"
)

const (
	defaultTargetUsers = 300
	virtualSignature   = "分享真实、有趣的直播内容"
)

var roomIDPattern = regexp.MustCompile(`^[0-9]{5,128}$`)

type options struct {
	avatarsDir string
	target     int
	roomID     string
	title      string
	category   string
}

type avatar struct {
	path      string
	objectKey string
	mimeType  string
	sha256    string
	size      int64
	width     int
	height    int
	data      []byte
	publicURL string
	assetID   int64
	uploaded  bool
}

type userResult struct {
	ID       int64
	Username string
	Nickname string
	Created  bool
}

type summary struct {
	AvatarFiles int    `json:"avatar_files"`
	TargetUsers int    `json:"target_users"`
	Created     int    `json:"created_users"`
	Reused      int    `json:"reused_users"`
	HostUserID  int64  `json:"host_user_id"`
	HostName    string `json:"host_name"`
	LiveRoomID  int64  `json:"live_room_id"`
	RoomNo      string `json:"room_no"`
	ProviderID  string `json:"provider_room_id"`
	Resolution  string `json:"resolution"`
	Format      string `json:"format"`
	Delivery    string `json:"delivery"`
	Status      string `json:"status"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	opts := parseOptions()
	if err := run(opts, logger); err != nil {
		logger.Error("bootstrap virtual live", "error", err)
		os.Exit(1)
	}
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.avatarsDir, "avatars", "../女生", "directory containing female avatar images")
	flag.IntVar(&opts.target, "target-users", defaultTargetUsers, "target number of virtual live users")
	flag.StringVar(&opts.roomID, "room-id", "", "Douyin live room ID")
	flag.StringVar(&opts.title, "title", "", "live room title")
	flag.StringVar(&opts.category, "category", "聊天", "live room category")
	flag.Parse()
	opts.avatarsDir = strings.TrimSpace(opts.avatarsDir)
	opts.roomID = strings.TrimSpace(opts.roomID)
	opts.title = strings.TrimSpace(opts.title)
	opts.category = strings.TrimSpace(opts.category)
	return opts
}

func run(opts options, logger *slog.Logger) error {
	if opts.target < 1 || opts.target > 1000 {
		return errors.New("target-users must be between 1 and 1000")
	}
	if !roomIDPattern.MatchString(opts.roomID) {
		return errors.New("room-id must be a numeric Douyin room ID")
	}
	if opts.title == "" {
		opts.title = "轻松聊聊天"
	}
	if len(opts.title) > 300 || len(opts.category) > 60 {
		return errors.New("live room title or category is too long")
	}

	paths, err := avatarFiles(opts.avatarsDir)
	if err != nil {
		return err
	}
	if len(paths) < opts.target {
		return fmt.Errorf("only %d usable avatars found, need %d", len(paths), opts.target)
	}
	logger.Info("avatar pool ready", "files", len(paths), "target_users", opts.target)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg.MySQLDSN)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer db.Close()
	objects, err := storage.New(cfg)
	if err != nil {
		return fmt.Errorf("connect media storage: %w", err)
	}

	passwordHash, err := inaccessiblePasswordHash()
	if err != nil {
		return err
	}
	result := summary{AvatarFiles: len(paths), TargetUsers: opts.target, ProviderID: opts.roomID}
	var host userResult
	var hostAvatar avatar
	for index, path := range paths[:opts.target] {
		sequence := index + 1
		item, loadErr := loadAvatar(path, cfg.MediaBaseURL)
		if loadErr != nil {
			return fmt.Errorf("load avatar %s: %w", filepath.Base(path), loadErr)
		}
		user, ensureErr := ensureVirtualUser(ctx, db, objects, passwordHash, sequence, &item)
		if ensureErr != nil {
			return fmt.Errorf("create virtual user %d: %w", sequence, ensureErr)
		}
		if user.Created {
			result.Created++
		} else {
			result.Reused++
		}
		if sequence == 1 {
			host = user
			hostAvatar = item
		}
		if sequence%25 == 0 || sequence == opts.target {
			logger.Info("virtual users ready", "processed", sequence, "created", result.Created, "reused", result.Reused)
		}
	}

	liveRoomID, roomNo, err := ensureLiveRoom(
		ctx, db, host, hostAvatar, opts.roomID, opts.title, opts.category,
	)
	if err != nil {
		return err
	}
	result.HostUserID = host.ID
	result.HostName = host.Nickname
	result.LiveRoomID = liveRoomID
	result.RoomNo = roomNo

	source, err := live.New(db, nil).Resolve(ctx, host.ID, roomNo, true)
	if err != nil {
		_, _ = db.ExecContext(ctx, "UPDATE live_rooms SET status=0 WHERE id=?", liveRoomID)
		return fmt.Errorf("Douyin room cannot be relayed and was left offline: %w", err)
	}
	result.Resolution = source.Resolution
	result.Format = source.Format
	result.Delivery = source.Delivery
	result.Status = "online"
	if err = writeAudit(ctx, db, result); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func avatarFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("open avatar directory: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("avatar path is not a directory")
	}
	files := make([]string, 0, defaultTargetUsers)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".jpg", ".jpeg", ".png":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan avatar directory: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func loadAvatar(path, mediaBaseURL string) (avatar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return avatar{}, err
	}
	if len(data) == 0 || len(data) > 20<<20 {
		return avatar{}, errors.New("avatar is empty or larger than 20MB")
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data[:min(len(data), 512)]), ";")[0]))
	extension := ""
	switch mimeType {
	case "image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	default:
		return avatar{}, fmt.Errorf("unsupported image type %s", mimeType)
	}
	dimensions, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || dimensions.Width < 1 || dimensions.Height < 1 {
		return avatar{}, errors.New("avatar image is invalid")
	}
	digest := sha256.Sum256(data)
	encodedDigest := hex.EncodeToString(digest[:])
	objectKey := "virtual-avatars/girls/" + encodedDigest + extension
	return avatar{
		path: path, objectKey: objectKey, mimeType: mimeType, sha256: encodedDigest,
		size: int64(len(data)), width: dimensions.Width, height: dimensions.Height,
		data: data, publicURL: strings.TrimRight(mediaBaseURL, "/") + "/" + objectKey,
	}, nil
}

func ensureVirtualUser(
	ctx context.Context,
	db *sql.DB,
	objects *storage.Service,
	passwordHash string,
	sequence int,
	item *avatar,
) (userResult, error) {
	username := "virtual_live_" + fmt.Sprintf("%06d", sequence)
	nickname := virtualNickname(sequence - 1)

	var existingUserID, existingAssetID int64
	var isVirtual int
	err := db.QueryRowContext(ctx, `
		SELECT id,is_virtual,avatar_asset_id
		FROM users WHERE country_code='86' AND username=?`,
		username,
	).Scan(&existingUserID, &isVirtual, &existingAssetID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return userResult{}, err
	}
	if err == nil && isVirtual != 1 {
		return userResult{}, fmt.Errorf("reserved username %s belongs to a non-virtual account", username)
	}

	var assetID int64
	err = db.QueryRowContext(ctx, `
		SELECT id FROM media_assets
		WHERE bucket=? AND object_key=? AND status=1`,
		storage.PublicBucket, item.objectKey,
	).Scan(&assetID)
	if errors.Is(err, sql.ErrNoRows) {
		if err = objects.PutObject(
			ctx, storage.PublicBucket, item.objectKey, bytes.NewReader(item.data),
			item.size, item.mimeType,
		); err != nil {
			return userResult{}, err
		}
		item.uploaded = true
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO media_assets
				(owner_user_id,bucket,object_key,media_type,mime_type,size_bytes,width,height,sha256,status)
			VALUES(0,?,?,'image',?,?,?,?,?,1)`,
			storage.PublicBucket, item.objectKey, item.mimeType, item.size,
			item.width, item.height, item.sha256,
		)
		if insertErr != nil {
			var mysqlErr *mysqlDriver.MySQLError
			if !errors.As(insertErr, &mysqlErr) || mysqlErr.Number != 1062 {
				return userResult{}, insertErr
			}
			if err = db.QueryRowContext(ctx, `
				SELECT id FROM media_assets WHERE bucket=? AND object_key=? AND status=1`,
				storage.PublicBucket, item.objectKey,
			).Scan(&assetID); err != nil {
				return userResult{}, err
			}
		} else {
			assetID, err = result.LastInsertId()
			if err != nil {
				return userResult{}, err
			}
		}
	} else if err != nil {
		return userResult{}, err
	}
	item.assetID = assetID

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return userResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	created := existingUserID == 0
	userID := existingUserID
	if created {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO users
				(username,country_code,password_hash,password_algo,nickname,avatar_asset_id,
				 gender,signature,status,is_virtual)
			VALUES(?,'86',?,'argon2id',?,?,2,?,1,1)`,
			username, passwordHash, nickname, assetID, virtualSignature,
		)
		if insertErr != nil {
			return userResult{}, insertErr
		}
		userID, err = result.LastInsertId()
		if err != nil {
			return userResult{}, err
		}
	} else {
		if _, err = tx.ExecContext(ctx, `
			UPDATE users
			SET nickname=?,avatar_asset_id=?,gender=2,signature=?,status=1,is_virtual=1
			WHERE id=?`,
			nickname, assetID, virtualSignature, userID,
		); err != nil {
			return userResult{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT IGNORE INTO wallet_accounts(user_id,currency,available,frozen,status)
		VALUES(?,'COIN',0,0,1)`,
		userID,
	); err != nil {
		return userResult{}, err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE media_assets SET owner_user_id=? WHERE id=? AND owner_user_id=0`,
		userID, assetID,
	); err != nil {
		return userResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return userResult{}, err
	}
	return userResult{ID: userID, Username: username, Nickname: nickname, Created: created}, nil
}

func ensureLiveRoom(
	ctx context.Context,
	db *sql.DB,
	host userResult,
	hostAvatar avatar,
	providerRoomID, title, category string,
) (int64, string, error) {
	roomNo, err := idgen.New()
	if err != nil {
		return 0, "", err
	}
	page := "https://live.douyin.com/" + providerRoomID
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(ctx, `
		INSERT INTO live_rooms
			(room_no,host_user_id,title,category,provider,provider_room_id,provider_page,status,sort_order,last_seen_at)
		VALUES(?,?,?,?,'douyin',?,?,1,100,CURRENT_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE
			id=LAST_INSERT_ID(id),host_user_id=VALUES(host_user_id),title=VALUES(title),
			category=VALUES(category),provider_page=VALUES(provider_page),status=1,
			sort_order=VALUES(sort_order),last_seen_at=CURRENT_TIMESTAMP(3)`,
		roomNo, host.ID, title, category, providerRoomID, page,
	)
	if err != nil {
		return 0, "", fmt.Errorf("save live room: %w", err)
	}
	liveRoomID, err := result.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO douyin_room_profiles
			(live_room_id,nickname,avatar_url,cover_url,verify_status,verified_by,verified_at,last_resolve_status)
		VALUES(?,?,?,?,1,0,CURRENT_TIMESTAMP(3),0)
		ON DUPLICATE KEY UPDATE
			nickname=VALUES(nickname),avatar_url=VALUES(avatar_url),cover_url=VALUES(cover_url),
			verify_status=1,verified_by=0,verified_at=CURRENT_TIMESTAMP(3)`,
		liveRoomID, host.Nickname, hostAvatar.publicURL, hostAvatar.publicURL,
	); err != nil {
		return 0, "", fmt.Errorf("save Douyin room profile: %w", err)
	}
	if err = tx.QueryRowContext(ctx, "SELECT room_no FROM live_rooms WHERE id=?", liveRoomID).Scan(&roomNo); err != nil {
		return 0, "", err
	}
	if err = tx.Commit(); err != nil {
		return 0, "", err
	}
	return liveRoomID, roomNo, nil
}

func inaccessiblePasswordHash() (string, error) {
	randomPassword := make([]byte, 32)
	if _, err := rand.Read(randomPassword); err != nil {
		return "", err
	}
	password := base64.RawURLEncoding.EncodeToString(randomPassword)
	hash, err := adminauth.HashPassword(password)
	for index := range randomPassword {
		randomPassword[index] = 0
	}
	if err != nil {
		return "", fmt.Errorf("hash virtual account password: %w", err)
	}
	return hash, nil
}

func virtualNickname(index int) string {
	first := []string{
		"晚晚", "小鹿", "橙子", "念念", "星禾", "夏沫", "青柠", "桃桃", "浅月", "南栀",
		"可可", "小满", "安然", "微凉", "糖糖", "鹿鸣", "初晴", "暖暖", "七七", "果果",
		"清欢", "若溪", "云朵", "小葵", "米粒", "阿梨", "柚子", "团子", "月牙", "芊芊",
	}
	last := []string{
		"日记", "慢生活", "的晚风", "在发光", "小屋", "随手拍", "有点甜", "看世界", "频道", "碎碎念",
		"的夏天", "来啦", "在路上", "的小确幸", "今日份", "不熬夜", "分享站", "的星光", "好心情", "生活志",
		"小宇宙", "的清晨", "听风", "放映室", "的日常", "记录簿", "漫游记", "聊天室", "的角落", "轻松一刻",
	}
	if index < 0 {
		index = 0
	}
	return first[index%len(first)] + last[(index/len(first))%len(last)]
}

func writeAudit(ctx context.Context, db *sql.DB, result summary) error {
	requestID, err := idgen.New()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO audit_logs
			(request_id,actor_type,actor_id,action,resource_type,resource_id,after_data,ip,user_agent)
		VALUES(?,3,0,'live.virtual.bootstrap','live_room',?,?,'127.0.0.1','virtual-live-bootstrap')`,
		requestID, strconv.FormatInt(result.LiveRoomID, 10), payload,
	)
	if err != nil {
		return fmt.Errorf("write bootstrap audit log: %w", err)
	}
	return nil
}
