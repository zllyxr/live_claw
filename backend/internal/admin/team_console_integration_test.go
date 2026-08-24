package admin

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/auth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/invite"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestTeamConsoleLoginIsolationAndMemberScope(t *testing.T) {
	dsn := os.Getenv("CLAW_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	firstTeamID, firstCode := insertTeamConsoleTestTeam(t, ctx, db, "团队后台测试一"+suffix)
	secondTeamID, _ := insertTeamConsoleTestTeam(t, ctx, db, "团队后台测试二"+suffix)
	thirdTeamID, _ := insertTeamConsoleTestTeam(t, ctx, db, "团队后台测试三"+suffix)
	password := "TeamConsoleTest!2026"
	passwordHash, err := adminauth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	userIDs := make([]int64, 0, 4)
	insertUser := func(index, teamStatus int, teamID int64) int64 {
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO users(username,password_hash,password_algo,nickname,team_id,status)
			VALUES(?,?,'argon2id',?,?,?)`,
			"team_console_"+suffix+strconv.Itoa(index), passwordHash,
			"团队成员"+strconv.Itoa(index), teamID, teamStatus,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		userID, _ := result.LastInsertId()
		userIDs = append(userIDs, userID)
		if _, insertErr = db.ExecContext(ctx, `
			INSERT INTO team_members(user_id,team_id,inviter_user_id,status)
			VALUES(?,?,0,1)`, userID, teamID); insertErr != nil {
			t.Fatal(insertErr)
		}
		return userID
	}
	ownerID := insertUser(0, 1, firstTeamID)
	normalMemberID := insertUser(1, 1, firstTeamID)
	frozenMemberID := insertUser(2, 2, firstTeamID)
	movedMemberID := insertUser(3, 1, secondTeamID)
	if _, err = db.ExecContext(ctx, `UPDATE teams SET owner_user_id=? WHERE id=?`, ownerID, firstTeamID); err != nil {
		t.Fatal(err)
	}

	defer func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM audit_logs WHERE actor_id IN (?,?,?,?)", ownerID, normalMemberID, frozenMemberID, movedMemberID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM team_console_sessions WHERE user_id IN (?,?,?,?)", ownerID, normalMemberID, frozenMemberID, movedMemberID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM user_sessions WHERE user_id IN (?,?,?,?)", ownerID, normalMemberID, frozenMemberID, movedMemberID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM team_members WHERE user_id IN (?,?,?,?)", ownerID, normalMemberID, frozenMemberID, movedMemberID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id IN (?,?,?,?)", ownerID, normalMemberID, frozenMemberID, movedMemberID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM teams WHERE id IN (?,?,?)", firstTeamID, secondTeamID, thirdTeamID)
	}()

	handler := &Handler{db: db, userAuth: auth.New(db), secureCookies: true}
	loginBody := `{"country_code":"86","login":"team_console_` + suffix + `0","password":"` + password + `"}`
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/team-console/api/login", strings.NewReader(loginBody))
	loginRequest.Header.Set("Content-Type", "application/json")
	httpx.RequestContext(http.HandlerFunc(handler.teamLogin)).ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("team login status=%d body=%s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == teamSessionCookie {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || sessionCookie.Path != teamConsolePath {
		t.Fatalf("team login did not issue path-scoped session: %#v", loginRecorder.Result().Cookies())
	}
	var appSessions, teamSessions int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_sessions WHERE user_id=?`, ownerID).Scan(&appSessions); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_console_sessions WHERE user_id=?`, ownerID).Scan(&teamSessions); err != nil {
		t.Fatal(err)
	}
	if appSessions != 0 || teamSessions != 1 {
		t.Fatalf("session isolation app=%d team=%d, want app=0 team=1", appSessions, teamSessions)
	}

	authenticatedRequest := httptest.NewRequest(http.MethodGet, "/team-console/api/members", nil)
	authenticatedRequest.AddCookie(sessionCookie)
	principal, err := handler.currentTeamPrincipal(authenticatedRequest)
	if err != nil {
		t.Fatal(err)
	}
	if principal.TeamID != firstTeamID || principal.TeamCode != firstCode || principal.MemberCount != 3 {
		t.Fatalf("unexpected principal: %#v", principal)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/team-console/api/members?page=1&page_size=20", nil)
	listRequest = listRequest.WithContext(withTeamPrincipal(listRequest, principal))
	httpx.RequestContext(http.HandlerFunc(handler.listOwnTeamMembers)).ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("member list status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var envelope struct {
		Data struct {
			Total int64            `json:"total"`
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err = json.Unmarshal(listRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Total != 3 || len(envelope.Data.Items) != 3 {
		t.Fatalf("unexpected team members: %s", listRecorder.Body.String())
	}
	foundFrozen := false
	for _, item := range envelope.Data.Items {
		if len(item) != 5 {
			t.Fatalf("team member response leaked extra fields: %#v", item)
		}
		if item["id"] == strconv.FormatInt(movedMemberID, 10) {
			t.Fatalf("member from another team leaked into response: %#v", item)
		}
		if item["id"] == strconv.FormatInt(frozenMemberID, 10) && item["status"] == float64(2) {
			foundFrozen = true
		}
	}
	if !foundFrozen {
		t.Fatalf("frozen member was not returned: %#v", envelope.Data.Items)
	}

	nonOwnerBody := `{"country_code":"86","login":"team_console_` + suffix + `1","password":"` + password + `"}`
	nonOwnerRecorder := httptest.NewRecorder()
	nonOwnerRequest := httptest.NewRequest(http.MethodPost, "/team-console/api/login", strings.NewReader(nonOwnerBody))
	nonOwnerRequest.Header.Set("Content-Type", "application/json")
	httpx.RequestContext(http.HandlerFunc(handler.teamLogin)).ServeHTTP(nonOwnerRecorder, nonOwnerRequest)
	if nonOwnerRecorder.Code != http.StatusForbidden {
		t.Fatalf("non-owner login status=%d body=%s", nonOwnerRecorder.Code, nonOwnerRecorder.Body.String())
	}

	assignRecorder := httptest.NewRecorder()
	assignRequest := httptest.NewRequest(
		http.MethodPost, "/admin/api/users/"+strconv.FormatInt(normalMemberID, 10)+"/team",
		strings.NewReader(fmt.Sprintf(`{"team_id":"%d","reason":"测试首成员自动成为负责人"}`, thirdTeamID)),
	)
	assignRequest.Header.Set("Content-Type", "application/json")
	assignRequest.SetPathValue("id", strconv.FormatInt(normalMemberID, 10))
	httpx.RequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.assignUserTeam(w, r.WithContext(withAdmin(r, adminauth.Admin{ID: ownerID})))
	})).ServeHTTP(assignRecorder, assignRequest)
	if assignRecorder.Code != http.StatusOK {
		t.Fatalf("seed member assignment status=%d body=%s", assignRecorder.Code, assignRecorder.Body.String())
	}
	var seededOwnerID int64
	if err = db.QueryRowContext(ctx, `SELECT owner_user_id FROM teams WHERE id=?`, thirdTeamID).Scan(&seededOwnerID); err != nil {
		t.Fatal(err)
	}
	if seededOwnerID != normalMemberID {
		t.Fatalf("empty team owner=%d, want first assigned user %d", seededOwnerID, normalMemberID)
	}

	if _, err = db.ExecContext(ctx, `UPDATE teams SET owner_user_id=? WHERE id=?`, normalMemberID, firstTeamID); err != nil {
		t.Fatal(err)
	}
	if _, err = handler.currentTeamPrincipal(authenticatedRequest); err == nil {
		t.Fatal("previous owner session remained valid after ownership transfer")
	}
}

func insertTeamConsoleTestTeam(t *testing.T, ctx context.Context, db *sql.DB, name string) (int64, string) {
	t.Helper()
	for attempt := 0; attempt < 64; attempt++ {
		code, err := invite.GeneratePart(rand.Reader, 3)
		if err != nil {
			t.Fatal(err)
		}
		if code == "sys" {
			continue
		}
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO teams(code,name,owner_user_id,status,created_by) VALUES(?,?,0,1,0)`, code, name)
		if insertErr != nil {
			continue
		}
		teamID, _ := result.LastInsertId()
		return teamID, code
	}
	t.Fatal("unable to create unique team code")
	return 0, ""
}
