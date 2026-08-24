package auth

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

var (
	ErrInvalidSession     = errors.New("invalid session")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type User struct {
	ID       int64
	Nickname string
}

type Session struct {
	User      User
	Token     string
	ExpiresAt time.Time
}

type Options struct {
	LegacyAuthCode    string
	LegacyTablePrefix string
}

type Service struct {
	db                *sql.DB
	legacyAuthCode    string
	legacyTablePrefix string
	now               func() time.Time
}

func New(db *sql.DB, options ...Options) *Service {
	service := &Service{db: db, legacyTablePrefix: "cmf_", now: time.Now}
	if len(options) > 0 {
		service.legacyAuthCode = options[0].LegacyAuthCode
		if options[0].LegacyTablePrefix != "" {
			service.legacyTablePrefix = options[0].LegacyTablePrefix
		}
	}
	return service
}

func (s *Service) Authenticate(ctx context.Context, userID int64, token string) (User, error) {
	token = strings.TrimSpace(token)
	if userID < 1 || token == "" {
		return User{}, ErrInvalidSession
	}
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT user.id,COALESCE(NULLIF(user.nickname,''),user.username)
		FROM user_sessions session
		JOIN users user ON user.id=session.user_id
		WHERE session.user_id=? AND session.token_hash=? AND session.revoked_at IS NULL
		  AND session.expires_at>CURRENT_TIMESTAMP(3) AND user.status=1
		LIMIT 1`,
		userID, tokenHash,
	).Scan(&user.ID, &user.Nickname)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidSession
	}
	return user, err
}

// VerifyCredentials authenticates a normal user without creating an App
// session. Independent web portals use it before issuing their own
// path-scoped session.
func (s *Service) VerifyCredentials(ctx context.Context, countryCode, login, password string) (User, error) {
	countryCode = strings.TrimSpace(countryCode)
	if countryCode == "" {
		countryCode = "86"
	}
	login = strings.TrimSpace(login)
	if login == "" || password == "" {
		return User{}, ErrInvalidCredentials
	}
	var user User
	var passwordHash, passwordAlgo string
	var status uint8
	err := s.db.QueryRowContext(ctx, `
		SELECT id,COALESCE(NULLIF(nickname,''),username),password_hash,password_algo,status
		FROM users
		WHERE (country_code=? AND (username=? OR mobile=?)) OR email=?
		ORDER BY id LIMIT 1`,
		countryCode, login, login, login,
	).Scan(&user.ID, &user.Nickname, &passwordHash, &passwordAlgo, &status)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = adminauth.HashPassword("user-not-found-password")
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	if status != 1 || !s.verifyPassword(passwordAlgo, passwordHash, password) {
		return User{}, ErrInvalidCredentials
	}
	if passwordAlgo != "argon2id" {
		upgraded, hashErr := adminauth.HashMigratedPassword(password)
		if hashErr != nil {
			return User{}, hashErr
		}
		if _, hashErr = s.db.ExecContext(ctx, `
			UPDATE users SET password_hash=?,password_algo='argon2id'
			WHERE id=? AND password_hash=?`,
			upgraded, user.ID, passwordHash,
		); hashErr != nil {
			return User{}, fmt.Errorf("upgrade user password: %w", hashErr)
		}
	}
	return user, nil
}

func (s *Service) Login(
	ctx context.Context,
	countryCode, login, password, deviceID, platform, ip, userAgent string,
) (Session, error) {
	user, err := s.VerifyCredentials(ctx, countryCode, login, password)
	if err != nil {
		return Session{}, err
	}

	sessionID, err := idgen.New()
	if err != nil {
		return Session{}, err
	}
	tokenBytes := make([]byte, 32)
	if _, err = rand.Read(tokenBytes); err != nil {
		return Session{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	tokenSum := sha256.Sum256([]byte(token))
	expiresAt := s.now().Add(30 * 24 * time.Hour)
	deviceID = bounded(deviceID, 190)
	platform = bounded(platform, 20)
	ip = bounded(ip, 45)
	userAgent = bounded(userAgent, 500)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO user_sessions
			(id,user_id,token_hash,device_id,platform,ip,user_agent,expires_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		sessionID, user.ID, hex.EncodeToString(tokenSum[:]), deviceID, platform, ip, userAgent, expiresAt,
	); err != nil {
		return Session{}, fmt.Errorf("create user session: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE users SET last_login_at=? WHERE id=?`,
		s.now(), user.ID,
	); err != nil {
		return Session{}, fmt.Errorf("record user login: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return Session{}, err
	}
	return Session{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Service) Revoke(ctx context.Context, userID int64, token string) error {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_sessions SET revoked_at=?
		WHERE user_id=? AND token_hash=? AND revoked_at IS NULL`,
		s.now(), userID, hex.EncodeToString(sum[:]),
	)
	return err
}

func (s *Service) verifyPassword(algorithm, encoded, password string) bool {
	switch algorithm {
	case "argon2id":
		return adminauth.VerifyPassword(encoded, password)
	case "legacy_cmf":
		if s.legacyAuthCode == "" {
			return false
		}
		first := md5.Sum([]byte(s.legacyAuthCode + password))   //nolint:gosec
		second := md5.Sum([]byte(hex.EncodeToString(first[:]))) //nolint:gosec
		return encoded == "###"+hex.EncodeToString(second[:])
	case "legacy_cmf_old":
		prefixHash := md5.Sum([]byte(s.legacyTablePrefix)) //nolint:gosec
		passwordHash := md5.Sum([]byte(password))          //nolint:gosec
		decor := hex.EncodeToString(prefixHash[:])
		return encoded == decor[:12]+hex.EncodeToString(passwordHash[:])+decor[len(decor)-4:]
	default:
		return false
	}
}

func Bearer(r *http.Request) (int64, string) {
	userID, _ := strconv.ParseInt(strings.TrimSpace(r.Header.Get("X-User-ID")), 10, 64)
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return userID, ""
	}
	return userID, strings.TrimSpace(authorization[7:])
}

func bounded(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) > maximum {
		return value[:maximum]
	}
	return value
}
