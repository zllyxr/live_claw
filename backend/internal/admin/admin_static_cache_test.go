package admin

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestAdminStaticAssetsDisableCaching(t *testing.T) {
	handler := noStoreAdminStatic(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/static/app.js", nil))

	if got := recorder.Header().Get("Cache-Control"); got != "no-store, max-age=0" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := recorder.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q, want no-cache", got)
	}
	if got := recorder.Header().Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q, want 0", got)
	}
}

func TestAdminApplicationAssetVersionsMatch(t *testing.T) {
	page, err := webFiles.ReadFile("web/app.html")
	if err != nil {
		t.Fatalf("read embedded admin page: %v", err)
	}
	bootstrap, err := webFiles.ReadFile("web/static/bootstrap.js")
	if err != nil {
		t.Fatalf("read embedded bootstrap script: %v", err)
	}

	styles := regexp.MustCompile(`/admin/static/app\.css\?v=([A-Za-z0-9-]+)"`).FindSubmatch(page)
	loader := regexp.MustCompile(`/admin/static/bootstrap\.js\?v=([A-Za-z0-9-]+)"`).FindSubmatch(page)
	app := regexp.MustCompile(`/admin/static/app\.js\?v=([A-Za-z0-9-]+)"`).FindSubmatch(bootstrap)
	if len(styles) != 2 || len(loader) != 2 || len(app) != 2 ||
		string(styles[1]) != string(loader[1]) || string(loader[1]) != string(app[1]) {
		t.Fatal("admin page, bootstrap and application must share a release version")
	}
}
