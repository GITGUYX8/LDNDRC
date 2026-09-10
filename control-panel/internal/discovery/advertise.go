// Package discovery advertises the control panel on the LAN via mDNS so
// joining laptops find the master without typing IP addresses.
//
// A pod cannot do LAN multicast from its own network namespace — on real
// masters the Deployment runs with hostNetwork (see
// manifests/control-panel-hostnetwork-patch.yaml). Without it the
// advertiser starts, serves nothing reachable, and logs the fact.
package discovery

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/mdns"
)

// ServiceType is the mDNS service laptops browse for.
const ServiceType = "_ldndrc-master._tcp"

// Advertiser wraps an mDNS server with a shutdown handle.
type Advertiser struct {
	server *mdns.Server
	addr   string
}

// Start begins advertising instance on port with TXT records. Callers must
// call Shutdown. An error starting (e.g. no multicast route) is returned
// for the caller to log — discovery degrades to MASTER_IP fallback.
func Start(instance string, port int, txt []string) (*Advertiser, error) {
	if instance == "" {
		host, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("mdns instance name: %w", err)
		}
		instance = host
	}
	service, err := mdns.NewMDNSService(instance, ServiceType, "", "", port, nil, txt)
	if err != nil {
		return nil, fmt.Errorf("mdns service: %w", err)
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("mdns server: %w", err)
	}
	log.Printf("discovery: advertising %s on %s:%d", ServiceType, instance, port)
	return &Advertiser{server: server, addr: fmt.Sprintf("%s:%d", instance, port)}, nil
}

// Shutdown stops advertisement.
func (a *Advertiser) Shutdown() error {
	return a.server.Shutdown()
}

// TXTRecords returns the standard announcement payload.
func TXTRecords() []string {
	return []string{"cluster=ldndrc", "version=1", "path=/api/nodes/register"}
}
