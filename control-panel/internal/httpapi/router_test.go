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

func TestSessionRoutesRequireOwnership(t *testing.T) {
	authService := auth.NewService([]byte("test-secret"), time.Hour)
	store := sessions.NewStore()
	handler := NewRouter(authService, store)

	aliceToken := issueTestToken(t, authService, "alice")
	bobToken := issueTestToken(t, authService, "bob")

	create := request(t, handler, http.MethodPost, "/api/sessions", aliceToken)
	if create.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want %d", create.Code, http.StatusAccepted)
	}
	var body struct {
		Session sessions.Session `json:"session"`
	}
	if err := json.NewDecoder(create.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	getOther := request(t, handler, http.MethodGet, "/api/sessions/"+body.Session.ID, bobToken)
	if getOther.Code != http.StatusForbidden {
		t.Fatalf("cross-user get status = %d, want %d", getOther.Code, http.StatusForbidden)
	}

	list := request(t, handler, http.MethodGet, "/api/sessions", aliceToken)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", list.Code, http.StatusOK)
	}

	duplicate := request(t, handler, http.MethodPost, "/api/sessions", aliceToken)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want %d", duplicate.Code, http.StatusConflict)
	}
}

func TestSessionRoutesRejectMissingToken(t *testing.T) {
	handler := NewRouter(auth.NewService([]byte("test-secret"), time.Hour), sessions.NewStore())
	response := request(t, handler, http.MethodGet, "/api/sessions", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func issueTestToken(t *testing.T, service *auth.Service, username string) string {
	t.Helper()
	token, err := service.Issue(username)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func request(t *testing.T, handler http.Handler, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}
