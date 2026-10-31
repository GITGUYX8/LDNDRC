// Package portal hosts the LAN onboarding captive portal inside the
// control panel binary: a welcome page, the enrollment scripts
// (joining-cluster.sh / joining-cluster.ps1) with a live download
// counter, and the counter APIs. Every portal route is reachable
// BEFORE device approval — no cookies, JWT, or CSRF involved — so a
// freshly connected laptop can download the enrollment script while
// still untrusted.
//
// Counting semantics (deliberate, and documented on the portal itself):
//
//   - "requests" counts every artifact endpoint hit: HEAD probes,
//     Range/partial fetches, browser pre-fetches, retries, aborted
//     transfers. It answers "how often was the file asked for".
//   - "downloads" counts only full 200 GET responses whose entire body
//     reached the client. This proves the server SERVED the file — it
//     says nothing about whether the user saved or executed it.
//   - "devices" is a distinct-downloader estimate: a hash of (client
//     IP, User-Agent) under a persisted random salt. IP and UA are
//     spoofable and are NOT identity; the number is directional only.
//     Raw IP/UA strings are never stored or exposed.
//
// Persistence mirrors internal/nodes: a JSON file guarded by a mutex,
// written with atomic tmp+rename after every mutation, so counts stay
// accurate across server restarts and concurrent downloads. Same
// storage caveat as NODES_DB/SESSIONS_DB: survives container restarts,
// not pod redeploys (emptyDir).
package portal

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Enrollment script artifact names. Exact filenames are part of the
// feature contract (portal links and docs reference them verbatim).
const (
	ArtifactSh  = "joining-cluster.sh"
	ArtifactPs1 = "joining-cluster.ps1"
)

// ErrUnknownArtifact guards against counting anything but the two
// enrollment scripts.
var ErrUnknownArtifact = errors.New("unknown portal artifact")

// artifactCounts is the persisted per-artifact state.
type artifactCounts struct {
	Requests  int64            `json:"requests"`
	Downloads int64            `json:"downloads"`
	Devices   map[string]int64 `json:"devices,omitempty"` // device hash -> downloads
}

// data is the on-disk shape of the downloads DB.
type data struct {
	Salt      string                     `json:"salt"` // random, persisted: keeps device hashes stable across restarts
	Artifacts map[string]*artifactCounts `json:"artifacts"`
}

// Store is the download counter. Safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	path string
	data data
}

// NewStore loads the downloads DB from path, starting empty when the
// file is absent (same convention as nodes.NewStore).
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, data: data{Artifacts: map[string]*artifactCounts{}}}
	if s.data.Artifacts == nil {
		s.data.Artifacts = map[string]*artifactCounts{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read downloads db: %w", err)
		}
	} else if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, fmt.Errorf("parse downloads db: %w", err)
	}
	if s.data.Salt == "" {
		if err := s.ensureSalt(); err != nil {
			return nil, err
		}
	}
	if s.data.Artifacts == nil {
		s.data.Artifacts = map[string]*artifactCounts{}
	}
	// Both artifacts always exist in the snapshot, even at zero — the
	// portal page and count endpoint reference both names up front.
	for _, name := range []string{ArtifactSh, ArtifactPs1} {
		if s.data.Artifacts[name] == nil {
			s.data.Artifacts[name] = &artifactCounts{}
		}
		if s.data.Artifacts[name].Devices == nil {
			s.data.Artifacts[name].Devices = map[string]int64{}
		}
	}
	return s, nil
}

func (s *Store) ensureSalt() error {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Errorf("downloads db salt: %w", err)
	}
	s.data.Salt = hex.EncodeToString(b[:])
	return nil
}

// RecordRequest counts one artifact endpoint hit (any method, any
// range, success or abort). Best-effort callers may ignore the error.
func (s *Store) RecordRequest(artifact string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.artifactLocked(artifact)
	if err != nil {
		return err
	}
	a.Requests++
	return s.saveLocked()
}

// RecordDownload counts one full-body served download and attributes
// it to deviceHash. Returns the artifact's new download total.
func (s *Store) RecordDownload(artifact, deviceHash string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.artifactLocked(artifact)
	if err != nil {
		return 0, err
	}
	a.Downloads++
	if deviceHash != "" {
		if a.Devices == nil {
			a.Devices = map[string]int64{}
		}
		a.Devices[deviceHash]++
	}
	return a.Downloads, s.saveLocked()
}

func (s *Store) artifactLocked(artifact string) (*artifactCounts, error) {
	a, ok := s.data.Artifacts[artifact]
	if !ok {
		return nil, ErrUnknownArtifact
	}
	return a, nil
}

// PublicCounts is the pre-approval public view: aggregate served
// downloads only. No request counts, no device data.
type PublicCounts struct {
	Artifacts map[string]int64 `json:"artifacts"`
	Total     int64            `json:"total"`
}

// PublicCounts snapshots the download numbers for the portal page and
// its polling endpoint.
func (s *Store) PublicCounts() PublicCounts {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := PublicCounts{Artifacts: make(map[string]int64, len(s.data.Artifacts))}
	for name, a := range s.data.Artifacts {
		out.Artifacts[name] = a.Downloads
		out.Total += a.Downloads
	}
	return out
}

// ArtifactStats is the authenticated per-artifact view.
type ArtifactStats struct {
	Requests  int64 `json:"requests"`
	Downloads int64 `json:"downloads"`
	Devices   int64 `json:"devices"` // distinct hashed downloaders
}

// Stats is the JWT-gated view for the admin console: it adds request
// counts and the unique-device estimate. Still aggregate — no hashes,
// no IPs, no user agents.
type Stats struct {
	Artifacts     map[string]ArtifactStats `json:"artifacts"`
	Total         int64                    `json:"total"`
	UniqueDevices int64                    `json:"unique_devices"`
}

// Stats snapshots everything the admin dashboard may see.
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := Stats{Artifacts: make(map[string]ArtifactStats, len(s.data.Artifacts))}
	seen := make(map[string]struct{})
	for name, a := range s.data.Artifacts {
		as := ArtifactStats{Requests: a.Requests, Downloads: a.Downloads, Devices: int64(len(a.Devices))}
		out.Artifacts[name] = as
		out.Total += a.Downloads
		for hash := range a.Devices {
			seen[hash] = struct{}{}
		}
	}
	out.UniqueDevices = int64(len(seen))
	return out
}

// HashDevice folds the client IP and User-Agent into the persisted
// salt. The salt never leaves the server, so the hash is not
// reversible off-box and identical values only link downloads made
// from the same (spoofable) source profile.
func (s *Store) HashDevice(ip, userAgent string) string {
	s.mu.RLock()
	salt := s.data.Salt
	s.mu.RUnlock()
	sum := sha256.Sum256([]byte(salt + "\x00" + ip + "\x00" + userAgent))
	return hex.EncodeToString(sum[:])
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
