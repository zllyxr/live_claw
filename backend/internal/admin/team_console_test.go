package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTeamConsoleOnlyMountsTeamMemberRoutes(t *testing.T) {
	handler := &Handler{}
	mux := http.NewServeMux()
	handler.registerTeamConsole(mux)

	for _, item := range []struct {
		method  string
		path    string
		mounted bool
	}{
		{http.MethodGet, "/team-console/api/me", true},
		{http.MethodGet, "/team-console/api/members", true},
		{http.MethodPost, "/team-console/api/logout", true},
		{http.MethodGet, "/team-console/api/users", false},
		{http.MethodGet, "/team-console/api/wallet/ledger", false},
		{http.MethodGet, "/team-console/api/agents", false},
		{http.MethodGet, "/team-console/api/team-prefixes", false},
	} {
		request := httptest.NewRequest(item.method, item.path, nil)
		matched, pattern := mux.Handler(request)
		got := pattern != "" && pattern != "GET /team-console/"
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

func TestTeamSessionCookiesArePathScoped(t *testing.T) {
	handler := &Handler{secureCookies: true}
	recorder := httptest.NewRecorder()
	handler.setTeamSessionCookies(recorder, teamConsoleSession{
		Token: "session", CSRFToken: "csrf", ExpiresAt: time.Now().Add(time.Hour),
	})
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookie count = %d, want 2", len(cookies))
	}
	byName := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
		if cookie.Path != teamConsolePath || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Errorf("cookie %#v is not isolated to the team console", cookie)
		}
	}
	if cookie := byName[teamSessionCookie]; cookie == nil || !cookie.HttpOnly {
		t.Error("team session cookie must be HttpOnly")
	}
	if cookie := byName[teamCSRFCookie]; cookie == nil || cookie.HttpOnly {
		t.Error("team CSRF cookie must be readable by the team console")
	}
}

func TestTeamConsoleWebContract(t *testing.T) {
	loginPage, err := webFiles.ReadFile("web/team-login.html")
	if err != nil {
		t.Fatal(err)
	}
	appPage, err := webFiles.ReadFile("web/team-app.html")
	if err != nil {
		t.Fatal(err)
	}
	application, err := webFiles.ReadFile("web/static/team.js")
	if err != nil {
		t.Fatal(err)
	}
	loginSource := string(loginPage)
	pageSource := string(appPage)
	applicationSource := string(application)
	for _, required := range []string{
		`name="country_code"`, `name="login"`, `name="password"`,
		`/admin/static/team-login.js`,
	} {
		if !strings.Contains(loginSource, required) {
			t.Errorf("team login page is missing %q", required)
		}
	}
	for _, required := range []string{
		"团队成员", "不包含手机号、邮箱、资金或邀请关系", `/admin/static/team.js`,
	} {
		if !strings.Contains(pageSource, required) {
			t.Errorf("team app page is missing %q", required)
		}
	}
	for _, forbidden := range []string{"资金管理", "用户管理", "代理管理", "业务管理"} {
		if strings.Contains(pageSource, forbidden) {
			t.Errorf("team app page exposes forbidden navigation %q", forbidden)
		}
	}
	for _, required := range []string{
		`api("/team-console/api/me")`, `api("/team-console/api/members?"`,
		`sessionStorage.removeItem("claw_team_csrf")`,
	} {
		if !strings.Contains(applicationSource, required) {
			t.Errorf("team application is missing %q", required)
		}
	}
}

func TestTeamOpaqueSecretHashesToken(t *testing.T) {
	token, hash, err := teamOpaqueSecret(32)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || hash == token || hash != teamSecretHash(token) || len(hash) != 64 {
		t.Fatalf("unexpected opaque secret token=%q hash=%q", token, hash)
	}
}
