package join

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type scriptServer struct {
	t        *testing.T
	mu       sync.Mutex
	polls    int
	uploaded [][]byte
	statuses []string
}

func (s *scriptServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/nodes/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "run01", "status": "pending"})
	})
	mux.HandleFunc("/api/nodes/run01", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.polls++
		n := s.polls
		s.mu.Unlock()
		switch {
		case n <= 2:
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
		case n == 3:
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "approved", "k3s_url": "https://master:6443",
				"k3s_token": "K10hash::tok.secret", "node_name": "ldndrc-run01",
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "joined"})
		}
	})
	mux.HandleFunc("/api/nodes/run01/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		var buf strings.Builder
		b := make([]byte, 1<<20+1)
		rd, _ := r.Body.Read(b)
		buf.Write(b[:rd])
		s.mu.Lock()
		s.uploaded = append(s.uploaded, []byte(buf.String()))
		s.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})
	return mux
}

func runCfg(t *testing.T, c *Client) RunConfig {
	t.Helper()
	var emitted []string
	return RunConfig{
		Client: c, Hostname: "h", OS: "linux", Arch: "amd64",
		HW:          Hardware{OS: "linux", CPU: 12, RAMGB: 32, GPU: "g"},
		Master:      Master{Host: "master", Port: 8082},
		ControlURL:  "http://master:8082",
		NeedToolkit: true,
		ToolkitFn:   func() error { emitted = append(emitted, "toolkit"); return nil },
		AgentFn: func(url, token, node string) error {
			emitted = append(emitted, "agent:"+node)
			if strings.Contains(url+node, token) && token != "" {
				// token must travel, but never into logs — checked below
			}
			return nil
		},
		Timeout:   30 * time.Second,
		Emit:      func(s string) { emitted = append(emitted, s) },
		BundleDir: t.TempDir(),
	}
}

func TestRunJoinHappyPath(t *testing.T) {
	ss := &scriptServer{}
	srv := httptest.NewServer(ss.handler())
	defer srv.Close()
	c := NewClient(srv.URL)
	cfg := runCfg(t, c)

	msg, err := RunJoin(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "joined!") {
		t.Fatalf("final message = %q", msg)
	}
	if len(ss.uploaded) != 1 {
		t.Fatalf("uploads = %d, want 1 (network healthy)", len(ss.uploaded))
	}
	for _, leak := range []string{"K10hash", "tok.secret"} {
		if strings.Contains(string(ss.uploaded[0]), leak) {
			t.Fatalf("bundle leaks token material %q", leak)
		}
		if strings.Contains(msg, leak) {
			t.Fatalf("final message leaks token material %q", leak)
		}
	}
}

func TestRunJoinDenied(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/nodes/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "d1", "status": "pending"})
	})
	mux.HandleFunc("/api/nodes/d1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "denied", "reason": "nope"})
	})
	mux.HandleFunc("/api/nodes/d1/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cfg := runCfg(t, NewClient(srv.URL))
	if _, err := RunJoin(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected denial error, got %v", err)
	}
}

func TestRunJoinApprovalTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/nodes/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "t1", "status": "pending"})
	})
	mux.HandleFunc("/api/nodes/t1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "pending"})
	})
	mux.HandleFunc("/api/nodes/t1/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cfg := runCfg(t, NewClient(srv.URL))
	cfg.Timeout = 60 * time.Millisecond
	if _, err := RunJoin(context.Background(), cfg); err == nil {
		t.Fatal("expected timeout error")
	}
}
