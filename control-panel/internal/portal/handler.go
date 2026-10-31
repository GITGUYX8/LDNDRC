// Handler serves the pre-approval portal: the welcome page, the
// enrollment script downloads (with request-vs-served counting), and
// the counter APIs. Routes are wired in internal/httpapi/router.go.
package portal

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"time"
)

//go:embed assets/portal.html assets/joining-cluster.sh assets/joining-cluster.ps1
var assetsFS embed.FS

// Handler bundles the portal store with the embedded assets. All
// methods are safe for concurrent use.
type Handler struct {
	store   *Store
	page    []byte
	scripts map[string][]byte
	modTime time.Time // fixed at boot; drives Last-Modified/304 revalidation
}

// NewHandler wires the store to the embedded assets.
func NewHandler(store *Store) *Handler {
	page, err := fs.ReadFile(assetsFS, "assets/portal.html")
	if err != nil {
		// Embed paths are compile-time checked; this is unreachable
		// unless the asset tree is renamed. Fail loudly in that case.
		log.Fatalf("portal: embedded page missing: %v", err)
	}
	scripts := make(map[string][]byte, 2)
	for _, name := range []string{ArtifactSh, ArtifactPs1} {
		body, err := fs.ReadFile(assetsFS, "assets/"+name)
		if err != nil {
			log.Fatalf("portal: embedded script %s missing: %v", name, err)
		}
		scripts[name] = body
	}
	return &Handler{
		store:   store,
		page:    page,
		scripts: scripts,
		modTime: time.Now(),
	}
}

// Store exposes the counter for tests and the admin wiring.
func (h *Handler) Store() *Store { return h.store }

// Page serves the captive-portal welcome page at GET /portal (and
// /portal/). no-store so the counter page itself is never cached.
func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(h.page); err != nil {
		log.Printf("portal page write: %v", err)
	}
}

// DownloadHandler serves one enrollment script. Counting rule: every
// hit bumps "requests"; a full 200 GET whose entire body was written
// additionally bumps "downloads". HEAD, Range/partial, conditional
// 304, and aborted transfers stay request-only — a retry or preview
// must not inflate the served-download counter.
func (h *Handler) DownloadHandler(artifact string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := h.scripts[artifact]
		if !ok {
			http.Error(w, "unknown artifact", http.StatusNotFound)
			return
		}
		if err := h.store.RecordRequest(artifact); err != nil {
			log.Printf("portal: record request %s: %v", artifact, err)
		}
		switch artifact {
		case ArtifactSh:
			w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		case ArtifactPs1:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+artifact+`"`)
		w.Header().Set("X-Content-Type-Options", "nosniff")

		cw := &countWriter{ResponseWriter: w}
		http.ServeContent(cw, r, artifact, h.modTime, bytes.NewReader(body))

		served := r.Method == http.MethodGet &&
			r.Header.Get("Range") == "" &&
			cw.err == nil &&
			cw.n == int64(len(body))
		if served {
			hash := h.store.HashDevice(clientIP(r), r.Header.Get("User-Agent"))
			if _, err := h.store.RecordDownload(artifact, hash); err != nil {
				log.Printf("portal: record download %s: %v", artifact, err)
			}
		}
	}
}

// PublicCount serves GET /api/downloads/count: aggregate served
// downloads only, for the portal page's live counter.
func (h *Handler) PublicCount(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.PublicCounts())
}

// Stats serves GET /api/downloads/stats. The route is JWT-gated in
// router.go (same bearer model as the nodes admin routes); the body
// adds request counts and the unique-device estimate.
func (h *Handler) Stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Stats())
}

// countWriter records how many bytes actually left the server and the
// first write error, so "served" means the full body reached the wire.
type countWriter struct {
	http.ResponseWriter
	n   int64
	err error
}

func (cw *countWriter) Write(p []byte) (int, error) {
	n, err := cw.ResponseWriter.Write(p)
	cw.n += int64(n)
	if err != nil && cw.err == nil {
		cw.err = err
	}
	return n, err
}

// clientIP is the plain remote address (no proxy headers — the only
// expected hop is the AP/gateway on the same L2 as the master).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("portal: write json: %v", err)
	}
}
