package server

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/zllyxr/live_claw/backend/internal/auth"
)

func (s *Server) compatExtended(w http.ResponseWriter, r *http.Request, service string) bool {
	if s.compatAuthUser(w, r, service) {
		return true
	}
	if s.compatSocial(w, r, service) {
		return true
	}
	if s.compatFinance(w, r, service) {
		return true
	}
	if s.compatLiveFeatures(w, r, service) {
		return true
	}
	if s.compatSystem(w, r, service) {
		return true
	}
	return false
}

func (s *Server) requireCompatUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID := compatInt64(r.FormValue("uid"))
	if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
		writeCompat(w, 700, "登录已失效", nil)
		return 0, false
	}
	return userID, true
}

func (s *Server) optionalCompatUser(r *http.Request) int64 {
	userID := compatInt64(r.FormValue("uid"))
	if userID < 1 {
		return 0
	}
	if _, err := s.auth.Authenticate(r.Context(), userID, r.FormValue("token")); err != nil {
		return 0
	}
	return userID
}

func compatInt64(value string) int64 {
	result, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return result
}

func compatInt(value string) int {
	result, _ := strconv.Atoi(strings.TrimSpace(value))
	return result
}

func compatPage(value string) (int, int) {
	page := compatInt(value)
	if page < 1 {
		page = 1
	}
	const pageSize = 20
	return pageSize, (page - 1) * pageSize
}

func boundedCompat(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) > maximum {
		return value[:maximum]
	}
	return value
}

func (s *Server) mediaURL(objectKey string) string {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return ""
	}
	return strings.TrimRight(s.mediaBaseURL, "/") + "/" + strings.TrimLeft(objectKey, "/")
}

func (s *Server) assetIDForValue(ctx context.Context, userID int64, value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
		var ownerID int64
		err = s.db.QueryRowContext(ctx,
			`SELECT owner_user_id FROM media_assets WHERE id=? AND status=1`, id,
		).Scan(&ownerID)
		if err != nil {
			return 0, err
		}
		if ownerID != 0 && ownerID != userID {
			return 0, errors.New("media asset does not belong to user")
		}
		return id, nil
	}
	objectKey := value
	prefix := strings.TrimRight(s.mediaBaseURL, "/") + "/"
	if strings.HasPrefix(objectKey, prefix) {
		objectKey = strings.TrimPrefix(objectKey, prefix)
	} else if index := strings.Index(objectKey, "/claw-public/"); index >= 0 {
		objectKey = objectKey[index+len("/claw-public/"):]
	}
	objectKey = strings.TrimLeft(objectKey, "/")
	var id, ownerID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id,owner_user_id FROM media_assets
		WHERE object_key=? AND status=1 LIMIT 1`, objectKey,
	).Scan(&id, &ownerID)
	if err != nil {
		return 0, err
	}
	if ownerID != 0 && ownerID != userID {
		return 0, errors.New("media asset does not belong to user")
	}
	return id, nil
}

func (s *Server) encryptSensitive(plaintext string) ([]byte, error) {
	key := sha256.Sum256([]byte(s.dataKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return []byte(base64.RawStdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil))), nil
}

func compatIsDuplicate(err error) bool {
	var duplicate *mysql.MySQLError
	return errors.As(err, &duplicate) && duplicate.Number == 1062
}

func scanNullableString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func compatAuthError(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, auth.ErrInvalidSession) {
		writeCompat(w, 700, "登录已失效", nil)
		return
	}
	writeCompat(w, 500, message, nil)
}
