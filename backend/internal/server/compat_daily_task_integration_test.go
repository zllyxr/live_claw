package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/internal/wallet"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestDailyTaskRuntimePermissionsIntegration(t *testing.T) {
	setupDSN, runtimeDSN := os.Getenv("CLAW_TEST_MYSQL_DSN"), os.Getenv("CLAW_TEST_API_MYSQL_DSN")
	if setupDSN == "" || runtimeDSN == "" {
		t.Skip("test setup and API runtime DSNs are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, setupDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	runtimeDB, err := database.Open(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtimeDB.Close() })
	uid := time.Now().UnixNano() & 0x3fffffffffffffff
	suffix := strconv.FormatInt(uid, 36)
	token := "daily-task-test-" + suffix
	digest := sha256.Sum256([]byte(token))
	sessionID, _ := idgen.New()
	var taskID int64
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		for _, statement := range []string{
			"DELETE FROM wallet_ledger_entries WHERE user_id=?", "DELETE FROM wallet_accounts WHERE user_id=?",
			"DELETE FROM user_task_progress WHERE user_id=?", "DELETE FROM user_sessions WHERE user_id=?", "DELETE FROM users WHERE id=?",
		} {
			_, _ = db.ExecContext(clean, statement, uid)
		}
		_, _ = db.ExecContext(clean, "DELETE FROM daily_tasks WHERE id=?", taskID)
	})
	if _, err = db.ExecContext(ctx, "INSERT INTO users(id,username,password_hash,status) VALUES(?,?,'test',1)", uid, "daily_"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "INSERT INTO wallet_accounts(user_id,currency,available,status) VALUES(?,'COIN',0,1)", uid); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "INSERT INTO user_sessions(id,user_id,token_hash,expires_at) VALUES(?,?,?,CURRENT_TIMESTAMP(3)+INTERVAL 1 HOUR)", sessionID, uid, hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}
	result, err := db.ExecContext(ctx, "INSERT INTO daily_tasks(task_key,name,description,target_count,reward_coin,status,rules) VALUES(?,'登录奖励测试','隔离测试',1,10,1,JSON_OBJECT('event','login'))", "daily_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	taskID, _ = result.LastInsertId()
	if _, err = db.ExecContext(ctx, "INSERT INTO user_task_progress(user_id,task_id,task_date,progress) VALUES(?,?,CURRENT_DATE,0)", uid, taskID); err != nil {
		t.Fatal(err)
	}
	var lockedID int64
	lockErr := runtimeDB.QueryRowContext(ctx, "SELECT id FROM daily_tasks WHERE id=? FOR UPDATE", taskID).Scan(&lockedID)
	if lockErr == nil || !strings.Contains(strings.ToLower(lockErr.Error()), "denied") {
		t.Fatalf("test runtime must deny task-definition write locks: %v", lockErr)
	}
	s := &Server{db: runtimeDB, auth: auth.New(runtimeDB), wallet: wallet.New(runtimeDB)}
	claim := func() int {
		t.Helper()
		values := url.Values{"uid": {strconv.FormatInt(uid, 10)}, "token": {token}, "taskid": {strconv.FormatInt(taskID, 10)}}
		r := httptest.NewRequest(http.MethodPost, "/appapi/", strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		s.compatClaimDailyTask(w, r)
		var response struct {
			Data struct {
				Code int `json:"code"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Data.Code
	}
	if code := claim(); code != 409 {
		t.Fatalf("incomplete task code=%d, want 409", code)
	}
	if _, err = db.ExecContext(ctx, "UPDATE user_task_progress SET progress=1 WHERE user_id=? AND task_id=?", uid, taskID); err != nil {
		t.Fatal(err)
	}
	if code := claim(); code != 0 {
		t.Fatalf("completed task code=%d, want 0", code)
	}
	if code := claim(); code != 409 {
		t.Fatalf("duplicate claim code=%d, want 409", code)
	}
	var balance int64
	var entries, claimed int
	if err = db.QueryRowContext(ctx, `SELECT
		(SELECT available FROM wallet_accounts WHERE user_id=? AND currency='COIN'),
		(SELECT COUNT(*) FROM wallet_ledger_entries WHERE user_id=? AND business_type='daily_task'),
		(SELECT COUNT(*) FROM user_task_progress WHERE user_id=? AND task_id=? AND claimed_at IS NOT NULL)`, uid, uid, uid, taskID).Scan(&balance, &entries, &claimed); err != nil {
		t.Fatal(err)
	}
	if balance != 10 || entries != 1 || claimed != 1 {
		t.Fatalf("reward not credited exactly once: balance=%d entries=%d claimed=%d", balance, entries, claimed)
	}
}
