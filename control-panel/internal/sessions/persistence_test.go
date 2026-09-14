package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorePersistsAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	s, err := NewStoreWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetStatus("alice", created.ID, StatusReady, ""); err != nil {
		t.Fatal(err)
	}

	loaded, err := NewStoreWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := loaded.Get("alice", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusReady || got.RosDomainID != created.RosDomainID {
		t.Fatalf("reloaded = %+v, want ready with domain %d", got, created.RosDomainID)
	}
	if _, err := loaded.Current("alice"); err != nil {
		t.Fatalf("Current after reload: %v", err)
	}
}

func TestStoreCorruptFileFailsFast(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStoreWithPath(path); err == nil {
		t.Fatal("corrupt db must fail, never start empty")
	}
}

func TestReconcileAdoptsAndTombstones(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	s, err := NewStoreWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	keep, err := s.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	gone, err := s.Create("bob")
	if err != nil {
		t.Fatal(err)
	}

	adopted, tombstoned := s.Reconcile(func(sess Session) bool {
		return sess.ID == keep.ID
	})
	if adopted != 1 || tombstoned != 1 {
		t.Fatalf("reconcile = %d/%d, want 1/1", adopted, tombstoned)
	}
	if _, err := s.Get("alice", keep.ID); err != nil {
		t.Fatalf("adopted session must survive: %v", err)
	}
	tomb, err := s.Get("bob", gone.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tomb.Status != StatusStopped {
		t.Fatalf("missing workload status = %s, want stopped", tomb.Status)
	}
	// Tombstone itself persists: reload still shows stopped, not resurrected.
	loaded, err := NewStoreWithPath(path)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := loaded.Get("bob", gone.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != StatusStopped {
		t.Fatalf("reloaded tombstone = %s, want stopped", reloaded.Status)
	}
}

func TestReconcileSkipsTerminal(t *testing.T) {
	s := NewStore()
	created, err := s.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete("alice", created.ID); err != nil {
		t.Fatal(err)
	}
	adopted, tombstoned := s.Reconcile(func(Session) bool { return false })
	if adopted != 0 || tombstoned != 0 {
		t.Fatalf("terminal sessions must be ignored, got %d/%d", adopted, tombstoned)
	}
}
