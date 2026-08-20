package remoteassist

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

const (
	maxFrameBytes          = 768 << 10
	frameLifetime          = 8 * time.Second
	controlSessionLifetime = 30 * time.Minute
)

type ScreenFrame struct {
	JPEG       []byte
	Width      int
	Height     int
	Rotation   int
	Sequence   int64
	CapturedAt time.Time
}

type controlSession struct {
	DeviceID     string    `json:"device_id"`
	AdminID      int64     `json:"admin_id"`
	CredentialID string    `json:"credential_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type realtimeStore struct {
	redis    *redis.Client
	mu       sync.RWMutex
	frames   map[string]ScreenFrame
	sessions map[string]controlSession
}

func newRealtimeStore(client *redis.Client) *realtimeStore {
	return &realtimeStore{
		redis: client, frames: make(map[string]ScreenFrame), sessions: make(map[string]controlSession),
	}
}

func (s *Service) StoreFrame(ctx context.Context, device Device, frame ScreenFrame) error {
	if device.Status != 1 || len(frame.JPEG) < 4 || len(frame.JPEG) > maxFrameBytes ||
		frame.JPEG[0] != 0xff || frame.JPEG[1] != 0xd8 || frame.JPEG[len(frame.JPEG)-2] != 0xff || frame.JPEG[len(frame.JPEG)-1] != 0xd9 ||
		frame.Width < 1 || frame.Width > 4096 || frame.Height < 1 || frame.Height > 4096 ||
		(frame.Rotation != 0 && frame.Rotation != 90 && frame.Rotation != 180 && frame.Rotation != 270) || frame.Sequence < 0 {
		return ErrInvalid
	}
	frame.JPEG = append([]byte(nil), frame.JPEG...)
	frame.CapturedAt = s.now()
	return s.realtime.storeFrame(ctx, device.ID, frame)
}

func (s *Service) Frame(ctx context.Context, deviceID string, adminID int64, token string) (ScreenFrame, error) {
	if err := s.verifyControlSession(ctx, deviceID, adminID, token); err != nil {
		return ScreenFrame{}, err
	}
	return s.realtime.frame(ctx, strings.TrimSpace(deviceID), s.now())
}

func (s *Service) QueueControlCommand(ctx context.Context, deviceID string, adminID int64, token, commandType string, payload map[string]any) error {
	deviceID = strings.TrimSpace(deviceID)
	if err := s.verifyControlSession(ctx, deviceID, adminID, token); err != nil {
		return err
	}
	device, err := s.deviceByID(ctx, deviceID)
	if err != nil {
		return err
	}
	if device.Status != 1 || !device.Online || device.ServiceStatus != "running" {
		return ErrOffline
	}
	commandType = strings.ToLower(strings.TrimSpace(commandType))
	validated, err := validateControlPayload(commandType, payload)
	if err != nil {
		return err
	}
	commandID, err := idgen.New()
	if err != nil {
		return err
	}
	body, err := json.Marshal(validated)
	if err != nil {
		return ErrInvalid
	}
	ciphertext, err := s.cipher.encrypt("command/"+commandID, body)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO remote_commands
		(id,remote_device_id,command_type,payload_ciphertext,status,expires_at)
		VALUES(?,?,?,?,'pending',?)`, commandID, deviceID, commandType, ciphertext, s.now().Add(15*time.Second))
	if err != nil {
		return err
	}
	if commandType == "end_session" {
		_ = s.endControlSession(ctx, deviceID, adminID, token)
	}
	return nil
}

func (s *Service) EndControlSession(ctx context.Context, deviceID string, adminID int64, token string) error {
	if err := s.verifyControlSession(ctx, deviceID, adminID, token); err != nil {
		return err
	}
	return s.endControlSession(ctx, deviceID, adminID, token)
}

func (s *Service) startControlSession(ctx context.Context, deviceID string, adminID int64, credentialID string) (string, time.Time, error) {
	token, hash, err := opaqueSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := s.now().Add(controlSessionLifetime)
	session := controlSession{DeviceID: deviceID, AdminID: adminID, CredentialID: credentialID, ExpiresAt: expires}
	if err = s.realtime.storeSession(ctx, hash, session); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *Service) verifyControlSession(ctx context.Context, deviceID string, adminID int64, token string) error {
	token = strings.TrimSpace(token)
	deviceID = strings.TrimSpace(deviceID)
	if token == "" || deviceID == "" || adminID < 1 {
		return ErrNotReady
	}
	sum := sha256.Sum256([]byte(token))
	session, err := s.realtime.session(ctx, hex.EncodeToString(sum[:]), s.now())
	if err != nil {
		return err
	}
	if session.DeviceID != deviceID || session.AdminID != adminID {
		return ErrNotReady
	}
	return nil
}

func (s *Service) endControlSession(ctx context.Context, deviceID string, adminID int64, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	session, err := s.realtime.session(ctx, hash, s.now())
	if err != nil && !errors.Is(err, ErrNotReady) {
		return err
	}
	if err == nil && (session.DeviceID != strings.TrimSpace(deviceID) || session.AdminID != adminID) {
		return ErrNotReady
	}
	return s.realtime.deleteSession(ctx, hash)
}

func (r *realtimeStore) storeFrame(ctx context.Context, deviceID string, frame ScreenFrame) error {
	if r.redis != nil {
		key := "remote:frame:" + deviceID
		pipe := r.redis.TxPipeline()
		pipe.HSet(ctx, key, map[string]any{
			"jpeg": frame.JPEG, "width": frame.Width, "height": frame.Height,
			"rotation": frame.Rotation, "sequence": frame.Sequence,
			"captured_at_ms": frame.CapturedAt.UnixMilli(),
		})
		pipe.Expire(ctx, key, frameLifetime)
		_, err := pipe.Exec(ctx)
		return err
	}
	r.mu.Lock()
	r.frames[deviceID] = frame
	r.mu.Unlock()
	return nil
}

func (r *realtimeStore) frame(ctx context.Context, deviceID string, now time.Time) (ScreenFrame, error) {
	if r.redis != nil {
		values, err := r.redis.HGetAll(ctx, "remote:frame:"+deviceID).Result()
		if err != nil {
			return ScreenFrame{}, err
		}
		if len(values) == 0 {
			return ScreenFrame{}, ErrNoFrame
		}
		frame := ScreenFrame{
			JPEG: []byte(values["jpeg"]), Width: atoi(values["width"]), Height: atoi(values["height"]),
			Rotation: atoi(values["rotation"]), Sequence: atoi64(values["sequence"]),
			CapturedAt: time.UnixMilli(atoi64(values["captured_at_ms"])),
		}
		if len(frame.JPEG) == 0 || now.Sub(frame.CapturedAt) > frameLifetime {
			return ScreenFrame{}, ErrNoFrame
		}
		return frame, nil
	}
	r.mu.RLock()
	frame, ok := r.frames[deviceID]
	r.mu.RUnlock()
	if !ok || now.Sub(frame.CapturedAt) > frameLifetime {
		return ScreenFrame{}, ErrNoFrame
	}
	frame.JPEG = append([]byte(nil), frame.JPEG...)
	return frame, nil
}

func (r *realtimeStore) storeSession(ctx context.Context, hash string, session controlSession) error {
	if r.redis != nil {
		body, err := json.Marshal(session)
		if err != nil {
			return err
		}
		return r.redis.Set(ctx, "remote:session:"+hash, body, time.Until(session.ExpiresAt)).Err()
	}
	r.mu.Lock()
	r.sessions[hash] = session
	r.mu.Unlock()
	return nil
}

func (r *realtimeStore) session(ctx context.Context, hash string, now time.Time) (controlSession, error) {
	var session controlSession
	if r.redis != nil {
		body, err := r.redis.Get(ctx, "remote:session:"+hash).Bytes()
		if errors.Is(err, redis.Nil) {
			return controlSession{}, ErrNotReady
		}
		if err != nil {
			return controlSession{}, err
		}
		if err = json.Unmarshal(body, &session); err != nil {
			return controlSession{}, err
		}
	} else {
		r.mu.RLock()
		var ok bool
		session, ok = r.sessions[hash]
		r.mu.RUnlock()
		if !ok {
			return controlSession{}, ErrNotReady
		}
	}
	if !session.ExpiresAt.After(now) {
		_ = r.deleteSession(ctx, hash)
		return controlSession{}, ErrNotReady
	}
	return session, nil
}

func (r *realtimeStore) deleteSession(ctx context.Context, hash string) error {
	if r.redis != nil {
		return r.redis.Del(ctx, "remote:session:"+hash).Err()
	}
	r.mu.Lock()
	delete(r.sessions, hash)
	r.mu.Unlock()
	return nil
}

func validateControlPayload(commandType string, payload map[string]any) (map[string]any, error) {
	switch commandType {
	case "tap":
		x, xOK := normalizedNumber(payload["x"])
		y, yOK := normalizedNumber(payload["y"])
		if !xOK || !yOK || len(payload) != 2 {
			return nil, ErrInvalid
		}
		return map[string]any{"x": x, "y": y}, nil
	case "swipe":
		x1, x1OK := normalizedNumber(payload["x1"])
		y1, y1OK := normalizedNumber(payload["y1"])
		x2, x2OK := normalizedNumber(payload["x2"])
		y2, y2OK := normalizedNumber(payload["y2"])
		duration, durationOK := integerNumber(payload["duration_ms"], 50, 2000)
		if !x1OK || !y1OK || !x2OK || !y2OK || !durationOK || len(payload) != 5 {
			return nil, ErrInvalid
		}
		return map[string]any{"x1": x1, "y1": y1, "x2": x2, "y2": y2, "duration_ms": duration}, nil
	case "system_action":
		action, ok := payload["action"].(string)
		action = strings.ToLower(strings.TrimSpace(action))
		if !ok || len(payload) != 1 || (action != "back" && action != "home" && action != "recents") {
			return nil, ErrInvalid
		}
		return map[string]any{"action": action}, nil
	case "text", "clipboard_set":
		text, ok := payload["text"].(string)
		limit := 2048
		if commandType == "clipboard_set" {
			limit = 4096
		}
		if !ok || len(payload) != 1 || !utf8.ValidString(text) || len([]rune(text)) > limit {
			return nil, ErrInvalid
		}
		return map[string]any{"text": text}, nil
	case "end_session":
		if len(payload) != 0 {
			return nil, ErrInvalid
		}
		return map[string]any{}, nil
	default:
		return nil, ErrInvalid
	}
}

func normalizedNumber(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok && number >= 0 && number <= 1
}

func integerNumber(value any, minimum, maximum int) (int, bool) {
	number, ok := value.(float64)
	integer := int(number)
	return integer, ok && number == float64(integer) && integer >= minimum && integer <= maximum
}

func atoi(value string) int {
	result, _ := strconv.Atoi(value)
	return result
}

func atoi64(value string) int64 {
	result, _ := strconv.ParseInt(value, 10, 64)
	return result
}
