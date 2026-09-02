package sessions

import (
	"errors"
	"testing"
)

func TestStoreEnforcesOneActiveSessionPerUser(t *testing.T) {
	store := NewStore()
	first, err := store.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create("alice"); !errors.Is(err, ErrSessionExists) {
		t.Fatalf("expected duplicate session error, got %v", err)
	}
	if got := store.List("alice"); len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("unexpected user session list: %#v", got)
	}
}

func TestStorePreventsCrossUserAccess(t *testing.T) {
	store := NewStore()
	session, err := store.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("bob", session.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestStoreStatusTransitions(t *testing.T) {
	store := NewStore()
	session, err := store.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	if session, err = store.SetStatus("alice", session.ID, StatusReady, ""); err != nil {
		t.Fatal(err)
	}
	if session, err = store.SetStatus("alice", session.ID, StatusStopping, ""); err != nil {
		t.Fatal(err)
	}
	if session, err = store.SetStatus("alice", session.ID, StatusStopped, ""); err != nil {
		t.Fatal(err)
	}
	if session.Status != StatusStopped {
		t.Fatalf("expected stopped, got %s", session.Status)
	}
	if _, err := store.SetStatus("alice", session.ID, StatusReady, ""); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected invalid transition error, got %v", err)
	}
}
