// Package nodes tracks Host-laptop join requests for zero-touch onboarding.
//
// A laptop registers itself (pending), an operator approves or denies it,
// and on approval the laptop polls until it receives a short-lived K3s
// bootstrap token. A watcher promotes nodes to joined once they appear
// Ready in the cluster. Tokens are never persisted — only their IDs.
package nodes

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
	"time"
)

// Status is the lifecycle state of a join request.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusDenied   Status = "denied"
	StatusJoined   Status = "joined"
	StatusExpired  Status = "expired"
)

// TokenTTL matches the design report: long enough to download the k3s
// installer, short enough to limit replay value.
const TokenTTL = 15 * time.Minute

// Node is one laptop's join request.
type Node struct {
	ID         string    `json:"id"`
	Hostname   string    `json:"hostname"`
	OS         string    `json:"os"`
	Arch       string    `json:"arch"`
	CPU        int       `json:"cpu"`
	RAMGB      float64   `json:"ram_gb"`
	GPU        string    `json:"gpu"`
	Status     Status    `json:"status"`
	NodeName   string    `json:"node_name"`
	Reason     string    `json:"reason,omitempty"`
	Requested  time.Time `json:"requested_at"`
	ApprovedAt time.Time `json:"approved_at,omitempty"`
	TokenID    string    `json:"token_id,omitempty"`
	MintedAt   time.Time `json:"minted_at,omitempty"`
}

var (
	ErrNotFound      = errors.New("node not found")
	ErrInvalidStatus = errors.New("invalid node status transition")
	ErrInvalidInput  = errors.New("hostname, cpu and ram_gb are required")
)

// Store persists join requests as a JSON file with atomic writes.
type Store struct {
	mu            sync.RWMutex
	path          string
	nodes         map[string]*Node
	byFingerprint map[string]string
}

// NewStore loads the store from path, starting empty when the file is absent.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, nodes: make(map[string]*Node), byFingerprint: make(map[string]string)}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read nodes db: %w", err)
	}
	var saved struct {
		Nodes map[string]*Node `json:"nodes"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("parse nodes db: %w", err)
	}
	for id, n := range saved.Nodes {
		s.nodes[id] = n
		s.byFingerprint[fingerprint(n)] = id
	}
	return s, nil
}

// Register creates a pending request, or returns the existing record when
// the same laptop re-registers (idempotent on hostname+fingerprint).
func (s *Store) Register(hostname, osName, arch string, cpu int, ramGB float64, gpu string) (Node, bool, error) {
	if hostname == "" || cpu <= 0 || ramGB <= 0 {
		return Node{}, false, ErrInvalidInput
	}
	fp := fingerprint(&Node{Hostname: hostname, OS: osName, Arch: arch, CPU: cpu, RAMGB: ramGB, GPU: gpu})

	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byFingerprint[fp]; ok {
		return *s.nodes[id], false, nil
	}
	id, err := newID()
	if err != nil {
		return Node{}, false, err
	}
	now := time.Now().UTC()
	n := &Node{
		ID: id, Hostname: hostname, OS: osName, Arch: arch,
		CPU: cpu, RAMGB: ramGB, GPU: gpu,
		Status: StatusPending, NodeName: "ldndrc-" + id,
		Requested: now,
	}
	s.nodes[id] = n
	s.byFingerprint[fp] = id
	if err := s.saveLocked(); err != nil {
		delete(s.nodes, id)
		delete(s.byFingerprint, fp)
		return Node{}, false, err
	}
	return *n, true, nil
}

// Get returns a copy of the node record.
func (s *Store) Get(id string) (Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[id]
	if !ok {
		return Node{}, ErrNotFound
	}
	return *n, nil
}

// List returns all records, oldest first.
func (s *Store) List() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		out = append(out, *n)
	}
	return out
}

// Approve moves pending (or expired) requests to approved, resetting any
// prior mint so a fresh token is issued on the next poll.
func (s *Store) Approve(id string) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[id]
	if !ok {
		return Node{}, ErrNotFound
	}
	if n.Status != StatusPending && n.Status != StatusExpired {
		return Node{}, fmt.Errorf("%w: cannot approve from %s", ErrInvalidStatus, n.Status)
	}
	n.Status = StatusApproved
	n.ApprovedAt = time.Now().UTC()
	n.TokenID = ""
	n.MintedAt = time.Time{}
	n.Reason = ""
	return *n, s.saveLocked()
}

// Deny moves a pending request to denied with an operator reason.
func (s *Store) Deny(id, reason string) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[id]
	if !ok {
		return Node{}, ErrNotFound
	}
	if n.Status != StatusPending {
		return Node{}, fmt.Errorf("%w: cannot deny from %s", ErrInvalidStatus, n.Status)
	}
	n.Status = StatusDenied
	n.Reason = reason
	return *n, s.saveLocked()
}

// MarkMinted records that a token was issued (token itself not stored).
func (s *Store) MarkMinted(id, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[id]
	if !ok {
		return ErrNotFound
	}
	n.TokenID = tokenID
	n.MintedAt = time.Now().UTC()
	return s.saveLocked()
}

// MarkJoined records that the node is Ready and labeled in the cluster.
func (s *Store) MarkJoined(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[id]
	if !ok {
		return ErrNotFound
	}
	if n.Status != StatusApproved {
		return fmt.Errorf("%w: cannot join from %s", ErrInvalidStatus, n.Status)
	}
	n.Status = StatusJoined
	return s.saveLocked()
}

// ExpireIfElapsed moves approved+minted records past the token TTL to
// expired. Returns true when the status changed.
func (s *Store) ExpireIfElapsed(id string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[id]
	if !ok {
		return false, ErrNotFound
	}
	if n.Status == StatusApproved && !n.MintedAt.IsZero() && now.Sub(n.MintedAt) > TokenTTL {
		n.Status = StatusExpired
		return true, s.saveLocked()
	}
	return false, nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(struct {
		Nodes map[string]*Node `json:"nodes"`
	}{s.nodes}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func fingerprint(n *Node) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d|%f|%s",
		n.Hostname, n.OS, n.Arch, n.CPU, n.RAMGB, n.GPU)))
	return hex.EncodeToString(sum[:])
}

func newID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
