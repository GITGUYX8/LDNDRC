package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/nodes"
	"github.com/ldndrc/control-panel/internal/sessions"
)

type fakeMinter struct {
	calls int
}

func (f *fakeMinter) MintToken(context.Context, string, string) (string, string, error) {
	f.calls++
	return fmt.Sprintf("K10fakehash::token%d.secret%d", f.calls, f.calls), fmt.Sprintf("token%d", f.calls), nil
}

func nodesHandler(t *testing.T, minter nodes.Minter) (http.Handler, *nodes.Store) {
	t.Helper()
	authService := auth.NewService([]byte("test-secret"), time.Hour)
	store, err := nodes.NewStore(filepath.Join(t.TempDir(), "nodes.json"))
	if err != nil {
		t.Fatal(err)
	}
	return NewRouterWithNodes(authService, sessions.NewStore(), store, minter), store
}

func postNodeJSON(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestNodesRegisterApprovePollFlow(t *testing.T) {
	t.Setenv("K3S_JOIN_URL", "https://master:6443")
	minter := &fakeMinter{}
	handler, _ := nodesHandler(t, minter)
	authService := auth.NewService([]byte("test-secret"), time.Hour)
	operator := issueTestToken(t, authService, "operator")

	reg := postNodeJSON(t, handler, http.MethodPost, "/api/nodes/register", "",
		`{"hostname":"laptop-a","os":"linux","arch":"amd64","cpu":12,"ram_gb":32,"gpu":"NVIDIA RTX"}`)
	if reg.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201: %s", reg.Code, reg.Body.String())
	}
	var regBody struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(reg.Body).Decode(&regBody); err != nil {
		t.Fatal(err)
	}

	list := postNodeJSON(t, handler, http.MethodGet, "/api/nodes", "", "")
	if list.Code != http.StatusUnauthorized {
		t.Fatalf("anon list status = %d, want 401", list.Code)
	}

	approve := postNodeJSON(t, handler, http.MethodPost, "/api/nodes/"+regBody.ID+"/approve", operator, "{}")
	if approve.Code != http.StatusOK {
		t.Fatalf("approve status = %d: %s", approve.Code, approve.Body.String())
	}

	poll := postNodeJSON(t, handler, http.MethodGet, "/api/nodes/"+regBody.ID, "", "")
	var pollBody struct {
		Status   string `json:"status"`
		K3sToken string `json:"k3s_token"`
		K3sURL   string `json:"k3s_url"`
		NodeName string `json:"node_name"`
	}
	if err := json.NewDecoder(poll.Body).Decode(&pollBody); err != nil {
		t.Fatal(err)
	}
	if pollBody.Status != "approved" || pollBody.K3sToken == "" || pollBody.NodeName == "" {
		t.Fatalf("poll missing token payload: %#v", pollBody)
	}
	if minter.calls != 1 {
		t.Fatalf("minter calls = %d, want 1", minter.calls)
	}

	again := postNodeJSON(t, handler, http.MethodGet, "/api/nodes/"+regBody.ID, "", "")
	var againBody struct {
		Status   string `json:"status"`
		K3sToken string `json:"k3s_token"`
	}
	if err := json.NewDecoder(again.Body).Decode(&againBody); err != nil {
		t.Fatal(err)
	}
	if againBody.K3sToken != "" || minter.calls != 1 {
		t.Fatalf("token must be issued once: %#v calls=%d", againBody, minter.calls)
	}
}

func TestNodesDiagnosticsUpload(t *testing.T) {
	handler, _ := nodesHandler(t, &fakeMinter{})

	reg := postNodeJSON(t, handler, http.MethodPost, "/api/nodes/register", "",
		`{"hostname":"laptop-d","os":"linux","arch":"amd64","cpu":12,"ram_gb":32}`)
	var regBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(reg.Body).Decode(&regBody); err != nil {
		t.Fatal(err)
	}
	up := postNodeJSON(t, handler, http.MethodPost, "/api/nodes/"+regBody.ID+"/diagnostics", "",
		"OS: linux\nchecks: ok\n")
	if up.Code != http.StatusCreated {
		t.Fatalf("upload status = %d: %s", up.Code, up.Body.String())
	}
	missing := postNodeJSON(t, handler, http.MethodPost, "/api/nodes/doesnotexist/diagnostics", "", "x")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown id status = %d, want 404", missing.Code)
	}
	big := postNodeJSON(t, handler, http.MethodPost, "/api/nodes/"+regBody.ID+"/diagnostics", "",
		strings.Repeat("x", nodes.MaxDiagnosticsBytes+1))
	if big.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status = %d, want 413", big.Code)
	}
}

func TestNodesDenyAndUnknown(t *testing.T) {
	handler, _ := nodesHandler(t, &fakeMinter{})
	authService := auth.NewService([]byte("test-secret"), time.Hour)
	operator := issueTestToken(t, authService, "operator")

	reg := postNodeJSON(t, handler, http.MethodPost, "/api/nodes/register", "",
		`{"hostname":"laptop-b","os":"linux","arch":"amd64","cpu":12,"ram_gb":32}`)
	var regBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(reg.Body).Decode(&regBody); err != nil {
		t.Fatal(err)
	}
	deny := postNodeJSON(t, handler, http.MethodPost, "/api/nodes/"+regBody.ID+"/deny", operator, `{"reason":"nope"}`)
	if deny.Code != http.StatusOK {
		t.Fatalf("deny status = %d", deny.Code)
	}
	poll := postNodeJSON(t, handler, http.MethodGet, "/api/nodes/"+regBody.ID, "", "")
	if !strings.Contains(poll.Body.String(), "denied") {
		t.Fatalf("expected denied status: %s", poll.Body.String())
	}
	missing := postNodeJSON(t, handler, http.MethodGet, "/api/nodes/doesnotexist", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown id status = %d, want 404", missing.Code)
	}
}
