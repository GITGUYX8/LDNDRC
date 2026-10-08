package httpapi

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	web "github.com/ldndrc/control-panel/web"
)

// SPAHandler serves the embedded student UI on the control host. API and
// gateway routes are matched first by the mux; everything else falls here:
// exact dist files when present, otherwise index.html so client-side routes
// (/login, /launch, /app) boot the SPA. Until UI-1 lands, dist holds a
// placeholder page.
func SPAHandler() http.Handler {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "ui not built"})
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		f, err := dist.Open(name)
		if err != nil {
			name = "index.html"
			f, err = dist.Open(name)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "ui not built"})
				return
			}
		}
		defer f.Close()
		switch {
		case strings.HasSuffix(name, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case strings.HasSuffix(name, ".js"):
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case strings.HasSuffix(name, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(name, ".json"):
			w.Header().Set("Content-Type", "application/json")
		case strings.HasSuffix(name, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
		}
		if _, err := io.Copy(w, f); err != nil {
			http.Error(w, "ui unavailable", http.StatusInternalServerError)
		}
	})
}
