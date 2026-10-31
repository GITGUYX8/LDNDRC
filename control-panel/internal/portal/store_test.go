package portal

import (
	"path/filepath"
	"sync"
	"testing"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "downloads.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestPublicCountsStartsAtZero(t *testing.T) {
	s, _ := newTestStore(t)
	pc := s.PublicCounts()
	if pc.Total != 0 {
		t.Fatalf("fresh total = %d, want 0", pc.Total)
	}
	for _, name := range []string{ArtifactSh, ArtifactPs1} {
		if pc.Artifacts[name] != 0 {
			t.Fatalf("fresh %s = %d, want 0", name, pc.Artifacts[name])
		}
	}
}

func TestRecordDownloadPersistsAcrossReload(t *testing.T) {
	s, path := newTestStore(t)
	hash := s.HashDevice("192.168.4.20", "curl/8.5.0")
	if _, err := s.RecordDownload(ArtifactSh, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordDownload(ArtifactPs1, hash); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	pc := reloaded.PublicCounts()
	if pc.Artifacts[ArtifactSh] != 1 || pc.Artifacts[ArtifactPs1] != 1 {
		t.Fatalf("after reload: %v — counts must survive restarts", pc.Artifacts)
	}
	if pc.Total != 2 {
		t.Fatalf("total = %d, want 2", pc.Total)
	}
	// The salt persists too: the same device hashes identically after a
	// restart, otherwise the unique-device metric would inflate.
	if reloaded.HashDevice("192.168.4.20", "curl/8.5.0") != hash {
		t.Fatal("device hash changed across reload — salt not persisted")
	}
}

func TestRecordUnknownArtifact(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.RecordRequest("other.sh"); err != ErrUnknownArtifact {
		t.Fatalf("RecordRequest err = %v, want ErrUnknownArtifact", err)
	}
	if _, err := s.RecordDownload("other.sh", ""); err != ErrUnknownArtifact {
		t.Fatalf("RecordDownload err = %v, want ErrUnknownArtifact", err)
	}
}

func TestConcurrentDownloadsAreExact(t *testing.T) {
	s, _ := newTestStore(t)
	const goroutines, perGoroutine = 8, 12
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if _, err := s.RecordDownload(ArtifactSh, ""); err != nil {
					t.Errorf("record download: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if got := s.PublicCounts().Artifacts[ArtifactSh]; got != goroutines*perGoroutine {
		t.Fatalf("concurrent total = %d, want %d (mutex must serialize increments)", got, goroutines*perGoroutine)
	}
}

func TestStatsUniqueDevices(t *testing.T) {
	s, _ := newTestStore(t)
	shA := s.HashDevice("192.168.4.10", "curl/8.5.0")
	shB := s.HashDevice("192.168.4.11", "curl/8.5.0")
	psA := s.HashDevice("192.168.4.10", "PowerShell/5.1") // same IP, different UA
	if _, err := s.RecordDownload(ArtifactSh, shA); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordDownload(ArtifactSh, shA); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordDownload(ArtifactSh, shB); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordDownload(ArtifactPs1, psA); err != nil {
		t.Fatal(err)
	}

	stats := s.Stats()
	sh := stats.Artifacts[ArtifactSh]
	if sh.Requests != 0 || sh.Downloads != 3 || sh.Devices != 2 {
		t.Fatalf("sh stats = %+v, want requests=0 downloads=3 devices=2", sh)
	}
	ps := stats.Artifacts[ArtifactPs1]
	if ps.Downloads != 1 || ps.Devices != 1 {
		t.Fatalf("ps1 stats = %+v, want downloads=1 devices=1", ps)
	}
	// Device A downloaded both artifacts: unique across the portal is 3
	// profiles (shA, shB, psA), not the per-artifact sum (4).
	if stats.UniqueDevices != 3 {
		t.Fatalf("unique_devices = %d, want 3", stats.UniqueDevices)
	}
	if stats.Total != 4 {
		t.Fatalf("total = %d, want 4", stats.Total)
	}
}

func TestHashDeviceDistinctInputs(t *testing.T) {
	s, _ := newTestStore(t)
	base := s.HashDevice("10.0.0.1", "agent")
	if again := s.HashDevice("10.0.0.1", "agent"); again != base {
		t.Fatal("same (ip, ua) must hash identically")
	}
	if otherIP := s.HashDevice("10.0.0.2", "agent"); otherIP == base {
		t.Fatal("different IP must hash differently")
	}
	if otherUA := s.HashDevice("10.0.0.1", "browser"); otherUA == base {
		t.Fatal("different UA must hash differently")
	}
}
