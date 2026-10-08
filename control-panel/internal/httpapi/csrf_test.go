package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/sessions"
)

func testMux() (http.Handler, *auth.Service) {
	svc := auth.NewService([]byte("test-secret"), time.Hour)
	return NewRouter(svc, sessions.NewStore()), svc
}

func loginTokens(t *testing.T, h http.Handler) (jwt, csrf string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"u","password":"p"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["token"] == "" || body["csrf_token"] == "" {
		t.Fatal("login must return token and csrf_token")
	}
	return body["token"], body["csrf_token"]
}

func csrfReq(t *testing.T, h http.Handler, method, path, bearer, cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "ldndrc_session", Value: cookie})
	}
	if csrf != "" {
		req.Header.Set(CSRFHeader, csrf)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCSRFGuard(t *testing.T) {
	h, svc := testMux()
	jwt, csrf := loginTokens(t, h)

	if got := csrfReq(t, h, http.MethodPost, "/api/sessions", "", jwt, "").Code; got != http.StatusForbidden {
		t.Errorf("cookie POST without token = %d, want 403", got)
	}
	if got := csrfReq(t, h, http.MethodPost, "/api/sessions", "", jwt, "deadbeef").Code; got != http.StatusForbidden {
		t.Errorf("cookie POST with bad token = %d, want 403", got)
	}
	// Valid token falls through to normal auth: no bearer -> 401 proves
	// the guard ran and passed the request onward.
	if got := csrfReq(t, h, http.MethodPost, "/api/sessions", "", jwt, csrf).Code; got != http.StatusUnauthorized {
		t.Errorf("cookie POST with valid token = %d, want 401 (handler auth)", got)
	}
	// Cross-user token must not validate another session's cookie.
	otherJWT, err := svc.Issue("mallory")
	if err != nil {
		t.Fatal(err)
	}
	if got := csrfReq(t, h, http.MethodPost, "/api/sessions", "", otherJWT, csrf).Code; got != http.StatusForbidden {
		t.Errorf("cookie POST with foreign token = %d, want 403", got)
	}
	// Bearer path unaffected: no cookie, no token header -> handler decides.
	if got := csrfReq(t, h, http.MethodPost, "/api/sessions", jwt, "", "").Code; got != http.StatusAccepted {
		t.Errorf("bearer POST without cookie = %d, want 202", got)
	}
	// Safe methods with a cookie but no token still pass the guard.
	if got := csrfReq(t, h, http.MethodGet, "/api/sessions", "", jwt, "").Code; got != http.StatusUnauthorized {
		t.Errorf("cookie GET without token = %d, want 401 (handler auth)", got)
	}
	// Pre-auth register carries no cookie and stays open (real route).
	nh, _ := nodesHandler(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/nodes/register", strings.NewReader(`{"hostname":"x","os":"linux","arch":"amd64","cpu":4,"ram_gb":8}`))
	rec := httptest.NewRecorder()
	nh.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Errorf("register without cookie must not 403 (got %d)", rec.Code)
	}
}
