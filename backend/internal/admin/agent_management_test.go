package admin

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/adminauth"
)

func TestNormalizeAgentPermissionKeysAddsReadDependencies(t *testing.T) {
	permissions, err := normalizeAgentPermissionKeys([]string{
		" sports.write ", "games.write", "games.read", "app.write",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"app.read", "app.write", "games.read", "games.write", "sports.read", "sports.write",
	}
	if !reflect.DeepEqual(permissions, want) {
		t.Fatalf("permissions = %#v, want %#v", permissions, want)
	}
}

func TestNormalizeAgentPermissionKeysRejectsSensitivePermissions(t *testing.T) {
	for _, permission := range []string{
		"users.read", "wallet.read", "payments.write", "im.read",
		"remote.read", "system.read", "rbac.read", "support.read", "agents.write",
	} {
		if _, err := normalizeAgentPermissionKeys([]string{permission}); err == nil {
			t.Fatalf("sensitive permission %q was accepted", permission)
		}
	}
}

func TestAgentConsoleOnlyMountsAllowlistedBusinessRoutes(t *testing.T) {
	handler := &Handler{}
	mux := http.NewServeMux()
	handler.registerAgentConsole(mux)

	for _, item := range []struct {
		method  string
		path    string
		mounted bool
	}{
		{http.MethodGet, "/agent-console/api/games", true},
		{http.MethodPost, "/agent-console/api/live/rooms", true},
		{http.MethodGet, "/agent-console/api/team-prefixes", true},
		{http.MethodGet, "/agent-console/api/team-prefixes/abc/members", true},
		{http.MethodPost, "/agent-console/api/team-prefixes/abc/account", true},
		{http.MethodPost, "/agent-console/api/team-prefixes/abc/account/password", true},
		{http.MethodGet, "/agent-console/api/users", false},
		{http.MethodGet, "/agent-console/api/wallet/ledger", false},
		{http.MethodGet, "/agent-console/api/rbac", false},
		{http.MethodGet, "/agent-console/api/remote/devices", false},
	} {
		request := httptest.NewRequest(item.method, item.path, nil)
		matched, pattern := mux.Handler(request)
		got := pattern != "" && pattern != "GET /agent-console/"
		if got != item.mounted {
			t.Errorf("%s %s mounted=%v, want %v (pattern %q)", item.method, item.path, got, item.mounted, pattern)
		}
		if !item.mounted {
			recorder := httptest.NewRecorder()
			matched.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Errorf("%s %s status=%d, want 404", item.method, item.path, recorder.Code)
			}
		}
	}
}

func TestAgentSessionCookiesAreIsolated(t *testing.T) {
	handler := &Handler{secureCookies: true}
	recorder := httptest.NewRecorder()
	handler.setAgentSessionCookies(recorder, adminauth.Session{
		Token: "session", CSRFToken: "csrf", ExpiresAt: time.Now().Add(time.Hour),
	})
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookie count = %d, want 2", len(cookies))
	}
	byName := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
		if cookie.Path != agentConsolePath || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Errorf("cookie %#v is not isolated to the agent console", cookie)
		}
	}
	if cookie := byName[agentSessionCookie]; cookie == nil || !cookie.HttpOnly {
		t.Error("agent session cookie must be HttpOnly")
	}
	if cookie := byName[agentCSRFCookie]; cookie == nil || cookie.HttpOnly {
		t.Error("agent CSRF cookie must be readable by the console")
	}
}

func TestAgentConsoleWebContract(t *testing.T) {
	page, err := webFiles.ReadFile("web/agent-app.html")
	if err != nil {
		t.Fatal(err)
	}
	application, err := webFiles.ReadFile("web/static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	pageSource := string(page)
	for _, required := range []string{
		`data-console-api-base="/agent-console/api"`,
		`data-console-default-route="agent-teams"`,
		`href="#agent-teams"`,
		`data-permission="games.read"`,
		`data-permission="app.read"`,
	} {
		if !strings.Contains(pageSource, required) {
			t.Errorf("agent page is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`href="#users"`, `href="#wallet"`, `href="#payments"`, `href="#rbac"`,
		`href="#system"`, `href="#remote"`, `href="#im"`,
	} {
		if strings.Contains(pageSource, forbidden) {
			t.Errorf("agent page exposes sensitive navigation %q", forbidden)
		}
	}
	for _, required := range []string{
		"function apiPath(path)",
		`"agent-teams": agentTeams`,
		`"agent-team-members"`,
		`"agent-team-account-create"`,
		`"agent-team-account-password"`,
		`consoleConfig.apiBase + "/logout"`,
		`sessionStorage.removeItem(consoleConfig.csrfKey)`,
	} {
		if !strings.Contains(string(application), required) {
			t.Errorf("shared application is missing agent-console behavior %q", required)
		}
	}
}
