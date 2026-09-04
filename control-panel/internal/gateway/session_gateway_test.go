package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/sessions"
)

type fakeSessionLookup struct {
	session sessions.Session
	err     error
}

func (f fakeSessionLookup) Current(string) (sessions.Session, error) {
	return f.session, f.err
}

func TestSessionGatewayRejectsUnauthenticatedToolRequest(t *testing.T) {
	t.Setenv("GATEWAY_HOST_SUFFIX", "ros-platform.local")
	authService := auth.NewService([]byte("test-secret"), time.Hour)
	handler := NewSessionHandler(authService, fakeSessionLookup{}, http.NotFoundHandler())

	request := httptest.NewRequest(http.MethodGet, "http://editor.ros-platform.local/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestSessionGatewayRejectsUnreadySession(t *testing.T) {
	t.Setenv("GATEWAY_HOST_SUFFIX", "ros-platform.local")
	authService := auth.NewService([]byte("test-secret"), time.Hour)
	token, err := authService.Issue("alice")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSessionHandler(authService, fakeSessionLookup{
		session: sessions.Session{Username: "alice", Status: sessions.StatusProvisioning},
	}, http.NotFoundHandler())

	request := httptest.NewRequest(http.MethodGet, "http://editor.ros-platform.local/", nil)
	cookieResponse := httptest.NewRecorder()
	authService.SetSessionCookie(cookieResponse, token)
	request.Header.Set("Cookie", cookieResponse.Header().Get("Set-Cookie"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
