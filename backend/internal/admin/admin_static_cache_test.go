package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

	const version = "v=20260822-1"
	for name, source := range map[string]string{
		"app page":  string(page),
		"bootstrap": string(bootstrap),
	} {
		if !strings.Contains(source, version) {
			t.Errorf("%s does not reference release asset version %q", name, version)
		}
	}
}
