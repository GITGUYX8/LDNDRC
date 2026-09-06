package nodes

import (
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "nodes.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRegisterIsIdempotent(t *testing.T) {
	s := testStore(t)
	first, created, err := s.Register("laptop-a", "linux", "amd64", 12, 32, "NVIDIA RTX 4060")
	if err != nil || !created {
		t.Fatalf("register = %v, %v, %v", first, created, err)
	}
	if first.Status != StatusPending || first.NodeName != "ldndrc-"+first.ID {
		t.Fatalf("unexpected record: %#v", first)
	}
	second, created, err := s.Register("laptop-a", "linux", "amd64", 12, 32, "NVIDIA RTX 4060")
	if err != nil || created || second.ID != first.ID {
		t.Fatalf("re-register = %v, %v, %v", second, created, err)
	}
}

func TestRegisterRejectsBadInput(t *testing.T) {
	s := testStore(t)
	for _, tc := range [][6]any{
		{"", "linux", "amd64", 12, 32.0, ""},
		{"laptop", "linux", "amd64", 0, 32.0, ""},
		{"laptop", "linux", "amd64", 12, 0.0, ""},
	} {
		_, _, err := s.Register(tc[0].(string), tc[1].(string), tc[2].(string), tc[3].(int), tc[4].(float64), tc[5].(string))
		if err == nil {
			t.Fatalf("expected error for %#v", tc)
		}
	}
}

func TestApproveDenyTransitions(t *testing.T) {
	s := testStore(t)
	n, _, _ := s.Register("laptop-a", "linux", "amd64", 12, 32, "")
	if _, err := s.Approve(n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(n.ID); err == nil {
		t.Fatal("double approve should fail")
	}
	m, _, _ := s.Register("laptop-b", "linux", "amd64", 12, 32, "")
	if _, err := s.Deny(m.ID, "unknown hardware"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(m.ID)
	if got.Status != StatusDenied || got.Reason != "unknown hardware" {
		t.Fatalf("unexpected denied record: %#v", got)
	}
}

func TestExpiryAndReapproval(t *testing.T) {
	s := testStore(t)
	n, _, _ := s.Register("laptop-a", "linux", "amd64", 12, 32, "")
	if _, err := s.Approve(n.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkMinted(n.ID, "abc123"); err != nil {
		t.Fatal(err)
	}
	changed, err := s.ExpireIfElapsed(n.ID, time.Now().Add(TokenTTL+time.Minute))
	if err != nil || !changed {
		t.Fatalf("expected expiry: %v %v", changed, err)
	}
	if _, err := s.Approve(n.ID); err != nil {
		t.Fatalf("re-approve after expiry: %v", err)
	}
	got, _ := s.Get(n.ID)
	if got.Status != StatusApproved || got.TokenID != "" {
		t.Fatalf("re-approve should reset mint: %#v", got)
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nodes.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	n, _, err := s.Register("laptop-a", "linux", "amd64", 12, 32, "")
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reloaded.Get(n.ID)
	if err != nil || got.Hostname != "laptop-a" {
		t.Fatalf("reload = %#v, %v", got, err)
	}
}
