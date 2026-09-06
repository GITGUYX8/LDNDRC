// Package gateway provides authenticated host-based routing to session tools.
package gateway

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/ldndrc/control-panel/internal/auth"
	"github.com/ldndrc/control-panel/internal/sessions"
)

type sessionLookup interface {
	Current(username string) (sessions.Session, error)
}

type sessionRoute struct {
	port int
	tool string
}

// NewSessionHandler returns the browser gateway used by editor, desktop, and
// Gazebo hostnames. Authentication happens before any upstream is selected.
func NewSessionHandler(authService *auth.Service, store sessionLookup, next http.Handler) http.Handler {
	suffix := strings.ToLower(envOr("GATEWAY_HOST_SUFFIX", "ros-platform.local"))
	routes := map[string]sessionRoute{
		"editor":  {tool: "editor", port: 7682},
		"desktop": {tool: "desktop", port: 8080},
		"gazebo":  {tool: "gazebo", port: 9002},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(strings.Split(r.Host, ":")[0])
		parts := strings.Split(host, ".")
		if len(parts) < 2 || !strings.HasSuffix(host, "."+suffix) {
			next.ServeHTTP(w, r)
			return
		}
		route, ok := routes[parts[0]]
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := authService.VerifyCookie(r)
		if err != nil {
			writeGatewayError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		session, err := store.Current(claims.Username)
		if err != nil {
			if errors.Is(err, sessions.ErrNotFound) {
				writeGatewayError(w, http.StatusNotFound, "no active workspace session")
				return
			}
			writeGatewayError(w, http.StatusInternalServerError, "session lookup failed")
			return
		}
		if session.Status != sessions.StatusReady {
			writeGatewayError(w, http.StatusServiceUnavailable, "workspace session is not ready")
			return
		}

		target, err := url.Parse("http://" + session.ServiceName + "." + envOr("SESSION_NAMESPACE", "ldndrc") + ".svc.cluster.local:" + strconv.Itoa(route.port))
		if err != nil {
			writeGatewayError(w, http.StatusInternalServerError, "invalid session target")
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(response http.ResponseWriter, _ *http.Request, _ error) {
			writeGatewayError(response, http.StatusBadGateway, "workspace service unavailable")
		}
		proxy.ServeHTTP(w, r)
	})
}

func writeGatewayError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
