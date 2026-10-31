package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/portal"
	"github.com/ldndrc/control-panel/internal/sessions"
)

func portalRouter(t *testing.T) (http.Handler, *portal.Store) {
	t.Helper()
	authService := auth.NewService([]byte("test-secret"), time.Hour)
	store, err := portal.NewStore(filepath.Join(t.TempDir(), "downloads.json"))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewRouterWithNodes(authService, sessions.NewStore(), nil, nil, portal.NewHandler(store))
	return handler, store
}

func TestPortalRoutesServePreApproval(t *testing.T) {
	handler, _ := portalRouter(t)

	// The welcome page: text/html, portal content (not the SPA fallback).
	page := get(t, handler, "/portal")
	if page.Code != http.StatusOK {
		t.Fatalf("GET /portal status = %d, want %d", page.Code, http.StatusOK)
	}
	if ct := page.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET /portal content-type = %q, want text/html", ct)
	}
	if !strings.Contains(page.Body.String(), "joining-cluster.sh") {
		t.Fatal("GET /portal served something other than the portal page")
	}

	// Trailing-slash variant must hit the portal too, not the SPA.
	pageSlash := get(t, handler, "/portal/")
	if pageSlash.Code != http.StatusOK ||
		!strings.Contains(pageSlash.Body.String(), "joining-cluster.sh") {
		t.Fatalf("GET /portal/ did not serve the portal page: %d", pageSlash.Code)
	}
}

func TestPortalScriptDownloadCountsThroughRouter(t *testing.T) {
	handler, store := portalRouter(t)

	rec := get(t, handler, "/portal/joining-cluster.sh")
	if rec.Code != http.StatusOK {
		t.Fatalf("download status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.HasPrefix(rec.Body.String(), "#!/") {
		t.Fatal("router served something other than the script")
	}
	if got := store.PublicCounts().Artifacts[portal.ArtifactSh]; got != 1 {
		t.Fatalf("downloads counted through router = %d, want 1", got)
	}

	count := get(t, handler, "/api/downloads/count")
	if count.Code != http.StatusOK {
		t.Fatalf("count status = %d, want %d", count.Code, http.StatusOK)
	}
	var pc portal.PublicCounts
	if err := json.Unmarshal(count.Body.Bytes(), &pc); err != nil {
		t.Fatal(err)
	}
	if pc.Total != 1 {
		t.Fatalf("count total = %d, want 1", pc.Total)
	}
}

func TestDownloadStatsRequireAuth(t *testing.T) {
	handler, _ := portalRouter(t)
	authService := auth.NewService([]byte("test-secret"), time.Hour)
	token, err := authService.Issue("instructor")
	if err != nil {
		t.Fatal(err)
	}

	if unauth := get(t, handler, "/api/downloads/stats"); unauth.Code != http.StatusUnauthorized {
		t.Fatalf("stats without token status = %d, want %d", unauth.Code, http.StatusUnauthorized)
	}
	authed := request(t, handler, http.MethodGet, "/api/downloads/stats", token)
	if authed.Code != http.StatusOK {
		t.Fatalf("stats with token status = %d, want %d", authed.Code, http.StatusOK)
	}
	if !strings.Contains(authed.Body.String(), "unique_devices") {
		t.Fatalf("stats body = %s, want unique_devices field", authed.Body.String())
	}
}

// get is the no-body request helper for portal routes.
func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
