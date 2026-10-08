package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/sessions"
)

func TestSPAServesPlaceholderAndFallback(t *testing.T) {
	h := NewRouter(auth.NewService([]byte("test-secret"), time.Hour), sessions.NewStore())

	for _, path := range []string{"/", "/login", "/launch", "/app?view=code"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("GET %s content-type = %q, want text/html", path, ct)
		}
	}
}

func TestSPADoesNotShadowAPI(t *testing.T) {
	h := NewRouter(auth.NewService([]byte("test-secret"), time.Hour), sessions.NewStore())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "LDNDRC student UI") {
		t.Fatal("/healthz must stay JSON, not the SPA")
	}
}
