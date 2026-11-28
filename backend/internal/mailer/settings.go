package mailer

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const SettingKey = "mail.smtp"

var ErrSettingsConflict = errors.New("email settings version conflict")
var ErrInvalidSettings = errors.New("invalid email settings")

type Settings struct {
	Enabled bool `json:"enabled"`
	Config
}

type Snapshot struct {
	Settings
	Version int64
	Source  string
}

// Public never exposes the password or encrypted payload to an HTTP response or audit.
func (s Snapshot) Public() map[string]any {
	return map[string]any{"enabled": s.Enabled, "host": s.Host, "port": s.Port,
		"username": s.Username, "from": s.From, "tls_mode": s.TLSMode,
		"password_configured": s.Password != "", "version": s.Version, "source": s.Source}
}

type Store struct {
	db       *sql.DB
	aead     cipher.AEAD
	fallback Snapshot
}

func NewStore(db *sql.DB, masterKey string, fallback Config) (*Store, error) {
	if len(strings.TrimSpace(masterKey)) < 16 {
		return nil, errors.New("email configuration key is too short")
	}
	sender, err := New(fallback)
	if err != nil {
		return nil, err
	}
	settings, err := normalizeSettings(Settings{Enabled: sender != nil, Config: fallback})
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256([]byte("claw:smtp:v1\x00" + masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Store{db: db, aead: aead, fallback: Snapshot{Settings: settings, Source: "environment"}}, nil
}

type settingsQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) load(ctx context.Context, q settingsQuery, lock bool) (Snapshot, error) {
	query := `SELECT setting_value,is_secret,version FROM system_settings WHERE setting_key=?`
	if lock {
		query += " FOR UPDATE"
	}
	var raw []byte
	var secret int
	var version int64
	err := q.QueryRowContext(ctx, query, SettingKey).Scan(&raw, &secret, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return s.fallback, nil
	}
	if err != nil {
		return Snapshot{}, errors.New("read email configuration failed")
	}
	if secret != 1 {
		return Snapshot{}, errors.New("email configuration must be encrypted")
	}
	settings, err := s.decode(raw)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Settings: settings, Version: version, Source: "admin"}, nil
}

func (s *Store) Read(ctx context.Context) (Snapshot, error) { return s.load(ctx, s.db, false) }

// Sender returns an immutable per-request snapshot. No caching or restart is
// needed when an administrator changes settings; disabled overrides env values.
func (s *Store) Sender(ctx context.Context) (*Sender, error) {
	settings, err := s.Read(ctx)
	if err != nil || !settings.Enabled {
		return nil, err
	}
	return New(settings.Config)
}

// Save participates in the caller's transaction so the setting and audit commit together.
// An empty password retains the existing secret, including an environment fallback.
func (s *Store) Save(ctx context.Context, tx *sql.Tx, settings Settings, version, adminID int64) (Snapshot, Snapshot, error) {
	previous, err := s.load(ctx, tx, true)
	if err != nil {
		return Snapshot{}, Snapshot{}, err
	}
	if version != previous.Version {
		return Snapshot{}, Snapshot{}, ErrSettingsConflict
	}
	if settings.Password == "" {
		settings.Password = previous.Password
	}
	settings, err = normalizeSettings(settings)
	if err != nil {
		return Snapshot{}, Snapshot{}, fmt.Errorf("%w: %s", ErrInvalidSettings, err)
	}
	raw, err := s.encode(settings)
	if err != nil {
		return Snapshot{}, Snapshot{}, err
	}
	if previous.Version == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO system_settings(setting_key,setting_value,is_secret,version,updated_by) VALUES(?,?,1,1,?)`, SettingKey, raw, adminID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE system_settings SET setting_value=?,is_secret=1,version=version+1,updated_by=? WHERE setting_key=? AND version=?`, raw, adminID, SettingKey, version)
	}
	if err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			return Snapshot{}, Snapshot{}, ErrSettingsConflict
		}
		return Snapshot{}, Snapshot{}, errors.New("save email configuration failed")
	}
	return previous, Snapshot{Settings: settings, Version: version + 1, Source: "admin"}, nil
}

func normalizeSettings(s Settings) (Settings, error) {
	s.Host = strings.TrimSpace(s.Host)
	s.Port = strings.TrimSpace(s.Port)
	s.Username = strings.TrimSpace(s.Username)
	s.From = strings.TrimSpace(s.From)
	s.TLSMode = strings.ToLower(strings.TrimSpace(s.TLSMode))
	if s.Port == "" {
		s.Port = "587"
	}
	if s.TLSMode == "" {
		s.TLSMode = "starttls"
	}
	port, err := strconv.Atoi(s.Port)
	if err != nil || port < 1 || port > 65535 {
		return Settings{}, errors.New("SMTP 端口必须为 1–65535")
	}
	if len(s.Host) > 253 || strings.ContainsAny(s.Host, "/:\\@ \t\r\n") {
		return Settings{}, errors.New("请输入 SMTP 服务器域名，不要填写网址或端口")
	}
	if len(s.Username) > 254 || len(s.Password) > 1024 || strings.ContainsAny(s.Username+s.Password, "\r\n") {
		return Settings{}, errors.New("SMTP 账号或密码格式无效")
	}
	if s.TLSMode != "tls" && s.TLSMode != "starttls" {
		return Settings{}, errors.New("请选择 TLS 或 STARTTLS 加密")
	}
	if s.From == "" && ValidAddress(s.Username) {
		s.From = s.Username
	}
	if s.From != "" && !ValidAddress(s.From) {
		return Settings{}, errors.New("发件邮箱格式无效")
	}
	if s.Enabled {
		if s.Host == "" || s.Username == "" || s.Password == "" || s.From == "" {
			return Settings{}, errors.New("启用发送需要服务器、账号、密码和发件邮箱")
		}
		if _, err = New(s.Config); err != nil {
			return Settings{}, err
		}
	}
	return s, nil
}

func (s *Store) encode(settings Settings) ([]byte, error) {
	plain, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := s.aead.Seal(nonce, nonce, plain, []byte(SettingKey))
	return json.Marshal(map[string]string{"ciphertext": base64.StdEncoding.EncodeToString(sealed)})
}

func (s *Store) decode(raw []byte) (Settings, error) {
	var envelope struct {
		Ciphertext string `json:"ciphertext"`
	}
	if len(raw) > 16384 || json.Unmarshal(raw, &envelope) != nil {
		return Settings{}, errors.New("invalid encrypted email configuration")
	}
	data, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	n := s.aead.NonceSize()
	if err != nil || len(data) < n+s.aead.Overhead() {
		return Settings{}, errors.New("invalid encrypted email configuration")
	}
	plain, err := s.aead.Open(nil, data[:n], data[n:], []byte(SettingKey))
	if err != nil {
		return Settings{}, errors.New("email configuration decryption failed")
	}
	var settings Settings
	if json.Unmarshal(plain, &settings) != nil {
		return Settings{}, errors.New("invalid email configuration payload")
	}
	return normalizeSettings(settings)
}
