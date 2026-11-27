package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/migrations"
)

type capturedVerification struct {
	code, purpose, email string
	fail                 bool
	sends                int
}

func (m *capturedVerification) SendCode(_ context.Context, email, purpose, code string) error {
	m.code, m.purpose, m.email = code, purpose, email
	m.sends++
	if m.fail {
		return errors.New("simulated SMTP failure")
	}
	return nil
}

func callVerificationAPI(t *testing.T, s *Server, service string, values url.Values) (int, string) {
	t.Helper()
	request := httptest.NewRequest("POST", "/appapi/?service="+service, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = "192.0.2.55:12345"
	recorder := httptest.NewRecorder()
	s.compat(recorder, request)
	var response struct {
		Data struct {
			Code int `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Data.Code, recorder.Body.String()
}

func TestProductionVerificationUnavailableWithoutSMTP(t *testing.T) {
	s := &Server{environment: "production"}
	code, body := callVerificationAPI(t, s, "Login.getCode", url.Values{"mobile": {"18800000001"}, "email": {"test@example.com"}})
	if code != 503 || strings.Contains(body, "123456") {
		t.Fatalf("unexpected unconfigured response: %s", body)
	}
}

func TestVerificationEmailLifecycleIntegration(t *testing.T) {
	dsn, socket := os.Getenv("CLAW_TEST_MYSQL_DSN"), os.Getenv("CLAW_TEST_REDIS_SOCKET")
	if dsn == "" || socket == "" {
		t.Skip("isolated MySQL and Redis are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(&redis.Options{Network: "unix", Addr: socket})
	defer rdb.Close()
	if err = rdb.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	m := &capturedVerification{}
	s := &Server{db: db, redis: rdb, auth: auth.New(db, auth.Options{}), environment: "production", verificationMailer: m, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	phone, email := suffix, "verify-"+suffix+"@example.invalid"
	target := "852:" + phone + "|" + email
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM user_sessions WHERE user_id IN (SELECT id FROM users WHERE username=?)`, phone)
		_, _ = db.Exec(`DELETE FROM wallet_accounts WHERE user_id IN (SELECT id FROM users WHERE username=?)`, phone)
		_, _ = db.Exec(`DELETE FROM users WHERE username=?`, phone)
		_, _ = db.Exec(`DELETE FROM auth_verification_codes WHERE target=?`, target)
	})
	values := url.Values{"mobile": {phone}, "email": {email}, "country_code": {"852"}}
	code, body := callVerificationAPI(t, s, "Login.getCode", values)
	if code != 0 || m.sends != 1 || !numericFormValue(m.code, 6, 6) || strings.Contains(body, m.code) || strings.Contains(body, "debug_code") {
		t.Fatalf("send failed or code leaked: %s sends=%d", body, m.sends)
	}
	firstCode := m.code
	code, _ = callVerificationAPI(t, s, "Login.getCode", values)
	if code != 429 || m.sends != 1 {
		t.Fatal("cooldown did not prevent duplicate mail")
	}
	var storedHash string
	if err = db.QueryRow(`SELECT code_hash FROM auth_verification_codes WHERE target=? ORDER BY id DESC LIMIT 1`, target).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash == firstCode || len(storedHash) != 64 {
		t.Fatal("code not stored as a hash")
	}

	// Failed delivery rolls back both the new code and old-code invalidation.
	m.fail = true
	if _, err = s.issueVerification(ctx, "register", target, email); err == nil {
		t.Fatal("delivery failure reported success")
	}
	var active int
	if err = db.QueryRow(`SELECT COUNT(*) FROM auth_verification_codes WHERE target=? AND consumed_at IS NULL`, target).Scan(&active); err != nil || active != 1 {
		t.Fatalf("failed delivery changed codes: %d %v", active, err)
	}
	m.fail = false
	// Successful resend invalidates the first message.
	if _, err = s.issueVerification(ctx, "register", target, email); err != nil {
		t.Fatal(err)
	}
	latest := m.code
	registration := url.Values{"user_login": {phone}, "email": {email}, "country_code": {"852"}, "code": {firstCode}, "user_pass": {"VerificationTest123!"}, "user_pass2": {"VerificationTest123!"}}
	if latest != firstCode {
		code, _ = callVerificationAPI(t, s, "Login.userReg", registration)
		if code != 400 {
			t.Fatal("old code accepted")
		}
	}
	registration.Set("code", latest)
	code, body = callVerificationAPI(t, s, "Login.userReg", registration)
	if code != 0 {
		t.Fatalf("actual frontend registration fields failed: %s", body)
	}
	code, _ = callVerificationAPI(t, s, "Login.userReg", registration)
	if code != 400 {
		t.Fatal("verification code reused")
	}

	// Reset uses the same sender but must match an existing account and purpose.
	emailHash := sha256.Sum256([]byte(email))
	if err = rdb.Del(ctx, fmt.Sprintf("auth:email:cooldown:%x", emailHash)).Err(); err != nil {
		t.Fatal(err)
	}
	code, body = callVerificationAPI(t, s, "Login.getForgetCode", values)
	if code != 0 || m.purpose != "reset" {
		t.Fatalf("reset delivery failed: %s", body)
	}
	reset := url.Values{"user_login": {phone}, "email": {email}, "country_code": {"852"}, "code": {m.code}, "user_pass": {"NewVerification123!"}, "user_pass2": {"NewVerification123!"}}
	code, body = callVerificationAPI(t, s, "Login.userFindPass", reset)
	if code != 0 {
		t.Fatalf("reset failed: %s", body)
	}
	if _, err = s.auth.Login(ctx, "852", phone, "NewVerification123!", "test", "test", "192.0.2.55", "test"); err != nil {
		t.Fatalf("new password cannot log in: %v", err)
	}

	// Wrong attempts persist even though registration's transaction aborts.
	if _, err = s.issueVerification(ctx, "register", target, email); err != nil {
		t.Fatal(err)
	}
	registration.Set("code", "bad")
	for i := 0; i < 5; i++ {
		if code, _ = callVerificationAPI(t, s, "Login.userReg", registration); code != 400 {
			t.Fatal("invalid code accepted")
		}
	}
	registration.Set("code", m.code)
	if code, _ = callVerificationAPI(t, s, "Login.userReg", registration); code != 400 {
		t.Fatal("exhausted code accepted")
	}
	if _, err = s.issueVerification(ctx, "register", target, email); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE auth_verification_codes SET expires_at=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE target=?`, target)
	if err != nil {
		t.Fatal(err)
	}
	registration.Set("code", m.code)
	if code, _ = callVerificationAPI(t, s, "Login.userReg", registration); code != 400 {
		t.Fatal("expired code accepted")
	}
}

func TestVerificationRateLimitAtomicIntegration(t *testing.T) {
	socket := os.Getenv("CLAW_TEST_REDIS_SOCKET")
	if socket == "" {
		t.Skip("isolated Redis required")
	}
	rdb := redis.NewClient(&redis.Options{Network: "unix", Addr: socket})
	defer rdb.Close()
	s := &Server{redis: rdb, environment: "production"}
	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	email, ip := "rate-"+suffix+"@example.invalid", "test-"+suffix
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.allowVerification(ctx, email, ip)
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("parallel requests accepted %d", accepted.Load())
	}
	hash := sha256.Sum256([]byte(email))
	for i := 1; i < 5; i++ {
		rdb.Del(ctx, fmt.Sprintf("auth:email:cooldown:%x", hash))
		if ok, err := s.allowVerification(ctx, email, ip); err != nil || !ok {
			t.Fatalf("hour quota prematurely blocked: %v", err)
		}
	}
	rdb.Del(ctx, fmt.Sprintf("auth:email:cooldown:%x", hash))
	if ok, err := s.allowVerification(ctx, email, ip); err != nil || ok {
		t.Fatal("recipient hourly quota not enforced")
	}
	for i := 0; i < 15; i++ {
		if ok, err := s.allowVerification(ctx, fmt.Sprintf("%d-%s", i, email), ip); err != nil || !ok {
			t.Fatal("IP quota prematurely blocked")
		}
	}
	if ok, err := s.allowVerification(ctx, "overflow-"+email, ip); err != nil || ok {
		t.Fatal("IP hourly quota not enforced")
	}
}
