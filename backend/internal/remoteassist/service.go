package remoteassist

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

var (
	ErrDisabled        = errors.New("remote assistance is disabled")
	ErrInvalid         = errors.New("invalid remote assistance request")
	ErrNotFound        = errors.New("remote assistance record not found")
	ErrConflict        = errors.New("remote assistance state conflict")
	ErrOffline         = errors.New("remote assistance device is offline")
	ErrNotReady        = errors.New("remote assistance credential is not ready")
	ErrNoFrame         = errors.New("remote assistance frame is not available")
	ErrAlreadyRevealed = errors.New("remote assistance credential was already revealed")
)

const (
	credentialLifetime = 120 * time.Second
	onlineWindow       = 20 * time.Second
)

type Config struct {
	Enabled        bool
	AllowedUserIDs []int64
}

type Service struct {
	db             *sql.DB
	cipher         *secretCipher
	config         Config
	now            func() time.Time
	allowedUserIDs map[int64]struct{}
	realtime       *realtimeStore
}

type EnrollRequest struct {
	InstallID      string `json:"install_id"`
	DeviceName     string `json:"device_name"`
	Manufacturer   string `json:"manufacturer"`
	Model          string `json:"model"`
	AndroidVersion string `json:"android_version"`
	AndroidSDK     int    `json:"android_sdk"`
	AppVersion     string `json:"app_version"`
	AppNativeCode  int    `json:"app_native_code"`
	PluginVersion  string `json:"plugin_version"`
}

type Enrollment struct {
	DeviceID         string `json:"device_id"`
	DeviceToken      string `json:"device_token"`
	HeartbeatSeconds int    `json:"heartbeat_seconds"`
}

type Device struct {
	ID               string          `json:"id"`
	UserID           int64           `json:"user_id,string"`
	Username         string          `json:"username,omitempty"`
	InstallID        string          `json:"install_id,omitempty"`
	DeviceName       string          `json:"device_name"`
	Manufacturer     string          `json:"manufacturer"`
	Model            string          `json:"model"`
	AndroidVersion   string          `json:"android_version"`
	AndroidSDK       int             `json:"android_sdk"`
	AppVersion       string          `json:"app_version"`
	AppNativeCode    int             `json:"app_native_code"`
	PluginVersion    string          `json:"plugin_version"`
	DeviceCode       string          `json:"device_code"`
	ServiceStatus    string          `json:"service_status"`
	PermissionStatus json.RawMessage `json:"permission_status,omitempty"`
	Capabilities     json.RawMessage `json:"capabilities,omitempty"`
	Status           int             `json:"status"`
	Online           bool            `json:"online"`
	LastSeenAt       *time.Time      `json:"last_seen_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	CurrentSession   bool            `json:"current_session"`
}

type Heartbeat struct {
	DeviceCode       string         `json:"device_code"`
	ServiceStatus    string         `json:"service_status"`
	PermissionStatus map[string]any `json:"permission_status"`
	Capabilities     map[string]any `json:"capabilities"`
	DeviceName       string         `json:"device_name"`
	Manufacturer     string         `json:"manufacturer"`
	Model            string         `json:"model"`
	AndroidVersion   string         `json:"android_version"`
	AndroidSDK       int            `json:"android_sdk"`
	AppVersion       string         `json:"app_version"`
	AppNativeCode    int            `json:"app_native_code"`
	PluginVersion    string         `json:"plugin_version"`
}

type Command struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	ExpiresAt time.Time       `json:"expires_at"`
}

type Ack struct {
	Status string         `json:"status"`
	Result map[string]any `json:"result,omitempty"`
}

type Event struct {
	Type       string         `json:"type"`
	SessionRef string         `json:"session_ref"`
	OccurredAt time.Time      `json:"occurred_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Credential struct {
	ID        string     `json:"id"`
	DeviceID  string     `json:"device_id"`
	Status    string     `json:"status"`
	ExpiresAt time.Time  `json:"expires_at"`
	ReadyAt   *time.Time `json:"ready_at,omitempty"`
}

type RevealedCredential struct {
	ID               string    `json:"id"`
	ControlToken     string    `json:"control_token"`
	ExpiresAt        time.Time `json:"expires_at"`
	HideAfterSeconds int       `json:"hide_after_seconds"`
}

func New(db *sql.DB, masterKey string, config Config, redisClients ...*redis.Client) (*Service, error) {
	cipher, err := newSecretCipher(masterKey)
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]struct{}, len(config.AllowedUserIDs))
	for _, userID := range config.AllowedUserIDs {
		if userID > 0 {
			allowed[userID] = struct{}{}
		}
	}
	var redisClient *redis.Client
	if len(redisClients) > 0 {
		redisClient = redisClients[0]
	}
	return &Service{
		db: db, cipher: cipher, config: config, now: time.Now,
		allowedUserIDs: allowed, realtime: newRealtimeStore(redisClient),
	}, nil
}

// Enabled reports that the management and device command channel is present.
// Enrollment and credential issuance have their own rollout gate so existing
// devices can still heartbeat and receive stop commands during an emergency.
func (s *Service) Enabled() bool { return s != nil }

func (s *Service) userEnabled(userID int64) bool {
	if s == nil {
		return false
	}
	if s.config.Enabled {
		return true
	}
	_, allowed := s.allowedUserIDs[userID]
	return allowed
}

func (s *Service) Enroll(ctx context.Context, userID int64, request EnrollRequest) (Enrollment, error) {
	if !s.userEnabled(userID) {
		return Enrollment{}, ErrDisabled
	}
	request.InstallID = strings.TrimSpace(request.InstallID)
	if userID < 1 || len(request.InstallID) < 16 || len(request.InstallID) > 190 || request.AndroidSDK < 29 || request.AndroidSDK > 100 {
		return Enrollment{}, ErrInvalid
	}
	deviceID, err := idgen.New()
	if err != nil {
		return Enrollment{}, err
	}
	token, tokenHash, err := opaqueSecret()
	if err != nil {
		return Enrollment{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Enrollment{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var existingID string
	var existingUser int64
	var existingStatus int
	err = tx.QueryRowContext(ctx, `SELECT id,user_id,status FROM remote_devices WHERE install_id=? FOR UPDATE`, request.InstallID).Scan(&existingID, &existingUser, &existingStatus)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `INSERT INTO remote_devices
			(id,user_id,install_id,device_token_hash,device_name,manufacturer,model,android_version,android_sdk,app_version,app_native_code,plugin_version,status)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,1)`, deviceID, userID, request.InstallID, tokenHash, bounded(request.DeviceName, 120), bounded(request.Manufacturer, 80), bounded(request.Model, 120), bounded(request.AndroidVersion, 32), request.AndroidSDK, bounded(request.AppVersion, 40), positive(request.AppNativeCode), bounded(request.PluginVersion, 40))
	case err != nil:
		return Enrollment{}, err
	case existingUser != userID && existingStatus != 3:
		return Enrollment{}, ErrConflict
	default:
		deviceID = existingID
		_, err = tx.ExecContext(ctx, `UPDATE remote_devices SET user_id=?,device_token_hash=?,device_name=?,manufacturer=?,model=?,android_version=?,android_sdk=?,app_version=?,app_native_code=?,plugin_version=?,status=1,revoked_at=NULL,service_status='stopped',device_code='',permission_status=NULL,capabilities=NULL,last_seen_at=NULL WHERE id=?`, userID, tokenHash, bounded(request.DeviceName, 120), bounded(request.Manufacturer, 80), bounded(request.Model, 120), bounded(request.AndroidVersion, 32), request.AndroidSDK, bounded(request.AppVersion, 40), positive(request.AppNativeCode), bounded(request.PluginVersion, 40), deviceID)
	}
	if err != nil {
		return Enrollment{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE remote_commands SET status='cancelled' WHERE remote_device_id=? AND status IN ('pending','delivered')`, deviceID); err != nil {
		return Enrollment{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE remote_credential_requests SET status='expired' WHERE remote_device_id=? AND status IN ('pending','ready','revealed')`, deviceID); err != nil {
		return Enrollment{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE remote_sessions SET status='ended' WHERE remote_device_id=? AND status='active'`, deviceID); err != nil {
		return Enrollment{}, err
	}
	if err = tx.Commit(); err != nil {
		return Enrollment{}, err
	}
	return Enrollment{DeviceID: deviceID, DeviceToken: token, HeartbeatSeconds: 2}, nil
}

func (s *Service) Current(ctx context.Context, userID int64, installID string) (Device, error) {
	if !s.Enabled() {
		return Device{}, ErrDisabled
	}
	installID = strings.TrimSpace(installID)
	if userID < 1 || installID == "" {
		return Device{}, ErrInvalid
	}
	return s.scanDevice(s.db.QueryRowContext(ctx, `SELECT id,user_id,'',install_id,device_name,manufacturer,model,android_version,android_sdk,app_version,app_native_code,plugin_version,device_code,service_status,permission_status,capabilities,status,last_seen_at,created_at,EXISTS(SELECT 1 FROM remote_sessions rs WHERE rs.remote_device_id=remote_devices.id AND rs.event_type='connected' AND rs.status='active') FROM remote_devices WHERE user_id=? AND install_id=? AND status<>3`, userID, installID))
}

func (s *Service) Unbind(ctx context.Context, userID int64, installID string) error {
	installID = strings.TrimSpace(installID)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(ctx, `UPDATE remote_devices SET status=3,service_status='stopped',device_token_hash=NULL,revoked_at=CURRENT_TIMESTAMP(3) WHERE user_id=? AND install_id=? AND status<>3`, userID, installID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `UPDATE remote_commands command_row JOIN remote_devices device ON device.id=command_row.remote_device_id SET command_row.status='cancelled' WHERE device.user_id=? AND device.install_id=? AND command_row.status IN ('pending','delivered')`, userID, installID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE remote_credential_requests credential JOIN remote_devices device ON device.id=credential.remote_device_id SET credential.status='expired' WHERE device.user_id=? AND device.install_id=? AND credential.status IN ('pending','ready','revealed')`, userID, installID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE remote_sessions session_row JOIN remote_devices device ON device.id=session_row.remote_device_id SET session_row.status='ended' WHERE device.user_id=? AND device.install_id=? AND session_row.status='active'`, userID, installID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) AuthenticateDevice(ctx context.Context, token string) (Device, error) {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	if strings.TrimSpace(token) == "" {
		return Device{}, ErrNotFound
	}
	return s.scanDevice(s.db.QueryRowContext(ctx, `SELECT id,user_id,'',install_id,device_name,manufacturer,model,android_version,android_sdk,app_version,app_native_code,plugin_version,device_code,service_status,permission_status,capabilities,status,last_seen_at,created_at,FALSE FROM remote_devices WHERE device_token_hash=? AND status IN (1,2)`, hex.EncodeToString(sum[:])))
}

func (s *Service) Heartbeat(ctx context.Context, device Device, report Heartbeat) ([]Command, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	permissions, err := safeBooleanState(report.PermissionStatus, map[string]struct{}{
		"notification": {}, "media_projection": {}, "accessibility": {}, "overlay": {},
		"all_files": {}, "system_audio": {}, "microphone": {}, "battery": {},
	})
	if err != nil {
		return nil, ErrInvalid
	}
	capabilities, err := safeBooleanState(report.Capabilities, map[string]struct{}{
		"screen": {}, "input": {}, "clipboard": {}, "file_transfer": {},
		"system_audio": {}, "chat": {}, "voice": {},
	})
	if err != nil {
		return nil, ErrInvalid
	}
	status := bounded(report.ServiceStatus, 32)
	if status == "" {
		status = "unknown"
	}
	deviceCode := bounded(report.DeviceCode, 80)
	if deviceCode != "" && !validDeviceCode(deviceCode) {
		return nil, ErrInvalid
	}
	_, err = s.db.ExecContext(ctx, `UPDATE remote_devices SET
		device_code=COALESCE(NULLIF(?,''),device_code),service_status=?,
		permission_status=COALESCE(?,permission_status),capabilities=COALESCE(?,capabilities),
		device_name=COALESCE(NULLIF(?,''),device_name),
		manufacturer=COALESCE(NULLIF(?,''),manufacturer),
		model=COALESCE(NULLIF(?,''),model),
		android_version=COALESCE(NULLIF(?,''),android_version),
		android_sdk=COALESCE(NULLIF(?,0),android_sdk),
		app_version=COALESCE(NULLIF(?,''),app_version),
		app_native_code=COALESCE(NULLIF(?,0),app_native_code),
		plugin_version=COALESCE(NULLIF(?,''),plugin_version),
			last_seen_at=? WHERE id=? AND status IN (1,2)`, deviceCode, status, nullableJSON(permissions), nullableJSON(capabilities), bounded(report.DeviceName, 120), bounded(report.Manufacturer, 80), bounded(report.Model, 120), bounded(report.AndroidVersion, 32), positive(report.AndroidSDK), bounded(report.AppVersion, 40), positive(report.AppNativeCode), bounded(report.PluginVersion, 40), s.now(), device.ID)
	if err != nil {
		return nil, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE remote_commands SET status='expired' WHERE remote_device_id=? AND status IN ('pending','delivered') AND expires_at<=?`, device.ID, s.now())
	_, _ = s.db.ExecContext(ctx, `UPDATE remote_credential_requests SET status='expired' WHERE remote_device_id=? AND status IN ('pending','ready') AND expires_at<=?`, device.ID, s.now())
	if device.Status == 2 {
		var pendingStop int
		if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM remote_commands WHERE remote_device_id=? AND command_type='stop' AND status IN ('pending','delivered') AND expires_at>?`, device.ID, s.now()).Scan(&pendingStop); err != nil {
			return nil, err
		}
		if pendingStop == 0 {
			stopID, idErr := idgen.New()
			if idErr != nil {
				return nil, idErr
			}
			if _, err = s.db.ExecContext(ctx, `INSERT INTO remote_commands(id,remote_device_id,command_type,status,expires_at) VALUES(?,?,'stop','pending',?)`, stopID, device.ID, s.now().Add(24*time.Hour)); err != nil {
				return nil, err
			}
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,command_type,payload_ciphertext,expires_at FROM remote_commands WHERE remote_device_id=? AND status IN ('pending','delivered') AND expires_at>? AND (?<>2 OR command_type='stop') ORDER BY created_at LIMIT 10`, device.ID, s.now(), device.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	commands := make([]Command, 0)
	for rows.Next() {
		var command Command
		var encrypted []byte
		if err = rows.Scan(&command.ID, &command.Type, &encrypted, &command.ExpiresAt); err != nil {
			return nil, err
		}
		// Some Android vendor builds reject RFC3339 timestamps carrying a
		// non-zero offset. UTC guarantees the portable trailing "Z" wire form.
		command.ExpiresAt = command.ExpiresAt.UTC()
		if len(encrypted) > 0 {
			plain, decryptErr := s.cipher.decrypt("command/"+command.ID, encrypted)
			if decryptErr != nil {
				return nil, decryptErr
			}
			command.Payload = json.RawMessage(plain)
		} else {
			command.Payload = json.RawMessage(`{}`)
		}
		commands = append(commands, command)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for _, command := range commands {
		_, _ = s.db.ExecContext(ctx, `UPDATE remote_commands SET status='delivered',delivered_at=COALESCE(delivered_at,?) WHERE id=? AND status='pending'`, s.now(), command.ID)
	}
	return commands, nil
}

func (s *Service) AckCommand(ctx context.Context, device Device, commandID string, ack Ack) error {
	commandID = strings.TrimSpace(commandID)
	if commandID == "" {
		return ErrInvalid
	}
	status := strings.ToLower(strings.TrimSpace(ack.Status))
	if status != "applied" && status != "failed" {
		return ErrInvalid
	}
	resultJSON, err := safeAckResult(ack.Result)
	if err != nil {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	var commandType, currentStatus string
	err = tx.QueryRowContext(ctx, `SELECT command_type,status FROM remote_commands WHERE id=? AND remote_device_id=? FOR UPDATE`, commandID, device.ID).Scan(&commandType, &currentStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if device.Status == 2 && commandType != "stop" {
		return ErrConflict
	}
	if currentStatus == "acknowledged" || currentStatus == "failed" {
		return nil
	}
	if currentStatus == "expired" || currentStatus == "cancelled" {
		return ErrConflict
	}
	next := "acknowledged"
	if status == "failed" {
		next = "failed"
	}
	_, err = tx.ExecContext(ctx, `UPDATE remote_commands SET status=?,acknowledged_at=?,result_data=? WHERE id=?`, next, s.now(), nullableJSON(resultJSON), commandID)
	if err != nil {
		return err
	}
	if commandType == "authorize_session" {
		credentialStatus := "ready"
		if status == "failed" {
			credentialStatus = "failed"
		}
		_, err = tx.ExecContext(ctx, `UPDATE remote_credential_requests SET status=?,ready_at=IF(?='ready',?,ready_at) WHERE command_id=? AND status='pending'`, credentialStatus, credentialStatus, s.now(), commandID)
		if err != nil {
			return err
		}
	}
	if commandType == "stop" && status == "applied" {
		_, err = tx.ExecContext(ctx, `UPDATE remote_devices SET status=3,service_status='stopped',device_token_hash=NULL,revoked_at=? WHERE id=?`, s.now(), device.ID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE remote_sessions SET status='ended' WHERE remote_device_id=? AND status='active'`, device.ID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) RecordEvents(ctx context.Context, device Device, events []Event) error {
	if device.Status != 1 {
		return ErrConflict
	}
	if len(events) == 0 || len(events) > 50 {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, event := range events {
		event.Type = strings.ToLower(strings.TrimSpace(event.Type))
		if !validEventType(event.Type) {
			return ErrInvalid
		}
		if event.OccurredAt.IsZero() {
			event.OccurredAt = s.now()
		}
		if event.OccurredAt.Before(s.now().Add(-24*time.Hour)) || event.OccurredAt.After(s.now().Add(5*time.Minute)) {
			return ErrInvalid
		}
		metadata, marshalErr := safeEventMetadata(event.Metadata)
		if marshalErr != nil {
			return ErrInvalid
		}
		eventID, idErr := idgen.New()
		if idErr != nil {
			return idErr
		}
		var credentialID sql.NullString
		if event.Type == "connected" {
			_ = tx.QueryRowContext(ctx, `SELECT id FROM remote_credential_requests WHERE remote_device_id=? AND status IN ('ready','revealed') AND expires_at>? ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, device.ID, s.now()).Scan(&credentialID)
			if credentialID.Valid {
				_, err = tx.ExecContext(ctx, `UPDATE remote_credential_requests SET status='consumed',consumed_at=? WHERE id=?`, s.now(), credentialID.String)
				if err != nil {
					return err
				}
			}
		}
		sessionStatus := "reported"
		if event.Type == "connected" {
			sessionStatus = "active"
		}
		if event.Type == "disconnected" {
			sessionStatus = "ended"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO remote_sessions(id,remote_device_id,credential_request_id,event_type,session_ref,status,metadata,occurred_at) VALUES(?,?,?,?,?,?,?,?)`, eventID, device.ID, nullableString(credentialID), event.Type, bounded(event.SessionRef, 100), sessionStatus, nullableJSON(metadata), event.OccurredAt)
		if err != nil {
			return err
		}
		if event.Type == "disconnected" && event.SessionRef != "" {
			_, _ = tx.ExecContext(ctx, `UPDATE remote_sessions SET status='ended' WHERE remote_device_id=? AND session_ref=? AND status='active'`, device.ID, bounded(event.SessionRef, 100))
		}
	}
	return tx.Commit()
}

func (s *Service) ListDevices(ctx context.Context, search string, online *bool, permission string, permissionGranted *bool, limit, offset int) ([]Device, int, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	search = strings.TrimSpace(search)
	like := "%" + search + "%"
	cutoff := s.now().Add(-onlineWindow)
	onlineFilter := -1
	if online != nil {
		if *online {
			onlineFilter = 1
		} else {
			onlineFilter = 0
		}
	}
	permission = strings.TrimSpace(permission)
	permissionValue := ""
	if permission != "" {
		if permissionGranted == nil || !validPermissionKey(permission) {
			return nil, 0, ErrInvalid
		}
		permissionValue = map[bool]string{true: "true", false: "false"}[*permissionGranted]
	}
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM remote_devices device JOIN users user ON user.id=device.user_id WHERE device.status<>3 AND (?='' OR user.username LIKE ? OR user.nickname LIKE ? OR device.device_name LIKE ? OR device.model LIKE ? OR device.device_code LIKE ?) AND (?=-1 OR (?=1 AND device.last_seen_at>?) OR (?=0 AND (device.last_seen_at IS NULL OR device.last_seen_at<=?))) AND (?='' OR COALESCE(JSON_UNQUOTE(JSON_EXTRACT(device.permission_status,CONCAT('$.',?))),'false')=?)`, search, like, like, like, like, like, onlineFilter, onlineFilter, cutoff, onlineFilter, cutoff, permission, permission, permissionValue).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT device.id,device.user_id,user.username,device.install_id,device.device_name,device.manufacturer,device.model,device.android_version,device.android_sdk,device.app_version,device.app_native_code,device.plugin_version,device.device_code,device.service_status,device.permission_status,device.capabilities,device.status,device.last_seen_at,device.created_at,EXISTS(SELECT 1 FROM remote_sessions rs WHERE rs.remote_device_id=device.id AND rs.status='active') FROM remote_devices device JOIN users user ON user.id=device.user_id WHERE device.status<>3 AND (?='' OR user.username LIKE ? OR user.nickname LIKE ? OR device.device_name LIKE ? OR device.model LIKE ? OR device.device_code LIKE ?) AND (?=-1 OR (?=1 AND device.last_seen_at>?) OR (?=0 AND (device.last_seen_at IS NULL OR device.last_seen_at<=?))) AND (?='' OR COALESCE(JSON_UNQUOTE(JSON_EXTRACT(device.permission_status,CONCAT('$.',?))),'false')=?) ORDER BY device.last_seen_at DESC,device.created_at DESC LIMIT ? OFFSET ?`, search, like, like, like, like, like, onlineFilter, onlineFilter, cutoff, onlineFilter, cutoff, permission, permission, permissionValue, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Device, 0)
	for rows.Next() {
		item, scanErr := s.scanDevice(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Service) CreateCredential(ctx context.Context, deviceID string, adminID int64) (Credential, error) {
	if !s.Enabled() {
		return Credential{}, ErrDisabled
	}
	device, err := s.deviceByID(ctx, deviceID)
	if err != nil {
		return Credential{}, err
	}
	if !s.userEnabled(device.UserID) {
		return Credential{}, ErrDisabled
	}
	if device.Status != 1 || !device.Online || strings.TrimSpace(device.DeviceCode) == "" || device.ServiceStatus != "running" {
		return Credential{}, ErrOffline
	}
	password, err := randomPassword(20)
	if err != nil {
		return Credential{}, err
	}
	credentialID, err := idgen.New()
	if err != nil {
		return Credential{}, err
	}
	commandID, err := idgen.New()
	if err != nil {
		return Credential{}, err
	}
	expires := s.now().Add(credentialLifetime)
	passwordCipher, err := s.cipher.encrypt("credential/"+credentialID, []byte(password))
	if err != nil {
		return Credential{}, err
	}
	payload, _ := json.Marshal(map[string]any{"credential_request_id": credentialID, "expires_at": expires})
	commandCipher, err := s.cipher.encrypt("command/"+commandID, payload)
	if err != nil {
		return Credential{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Credential{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var lockedStatus int
	var lockedLastSeen *time.Time
	var lockedServiceStatus, lockedDeviceCode string
	err = tx.QueryRowContext(ctx, `SELECT status,last_seen_at,service_status,device_code FROM remote_devices WHERE id=? FOR UPDATE`, device.ID).Scan(&lockedStatus, &lockedLastSeen, &lockedServiceStatus, &lockedDeviceCode)
	if errors.Is(err, sql.ErrNoRows) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, err
	}
	if lockedStatus != 1 || lockedLastSeen == nil || !lockedLastSeen.After(s.now().Add(-onlineWindow)) || lockedServiceStatus != "running" || strings.TrimSpace(lockedDeviceCode) == "" {
		return Credential{}, ErrOffline
	}
	_, _ = tx.ExecContext(ctx, `UPDATE remote_credential_requests SET status='expired' WHERE remote_device_id=? AND status IN ('pending','ready')`, device.ID)
	_, _ = tx.ExecContext(ctx, `UPDATE remote_commands command_row
		JOIN remote_credential_requests credential ON credential.command_id=command_row.id
		SET command_row.status='cancelled'
		WHERE credential.remote_device_id=? AND credential.id<>? AND command_row.status IN ('pending','delivered')`, device.ID, credentialID)
	_, err = tx.ExecContext(ctx, `INSERT INTO remote_commands(id,remote_device_id,command_type,payload_ciphertext,status,expires_at) VALUES(?,?, 'authorize_session',?,'pending',?)`, commandID, device.ID, commandCipher, expires)
	if err != nil {
		return Credential{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO remote_credential_requests(id,remote_device_id,requested_by,command_id,authorization_ciphertext,status,expires_at) VALUES(?,?,?,?,?,'pending',?)`, credentialID, device.ID, adminID, commandID, passwordCipher, expires)
	if err != nil {
		return Credential{}, err
	}
	if err = tx.Commit(); err != nil {
		return Credential{}, err
	}
	return Credential{ID: credentialID, DeviceID: device.ID, Status: "pending", ExpiresAt: expires}, nil
}

func (s *Service) CredentialStatus(ctx context.Context, id string, adminID int64) (Credential, error) {
	_, _ = s.db.ExecContext(ctx, `UPDATE remote_credential_requests SET status='expired' WHERE id=? AND status IN ('pending','ready') AND expires_at<=?`, id, s.now())
	var item Credential
	err := s.db.QueryRowContext(ctx, `SELECT id,remote_device_id,status,expires_at,ready_at FROM remote_credential_requests WHERE id=? AND requested_by=?`, id, adminID).Scan(&item.ID, &item.DeviceID, &item.Status, &item.ExpiresAt, &item.ReadyAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Credential{}, ErrNotFound
	}
	return item, err
}

func (s *Service) RevealCredential(ctx context.Context, id string, adminID int64) (RevealedCredential, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return RevealedCredential{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	var encrypted []byte
	var status, deviceID string
	var expires time.Time
	err = tx.QueryRowContext(ctx, `SELECT credential.authorization_ciphertext,credential.status,credential.expires_at,device.id FROM remote_credential_requests credential JOIN remote_devices device ON device.id=credential.remote_device_id WHERE credential.id=? AND credential.requested_by=? FOR UPDATE`, id, adminID).Scan(&encrypted, &status, &expires, &deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return RevealedCredential{}, ErrNotFound
	}
	if err != nil {
		return RevealedCredential{}, err
	}
	if !expires.After(s.now()) {
		_, _ = tx.ExecContext(ctx, `UPDATE remote_credential_requests SET status='expired' WHERE id=?`, id)
		_ = tx.Commit()
		return RevealedCredential{}, ErrNotReady
	}
	if status == "revealed" || status == "consumed" {
		return RevealedCredential{}, ErrAlreadyRevealed
	}
	if status != "ready" {
		return RevealedCredential{}, ErrNotReady
	}
	_, err = s.cipher.decrypt("credential/"+id, encrypted)
	if err != nil {
		return RevealedCredential{}, err
	}
	controlToken, controlExpires, err := s.startControlSession(ctx, deviceID, adminID, id)
	if err != nil {
		return RevealedCredential{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE remote_credential_requests SET status='revealed',revealed_at=? WHERE id=? AND status='ready'`, s.now(), id)
	if err != nil {
		_ = s.endControlSession(ctx, deviceID, adminID, controlToken)
		return RevealedCredential{}, err
	}
	if err = tx.Commit(); err != nil {
		_ = s.endControlSession(ctx, deviceID, adminID, controlToken)
		return RevealedCredential{}, err
	}
	return RevealedCredential{ID: id, ControlToken: controlToken, ExpiresAt: controlExpires, HideAfterSeconds: 60}, nil
}

func (s *Service) RevokeDevice(ctx context.Context, deviceID string) error {
	commandID, err := idgen.New()
	if err != nil {
		return err
	}
	expires := s.now().Add(24 * time.Hour)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	result, err := tx.ExecContext(ctx, `UPDATE remote_devices SET status=2 WHERE id=? AND status=1`, deviceID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `UPDATE remote_commands SET status='cancelled' WHERE remote_device_id=? AND status IN ('pending','delivered')`, deviceID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO remote_commands(id,remote_device_id,command_type,status,expires_at) VALUES(?,?,'stop','pending',?)`, commandID, deviceID, expires)
	if err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `UPDATE remote_credential_requests SET status='expired' WHERE remote_device_id=? AND status IN ('pending','ready','revealed')`, deviceID)
	return tx.Commit()
}

func (s *Service) deviceByID(ctx context.Context, id string) (Device, error) {
	return s.scanDevice(s.db.QueryRowContext(ctx, `SELECT id,user_id,'',install_id,device_name,manufacturer,model,android_version,android_sdk,app_version,app_native_code,plugin_version,device_code,service_status,permission_status,capabilities,status,last_seen_at,created_at,EXISTS(SELECT 1 FROM remote_sessions rs WHERE rs.remote_device_id=remote_devices.id AND rs.status='active') FROM remote_devices WHERE id=?`, id))
}

type scanner interface{ Scan(...any) error }

func (s *Service) scanDevice(row scanner) (Device, error) {
	var item Device
	var permissions, capabilities []byte
	err := row.Scan(&item.ID, &item.UserID, &item.Username, &item.InstallID, &item.DeviceName, &item.Manufacturer, &item.Model, &item.AndroidVersion, &item.AndroidSDK, &item.AppVersion, &item.AppNativeCode, &item.PluginVersion, &item.DeviceCode, &item.ServiceStatus, &permissions, &capabilities, &item.Status, &item.LastSeenAt, &item.CreatedAt, &item.CurrentSession)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	if err != nil {
		return Device{}, err
	}
	item.PermissionStatus = json.RawMessage(permissions)
	item.Capabilities = json.RawMessage(capabilities)
	item.Online = item.LastSeenAt != nil && item.LastSeenAt.After(s.now().Add(-onlineWindow))
	return item, nil
}

func opaqueSecret() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

const passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

func randomPassword(length int) (string, error) {
	if length < 20 {
		return "", ErrInvalid
	}
	result := make([]byte, length)
	limit := byte(256 - (256 % len(passwordAlphabet)))
	raw := make([]byte, length*2)
	index := 0
	for index < length {
		if _, err := io.ReadFull(rand.Reader, raw); err != nil {
			return "", err
		}
		for _, value := range raw {
			if value >= limit {
				continue
			}
			result[index] = passwordAlphabet[int(value)%len(passwordAlphabet)]
			index++
			if index == length {
				break
			}
		}
	}
	return string(result), nil
}
func bounded(value string, max int) string {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return ""
	}
	runes := []rune(value)
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes)
}
func positive(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
func safeJSON(value any, max int) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	body, err := json.Marshal(value)
	if err != nil || len(body) > max {
		return nil, ErrInvalid
	}
	return body, nil
}
func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
func nullableString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}
func validEventType(value string) bool {
	switch value {
	case "request", "authorized", "denied", "connected", "disconnected", "file_transfer_started", "file_transfer_finished", "chat_started", "voice_started", "voice_ended":
		return true
	default:
		return false
	}
}
func safeEventMetadata(value map[string]any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	allowed := map[string]map[string]struct{}{
		"direction":  {"incoming": {}, "outgoing": {}, "upload": {}, "download": {}},
		"transport":  {"p2p": {}, "relay": {}},
		"result":     {"success": {}, "failed": {}, "cancelled": {}, "denied": {}},
		"capability": {"screen": {}, "input": {}, "clipboard": {}, "file_transfer": {}, "system_audio": {}, "chat": {}, "voice": {}},
	}
	for key, raw := range value {
		values, exists := allowed[key]
		text, isString := raw.(string)
		if !exists || !isString {
			return nil, ErrInvalid
		}
		if _, exists = values[strings.ToLower(strings.TrimSpace(text))]; !exists {
			return nil, ErrInvalid
		}
	}
	return safeJSON(value, 4<<10)
}

func safeBooleanState(value map[string]any, allowed map[string]struct{}) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	for key, item := range value {
		if _, exists := allowed[key]; !exists {
			return nil, ErrInvalid
		}
		if _, isBoolean := item.(bool); !isBoolean {
			return nil, ErrInvalid
		}
	}
	return safeJSON(value, 16<<10)
}

func safeAckResult(value map[string]any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	if len(value) != 1 {
		return nil, ErrInvalid
	}
	code, isString := value["code"].(string)
	if !isString {
		return nil, ErrInvalid
	}
	switch code {
	case "", "unsupported_command", "apply_failed", "invalid_payload", "expired", "core_unavailable", "input_unavailable", "gesture_failed":
		return safeJSON(value, 256)
	default:
		return nil, ErrInvalid
	}
}

func validDeviceCode(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func validPermissionKey(value string) bool {
	switch value {
	case "notification", "media_projection", "system_audio", "accessibility", "overlay", "all_files", "microphone", "battery":
		return true
	default:
		return false
	}
}
