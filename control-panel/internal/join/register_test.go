package join

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeNodes emulates the control panel nodes API: first poll pending,
// then approved with a one-time token.
func fakeNodes(t *testing.T) *httptest.Server {
	t.Helper()
	polls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/nodes/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "test01", "status": "pending"})
	})
	mux.HandleFunc("/api/nodes/test01", func(w http.ResponseWriter, r *http.Request) {
		polls++
		if polls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "approved", "k3s_url": "https://master:6443",
			"k3s_token": "K10fake::id.secret", "node_name": "ldndrc-test01",
		})
	})
	mux.HandleFunc("/api/nodes/test01/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})
	return httptest.NewServer(mux)
}

func TestRegisterPollFlow(t *testing.T) {
	srv := fakeNodes(t)
	defer srv.Close()
	c := NewClient(srv.URL)
	ctx := context.Background()

	id, err := c.Register(ctx, Fingerprint{Hostname: "h", OS: "linux", Arch: "amd64", CPU: 12, RAMGB: 32, GPU: "g"})
	if err != nil || id != "test01" {
		t.Fatalf("register = %q, %v", id, err)
	}
	first, err := c.Poll(ctx, id)
	if err != nil || first.Status != "pending" {
		t.Fatalf("first poll = %#v, %v", first, err)
	}
	res, err := c.WaitApproval(ctx, id, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.K3sToken == "" || res.NodeName != "ldndrc-test01" {
		t.Fatalf("approval missing token payload: %#v", res)
	}
	if strings.Contains(res.K3sToken, "\n") {
		t.Fatal("token must be a single line")
	}
}

func TestDiagnosticsRoundTrip(t *testing.T) {
	srv := fakeNodes(t)
	defer srv.Close()
	c := NewClient(srv.URL)
	b := Bundle{Hostname: "h", OS: "linux", Checks: []string{"cpu ok"}, Steps: []string{"registered"}}
	path, err := b.WriteLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "ldndrc-diagnostics-") {
		t.Fatalf("bundle path = %s", path)
	}
	if err := c.MaybeUpload(context.Background(), "test01", b.Render(), true); err != nil {
		t.Fatal(err)
	}
	// network unhealthy → no upload, no error
	if err := c.MaybeUpload(context.Background(), "test01", b.Render(), false); err != nil {
		t.Fatal(err)
	}
}
