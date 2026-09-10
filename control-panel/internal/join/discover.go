package join

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/hashicorp/mdns"
)

// Master is a discovered (or configured) control panel endpoint.
type Master struct {
	Host string
	Port int
}

// Addr returns host:port for HTTP calls.
func (m Master) Addr() string {
	return fmt.Sprintf("%s:%d", m.Host, m.Port)
}

// DiscoverMaster finds the master via mDNS, falling back to MASTER_IP env
// or the -master flag value. mDNS absence never errors — it just yields
// the fallback (managed Wi-Fi often blocks multicast).
func DiscoverMaster(flagMaster string, port int) Master {
	if flagMaster != "" {
		return Master{Host: flagMaster, Port: port}
	}
	if env := os.Getenv("MASTER_IP"); env != "" {
		return Master{Host: env, Port: port}
	}
	if host, ok := browseMDNS(port); ok {
		return Master{Host: host, Port: port}
	}
	return Master{Port: port}
}

func browseMDNS(port int) (string, bool) {
	entries := make(chan *mdns.ServiceEntry, 4)
	done := make(chan string, 1)
	go func() {
		for e := range entries {
			if e.Port == port || port == 0 {
				done <- e.Host
				return
			}
		}
	}()
	params := mdns.DefaultParams("_ldndrc-master._tcp")
	params.Timeout = 3 * time.Second
	params.Entries = entries
	if err := mdns.Query(params); err != nil {
		return "", false
	}
	close(entries)
	select {
	case host := <-done:
		return host, true
	default:
		return "", false
	}
}

// ProbeMaster dials master:6443 with a short timeout for the reachability
// gate and the fix card's ufw guidance.
func ProbeMaster(host string) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "6443"), 5*time.Second)
	if err != nil {
		return err
	}
	return conn.Close()
}
