package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/idgen"
)

var ErrInvalidCredentials = errors.New("invalid administrator credentials")

const (
	PortalAdmin   = "admin"
	PortalSupport = "support"
)

type Service struct {
	db  *sql.DB
	now func() time.Time
}

type Admin struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
}

type Session struct {
	Admin     Admin
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

func New(db *sql.DB) *Service {
	return &Service{db: db, now: time.Now}
}

func (s *Service) Login(ctx context.Context, username, password, ip, userAgent string) (Session, error) {
	return s.LoginForPortal(ctx, PortalAdmin, username, password, ip, userAgent)
}

func (s *Service) LoginForPortal(
	ctx context.Context,
	portal string,
	username string,
	password string,
	ip string,
	userAgent string,
) (Session, error) {
	if !validPortal(portal) {
		return Session{}, ErrInvalidCredentials
	}
	username = strings.TrimSpace(username)
	var admin Admin
	var passwordHash string
	var status uint8
	err := s.db.QueryRowContext(ctx, `
		SELECT id,username,COALESCE(NULLIF(display_name,''),username),password_hash,status
		FROM admin_users WHERE username=?`,
		username,
	).Scan(&admin.ID, &admin.Username, &admin.DisplayName, &passwordHash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		// Keep the missing-user path expensive enough to avoid a cheap username
		// enumeration oracle.
		argon2Dummy(password)
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if status != 1 || !VerifyPassword(passwordHash, password) {
		return Session{}, ErrInvalidCredentials
	}
	permissions, err := s.permissions(ctx, admin.ID)
	if err != nil {
		return Session{}, err
	}
	admin.Permissions = permissions

	sessionID, err := idgen.New()
	if err != nil {
		return Session{}, err
	}
	token, tokenHash, err := opaqueSecret(32)
	if err != nil {
		return Session{}, err
	}
	csrf, csrfHash, err := opaqueSecret(24)
	if err != nil {
		return Session{}, err
	}
	expiresAt := s.now().Add(12 * time.Hour)
	if len(ip) > 45 {
		ip = ip[:45]
	}
	if len(userAgent) > 500 {
		userAgent = userAgent[:500]
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO admin_sessions(id,admin_user_id,portal,token_hash,csrf_hash,ip,user_agent,expires_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		sessionID, admin.ID, portal, tokenHash, csrfHash, ip, userAgent, expiresAt,
	); err != nil {
		return Session{}, fmt.Errorf("create administrator session: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE admin_users SET last_login_at=?,last_login_ip=? WHERE id=?`,
		s.now(), ip, admin.ID,
	); err != nil {
		return Session{}, fmt.Errorf("record administrator login: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs
			(actor_type,actor_id,action,resource_type,resource_id,request_id,ip,user_agent,after_data)
		VALUES(1,?,?, 'admin_user',?,?,?, ?,JSON_OBJECT('username',?,'portal',?))`,
		admin.ID, portal+".login", admin.ID, sessionID, ip, userAgent, admin.Username, portal,
	); err != nil {
		return Session{}, fmt.Errorf("audit administrator login: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return Session{}, err
	}
	return Session{Admin: admin, Token: token, CSRFToken: csrf, ExpiresAt: expiresAt}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Admin, error) {
	return s.AuthenticateForPortal(ctx, PortalAdmin, token)
}

// Reauthenticate verifies the currently authenticated administrator's password
// without creating a second session. Callers use it before revealing secrets.
func (s *Service) Reauthenticate(ctx context.Context, adminID int64, password string) error {
	if adminID < 1 || password == "" {
		return ErrInvalidCredentials
	}
	var passwordHash string
	var status uint8
	err := s.db.QueryRowContext(ctx, `
		SELECT password_hash,status FROM admin_users WHERE id=?`, adminID,
	).Scan(&passwordHash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		argon2Dummy(password)
		return ErrInvalidCredentials
	}
	if err != nil {
		return err
	}
	if status != 1 || !VerifyPassword(passwordHash, password) {
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) AuthenticateForPortal(ctx context.Context, portal string, token string) (Admin, error) {
	if !validPortal(portal) {
		return Admin{}, ErrInvalidCredentials
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Admin{}, ErrInvalidCredentials
	}
	sum := sha256.Sum256([]byte(token))
	var admin Admin
	var sessionID string
	err := s.db.QueryRowContext(ctx, `
		SELECT admin.id,admin.username,COALESCE(NULLIF(admin.display_name,''),admin.username),session.id
		FROM admin_sessions session
		JOIN admin_users admin ON admin.id=session.admin_user_id
		WHERE session.token_hash=? AND session.portal=? AND session.revoked_at IS NULL
		  AND session.expires_at>CURRENT_TIMESTAMP(3) AND admin.status=1
		LIMIT 1`,
		hex.EncodeToString(sum[:]), portal,
	).Scan(&admin.ID, &admin.Username, &admin.DisplayName, &sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, ErrInvalidCredentials
	}
	if err != nil {
		return Admin{}, err
	}
	admin.Permissions, err = s.permissions(ctx, admin.ID)
	if err != nil {
		return Admin{}, err
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE admin_sessions SET last_seen_at=? WHERE id=? AND last_seen_at<?`,
		s.now(), sessionID, s.now().Add(-5*time.Minute),
	)
	return admin, nil
}

func (s *Service) VerifyCSRF(ctx context.Context, token, csrf string) bool {
	return s.VerifyCSRFForPortal(ctx, PortalAdmin, token, csrf)
}

func (s *Service) VerifyCSRFForPortal(ctx context.Context, portal, token, csrf string) bool {
	if !validPortal(portal) {
		return false
	}
	if token == "" || csrf == "" {
		return false
	}
	tokenSum := sha256.Sum256([]byte(token))
	csrfSum := sha256.Sum256([]byte(csrf))
	var exists int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM admin_sessions
		WHERE token_hash=? AND csrf_hash=? AND portal=? AND revoked_at IS NULL
		  AND expires_at>CURRENT_TIMESTAMP(3)`,
		hex.EncodeToString(tokenSum[:]), hex.EncodeToString(csrfSum[:]), portal,
	).Scan(&exists)
	return err == nil && exists == 1
}

func (s *Service) RefreshCSRF(ctx context.Context, token string) (string, error) {
	return s.RefreshCSRFForPortal(ctx, PortalAdmin, token)
}

func (s *Service) RefreshCSRFForPortal(ctx context.Context, portal, token string) (string, error) {
	if !validPortal(portal) {
		return "", ErrInvalidCredentials
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrInvalidCredentials
	}
	csrf, csrfHash, err := opaqueSecret(24)
	if err != nil {
		return "", err
	}
	tokenSum := sha256.Sum256([]byte(token))
	result, err := s.db.ExecContext(ctx, `
		UPDATE admin_sessions SET csrf_hash=?,last_seen_at=?
		WHERE token_hash=? AND portal=? AND revoked_at IS NULL
		  AND expires_at>CURRENT_TIMESTAMP(3)`,
		csrfHash, s.now(), hex.EncodeToString(tokenSum[:]), portal,
	)
	if err != nil {
		return "", err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return "", ErrInvalidCredentials
	}
	return csrf, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.LogoutForPortal(ctx, PortalAdmin, token)
}

func (s *Service) LogoutForPortal(ctx context.Context, portal, token string) error {
	if !validPortal(portal) {
		return ErrInvalidCredentials
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	_, err := s.db.ExecContext(ctx, `
		UPDATE admin_sessions SET revoked_at=? WHERE token_hash=? AND portal=? AND revoked_at IS NULL`,
		s.now(), hex.EncodeToString(sum[:]), portal,
	)
	return err
}

func (s *Service) permissions(ctx context.Context, adminID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT permission.permission_key
		FROM admin_user_roles assignment
		JOIN admin_roles role ON role.id=assignment.role_id AND role.status=1
		JOIN admin_role_permissions grant_row ON grant_row.role_id=role.id
		JOIN admin_permissions permission ON permission.id=grant_row.permission_id
		WHERE assignment.admin_user_id=?
		ORDER BY permission.permission_key`,
		adminID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0, 16)
	for rows.Next() {
		var permission string
		if err = rows.Scan(&permission); err != nil {
			return nil, err
		}
		result = append(result, permission)
	}
	return result, rows.Err()
}

func opaqueSecret(size int) (string, string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(value))
	return value, hex.EncodeToString(sum[:]), nil
}

func argon2Dummy(_ string) {
	_, _ = HashPassword("administrator-not-found")
}

func validPortal(portal string) bool {
	return portal == PortalAdmin || portal == PortalSupport
}
