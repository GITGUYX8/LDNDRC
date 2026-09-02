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

type fakeProvisioner struct {
	applyCalls  []Session
	deleteCalls []Session
	err         error
}

func (f *fakeProvisioner) Apply(session Session) error {
	f.applyCalls = append(f.applyCalls, session)
	return f.err
}

func (f *fakeProvisioner) Delete(session Session) error {
	f.deleteCalls = append(f.deleteCalls, session)
	return f.err
}

func TestStoreDelegatesProvisioningAndDeletion(t *testing.T) {
	provisioner := &fakeProvisioner{}
	store := NewStoreWithProvisioner(provisioner)
	session, err := store.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(provisioner.applyCalls) != 1 || provisioner.applyCalls[0].ID != session.ID {
		t.Fatalf("unexpected apply calls: %#v", provisioner.applyCalls)
	}
	if _, err := store.Delete("alice", session.ID); err != nil {
		t.Fatal(err)
	}
	if len(provisioner.deleteCalls) != 1 || provisioner.deleteCalls[0].ID != session.ID {
		t.Fatalf("unexpected delete calls: %#v", provisioner.deleteCalls)
	}
}

func TestStoreMarksProvisioningFailure(t *testing.T) {
	provisioner := &fakeProvisioner{err: errors.New("cluster unavailable")}
	store := NewStoreWithProvisioner(provisioner)
	if _, err := store.Create("alice"); err == nil {
		t.Fatal("expected provisioning error")
	}
	session := store.List("alice")[0]
	if session.Status != StatusError || session.Error != "cluster unavailable" {
		t.Fatalf("unexpected failed session: %#v", session)
	}
}
