package portal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler(t *testing.T) (*Handler, *Store) {
	t.Helper()
	store, _ := newTestStore(t)
	return NewHandler(store), store
}

func serveDownload(t *testing.T, h *Handler, method, path, rangeHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	rec := httptest.NewRecorder()
	h.DownloadHandler(ArtifactSh)(rec, req)
	return rec
}

func TestPageServesHTML(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/portal", nil)
	rec := httptest.NewRecorder()
	h.Page(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{"joining-cluster.sh", "joining-cluster.ps1", "/api/downloads/count"} {
		if !strings.Contains(body, want) {
			t.Fatalf("portal page missing %q", want)
		}
	}
}

func TestDownloadServedCountsOnce(t *testing.T) {
	h, store := newTestHandler(t)
	rec := serveDownload(t, h, http.MethodGet, "/portal/joining-cluster.sh", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/x-shellscript") {
		t.Fatalf("content-type = %q, want text/x-shellscript", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, ArtifactSh) {
		t.Fatalf("content-disposition = %q, want attachment for %s", cd, ArtifactSh)
	}
	if !strings.HasPrefix(rec.Body.String(), "#!/") {
		t.Fatal("served body is not a shell script")
	}
	pc := store.PublicCounts()
	if pc.Artifacts[ArtifactSh] != 1 {
		t.Fatalf("downloads = %d, want 1 (full 200 GET)", pc.Artifacts[ArtifactSh])
	}
	stats := store.Stats()
	if stats.Artifacts[ArtifactSh].Requests != 1 {
		t.Fatalf("requests = %d, want 1 (the same hit is also a request)", stats.Artifacts[ArtifactSh].Requests)
	}
}

func TestHEADAndPartialRangeAreRequestsOnly(t *testing.T) {
	cases := []struct {
		name string
		run  func(h *Handler) *httptest.ResponseRecorder
	}{
		{"HEAD", func(h *Handler) *httptest.ResponseRecorder {
			return serveDownload(t, h, http.MethodHead, "/portal/joining-cluster.sh", "")
		}},
		{"RangeFull", func(h *Handler) *httptest.ResponseRecorder {
			return serveDownload(t, h, http.MethodGet, "/portal/joining-cluster.sh", "bytes=0-")
		}},
		{"RangePartial", func(h *Handler) *httptest.ResponseRecorder {
			return serveDownload(t, h, http.MethodGet, "/portal/joining-cluster.sh", "bytes=0-99")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, store := newTestHandler(t)
			rec := tc.run(h)
			if rec.Code != http.StatusOK && rec.Code != http.StatusPartialContent {
				t.Fatalf("status = %d", rec.Code)
			}
			stats := store.Stats()
			a := stats.Artifacts[ArtifactSh]
			if a.Downloads != 0 {
				t.Fatalf("downloads = %d, want 0 — only full 200 GETs count", a.Downloads)
			}
			if a.Requests != 1 {
				t.Fatalf("requests = %d, want 1", a.Requests)
			}
		})
	}
}

func TestPublicCountShape(t *testing.T) {
	h, store := newTestHandler(t)
	if _, err := store.RecordDownload(ArtifactSh, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordDownload(ArtifactPs1, ""); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/downloads/count", nil)
	rec := httptest.NewRecorder()
	h.PublicCount(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var pc PublicCounts
	if err := json.Unmarshal(rec.Body.Bytes(), &pc); err != nil {
		t.Fatal(err)
	}
	if pc.Total != 2 || pc.Artifacts[ArtifactSh] != 1 || pc.Artifacts[ArtifactPs1] != 1 {
		t.Fatalf("public counts = %+v", pc)
	}
	raw := rec.Body.String()
	// Public endpoint must not leak request counts or device data.
	for _, leaked := range []string{"requests", "devices", "unique"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("public count leaks %q: %s", leaked, raw)
		}
	}
}

func TestStatsShape(t *testing.T) {
	h, store := newTestHandler(t)
	hash := store.HashDevice("192.168.4.5", "test-agent")
	if _, err := store.RecordDownload(ArtifactSh, hash); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/downloads/stats", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var stats Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Total != 1 || stats.UniqueDevices != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	a := stats.Artifacts[ArtifactSh]
	if a.Requests != 0 || a.Downloads != 1 || a.Devices != 1 {
		t.Fatalf("artifact stats = %+v", a)
	}
	if strings.Contains(rec.Body.String(), hash) {
		t.Fatal("stats must not expose device hashes")
	}
}
