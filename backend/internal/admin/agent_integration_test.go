package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func TestPlatformAgentPrefixIsolationAndCurrentMemberCount(t *testing.T) {
	dsn := os.Getenv("CLAW_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	agentIDs := make([]int64, 0, 2)
	for index := 0; index < 2; index++ {
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO admin_users(username,password_hash,display_name,status)
			VALUES(?, 'test', ?, 1)`,
			"agent_prefix_"+suffix+strconv.Itoa(index), "代理测试"+strconv.Itoa(index),
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		agentID, _ := result.LastInsertId()
		agentIDs = append(agentIDs, agentID)
		agentNo, idErr := idgen.New()
		if idErr != nil {
			t.Fatal(idErr)
		}
		if _, insertErr = db.ExecContext(ctx, `
			INSERT INTO platform_agents(admin_user_id,agent_no,status) VALUES(?,?,1)`,
			agentID, agentNo,
		); insertErr != nil {
			t.Fatal(insertErr)
		}
	}

	handler := &Handler{db: db}
	firstAgent := adminauth.Admin{ID: agentIDs[0], Username: "agent_prefix_" + suffix + "0"}
	createRecorder := serveAgentEndpoint(
		http.MethodPost, "/agent-console/api/team-prefixes", firstAgent, handler.createOwnTeamPrefix,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create prefix status=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var createEnvelope struct {
		Code int `json:"code"`
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	if err = json.Unmarshal(createRecorder.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatal(err)
	}
	if createEnvelope.Code != 0 || len(createEnvelope.Data.Code) != 3 || createEnvelope.Data.Code == "sys" {
		t.Fatalf("unexpected create response: %s", createRecorder.Body.String())
	}

	var teamID, systemTeamID int64
	if err = db.QueryRowContext(ctx, `SELECT id FROM teams WHERE code=?`, createEnvelope.Data.Code).Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT id FROM teams WHERE code='sys'`).Scan(&systemTeamID); err != nil {
		t.Fatal(err)
	}
	userIDs := make([]int64, 0, 3)
	for index, status := range []int{1, 2, 1} {
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO users(username,password_hash,team_id,status)
			VALUES(?, 'test', ?, ?)`,
			"agent_member_"+suffix+strconv.Itoa(index), teamID, status,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		userID, _ := result.LastInsertId()
		userIDs = append(userIDs, userID)
		if _, insertErr = db.ExecContext(ctx, `
			INSERT INTO team_members(user_id,team_id,inviter_user_id,status) VALUES(?,?,0,1)`,
			userID, teamID,
		); insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	// A transferred user must no longer count toward the original prefix.
	if _, err = db.ExecContext(ctx, `UPDATE users SET team_id=? WHERE id=?`, systemTeamID, userIDs[2]); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `
		UPDATE team_members SET team_id=?,joined_at=CURRENT_TIMESTAMP(3) WHERE user_id=?`,
		systemTeamID, userIDs[2],
	); err != nil {
		t.Fatal(err)
	}

	defer func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM audit_logs WHERE actor_id IN (?,?)", agentIDs[0], agentIDs[1])
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM team_members WHERE user_id IN (?,?,?)", userIDs[0], userIDs[1], userIDs[2])
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id IN (?,?,?)", userIDs[0], userIDs[1], userIDs[2])
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM platform_agent_teams WHERE team_id=?", teamID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM teams WHERE id=?", teamID)
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM platform_agent_permissions WHERE admin_user_id IN (?,?)", agentIDs[0], agentIDs[1])
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM platform_agents WHERE admin_user_id IN (?,?)", agentIDs[0], agentIDs[1])
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM admin_sessions WHERE admin_user_id IN (?,?)", agentIDs[0], agentIDs[1])
		_, _ = db.ExecContext(cleanupCtx, "DELETE FROM admin_users WHERE id IN (?,?)", agentIDs[0], agentIDs[1])
	}()

	listRecorder := serveAgentEndpoint(
		http.MethodGet, "/agent-console/api/team-prefixes?page=1&page_size=20",
		firstAgent, handler.listOwnTeamPrefixes,
	)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list prefix status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listEnvelope struct {
		Data struct {
			Total int64            `json:"total"`
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err = json.Unmarshal(listRecorder.Body.Bytes(), &listEnvelope); err != nil {
		t.Fatal(err)
	}
	if listEnvelope.Data.Total != 1 || len(listEnvelope.Data.Items) != 1 {
		t.Fatalf("unexpected own prefix list: %s", listRecorder.Body.String())
	}
	item := listEnvelope.Data.Items[0]
	if item["code"] != createEnvelope.Data.Code || item["member_count"] != float64(2) {
		t.Fatalf("unexpected prefix statistics: %#v", item)
	}
	if len(item) != 2 {
		t.Fatalf("agent prefix response leaked extra fields: %#v", item)
	}

	otherRecorder := serveAgentEndpoint(
		http.MethodGet, "/agent-console/api/team-prefixes?page=1&page_size=20",
		adminauth.Admin{ID: agentIDs[1]}, handler.listOwnTeamPrefixes,
	)
	if otherRecorder.Code != http.StatusOK {
		t.Fatalf("other agent list status=%d body=%s", otherRecorder.Code, otherRecorder.Body.String())
	}
	var otherEnvelope struct {
		Data struct {
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err = json.Unmarshal(otherRecorder.Body.Bytes(), &otherEnvelope); err != nil {
		t.Fatal(err)
	}
	if otherEnvelope.Data.Total != 0 {
		t.Fatalf("other agent could see assigned prefix: %s", otherRecorder.Body.String())
	}
}

func serveAgentEndpoint(
	method string,
	target string,
	agent adminauth.Admin,
	endpoint http.HandlerFunc,
) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, nil)
	handler := httpx.RequestContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint(w, r.WithContext(withAdmin(r, agent)))
	}))
	handler.ServeHTTP(recorder, request)
	return recorder
}
