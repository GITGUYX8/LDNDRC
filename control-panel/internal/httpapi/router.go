// Package httpapi wires the HTTP routes for the control panel: health, login,
// and (when the client-go session store is wired) the session CRUD API.
package httpapi

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/nodes"
	"github.com/ldndrc/control-panel/internal/sessions"
)

// sessionStore is the seam for the Kubernetes-backed session manager.
// The client-go implementation lives in internal/sessions and is wired in
// once the cluster manifests are finalised.
type sessionStore interface {
	Create(username string) (sessions.Session, error)
	List(username string) []sessions.Session
	Get(username, id string) (sessions.Session, error)
	Delete(username, id string) (sessions.Session, error)
}

// NewRouter builds the root handler. authSvc provides JWT issuing/verifying;
// store, when non-nil, enables the session endpoints; nodeStore, when
// non-nil, enables the Host-onboarding node endpoints (minter may be nil
// outside the cluster, in which case approved polls report unavailable).
func NewRouter(authSvc *auth.Service, store ...sessionStore) http.Handler {
	return NewRouterWithNodes(authSvc, firstStore(store), nil, nil)
}

func firstStore(store []sessionStore) sessionStore {
	if len(store) > 0 {
		return store[0]
	}
	return nil
}

// NewRouterWithNodes wires sessions plus the nodes onboarding API.
func NewRouterWithNodes(authSvc *auth.Service, store sessionStore, nodeStore *nodes.Store, minter nodes.Minter) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthHandler)

	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		// TODO(auth): replace with a real credential check (htpasswd, OIDC,
		// or a small user table). For now any non-empty password logs in.
		if req.Username == "" || req.Password == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "username and password required"})
			return
		}
		token, err := authSvc.Issue(req.Username)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token issue failed"})
			return
		}
		authSvc.SetSessionCookie(w, token)
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	})

	if store != nil {
		mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
			claims, ok := bearerClaims(r, authSvc)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			session, err := store.Create(claims.Username)
			if err != nil {
				if err == sessions.ErrSessionExists {
					writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
					return
				}
				log.Printf("provision failed for %s: %v", claims.Username, err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "provision failed"})
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]any{"session": session})
		})

		mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
			claims, ok := bearerClaims(r, authSvc)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"sessions": store.List(claims.Username)})
		})

		mux.HandleFunc("GET /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
			claims, ok := bearerClaims(r, authSvc)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			session, err := store.Get(claims.Username, r.PathValue("id"))
			if err != nil {
				writeSessionError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"session": session})
		})

		mux.HandleFunc("DELETE /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
			claims, ok := bearerClaims(r, authSvc)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			session, err := store.Delete(claims.Username, r.PathValue("id"))
			if err != nil {
				writeSessionError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"session": session})
		})
	}

	if nodeStore != nil {
		limiter := newRateLimiter(10, time.Minute)
		joinKey := os.Getenv("JOIN_KEY")
		autoApprove := os.Getenv("AUTO_APPROVE") == "true"
		joinURL := os.Getenv("K3S_JOIN_URL")

		mux.HandleFunc("POST /api/nodes/register", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Hostname string  `json:"hostname"`
				OS       string  `json:"os"`
				Arch     string  `json:"arch"`
				CPU      int     `json:"cpu"`
				RAMGB    float64 `json:"ram_gb"`
				GPU      string  `json:"gpu"`
				JoinKey  string  `json:"join_key"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
				return
			}
			if joinKey != "" && req.JoinKey != joinKey {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid join key"})
				return
			}
			if !limiter.allow(clientIP(r)) {
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
				return
			}
			node, created, err := nodeStore.Register(req.Hostname, req.OS, req.Arch, req.CPU, req.RAMGB, req.GPU)
			if err != nil {
				if err == nodes.ErrInvalidInput {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				log.Printf("node register failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "registration failed"})
				return
			}
			if created && autoApprove {
				node, err = nodeStore.Approve(node.ID)
				if err != nil {
					log.Printf("node auto-approve failed: %v", err)
				}
			}
			status := http.StatusCreated
			if !created {
				status = http.StatusOK
			}
			writeJSON(w, status, map[string]any{"id": node.ID, "status": node.Status})
		})

		mux.HandleFunc("GET /api/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
			node, err := nodeStore.Get(r.PathValue("id"))
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown node id"})
				return
			}
			if changed, err := nodeStore.ExpireIfElapsed(node.ID, time.Now()); err == nil && changed {
				node.Status = nodes.StatusExpired
			}
			switch node.Status {
			case nodes.StatusApproved:
				if minter == nil {
					writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "join service unavailable"})
					return
				}
				if node.TokenID == "" {
					token, tokenID, err := minter.MintToken(r.Context(), node.NodeName, "ldndrc-"+node.ID)
					if err != nil {
						log.Printf("token mint failed for %s: %v", node.NodeName, err)
						writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token mint failed"})
						return
					}
					if err := nodeStore.MarkMinted(node.ID, tokenID); err != nil {
						log.Printf("mark minted failed for %s: %v", node.ID, err)
					}
					if joinURL == "" {
						writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "join URL not configured"})
						return
					}
					writeJSON(w, http.StatusOK, map[string]any{
						"status": node.Status, "k3s_url": joinURL,
						"k3s_token": token, "node_name": node.NodeName,
					})
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"status": node.Status})
			default:
				writeJSON(w, http.StatusOK, map[string]any{"status": node.Status, "reason": node.Reason})
			}
		})

		mux.HandleFunc("GET /api/nodes", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := bearerClaims(r, authSvc); !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"nodes": nodeStore.List()})
		})

		mux.HandleFunc("POST /api/nodes/{id}/approve", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := bearerClaims(r, authSvc); !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			node, err := nodeStore.Approve(r.PathValue("id"))
			if err != nil {
				writeNodeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"id": node.ID, "status": node.Status})
		})

		mux.HandleFunc("POST /api/nodes/{id}/deny", func(w http.ResponseWriter, r *http.Request) {
			if _, ok := bearerClaims(r, authSvc); !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			var req struct {
				Reason string `json:"reason"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			node, err := nodeStore.Deny(r.PathValue("id"), req.Reason)
			if err != nil {
				writeNodeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"id": node.ID, "status": node.Status})
		})

		// Diagnostics upload from the joining laptop. Auth is knowledge of
		// the node id (same model as the poll endpoint): bundles carry no
		// secrets by construction, and the id is unguessable enough for LAN.
		mux.HandleFunc("POST /api/nodes/{id}/diagnostics", func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, nodes.MaxDiagnosticsBytes)
			if err := nodeStore.SaveDiagnostics(r.PathValue("id"), r.Body); err != nil {
				if err == nodes.ErrNotFound {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
					return
				}
				if strings.Contains(err.Error(), "request body too large") {
					writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "bundle exceeds 1MB"})
					return
				}
				log.Printf("diagnostics save failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save failed"})
				return
			}
			writeJSON(w, http.StatusCreated, map[string]string{"status": "received"})
		})
	}

	return logRequests(mux)
}

func writeNodeError(w http.ResponseWriter, err error) {
	if err == nodes.ErrNotFound {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	} else {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	}
}

// rateLimiter is a tiny per-IP sliding-window limiter for the public
// register endpoint (LAN spam control, not DDoS protection).
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: make(map[string][]time.Time), limit: limit, window: window}
}

func (l *rateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	kept := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.hits[ip] = kept
		return false
	}
	l.hits[ip] = append(kept, now)
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeSessionError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if err == sessions.ErrNotFound {
		status = http.StatusNotFound
	} else if err == sessions.ErrForbidden {
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// bearerClaims extracts and verifies the Authorization: Bearer token.
func bearerClaims(r *http.Request, authSvc *auth.Service) (*auth.Claims, bool) {
	h := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(h, "Bearer ")
	if !ok || token == "" {
		return nil, false
	}
	claims, err := authSvc.Verify(token)
	if err != nil {
		return nil, false
	}
	return claims, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write json: %v", err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
