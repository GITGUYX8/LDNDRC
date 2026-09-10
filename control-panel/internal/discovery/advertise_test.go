package discovery

import (
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

func TestTXTRecords(t *testing.T) {
	txt := TXTRecords()
	want := map[string]bool{"cluster=ldndrc": false, "version=1": false, "path=/api/nodes/register": false}
	for _, v := range txt {
		if _, ok := want[v]; ok {
			want[v] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Fatalf("missing TXT record %q in %v", k, txt)
		}
	}
}

func TestAdvertiseAndBrowse(t *testing.T) {
	adv, err := Start("test-master", 18082, TXTRecords())
	if err != nil {
		t.Skipf("multicast unavailable in this sandbox: %v", err)
	}
	defer adv.Shutdown()

	entries := make(chan *mdns.ServiceEntry, 4)
	params := mdns.DefaultParams(ServiceType)
	params.Timeout = 4 * time.Second
	params.Entries = entries
	go func() {
		_ = mdns.Query(params)
		close(entries)
	}()
	deadline := time.After(6 * time.Second)
	for {
		select {
		case e, ok := <-entries:
			if !ok {
				t.Fatal("browse finished without finding the advertisement")
			}
			if e.Port == 18082 {
				return // found ours
			}
		case <-deadline:
			t.Fatal("timed out waiting for mDNS advertisement")
		}
	}
}
