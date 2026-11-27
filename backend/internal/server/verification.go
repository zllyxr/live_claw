package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/redis/go-redis/v9"
)

type verificationMailer interface {
	SendCode(context.Context, string, string, string) error
}

func (s *Server) SetVerificationMailer(sender verificationMailer) { s.verificationMailer = sender }

func (s *Server) debugVerification() bool {
	return s.verificationMailer == nil && (s.environment == "local" || s.environment == "development" || s.environment == "test")
}

// Rate limits share an email key across phone numbers and purposes to prevent
// bypassing the recipient cooldown by changing form values.
var verificationRateScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then return 0 end
if tonumber(redis.call('GET', KEYS[2]) or '0') >= 5 then return 0 end
if tonumber(redis.call('GET', KEYS[3]) or '0') >= 20 then return 0 end
redis.call('SET', KEYS[1], '1', 'EX', 60)
if redis.call('INCR', KEYS[2]) == 1 then redis.call('EXPIRE', KEYS[2], 3600) end
if redis.call('INCR', KEYS[3]) == 1 then redis.call('EXPIRE', KEYS[3], 3600) end
return 1`)

func (s *Server) allowVerification(ctx context.Context, email, ip string) (bool, error) {
	if s.debugVerification() && s.redis == nil {
		return true, nil
	}
	if s.redis == nil {
		return false, errors.New("verification rate limiter unavailable")
	}
	emailHash, ipHash := sha256.Sum256([]byte(email)), sha256.Sum256([]byte(ip))
	allowed, err := verificationRateScript.Run(ctx, s.redis, []string{
		fmt.Sprintf("auth:email:cooldown:%x", emailHash),
		fmt.Sprintf("auth:email:hour:%x", emailHash),
		fmt.Sprintf("auth:email:ip:%x", ipHash),
	}).Int()
	return allowed == 1, err
}

func (s *Server) issueVerification(ctx context.Context, purpose, target, email string) (string, error) {
	code := "123456"
	if !s.debugVerification() {
		n, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err != nil {
			return "", err
		}
		code = fmt.Sprintf("%06d", n)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// Keep old codes invalid after a successful resend; roll back on send failure.
	if _, err = tx.ExecContext(ctx, `UPDATE auth_verification_codes SET consumed_at=CURRENT_TIMESTAMP(3)
		WHERE purpose=? AND target=? AND consumed_at IS NULL`, purpose, target); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(code))
	if _, err = tx.ExecContext(ctx, `INSERT INTO auth_verification_codes(purpose,target,code_hash,expires_at)
		VALUES(?,?,?,DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 10 MINUTE))`, purpose, target, hex.EncodeToString(sum[:])); err != nil {
		return "", err
	}
	if !s.debugVerification() {
		if s.verificationMailer == nil {
			return "", errors.New("verification mailer not configured")
		}
		if err = s.verificationMailer.SendCode(ctx, email, purpose, code); err != nil {
			return "", err
		}
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return code, nil
}

var errInvalidVerificationCode = errors.New("invalid verification code")

func numericFormValue(value string, min, max int) bool {
	return len(value) >= min && len(value) <= max && strings.Trim(value, "0123456789") == ""
}
