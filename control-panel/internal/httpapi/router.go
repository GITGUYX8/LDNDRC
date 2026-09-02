// Package httpapi wires the HTTP routes for the control panel: health, login,
// and (when the client-go session store is wired) the session CRUD API.
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/ldndrc/control-panel/internal/auth"
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
// store, when non-nil, enables the session endpoints.
func NewRouter(authSvc *auth.Service, store ...sessionStore) http.Handler {
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
		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	})

	if len(store) > 0 && store[0] != nil {
		mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
			claims, ok := bearerClaims(r, authSvc)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			session, err := store[0].Create(claims.Username)
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
			writeJSON(w, http.StatusOK, map[string]any{"sessions": store[0].List(claims.Username)})
		})

		mux.HandleFunc("GET /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
			claims, ok := bearerClaims(r, authSvc)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			session, err := store[0].Get(claims.Username, r.PathValue("id"))
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
			session, err := store[0].Delete(claims.Username, r.PathValue("id"))
			if err != nil {
				writeSessionError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"session": session})
		})
	}

	return logRequests(mux)
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
