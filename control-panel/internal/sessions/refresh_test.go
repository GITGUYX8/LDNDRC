package sessions

import "testing"

type readyProvisioner struct{ ready bool }

func (p readyProvisioner) Apply(Session) error         { return nil }
func (p readyProvisioner) Delete(Session) error        { return nil }
func (p readyProvisioner) Ready(Session) (bool, error) { return p.ready, nil }

func TestGetRefreshesProvisioningToReady(t *testing.T) {
	s := NewStoreWithProvisioner(readyProvisioner{ready: true})
	created, err := s.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("alice", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusReady {
		t.Fatalf("Get status = %s, want ready (launch stepper polls this)", got.Status)
	}
}

func TestGetStaysProvisioningWhenNotReady(t *testing.T) {
	s := NewStoreWithProvisioner(readyProvisioner{ready: false})
	created, err := s.Create("alice")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("alice", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusProvisioning {
		t.Fatalf("Get status = %s, want provisioning", got.Status)
	}
}
